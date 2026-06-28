package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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

	mu      sync.Mutex // serializes writes to enc/out
	enc     *json.Encoder
	pending sync.Map // map[int64]chan samplingResult
	nextID  atomic.Int64
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

// Chat converts messages to the MCP sampling format, emits a
// sampling/createMessage request to the host client, and blocks until the
// correlated response arrives via Dispatch or the context is cancelled.
func (sb *SamplingBackend) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if sb.enc == nil {
		return nil, fmt.Errorf("sampling backend: no output writer configured")
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

	sb.mu.Lock()
	err := sb.enc.Encode(req)
	sb.mu.Unlock()
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
