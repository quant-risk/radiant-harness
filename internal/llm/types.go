package llm

// Shared LLM types. LLM-agnostic so the same Message / ChatResponse /
// Model types flow through both the sampling backend (host-agent) and
// any future backend implementation, without coupling them to a
// specific HTTP provider.

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
