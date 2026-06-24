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
	"strings"
	"time"
)

// Provider represents an LLM API provider. All providers are reached via an
// OpenAI-compatible /chat/completions endpoint, so adding a new one is a
// single entry in the baseURL switch below.
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderGroq       Provider = "groq"
	ProviderMistral    Provider = "mistral"
	ProviderXAI        Provider = "xai"
	ProviderCustom     Provider = "custom"
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
const MaxRetries = 4

// Model represents an LLM model configuration.
type Model struct {
	Provider    Provider `json:"provider" yaml:"provider"`
	Model       string   `json:"model" yaml:"model"`
	APIKey      string   `json:"api_key" yaml:"api_key"`
	BaseURL     string   `json:"base_url" yaml:"base_url"`
	MaxTokens   int      `json:"max_tokens" yaml:"max_tokens"`
	Temperature float64  `json:"temperature" yaml:"temperature"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body sent to /chat/completions.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse is the parsed response from /chat/completions.
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// StreamCallback is called for each chunk of a streaming response.
type StreamCallback func(chunk string)

// Client is a universal LLM client.
type Client struct {
	model  Model
	client *http.Client
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

// Chat sends a chat request with automatic retry on transient failures.
// Retries use exponential backoff with full jitter (AWS-style) so a burst
// of failed requests across multiple orchestrator runs doesn't synchronize.
func (c *Client) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
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
			if err := sleepWithJitter(ctx, attempt); err != nil {
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

// ChatStream streams the response. Retries on the initial connection only;
// once streaming starts we let it ride (a mid-stream reconnect would lose
// already-generated tokens and confuse the caller).
func (c *Client) ChatStream(ctx context.Context, messages []Message, callback StreamCallback) (*ChatResponse, error) {
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

func isRetryable(err error) bool {
	var r *retryableError
	var n *nonRetryableError
	switch {
	case errors.As(err, &r):
		return true
	case errors.As(err, &n):
		return false
	}
	return false
}

// sleepWithJitter implements exponential backoff with full jitter, capped at
// 30s. Returns ctx.Err() if cancellation fires during the sleep.
func sleepWithJitter(ctx context.Context, attempt int) error {
	base := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base)))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(jitter):
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

// ── Model Presets ──
//
// Presets use OpenRouter model IDs so a single API key covers everything.
// MaxTokens is set per-model to the model's documented output limit; using
// a higher cap silently truncates, using a lower one underutilizes.

// PresetModels contains commonly used model configurations. The list is
// intentionally small: a few well-curated options beat a sprawling menu of
// half-broken aliases. Add to this list when a model proves its worth over
// at least a sprint of real SDD workloads.
//
// All presets use OpenRouter by default (one API key covers everything),
// but a preset can be redirected to its native provider by editing the
// `Provider` field — e.g. set `mistral-large-2` to `Provider: ProviderMistral`
// if the operator wants the Mistral-native endpoint with its own API key.
var PresetModels = map[string]Model{
	// Anthropic — strong at SDD: long context, precise instruction following.
	"claude-opus-4.1": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-opus-4.1",
		MaxTokens: 32000,
	},
	"claude-sonnet-4.5": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-sonnet-4.5",
		MaxTokens: 32000,
	},
	"claude-sonnet-4": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-sonnet-4",
		MaxTokens: 16000,
	},
	// OpenAI — fast, good at code generation.
	"gpt-5": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5",
		MaxTokens: 32000,
	},
	"gpt-5-codex": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5-codex",
		MaxTokens: 32000,
	},
	"gpt-4o": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-4o",
		MaxTokens: 16000,
	},
	// Google — strong math/reasoning, competitive price.
	"gemini-2.5-pro": {
		Provider:  ProviderOpenRouter,
		Model:     "google/gemini-2.5-pro",
		MaxTokens: 32000,
	},
	// DeepSeek — best $/quality ratio for spec drafting; weaker at code.
	"deepseek-v4-pro": {
		Provider:  ProviderOpenRouter,
		Model:     "deepseek/deepseek-v4-pro",
		MaxTokens: 16000,
	},
	"deepseek-v4-flash": {
		Provider:  ProviderOpenRouter,
		Model:     "deepseek/deepseek-v4-flash",
		MaxTokens: 16000,
	},
	// Xiaomi — surprisingly strong on PT-BR, useful for Fortvna's docs.
	"mimo-v2.5-pro": {
		Provider:  ProviderOpenRouter,
		Model:     "xiaomi/mimo-v2.5-pro",
		MaxTokens: 16000,
	},
	// Mistral — European-hosted, strong on code, competitive price.
	// Use Mistral-native by default; switch to OpenRouter if you want
	// access through a single API key.
	"mistral-large-2": {
		Provider:  ProviderMistral,
		Model:     "mistral-large-latest",
		MaxTokens: 16000,
	},
	"codestral-22b": {
		Provider:  ProviderMistral,
		Model:     "codestral-latest",
		MaxTokens: 16000,
	},
	// Groq — ultra-low latency. Best for CI / fast feedback loops where
	// you can trade a small quality loss for ~300 tok/s throughput.
	"groq-llama-3.3-70b": {
		Provider:  ProviderGroq,
		Model:     "llama-3.3-70b-versatile",
		MaxTokens: 16000,
	},
	"groq-mixtral-8x7b": {
		Provider:  ProviderGroq,
		Model:     "mixtral-8x7b-32768",
		MaxTokens: 16000,
	},
	// xAI — Grok 2, native API.
	"grok-2": {
		Provider:  ProviderXAI,
		Model:     "grok-2-latest",
		MaxTokens: 16000,
	},
}

// GetPreset returns a preset model configuration, optionally overriding the
// API key with one supplied by the caller (e.g. from --api-key or env).
func GetPreset(name string, apiKey string) (Model, bool) {
	m, ok := PresetModels[name]
	if ok {
		m.APIKey = apiKey
	}
	return m, ok
}

// ListPresets returns all available preset names in sorted order for stable
// output.
func ListPresets() []string {
	names := make([]string, 0, len(PresetModels))
	for name := range PresetModels {
		names = append(names, name)
	}
	// Tiny sort: insertion sort is fine for ~10 entries and avoids pulling
	// in `sort` for a one-line helper.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}
