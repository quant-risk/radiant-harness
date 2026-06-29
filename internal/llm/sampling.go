package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// samplingResult carries the outcome of a sampling request.
type samplingResult struct {
	text string
	err  error
}

// ── MCP sampling wire types ──────────────────────────────────────────────────

// samplingRequest is the JSON-RPC request emitted to the host client.
type samplingRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"` // always "sampling/createMessage"
	Params  samplingParams  `json:"params"`
}

type samplingParams struct {
	Messages         []samplingMessage  `json:"messages"`
	ModelPreferences *modelPreferences  `json:"modelPreferences,omitempty"`
	MaxTokens        int                `json:"maxTokens"`
}

type samplingMessage struct {
	Role    string          `json:"role"`
	Content samplingContent `json:"content"`
}

type samplingContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

type modelPreferences struct {
	Hints []modelHintEntry `json:"hints,omitempty"`
}

type modelHintEntry struct {
	Name string `json:"name"`
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
	Content     samplingContent `json:"content"`
	Model       string          `json:"model"`
	StopReason  string          `json:"stopReason"`
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
		ch <- samplingResult{err: fmt.Errorf("sampling error %d: %s", resp.Error.Code, resp.Error.Message)}
		return
	}
	if resp.Result == nil {
		ch <- samplingResult{err: fmt.Errorf("sampling: empty result for id %d", id)}
		return
	}
	ch <- samplingResult{text: resp.Result.Content.Text}
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
// matching the ChatResponse shape the runner expects.
func samplingToChatResponse(text string) *ChatResponse {
	return &ChatResponse{
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: "stop",
			},
		},
	}
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
