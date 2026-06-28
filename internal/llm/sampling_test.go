package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSamplingBackend_Chat_SendsRequest verifies that Chat() writes a valid
// sampling/createMessage JSON-RPC request to the output writer.
func TestSamplingBackend_Chat_SendsRequest(t *testing.T) {
	var buf bytes.Buffer
	sb := NewSamplingBackend(SamplingOptions{
		ModelHint: "test-model",
		MaxTokens: 4096,
		Out:       &buf,
	})

	// We won't provide a response, so use a short timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _ = sb.Chat(ctx, []Message{
		{Role: "user", Content: "hello"},
	})

	// Even though Chat timed out, the request must have been written.
	raw := buf.String()
	if raw == "" {
		t.Fatal("Chat() should have written a sampling request to Out")
	}

	var req samplingRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("written line is not valid JSON: %v\nraw: %s", err, raw)
	}
	if req.Method != "sampling/createMessage" {
		t.Errorf("method = %q, want sampling/createMessage", req.Method)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", req.JSONRPC)
	}
	if req.ID <= 0 {
		t.Errorf("id should be positive, got %d", req.ID)
	}
	if req.Params.MaxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", req.Params.MaxTokens)
	}
	if len(req.Params.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Params.Messages))
	}
	if req.Params.Messages[0].Content.Text != "hello" {
		t.Errorf("message text = %q, want hello", req.Params.Messages[0].Content.Text)
	}
	if req.Params.ModelPreferences == nil || len(req.Params.ModelPreferences.Hints) != 1 {
		t.Error("expected modelPreferences with 1 hint")
	}
}

// TestSamplingBackend_Chat_ReturnsResponse simulates a host client replying
// and verifies Chat() returns the correct text.
func TestSamplingBackend_Chat_ReturnsResponse(t *testing.T) {
	lb := &lockedBuffer{}
	sb := NewSamplingBackend(SamplingOptions{
		Out: lb,
	})

	ctx := context.Background()

	// Start Chat in a goroutine — it will block until we Dispatch.
	type result struct {
		resp *ChatResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := sb.Chat(ctx, []Message{
			{Role: "user", Content: "what is 2+2?"},
		})
		done <- result{resp, err}
	}()

	// Wait for the request to appear in the buffer (short retry loop).
	var reqID int64
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data := lb.Bytes()
		if len(data) > 0 {
			var req samplingRequest
			if json.Unmarshal(data, &req) == nil {
				reqID = req.ID
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reqID == 0 {
		t.Fatal("timed out waiting for sampling request")
	}

	// Simulate the host client response.
	respLine := samplingResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(jsonInt64(reqID)),
		Result: &samplingResultBody{
			Role:    "assistant",
			Content: samplingContent{Type: "text", Text: "4"},
			Model:   "test-model",
		},
	}
	raw, _ := json.Marshal(respLine)
	sb.Dispatch(raw)

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("Chat returned error: %v", res.err)
		}
		if len(res.resp.Choices) != 1 {
			t.Fatalf("expected 1 choice, got %d", len(res.resp.Choices))
		}
		if res.resp.Choices[0].Message.Content != "4" {
			t.Errorf("content = %q, want 4", res.resp.Choices[0].Message.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Chat to return")
	}
}

// TestSamplingBackend_Chat_ContextCancel verifies that a cancelled context
// doesn't block Chat() indefinitely.
func TestSamplingBackend_Chat_ContextCancel(t *testing.T) {
	var buf bytes.Buffer
	sb := NewSamplingBackend(SamplingOptions{Out: &buf})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := sb.Chat(ctx, []Message{{Role: "user", Content: "hi"}})
		done <- err
	}()

	// Give the goroutine time to write the request and start waiting.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error on cancelled context, got nil")
		}
		if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "cancel") {
			t.Errorf("expected context cancellation error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Chat did not return after context cancellation — possible goroutine leak")
	}
}

// TestSamplingBackend_Dispatch_UnknownID verifies that Dispatch on an ID with
// no pending request doesn't panic.
func TestSamplingBackend_Dispatch_UnknownID(t *testing.T) {
	sb := NewSamplingBackend(SamplingOptions{Out: &bytes.Buffer{}})

	respLine := samplingResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`99999`),
		Result: &samplingResultBody{
			Role:    "assistant",
			Content: samplingContent{Type: "text", Text: "orphan"},
		},
	}
	raw, _ := json.Marshal(respLine)

	// Should not panic.
	sb.Dispatch(raw)
}

