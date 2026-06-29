//go:build !light_only

// Package llm provides a universal LLM client that works with any model via
// any OpenAI-compatible provider. No agent dependency — the harness calls
// LLM APIs directly when `radiant run --provider=…` is used.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MaxTokensDefault is the default cap on completion tokens when callers
// don't set Model.MaxTokens. 32k matches the spec-driven workflow: a single
// feature spec + tasks + code context routinely exceeds 8k of input plus a
// multi-paragraph implementation. The default was raised from 4k → 8k →
// 32k as SDD specs grew more expressive (design.md + tasks.md + ACs).
const MaxTokensDefault = 32768

// DefaultTemperature biases toward deterministic-but-not-frozen output. SDD
// generators prefer consistency for tests/code, but a tiny dose of variation
// helps avoid identical (and usually broken) suggestions across runs.
const DefaultTemperature = 0.2

// DefaultTimeout caps any single HTTP request. LLM responses can take
// minutes for long generations; below this the context cancellation path
// triggers and the caller gets a clean error.
const DefaultTimeout = 5 * time.Minute

// MaxRetries caps automatic retries on transient failures (5xx, network
// resets, timeouts). Each retry uses exponential backoff with full jitter.
// MaxRetries, Model, Message, ChatRequest, ChatResponse, StreamCallback
// moved to types.go (untagged, shared by sampling + HTTP backends).


// Client is a universal LLM client.
type Client struct {
	model  Model
	client *http.Client
}

// Model returns the model the client was configured with. Used by the
// engine's trace logger to record which model produced each call.
func (c *Client) Model() Model {
	return c.model
}

// NewClient creates a new LLM client with sensible defaults applied.
func NewClient(model Model) *Client {
	if model.MaxTokens == 0 {
		model.MaxTokens = MaxTokensDefault
	}
	if model.Temperature == 0 {
		model.Temperature = DefaultTemperature
	}
	return &Client{
		model:  model,
		client: &http.Client{Timeout: DefaultTimeout},
	}
}

// shouldUseAnthropicNative reports whether Chat should dispatch to the
// native Anthropic Messages API client. Triggered when the provider is
// Anthropic — BaseURL only changes the endpoint, not the API shape. A
// custom BaseURL is still useful (e.g. for a localhost mock that
// mimics Anthropic) but doesn't change which client handles the call.
func (c *Client) shouldUseAnthropicNative() bool {
	return c.model.Provider == ProviderAnthropic
}

// Chat sends a chat request with automatic retry on transient failures.
// Retries use exponential backoff with full jitter (AWS-style) so a burst
// of failed requests across multiple orchestrator runs doesn't synchronize.
//
// 429 responses are handled specially: the Retry-After header (seconds
// or HTTP date) is honored instead of the exponential backoff, so we
// don't burn retries hammering a rate-limited provider.
//
// Routing: when the configured provider is Anthropic AND a custom
// BaseURL isn't set, Chat dispatches to the native Messages API client
// (chatAnthropic in anthropic.go) instead of the OpenAI-compatible
// adapter. This unlocks features the OpenAI shim doesn't expose
// (extended thinking, prompt caching) and avoids the per-token
// surcharge that OpenRouter's Claude routing imposes.
func (c *Client) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if c.shouldUseAnthropicNative() {
		return c.chatAnthropic(ctx, messages)
	}

	req := ChatRequest{
		Model:       c.model.Model,
		Messages:    messages,
		MaxTokens:   c.model.MaxTokens,
		Temperature: c.model.Temperature,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(0)
			// Prefer Retry-After from a 429 over exponential backoff.
			var rl *RateLimitError
			if errors.As(lastErr, &rl) && rl.RetryAfter > 0 {
				wait = rl.RetryAfter
			} else {
				wait = backoffFor(attempt)
			}
			if err := sleepFor(ctx, wait); err != nil {
				return nil, err
			}
		}
		resp, err := c.doOnce(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("after %d retries: %w", MaxRetries, lastErr)
}

