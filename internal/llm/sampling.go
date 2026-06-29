package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SamplingOptions configures a SamplingBackend.
type SamplingOptions struct {
	// ModelHint is an optional hint about which model the client should use.
	// The client may ignore it entirely (MCP spec: modelPreferences are hints).
	ModelHint string

	// MaxTokens caps the requested completion length. Default 8192.
	MaxTokens int

	// Out is where JSON-RPC sampling/createMessage requests are written.
	// Typically os.Stdout of the MCP server process.
	Out io.Writer

	// Timeout is the maximum wait for a host response when the caller did
	// not set a context deadline. Zero means legacy default (5 s, suitable
	// for plain CLI invocations that will deadlock otherwise). Set to a
	// larger value (e.g. 120 s) when the MCP host is known to occasionally
	// take long to cold-start its underlying model — Hermes' mimo / xiaomi /
	// OpenRouter-backed sampling can take 20–40 s on the first call of a
	// session, and the third call of a long run can take a similar amount
	// due to cumulative latency. The MCP server runtime sets this to 120 s.
	Timeout time.Duration
}

// SamplingBackend implements Backend using the MCP sampling/createMessage
// protocol (MCP spec §6.5). When the harness needs a completion it emits a
// JSON-RPC request back to the host client over the Out writer and waits for
// a correlated response. The host client (Claude Code, Hermes, etc.) performs
// inference with its own credentials and returns the result.
//
// The host response is delivered to the server's stdin; the caller's read loop
// (runMCPServe) detects JSON-RPC responses (messages with "result" or "error"
// but no "method") and dispatches them via Dispatch().
type SamplingBackend struct {
	modelHint string
	maxTokens int
	out       io.Writer
	timeout   time.Duration

	muPtr   *sync.Mutex  // external mutex shared with the MCP write loop (set via SetWriteMu)
	mu      sync.Mutex   // fallback mutex used when muPtr is nil
	enc     *json.Encoder
	pending sync.Map // map[int64]chan samplingResult
	nextID  atomic.Int64
}

// SetWriteMu replaces the internal write mutex with an external one shared
// with the caller's encoder. This ensures that sampling/createMessage requests
// and regular JSON-RPC responses never interleave on the same io.Writer.
// Must be called before the first Chat() call.
func (sb *SamplingBackend) SetWriteMu(mu *sync.Mutex) { sb.muPtr = mu }

func (sb *SamplingBackend) writeMu() *sync.Mutex {
	if sb.muPtr != nil {
		return sb.muPtr
	}
	return &sb.mu
}

// NewSamplingBackend creates a SamplingBackend from the given options.
func NewSamplingBackend(opts SamplingOptions) *SamplingBackend {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 8192
	}
	sb := &SamplingBackend{
		modelHint: opts.ModelHint,
		maxTokens: opts.MaxTokens,
		out:       opts.Out,
		timeout:   opts.Timeout,
	}
	if opts.Out != nil {
		sb.enc = json.NewEncoder(opts.Out)
	}
	return sb
}

// samplingResult carries the outcome of a sampling request. Exactly
// one of text / rendered / err is non-zero.
type samplingResult struct {
	text     string      // pure-text path (legacy)
	rendered renderedContent // mixed text + tool_use path (v3.7.0+)
	err      error
}

// ── MCP sampling wire types ──────────────────────────────────────────────────

// samplingRequest is the JSON-RPC request emitted to the host client.
type samplingRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"` // always "sampling/createMessage"
	Params  samplingParams  `json:"params"`
}

// samplingParams is the body of a sampling/createMessage. v3.7.0 adds
// Tools/ToolChoice so the harness can offer native tool-use to the
// host agent's model. Hosts that don't pass `tools` see the prior
// text-only shape — Tool calls on the wire are opt-in by the caller.
type samplingParams struct {
	Messages         []samplingMessage  `json:"messages"`
	ModelPreferences *modelPreferences  `json:"modelPreferences,omitempty"`
	MaxTokens        int                `json:"maxTokens"`
	Tools            []samplingTool     `json:"tools,omitempty"`
	ToolChoice       *samplingChoice    `json:"tool_choice,omitempty"`
}