// TestSamplingBackend_Dispatch_MalformedJSON verifies Dispatch handles bad JSON.
func TestSamplingBackend_Dispatch_MalformedJSON(t *testing.T) {
	sb := NewSamplingBackend(SamplingOptions{Out: &bytes.Buffer{}})
	// Should not panic.
	sb.Dispatch([]byte(`{broken json`))
	sb.Dispatch([]byte(``))
}

// lockedBuffer is a thread-safe bytes.Buffer for concurrent test access.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *lockedBuffer) Bytes() []byte {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	// Return a copy to avoid races on the internal slice.
	out := make([]byte, lb.buf.Len())
	copy(out, lb.buf.Bytes())
	return out
}

// TestSamplingBackend_ConcurrentRequests fires 10 simultaneous Chat calls,
// each getting a response, and verifies all resolve correctly.
func TestSamplingBackend_ConcurrentRequests(t *testing.T) {
	lb := &lockedBuffer{}
	sb := NewSamplingBackend(SamplingOptions{Out: lb})

	ctx := context.Background()
	const n = 10

	type result struct {
		text string
		err  error
	}
	results := make([]chan result, n)
	for i := range results {
		results[i] = make(chan result, 1)
	}

	// Start all requests.
	for i := 0; i < n; i++ {
		go func(idx int) {
			resp, err := sb.Chat(ctx, []Message{
				{Role: "user", Content: "ping"},
			})
			if err != nil {
				results[idx] <- result{err: err}
				return
			}
			text := ""
			if len(resp.Choices) > 0 {
				text = resp.Choices[0].Message.Content
			}
			results[idx] <- result{text: text}
		}(i)
	}

	// Collect all request IDs by polling the buffer until we have n.
	var ids []int64
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(ids) < n {
		time.Sleep(20 * time.Millisecond)
		ids = nil // reset and re-parse (lines may accumulate)
		for _, line := range bytes.Split(lb.Bytes(), []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			var req samplingRequest
			if json.Unmarshal(line, &req) == nil && req.Method == "sampling/createMessage" {
				ids = append(ids, req.ID)
			}
		}
	}

	if len(ids) != n {
		t.Fatalf("expected %d request IDs, got %d", n, len(ids))
	}

	// Respond to each.
	for _, id := range ids {
		respLine := samplingResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(jsonInt64(id)),
			Result: &samplingResultBody{
				Role:    "assistant",
				Content: samplingContent{Type: "text", Text: "pong"},
				Model:   "test-model",
			},
		}
		raw, _ := json.Marshal(respLine)
		sb.Dispatch(raw)
	}

	// Verify all goroutines resolved.
	for i, ch := range results {
		select {
		case res := <-ch:
			if res.err != nil {
				t.Errorf("goroutine %d error: %v", i, res.err)
			}
			if res.text != "pong" {
				t.Errorf("goroutine %d text = %q, want pong", i, res.text)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("goroutine %d timed out", i)
		}
	}
}

// TestSamplingBackend_SystemMessageCollapse verifies that system messages are
// collapsed into the first user message with a [SYSTEM] prefix.
func TestSamplingBackend_SystemMessageCollapse(t *testing.T) {
	var buf bytes.Buffer
	sb := NewSamplingBackend(SamplingOptions{Out: &buf})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _ = sb.Chat(ctx, []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "hi"},
	})

	// Parse the request.
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var req samplingRequest
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		if req.Method != "sampling/createMessage" {
			continue
		}
		msgs := req.Params.Messages
		if len(msgs) != 1 {
			t.Errorf("expected 1 message (system collapsed into user), got %d", len(msgs))
			return
		}
		text := msgs[0].Content.Text
		if !strings.Contains(text, "[SYSTEM]") {
			t.Errorf("expected [SYSTEM] prefix in collapsed message, got: %s", text)
		}
		if !strings.Contains(text, "You are helpful.") {
			t.Errorf("expected system content in collapsed message, got: %s", text)
		}
		if !strings.Contains(text, "hi") {
			t.Errorf("expected original user content, got: %s", text)
		}
		return
	}
}

// TestIsSamplingResponse verifies the routing heuristic.
func TestIsSamplingResponse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"response with result", `{"jsonrpc":"2.0","id":1,"result":{"role":"assistant"}}`, true},
		{"error response", `{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"x"}}`, true},
		{"request with method", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, false},
		{"notification", `{"jsonrpc":"2.0","method":"initialized"}`, false},
		{"garbage", `{not json`, false},
		{"empty", ``, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSamplingResponse([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("IsSamplingResponse(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// jsonInt64 marshals an int64 to its JSON representation as a string.
func jsonInt64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
