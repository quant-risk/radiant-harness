//go:build !light_only

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient spins up an httptest server that mimics the OpenAI
// /chat/completions shape and returns a Client wired to it.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := &Client{
		model: Model{
			Provider:    ProviderCustom,
			Model:       "test-model",
			APIKey:      "test-key",
			BaseURL:     srv.URL,
			MaxTokens:   1000,
			Temperature: 0.0,
		},
		client: &http.Client{Timeout: 5 * time.Second},
	}
	return c, srv
}

func TestChatSendsCorrectRequest(t *testing.T) {
	var captured ChatRequest
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing bearer header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("malformed request body: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"id": "test",
			"choices": [{"message": {"role": "assistant", "content": "hi"}}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
		}`))
	})

	resp, err := c.Chat(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Model != "test-model" {
		t.Errorf("expected model test-model, got %q", captured.Model)
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Content != "hello" {
		t.Errorf("messages not propagated: %+v", captured.Messages)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hi" {
		t.Errorf("response content lost: %+v", resp.Choices)
	}
}

func TestChatRetriesOn5xxThenSucceeds(t *testing.T) {
	attempts := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream down"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"recovered"}}]}`))
	})

	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if resp.Choices[0].Message.Content != "recovered" {
		t.Errorf("wrong content: %s", resp.Choices[0].Message.Content)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (2 retries), got %d", attempts)
	}
}

func TestChatDoesNotRetryOn4xx(t *testing.T) {
	attempts := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	})

	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("4xx should not retry, got %d attempts", attempts)
	}
}

func TestChatRespectsContextCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Chat(ctx, []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestListPresetsIsSorted(t *testing.T) {
	got := ListPresets()
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("presets not sorted at %d: %q > %q", i, got[i-1], got[i])
		}
	}
}

func TestListPresetsContainsExpectedModels(t *testing.T) {
	got := ListPresets()
	want := []string{"claude-sonnet-4-6", "gpt-5", "gemini-2.5-pro",
		"mistral-large-2", "codestral-22b", "groq-llama-3.3-70b"}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected preset %q in %v", w, got)
		}
	}
}

func TestGetPresetOverridesAPIKey(t *testing.T) {
	m, ok := GetPreset("claude-sonnet-4-6", "my-key")
	if !ok {
		t.Fatal("expected to find preset")
	}
	if m.APIKey != "my-key" {
		t.Errorf("API key not overridden: %q", m.APIKey)
	}
	if m.MaxTokens < 8192 {
		t.Errorf("preset MaxTokens too small: %d", m.MaxTokens)
	}
}

func TestGetPresetUnknownReturnsFalse(t *testing.T) {
	_, ok := GetPreset("nonexistent-model", "")
	if ok {
		t.Error("expected unknown preset to return false")
	}
}

func TestNewClientAppliesDefaults(t *testing.T) {
	c := NewClient(Model{Provider: ProviderOpenAI, Model: "x", APIKey: "k"})
	if c.model.MaxTokens != MaxTokensDefault {
		t.Errorf("MaxTokens default not applied: %d", c.model.MaxTokens)
	}
	if c.model.Temperature != DefaultTemperature {
		t.Errorf("Temperature default not applied: %f", c.model.Temperature)
	}
}

func TestBaseURLForAllProviders(t *testing.T) {
	// Make sure every documented provider resolves to a non-empty,
	// well-formed URL. Adding a new Provider without a baseURL switch
	// case would silently route to OpenRouter — this test catches it.
	cases := map[Provider]string{
		ProviderOpenRouter: "openrouter.ai",
		ProviderOpenAI:     "api.openai.com",
		ProviderAnthropic:  "api.anthropic.com",
		ProviderGroq:       "api.groq.com",
		ProviderMistral:    "api.mistral.ai",
		ProviderXAI:        "api.x.ai",
	}
	for prov, fragment := range cases {
		c := NewClient(Model{Provider: prov, Model: "x", APIKey: "k"})
		url := c.baseURL()
		if !strings.Contains(url, fragment) {
			t.Errorf("baseURL for %s = %q, expected to contain %q", prov, url, fragment)
		}
	}
}

func TestParseRetryAfterDeltaSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"30", 30 * time.Second},
		{"0", 0},
		{"120", 2 * time.Minute},
		{"", 0},
		{"abc", 0},
		{"-5", 0}, // negative ignored
	}
	for _, c := range cases {
		got := parseRetryAfter(c.in)
		if got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	// 5 seconds in the future
	future := time.Now().UTC().Add(5 * time.Second)
	dateStr := future.Format(http.TimeFormat)
	got := parseRetryAfter(dateStr)
	if got < 4*time.Second || got > 6*time.Second {
		t.Errorf("parseRetryAfter(%q) = %v, want ~5s", dateStr, got)
	}
}

func TestParseRetryAfterPastDate(t *testing.T) {
	// Past date should return 0 (don't wait negative)
	past := time.Now().UTC().Add(-1 * time.Hour)
	dateStr := past.Format(http.TimeFormat)
	got := parseRetryAfter(dateStr)
	if got != 0 {
		t.Errorf("parseRetryAfter(%q past) = %v, want 0", dateStr, got)
	}
}

func TestChatHonorsRetryAfterOn429(t *testing.T) {
	var requestCount int
	var requestTimes []time.Time
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		requestTimes = append(requestTimes, time.Now())
		if requestCount < 2 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	})

	start := time.Now()
	resp, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("expected eventual success: %v", err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Errorf("wrong content: %s", resp.Choices[0].Message.Content)
	}
	// Verify we waited at least ~1 second before the retry.
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("retry happened too fast (%v); Retry-After should have been honored", elapsed)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 attempts, got %d", requestCount)
	}
}

func TestIsRetryableRecognizesRateLimit(t *testing.T) {
	err := &RateLimitError{Err: errors.New("rate limited"), RetryAfter: 5 * time.Second}
	if !isRetryable(err) {
		t.Error("RateLimitError should be retryable")
	}
}

func TestChatStreamAssemblesContent(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`{"choices":[{"delta":{"content":"hello "}}]}`,
			`{"choices":[{"delta":{"content":"world"}}]}`,
			`{"choices":[]}`,
			`[DONE]`,
		}
		for _, ch := range chunks {
			_, _ = w.Write([]byte("data: " + ch + "\n\n"))
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
		t.Errorf("stream assembly wrong: %q", got)
	}
	if resp.Choices[0].Message.Content != "hello world" {
		t.Errorf("final content wrong: %q", resp.Choices[0].Message.Content)
	}
}

func TestSimpleChatEndToEnd(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ack"}}]}`))
	})
	got, err := c.SimpleChat(context.Background(), "system", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ack" {
		t.Errorf("SimpleChat: %q", got)
	}
}