// samplingTool is the wire format of one offered tool. Mirrors the
// Anthropic / OpenAI function-calling shape so any host MCP client
// that already supports tool-use passes this through transparently.
// Hosts that don't understand `tools` get an empty slice and behave
// identically to the prior text-only contract.
type samplingTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// samplingChoice is the wire format of tool_choice (auto|any|tool).
type samplingChoice struct {
	Type string `json:"type"` // "auto" | "any" | "tool"
	Name string `json:"name,omitempty"`
}

type modelPreferences struct {
	Hints []modelHintEntry `json:"hints,omitempty"`
}

type modelHintEntry struct {
	Name string `json:"name"`
}

// samplingToolUse is one tool-use block in the assistant response.
// We render it as content: [{type: "tool_use", id, name, input}] in
// the wire — the format Anthropic and most MCP-host proxies emit.
type samplingToolUse struct {
	Type  string          `json:"type"`  // "tool_use"
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// samplingToolResult is the assistant-side echo of a tool result the
// driver feeds back as the next user message.
type samplingToolResult struct {
	Type      string          `json:"type"` // "tool_result"
	ToolUseID string          `json:"tool_use_id"`
	Content   samplingContent `json:"content"`
	IsError   bool            `json:"is_error,omitempty"`
}

type samplingMessage struct {
	Role    string          `json:"role"`
	Content samplingContent `json:"content"`
}

type samplingContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// samplingResponse is the JSON-RPC response received from the host client.
// It is parsed from a raw line by Dispatch.
type samplingResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"` // may be number or string
	Result  *samplingResultBody `json:"result,omitempty"`
	Error   *samplingRespError `json:"error,omitempty"`
}

type samplingResultBody struct {
	Role        string          `json:"role"`
	// Content is raw JSON so we can decode the response lazily —
	// hosts that emit a single string field render the same shape
	// as prior versions, while hosts that emit an array of
	// mixed {type, ...} blocks (Anthropic tool_use interleaved with
	// text) still parse. See parseSamplingContent.
	Content     json.RawMessage `json:"content"`
	Model       string          `json:"model"`
	StopReason  string          `json:"stopReason"`
}

// renderedContent is the post-decode shape produced by parseSamplingContent.
// Stored as a slice of {type, payload} pairs so text + tool_use blocks
// can travel together; the ChatWithTools dispatcher flattens text into
// ChatResponse.Choices[i].Message.Content and tool_use into
// ChatResponse.Choices[i].Message.ToolCalls.
type renderedContent struct {
	Text     []string
	ToolUse  []samplingToolUse
}

type samplingRespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── Backend implementation ──────────────────────────────────────────────────

// ModelID returns the model hint, or a placeholder if none was set.
func (sb *SamplingBackend) ModelID() string {
	if sb.modelHint != "" {
		return sb.modelHint
	}
	return "mcp-sampling"
}

// defaultSamplingTimeout caps how long Chat() waits for a host-agent
// response before returning a clear error. The Light build never sets a
// deadline on the context for ordinary subcommand calls (loop, run,
// fleet, eval, …); without this, running `radiant loop start X` from a
// shell without a wired host agent would block forever until the
// process is killed.
const defaultSamplingTimeout = 5 * time.Second

