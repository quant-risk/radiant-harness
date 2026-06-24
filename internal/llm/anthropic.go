// Anthropic native Messages API client.
//
// The OpenAI-compatible adapter in client.go works for OpenRouter-routed
// Anthropic traffic, but going through Anthropic directly is faster,
// cheaper, and unlocks features that the OpenAI-compatible shim
// doesn't expose (extended thinking, prompt caching, computer use).
//
// API reference: https://docs.anthropic.com/en/api/messages
//
// Key shape differences vs OpenAI:
//   - URL:             POST /v1/messages  (not /v1/chat/completions)
//   - Auth header:     x-api-key  (not Authorization: Bearer ...)
//   - Version header:  anthropic-version: 2023-06-01
//   - System prompt:   top-level "system" field, not a message
//   - Messages:        {role, content: [{type: "text", text: "..."}]}
//   - Response:        {content: [{type: "text", text: "..."}], usage: {...}}
//   - Tokens:          input_tokens / output_tokens (vs prompt/completion)
//   - Errors:          {"type": "...error", "error": {"type": "...", "message": "..."}}
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicVersion is the API version we target. Bumped when Anthropic
// ships breaking changes; older clients continue working as Anthropic
// commits to versioned compatibility.
const AnthropicVersion = "2023-06-01"

// anthropicRequest is the JSON body posted to /v1/messages.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// anthropicMessage represents one entry in the `messages` array.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the parsed body of a non-streaming response.
type anthropicResponse struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
	Model   string             `json:"model"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Error        *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// anthropicContent is one element in the response's content array.
// Today we only handle type=text (the default for non-tool-use calls),
// but the response can carry type=tool_use blocks too — those are
// ignored for now and logged as a non-fatal warning.
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// chatAnthropic is the Anthropic-native equivalent of Chat(). Same
// retry/backoff/timeout behavior; different transport shape. Routing
// is decided by Client.Chat() based on the model provider.
//
// We model the system prompt separately from the message stream,
// matching Anthropic's API. The first user message becomes the
// only message; if there are several, we emit them as separate
// "user"/"assistant" turns.
func (c *Client) chatAnthropic(ctx context.Context, messages []Message) (*ChatResponse, error) {
	// Anthropic splits system prompt from messages.
	var systemPrompt string
	var turns []anthropicMessage
	for _, m := range messages {
		switch m.Role {
		case "system":
			systemPrompt = m.Content
		case "user", "assistant":
			turns = append(turns, anthropicMessage{Role: m.Role, Content: m.Content})
		default:
			// Unknown role — treat as user rather than failing. Real
			// providers rarely emit roles outside user/assistant, so
			// this is the safest default.
			turns = append(turns, anthropicMessage{Role: "user", Content: m.Content})
		}
	}

	body, err := json.Marshal(anthropicRequest{
		Model:       c.model.Model,
		MaxTokens:   c.model.MaxTokens,
		System:      systemPrompt,
		Messages:    turns,
		Temperature: c.model.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(0)
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
		resp, err := c.doAnthropic(ctx, body)
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

// doAnthropic performs one HTTP request to the Anthropic /v1/messages
// endpoint. Retry classification mirrors doOnce: 429 + 5xx retryable,
// other 4xx fail-fast.
func (c *Client) doAnthropic(ctx context.Context, body []byte) (*ChatResponse, error) {
	// Honor a custom BaseURL (e.g. localhost proxy for tests, or a
	// private Anthropic-compatible gateway). Default points at the
	// canonical Anthropic endpoint.
	url := c.baseURL() + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("create anthropic request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.model.APIKey)
	httpReq.Header.Set("anthropic-version", AnthropicVersion)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, &retryableError{err: fmt.Errorf("anthropic send: %w", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &retryableError{err: fmt.Errorf("read anthropic response: %w", err)}
	}

	if resp.StatusCode == 429 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &RateLimitError{
			Err:        fmt.Errorf("anthropic rate limited (429): %s", truncateForError(string(respBody), 256)),
			RetryAfter: retryAfter,
		}
	}
	if resp.StatusCode >= 500 {
		return nil, &retryableError{err: fmt.Errorf("anthropic API %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))}
	}
	if resp.StatusCode != 200 {
		return nil, &nonRetryableError{err: fmt.Errorf("anthropic API %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))}
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("unmarshal anthropic response: %w", err)}
	}
	if ar.Error != nil {
		return nil, &nonRetryableError{err: fmt.Errorf("anthropic error: %s", ar.Error.Message)}
	}

	// Flatten the content array into a single string. Anthropic's
	// content array can hold multiple text blocks (rare) or tool_use
	// blocks (we ignore for now — would need a richer return type).
	text := extractAnthropicText(ar.Content)

	return &ChatResponse{
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: text},
			FinishReason: ar.StopReason,
		}},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
		},
		ID: ar.ID,
	}, nil
}

// extractAnthropicText concatenates the text blocks from Anthropic's
// content array. Non-text blocks (tool_use, image) are skipped — they
// would need a richer return type than ChatResponse supports.
func extractAnthropicText(content []anthropicContent) string {
	var sb strings.Builder
	for _, c := range content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// chatStreamAnthropic streams a response from Anthropic. Anthropic SSE
// uses `event:` + `data:` pairs; we follow the same `data:` semantics
// as the OpenAI-compatible client.
func (c *Client) chatStreamAnthropic(ctx context.Context, messages []Message, callback StreamCallback) (*ChatResponse, error) {
	var systemPrompt string
	var turns []anthropicMessage
	for _, m := range messages {
		switch m.Role {
		case "system":
			systemPrompt = m.Content
		case "user", "assistant":
			turns = append(turns, anthropicMessage{Role: m.Role, Content: m.Content})
		}
	}

	body, err := json.Marshal(anthropicRequest{
		Model:       c.model.Model,
		MaxTokens:   c.model.MaxTokens,
		System:      systemPrompt,
		Messages:    turns,
		Temperature: c.model.Temperature,
		Stream:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	url := c.baseURL() + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.model.APIKey)
	httpReq.Header.Set("anthropic-version", AnthropicVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic stream %d: %s", resp.StatusCode, truncateForError(string(respBody), 512))
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		// Anthropic SSE uses both "event:" and "data:" lines. We only
		// care about the data lines — events like "content_block_delta"
		// carry the actual text chunks.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		// Anthropic streams three relevant event types:
		//   message_start, content_block_start, content_block_delta,
		//   content_block_stop, message_delta, message_stop.
		// Only content_block_delta carries text. Parse generically.
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" {
			fullContent.WriteString(ev.Delta.Text)
			if callback != nil {
				callback(ev.Delta.Text)
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
		}{{
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: fullContent.String()},
		}},
	}, nil
}