// doOnce performs a single HTTP request without retry. Surfacing retryability
// classification lets callers decide whether to surface a 4xx (bad prompt —
// caller error) vs a 5xx (provider problem — retry).
func (c *Client) doOnce(ctx context.Context, body []byte) (*ChatResponse, error) {
	url := c.baseURL() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("create request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.model.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		// Network errors are usually retryable (timeout, reset, DNS blip).
		return nil, &retryableError{err: fmt.Errorf("send request: %w", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &retryableError{err: fmt.Errorf("read response: %w", err)}
	}

	// 429 is the special case: retryable, but the server tells us exactly
	// how long to wait via Retry-After (delta-seconds or HTTP-date).
	if resp.StatusCode == 429 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &RateLimitError{
			Err:        fmt.Errorf("rate limited (429): %s", truncateForError(string(respBody), 256)),
			RetryAfter: retryAfter,
		}
	}
	if resp.StatusCode >= 500 {
		return nil, &retryableError{err: fmt.Errorf("API error %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))}
	}
	if resp.StatusCode != 200 {
		// 4xx is a caller error (bad key, bad model, content policy).
		// Retry won't help; surface verbatim.
		return nil, &nonRetryableError{err: fmt.Errorf("API error %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))}
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("unmarshal response: %w", err)}
	}
	if chatResp.Error != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("LLM error: %s", chatResp.Error.Message)}
	}
	return &chatResp, nil
}

// parseRetryAfter returns the duration to wait before retrying. Supports
// both formats defined by RFC 7231: delta-seconds ("Retry-After: 30")
// and HTTP-date ("Retry-After: Wed, 21 Oct 2015 07:28:00 GMT"). If
// the header is missing or unparseable, returns 0 — the caller falls
// back to exponential backoff.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	// Try delta-seconds first.
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	// Fall back to HTTP-date.
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

// ChatStream streams the response. Retries on the initial connection only;
// once streaming starts we let it ride (a mid-stream reconnect would lose
// already-generated tokens and confuse the caller).
func (c *Client) ChatStream(ctx context.Context, messages []Message, callback StreamCallback) (*ChatResponse, error) {
	if c.shouldUseAnthropicNative() {
		return c.chatStreamAnthropic(ctx, messages, callback)
	}

	req := ChatRequest{
		Model:       c.model.Model,
		Messages:    messages,
		MaxTokens:   c.model.MaxTokens,
		Temperature: c.model.Temperature,
		Stream:      true,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.model.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// SSE lines can be long for large deltas; allow up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)
			if callback != nil {
				callback(content)
			}
		}
	}

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
					Content: fullContent.String(),
				},
			},
		},
	}, nil
}

// SimpleChat is a convenience method for single-prompt interactions (most of
// the orchestrator's calls). It uses the default system prompt style.
func (c *Client) SimpleChat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := c.Chat(ctx, messages)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	return resp.Choices[0].Message.Content, nil
}

// ── Retry classification ──

type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

type nonRetryableError struct{ err error }

func (e *nonRetryableError) Error() string { return e.err.Error() }
func (e *nonRetryableError) Unwrap() error { return e.err }

// RateLimitError is returned for HTTP 429 responses. The RetryAfter
// field carries the server's hint (parsed from the Retry-After header)
// so the retry loop can honor it instead of guessing.
type RateLimitError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return e.Err.Error() }
func (e *RateLimitError) Unwrap() error { return e.Err }

func isRetryable(err error) bool {
	var r *retryableError
	var n *nonRetryableError
	var rl *RateLimitError
	switch {
	case errors.As(err, &rl):
		return true // 429 — always retryable
	case errors.As(err, &r):
		return true
	case errors.As(err, &n):
		return false
	}
	return false
}

// backoffFor returns the exponential-backoff-with-jitter sleep duration
// for the given retry attempt. Capped at 30s. The randomness (full
// jitter per AWS) prevents synchronized retry storms across multiple
// orchestrator runs hitting the same provider.
func backoffFor(attempt int) time.Duration {
	base := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	return time.Duration(rand.Int63n(int64(base)))
}

// sleepFor waits for `d` or until ctx is cancelled, whichever comes first.
func sleepFor(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func truncateForError(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func (c *Client) baseURL() string {
	if c.model.BaseURL != "" {
		return c.model.BaseURL
	}
	switch c.model.Provider {
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1"
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderAnthropic:
		// Anthropic has its own Messages API shape; this OpenAI-compatible
		// endpoint works for OpenRouter-routed Claude traffic. For direct
		// Anthropic, use a custom BaseURL or a future Anthropic-native client.
		return "https://api.anthropic.com/v1"
	case ProviderGroq:
		return "https://api.groq.com/openai/v1"
	case ProviderMistral:
		return "https://api.mistral.ai/v1"
	case ProviderXAI:
		return "https://api.x.ai/v1"
	default:
		return "https://openrouter.ai/api/v1"
	}
}