// Chat converts messages to the MCP sampling format, emits a
// sampling/createMessage request to the host client, and blocks until the
// correlated response arrives via Dispatch or the context is cancelled.
//
// If the context has no deadline, Chat enforces the configured timeout so
// standalone CLI invocations fail fast with ErrNoHostAgent instead of
// hanging. The timeout comes from SamplingOptions.Timeout; if that is zero
// (legacy behaviour for non-MCP callers), Chat falls back to
// defaultSamplingTimeout (5 s).
func (sb *SamplingBackend) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if sb.enc == nil {
		return nil, fmt.Errorf("sampling backend: no output writer configured")
	}

	// Apply the default timeout only when the caller didn't set one. The
	// MCP server runtime already uses contexts with its own deadlines
	// (per-request); the CLI path uses context.Background().
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := sb.timeout
		if timeout <= 0 {
			timeout = defaultSamplingTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	id := sb.nextID.Add(1)
	ch := make(chan samplingResult, 1)
	sb.pending.Store(id, ch)
	defer sb.pending.Delete(id)

	req := samplingRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "sampling/createMessage",
		Params: samplingParams{
			Messages:  toSamplingMessages(messages),
			MaxTokens: sb.maxTokens,
		},
	}
	if sb.modelHint != "" {
		req.Params.ModelPreferences = &modelPreferences{
			Hints: []modelHintEntry{{Name: sb.modelHint}},
		}
	}

	mu := sb.writeMu()
	mu.Lock()
	err := sb.enc.Encode(req)
	mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("sampling: write request: %w", err)
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return samplingToChatResponse(res.text), nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			timeout := sb.timeout
			if timeout <= 0 {
				timeout = defaultSamplingTimeout
			}
			return nil, fmt.Errorf(
				"%w: no host agent responded within %s — wire one via `radiant setup-mcp` "+
					"from inside Claude Code / Cursor / Hermes, then retry",
				ErrNoHostAgent, timeout)
		}
		return nil, fmt.Errorf("sampling: context cancelled: %w", ctx.Err())
	}
}

// ChatStream is not natively supported by MCP sampling. We delegate to Chat
// and invoke the callback once with the full text (MCP sampling is atomic).
func (sb *SamplingBackend) ChatStream(ctx context.Context, messages []Message, cb StreamCallback) (*ChatResponse, error) {
	resp, err := sb.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	if cb != nil && len(resp.Choices) > 0 {
		cb(resp.Choices[0].Message.Content)
	}
	return resp, nil
}

// Dispatch routes a raw JSON-RPC line (received on the server's stdin) to the
// correct pending Chat call. It is called by the MCP read loop when a line is
// identified as a response (has "result" or "error", lacks "method").
// Unknown or malformed IDs are silently dropped — they may belong to a
// different conversation or a timed-out request already cleaned up.
func (sb *SamplingBackend) Dispatch(raw []byte) {
	var resp samplingResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return // malformed — nothing we can do
	}
	id, ok := parseResponseID(resp.ID)
	if !ok {
		return
	}
	val, ok := sb.pending.LoadAndDelete(id)
	if !ok {
		return // no pending request for this ID (cancelled / stale)
	}
	ch := val.(chan samplingResult)

	if resp.Error != nil {
		// JSON-RPC -32601 = "Method not found". When the host doesn't
		// implement sampling/createMessage at all, surface a sentinel
		// so callers can fall back gracefully.
		if resp.Error.Code == -32601 {
			ch <- samplingResult{err: fmt.Errorf("%w (method=%s)", ErrSamplingUnsupported, resp.Error.Message)}
			return
		}
		ch <- samplingResult{err: fmt.Errorf("sampling error %d: %s", resp.Error.Code, resp.Error.Message)}
		return
	}
	if resp.Result == nil {
		ch <- samplingResult{err: fmt.Errorf("sampling: empty result for id %d", id)}
		return
	}
	// Lazy-decode content (text | text-array | text+tool_use mixed).
	// We always populate `text` with the concatenated text blocks so
	// the legacy Chat() path keeps working unchanged; the `rendered`
	// field carries the parsed tool_use blocks for ChatWithTools().
	rendered := parseSamplingContent(resp.Result.Content)
	ch <- samplingResult{
		text:     strings.Join(rendered.Text, ""),
		rendered: rendered,
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// toSamplingMessages converts internal llm.Message slice to MCP sampling
// messages. MCP sampling has no "system" role — system messages are collapsed
// into the first user message with a [SYSTEM] prefix.
func toSamplingMessages(msgs []Message) []samplingMessage {
	var out []samplingMessage
	var systemParts []string

	for _, m := range msgs {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
			continue
		}
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user" // unknown roles default to user
		}
		out = append(out, samplingMessage{
			Role:    role,
			Content: samplingContent{Type: "text", Text: m.Content},
		})
	}

	if len(systemParts) > 0 {
		prefix := "[SYSTEM]\n" + joinStrings(systemParts, "\n") + "\n[/SYSTEM]\n\n"
		if len(out) > 0 && out[0].Role == "user" {
			out[0].Content.Text = prefix + out[0].Content.Text
		} else {
			out = append([]samplingMessage{{
				Role:    "user",
				Content: samplingContent{Type: "text", Text: prefix},
			}}, out...)
		}
	}

	return out
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// parseResponseID accepts numeric or string JSON-RPC IDs and returns an int64.
// Since our requests always use int64 IDs, string IDs from a misbehaving client
// are ignored (they can't match a pending request).
func parseResponseID(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	// Try numeric first.
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	return 0, false
}

