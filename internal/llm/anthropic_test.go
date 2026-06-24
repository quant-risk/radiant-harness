package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newAnthropicTestClient spins up an httptest server that mimics the
// Anthropic Messages API shape and returns a Client wired to it via
// the BaseURL override. Without BaseURL set, Chat would route to the
// real api.anthropic.com and 404 against our fake.
func newAnthropicTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		model: Model{
			Provider:  ProviderAnthropic,
			Model:     "claude-sonnet-4-5",
			APIKey:    "test-key",
			BaseURL:   srv.URL,
			MaxTokens: 4096,
		},
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func TestAnthropicSendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	var capturedBody anthropicRequest
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		_, _ = w.Write([]byte(`{
			"id": "msg_test",
			"content": [{"type": "text", "text": "hello"}],
			"usage": {"input_tokens": 12, "output_tokens": 5}
		}`))
	})

	_, err := c.Chat(context.Background(), []Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := capturedHeaders.Get("x-api-key"); got != "test-key" {
		t.Errorf("x-api-key = %q, want test-key", got)
	}
	if got := capturedHeaders.Get("anthropic-version"); got != AnthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", got, AnthropicVersion)
	}
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestAnthropicSystemPromptSeparatedFromMessages(t *testing.T) {
	var capturedBody anthropicRequest
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	})

	_, err := c.Chat(context.Background(), []Message{
		{Role: "system", Content: "system prompt here"},
		{Role: "user", Content: "user message"},
		{Role: "assistant", Content: "previous reply"},
		{Role: "user", Content: "follow up"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if capturedBody.System != "system prompt here" {
		t.Errorf("System = %q, want %q", capturedBody.System, "system prompt here")
	}
	if len(capturedBody.Messages) != 3 {
		t.Fatalf("Messages = %d, want 3 (system excluded)", len(capturedBody.Messages))
	}
	if capturedBody.Messages[0].Role != "user" || capturedBody.Messages[0].Content != "user message" {
		t.Errorf("Messages[0] = %+v, want user msg", capturedBody.Messages[0])
	}
	if capturedBody.Messages[1].Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want assistant", capturedBody.Messages[1].Role)
	}
	if capturedBody.Messages[2].Content != "follow up" {
		t.Errorf("Messages[2] = %+v, want follow up", capturedBody.Messages[2])
	}
}

func TestAnthropicRespondsCorrectly(t *testing.T) {
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": "msg_abc",
			"role": "assistant",
			"content": [{"type": "text", "text": "Bonjour!"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 100, "output_tokens": 3}
		}`))
	})

	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "say hi in French"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Bonjour!" {
		t.Errorf("content = %q, want Bonjour!", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 3 {
		t.Errorf("CompletionTokens = %d, want 3", resp.Usage.CompletionTokens)
	}
	if resp.Choices[0].FinishReason != "end_turn" {
		t.Errorf("FinishReason = %q, want end_turn", resp.Choices[0].FinishReason)
	}
}

func TestAnthropicConcatentatesMultipleTextBlocks(t *testing.T) {
	// Anthropic can return multiple text blocks in content. We join
	// them with no separator — they're emitted contiguously.
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"content": [
				{"type": "text", "text": "Hello, "},
				{"type": "text", "text": "world!"}
			]
		}`))
	})

	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Choices[0].Message.Content; got != "Hello, world!" {
		t.Errorf("content = %q, want %q", got, "Hello, world!")
	}
}

func TestAnthropicHandles429WithRetryAfter(t *testing.T) {
	var attempts int
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit","message":"too many"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	})

	start := time.Now()
	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Errorf("content = %q, want ok", resp.Choices[0].Message.Content)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("retry happened too fast (%v); Retry-After should have been honored", elapsed)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestAnthropicStreamsText(t *testing.T) {
	c := newAnthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		events := []string{
			`event: message_start
data: {"type":"message_start"}

`,
			`event: content_block_start
data: {"type":"content_block_start","index":0}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}

`,
			`event: message_stop
data: {"type":"message_stop"}

`,
		}
		for _, e := range events {
			_, _ = w.Write([]byte(e))
			if flusher != nil {
				flusher.Flush()
			}
		}
	})

	var got []string
	resp, err := c.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(chunk string) {
		got = append(got, chunk)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "") != "hello world" {
		t.Errorf("stream = %q, want %q", got, "hello world")
	}
	if resp.Choices[0].Message.Content != "hello world" {
		t.Errorf("final content = %q, want %q", resp.Choices[0].Message.Content, "hello world")
	}
}

func TestShouldUseAnthropicNative(t *testing.T) {
	cases := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			"pure anthropic",
			Model{Provider: ProviderAnthropic, Model: "claude-sonnet-4-5"},
			true,
		},
		{
			"anthropic with custom BaseURL (localhost mock)",
			Model{Provider: ProviderAnthropic, Model: "x", BaseURL: "http://localhost:8080"},
			true,
		},
		{
			"openrouter-routed anthropic",
			Model{Provider: ProviderOpenRouter, Model: "anthropic/claude-sonnet-4.5"},
			false,
		},
		{
			"openai",
			Model{Provider: ProviderOpenAI, Model: "gpt-5"},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := NewClient(c.model)
			if got := client.shouldUseAnthropicNative(); got != c.expected {
				t.Errorf("shouldUseAnthropicNative() = %v, want %v", got, c.expected)
			}
		})
	}
}
