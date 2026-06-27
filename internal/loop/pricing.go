package loop

// ModelPricing holds the cost per 1K output tokens for a model.
type ModelPricing struct {
	CostPer1KOutput float64 // USD per 1,000 output tokens
}

// providerPricing is the static pricing table.
// Prices are per 1K output tokens (USD). Update when providers change rates.
// Source: provider pricing pages as of June 2026.
var providerPricing = map[string]ModelPricing{
	// Anthropic
	"claude-opus-4-8":   {CostPer1KOutput: 0.015},
	"claude-sonnet-4-6": {CostPer1KOutput: 0.003},
	"claude-haiku-4-5":  {CostPer1KOutput: 0.00025},

	// OpenAI
	"gpt-5":       {CostPer1KOutput: 0.005},
	"gpt-5-mini":  {CostPer1KOutput: 0.00040},
	"gpt-5-nano":  {CostPer1KOutput: 0.00010},
	"gpt-5-codex": {CostPer1KOutput: 0.005},

	// Google
	"gemini-2.5-pro":   {CostPer1KOutput: 0.005},
	"gemini-2.5-flash": {CostPer1KOutput: 0.00035},

	// DeepSeek
	"deepseek-v4-pro":   {CostPer1KOutput: 0.00028},
	"deepseek-v4-flash": {CostPer1KOutput: 0.00007},
	"deepseek-r1":       {CostPer1KOutput: 0.00219},

	// Xiaomi
	"mimo-v2.5-pro":  {CostPer1KOutput: 0.00030},
	"mimo-v2.5-lite": {CostPer1KOutput: 0.00007},

	// Z.AI / GLM
	"glm-5.2":     {CostPer1KOutput: 0.00050},
	"glm-5.2-air": {CostPer1KOutput: 0.00014},

	// Kimi / Moonshot
	"kimi-k2":       {CostPer1KOutput: 0.00055},
	"kimi-k2-flash": {CostPer1KOutput: 0.00014},

	// MiniMax
	"minimax-m1":      {CostPer1KOutput: 0.00070},
	"minimax-text-01": {CostPer1KOutput: 0.00028},
	"abab-7":          {CostPer1KOutput: 0.00014},

	// Qwen
	"qwen-3-coder-plus":   {CostPer1KOutput: 0.00050},
	"qwen-2.5-coder-plus": {CostPer1KOutput: 0.00018},

	// Mistral
	"mistral-large-2": {CostPer1KOutput: 0.002},
	"codestral-22b":   {CostPer1KOutput: 0.00030},

	// Groq
	"groq-llama-3.3-70b": {CostPer1KOutput: 0.00059},
	"groq-llama-3.3-8b":  {CostPer1KOutput: 0.00005},
}

// PriceFor returns the cost per 1K output tokens for the given model ID.
// Returns (price, true) if the model is known, or (0, false) if not.
func PriceFor(modelID string) (float64, bool) {
	p, ok := providerPricing[modelID]
	return p.CostPer1KOutput, ok
}

// KnownModels returns all model IDs in the pricing table.
func KnownModels() []string {
	ids := make([]string, 0, len(providerPricing))
	for id := range providerPricing {
		ids = append(ids, id)
	}
	return ids
}