// samplingToChatResponse builds a ChatResponse from a sampling result text,
// matching the ChatResponse shape the runner expects. Legacy
// text-only path used by Chat() — ChatWithTools uses
// samplingRenderedToChatResponse to also capture tool_use blocks.
func samplingToChatResponse(text string) *ChatResponse {
	return &ChatResponse{
		Choices: []ChatResponseChoice{
			{
				Message: ChatResponseMessage{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: "stop",
			},
		},
	}
}

// ErrSamplingUnsupported is returned when the host agent's MCP server
// replies with the JSON-RPC "method not found" error (code -32601) for
// sampling/createMessage — i.e. the host does NOT implement the sampling
// method. Possession flows should detect this and fall back to stub mode
// rather than failing outright.
var ErrSamplingUnsupported = errors.New("sampling unsupported on host (json-rpc -32601)")

// IsSamplingUnsupported reports whether an error chain originated from
// ErrSamplingUnsupported. Use this rather than direct equality so callers
// stay correct when wrap-format strings change.
func IsSamplingUnsupported(err error) bool {
	return errors.Is(err, ErrSamplingUnsupported)
}

// IsSamplingResponse reports whether a raw JSON-RPC line is a response
// (has "result" or "error") rather than a request/notification (has "method").
// Used by the MCP read loop to route lines correctly in sampling mode.
func IsSamplingResponse(raw []byte) bool {
	var probe struct {
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Method == "" && (len(probe.Result) > 0 || len(probe.Error) > 0)
}

// ── Tool-calling surface (v3.7.0+) ────────────────────────────────────────────

// ChatWithTools sends messages + an offered tool set to the host's
// sampling model and returns a ChatResponse whose Message.ToolCalls
// carries any native tool_use blocks the model emitted. The text
// portion of the response (if any) lands in Message.Content as before.
//
// Hosts that don't implement tool calling simply return a normal
// text-only ChatResponse — ChatWithTools degrades silently because
// the driver treats an empty ToolCalls slice as "model didn't want
// to call any tool this round".
//
// This method is the ToolCapable surface that internal/possess/driver
// looks for via type-assertion. Not on the Backend interface so old
// callers that build a Backend from a text-only path keep compiling.
func (sb *SamplingBackend) ChatWithTools(ctx context.Context, messages []Message, tools []Tool, choice *ToolChoice) (*ChatResponse, error) {
	if sb.enc == nil {
		return nil, fmt.Errorf("sampling backend: no output writer configured")
	}

	// Apply the default timeout only when the caller didn't set one.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := sb.timeout
		if timeout <= 0 {
			timeout = defaultSamplingTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	id := sb.nextID.Add(1)
	ch := make(chan samplingResult, 1)
	sb.pending.Store(id, ch)
	defer sb.pending.Delete(id)

	// Translate llm.Tool → samplingTool (different field names: the
	// public llm.Tool uses snake_case "input_schema"; the wire shape
	// uses Anthropic-style snake-case too so most hosts accept it
	// transparently).
	wireTools := make([]samplingTool, 0, len(tools))
	for _, t := range tools {
		wireTools = append(wireTools, samplingTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	var wireChoice *samplingChoice
	if choice != nil {
		wireChoice = &samplingChoice{Type: choice.Type, Name: choice.Name}
	}

	req := samplingRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "sampling/createMessage",
		Params: samplingParams{
			Messages:  toSamplingMessages(messages),
			MaxTokens: sb.maxTokens,
			Tools:     wireTools,
			ToolChoice: wireChoice,
		},
	}
	if sb.modelHint != "" {
		req.Params.ModelPreferences = &modelPreferences{
			Hints: []modelHintEntry{{Name: sb.modelHint}},
		}
	}

	mu := sb.writeMu()
	mu.Lock()
	err := sb.enc.Encode(req)
	mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("sampling: write request: %w", err)
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return samplingRenderedToChatResponse(res.rendered), nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			timeout := sb.timeout
			if timeout <= 0 {
				timeout = defaultSamplingTimeout
			}
			return nil, fmt.Errorf(
				"%w: no host agent responded within %s — wire one via `radiant setup-mcp` "+
					"from inside Claude Code / Cursor / Hermes, then retry",
				ErrNoHostAgent, timeout)
		}
		return nil, fmt.Errorf("sampling: context cancelled: %w", ctx.Err())
	}
}

// parseSamplingContent lazily decodes the `content` field of a
// sampling/createMessage response. Three shapes are observed in the
// wild:
//
//   1. Plain text → just a JSON string.
//   2. Array of plain-text blocks (Anthropic shape with no tools).
//   3. Array of mixed text + tool_use blocks.
//
// Anything we can't parse falls back to "treat the raw bytes as a
// text block" so the driver still gets a useful answer (the tool_use
// won't be detected but the model can keep going in prose).
func parseSamplingContent(raw json.RawMessage) renderedContent {
	out := renderedContent{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return out
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			out.Text = append(out.Text, s)
		}
		return out
	}
	if trimmed[0] != '[' {
		// Unknown shape — fall through with empty out; the legacy
		// text-channel will be empty and the driver should retry /
		// fall back to self-driven.
		return out
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		// Treat as plain text in case the host returned a non-array blob.
		out.Text = append(out.Text, string(raw))
		return out
	}
	for _, b := range blocks {
		probe := struct {
			Type string          `json:"type"`
			Text string          `json:"text"`
			ID   string          `json:"id"`
			Name string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}{}
		if err := json.Unmarshal(b, &probe); err != nil {
			continue
		}
		switch probe.Type {
		case "text":
			if probe.Text != "" {
				out.Text = append(out.Text, probe.Text)
			}
		case "tool_use":
			if probe.Name == "" {
				continue
			}
			out.ToolUse = append(out.ToolUse, samplingToolUse{
				Type:  "tool_use",
				ID:    probe.ID,
				Name:  probe.Name,
				Input: probe.Input,
			})
		}
	}
	return out
}

// samplingRenderedToChatResponse produces a ChatResponse from a parsed
// rendered content. Text blocks are concatenated into
// Message.Content; tool_use blocks populate Message.ToolCalls with
// Anthropic-style IDs so the driver can correlate tool_result echoes
// back to the originating assistant message.
func samplingRenderedToChatResponse(r renderedContent) *ChatResponse {
	resp := &ChatResponse{}
	resp.Choices = append(resp.Choices, ChatResponseChoice{})
	choice := &resp.Choices[0]
	choice.Message.Role = "assistant"
	choice.Message.Content = strings.Join(r.Text, "")
	for _, tu := range r.ToolUse {
		choice.Message.ToolCalls = append(choice.Message.ToolCalls, ToolCall{
			ID:    tu.ID,
			Name:  tu.Name,
			Input: tu.Input,
		})
	}
	choice.FinishReason = "tool_use"
	if len(r.ToolUse) == 0 {
		choice.FinishReason = "stop"
	}
	return resp
}

// Compile-time check that SamplingBackend satisfies ToolCapable.
var _ ToolCapable = (*SamplingBackend)(nil)
