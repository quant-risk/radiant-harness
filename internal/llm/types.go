package llm

import "encoding/json"

// Shared LLM types — used by both sampling (Light + Full) and HTTP (Full
// only). These types are intentionally LLM-agnostic so the Light build
// can compile them without the HTTP client code.
//
// Model/Provider live alongside these types because the Backend
// interface (in backend.go) uses Model.ModelID. The HTTPBackend (in
// backend_http.go, Full only) extends Client which carries Model.
//
// Splitting these out of client.go keeps client.go tag-excludable on
// light_only without breaking sampling.go (which depends on Message +
// ChatResponse).

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

// ChatResponseMessage is the assistant message returned in a chat
// completion. v3.7.0 adds ToolCalls for backends that emit native
// function-calling output; the Content field carries any text the
// model emitted alongside the tool calls.
type ChatResponseMessage struct {
	Role    string     `json:"role"`
	Content string     `json:"content"`
	// ToolCalls is populated when the model emits native
	// function-calling output. Each call has a stable ID
	// (Anthropic: "toolu_xxx"; OpenAI: "call_xxx") so the
	// driver can correlate tool_result messages back to
	// the originating assistant turn.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatResponseChoice wraps one assistant message with its finish
// reason. Pulled out of the inline anonymous struct in v3.7.0 so
// callers (and other packages) can name the type when needed.
type ChatResponseChoice struct {
	Message      ChatResponseMessage `json:"message"`
	FinishReason string             `json:"finish_reason"`
}

// ChatResponse is the parsed response from /chat/completions.
//
// v3.7.0: Choices is now a named type carrying ToolCalls to support
// agentic backends. Field access (`resp.Choices[0].Message.Content`,
// `resp.Choices[0].Message.ToolCalls`) keeps the same shape as the
// previous anonymous-struct layout so existing call sites compile
// unchanged.
type ChatResponse struct {
	ID      string              `json:"id"`
	Choices []ChatResponseChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Tool is the wire shape of a tool offered to the model. Mirrors
// the Anthropic / OpenAI function-calling format so the same payload
// works on most hosted backends. InputSchema is JSON-schema-shaped
// (object with properties / required); backends that require a
// different shape (e.g. flat parameters) translate at the boundary.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ToolCall is a single tool-use block emitted by the model. ID lets
// the driver pass a tool_result back correlated to the request;
// Name + Args drive the actual Registry.Call.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolChoice controls which tool the model picks.
//
//   - {Type: "auto"}  ← default; model decides whether to call
//   - {Type: "any"}   ← model MUST call one of the offered tools
//   - {Type: "tool", Name: "x"}  ← model MUST call tool x
//   - nil             ← host backend default (usually "auto")
type ToolChoice struct {
	Type string `json:"type,omitempty"` // "auto" | "any" | "tool"
	Name string `json:"name,omitempty"`
}

// StreamCallback is called for each chunk of a streaming response.
type StreamCallback func(chunk string)

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
