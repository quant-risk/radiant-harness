package loop

import "fmt"

// ModelPricing holds the cost per 1K tokens for a model (USD).
// Input tokens typically cost less than output tokens.
// When CostPer1KInput is zero it defaults to CostPer1KOutput (safe over-estimate).
type ModelPricing struct {
	CostPer1KInput  float64 // USD per 1,000 input tokens
	CostPer1KOutput float64 // USD per 1,000 output tokens
}

// providerPricing is the static pricing table (USD, June 2026).
// Source: provider pricing pages as of June 2026.
var providerPricing = map[string]ModelPricing{
	// Anthropic
	"claude-opus-4-8":   {CostPer1KInput: 0.003, CostPer1KOutput: 0.015},
	"claude-sonnet-4-6": {CostPer1KInput: 0.003, CostPer1KOutput: 0.015},
	"claude-haiku-4-5":  {CostPer1KInput: 0.00080, CostPer1KOutput: 0.00025},

	// OpenAI
	"gpt-5":       {CostPer1KInput: 0.002, CostPer1KOutput: 0.005},
	"gpt-5-mini":  {CostPer1KInput: 0.00015, CostPer1KOutput: 0.00040},
	"gpt-5-nano":  {CostPer1KInput: 0.00004, CostPer1KOutput: 0.00010},
	"gpt-5-codex": {CostPer1KInput: 0.002, CostPer1KOutput: 0.005},

	// Google
	"gemini-2.5-pro":   {CostPer1KInput: 0.00125, CostPer1KOutput: 0.005},
	"gemini-2.5-flash": {CostPer1KInput: 0.000075, CostPer1KOutput: 0.00035},

	// DeepSeek
	"deepseek-v4-pro":   {CostPer1KInput: 0.00014, CostPer1KOutput: 0.00028},
	"deepseek-v4-flash": {CostPer1KInput: 0.000035, CostPer1KOutput: 0.00007},
	"deepseek-r1":       {CostPer1KInput: 0.00055, CostPer1KOutput: 0.00219},

	// Xiaomi
	"mimo-v2.5-pro":  {CostPer1KInput: 0.00015, CostPer1KOutput: 0.00030},
	"mimo-v2.5-lite": {CostPer1KInput: 0.000035, CostPer1KOutput: 0.00007},

	// Z.AI / GLM
	"glm-5.2":     {CostPer1KInput: 0.00025, CostPer1KOutput: 0.00050},
	"glm-5.2-air": {CostPer1KInput: 0.00007, CostPer1KOutput: 0.00014},

	// Kimi / Moonshot
	"kimi-k2":       {CostPer1KInput: 0.00028, CostPer1KOutput: 0.00055},
	"kimi-k2-flash": {CostPer1KInput: 0.00007, CostPer1KOutput: 0.00014},

	// MiniMax
	"minimax-m1":      {CostPer1KInput: 0.00035, CostPer1KOutput: 0.00070},
	"minimax-text-01": {CostPer1KInput: 0.00014, CostPer1KOutput: 0.00028},
	"abab-7":          {CostPer1KInput: 0.00007, CostPer1KOutput: 0.00014},

	// Qwen
	"qwen-3-coder-plus":   {CostPer1KInput: 0.00025, CostPer1KOutput: 0.00050},
	"qwen-2.5-coder-plus": {CostPer1KInput: 0.00009, CostPer1KOutput: 0.00018},

	// Mistral
	"mistral-large-2": {CostPer1KInput: 0.001, CostPer1KOutput: 0.002},
	"codestral-22b":   {CostPer1KInput: 0.00015, CostPer1KOutput: 0.00030},

	// Groq
	"groq-llama-3.3-70b": {CostPer1KInput: 0.00059, CostPer1KOutput: 0.00059},
	"groq-llama-3.3-8b":  {CostPer1KInput: 0.00005, CostPer1KOutput: 0.00005},
}

// PriceFor returns the cost per 1K output tokens for the given model ID.
// Returns (price, true) if the model is known, or (0, false) if not.
func PriceFor(modelID string) (float64, bool) {
	p, ok := providerPricing[modelID]
	return p.CostPer1KOutput, ok
}

// EstimateCost returns the estimated USD cost for a given number of input and
// output tokens. When the model is unknown it returns 0 and ok=false.
// Input cost defaults to output cost when CostPer1KInput is not set.
func EstimateCost(modelID string, tokensIn, tokensOut int) (usd float64, ok bool) {
	p, found := providerPricing[modelID]
	if !found {
		return 0, false
	}
	inRate := p.CostPer1KInput
	if inRate == 0 {
		inRate = p.CostPer1KOutput // safe over-estimate
	}
	return float64(tokensIn)/1000*inRate + float64(tokensOut)/1000*p.CostPer1KOutput, true
}

// FormatCost formats a USD cost for human display, e.g. "$0.0042" or "< $0.0001".
func FormatCost(usd float64) string {
	if usd == 0 {
		return "$0.0000"
	}
	if usd < 0.0001 {
		return "< $0.0001"
	}
	return fmt.Sprintf("$%.4f", usd)
}

// KnownModels returns all model IDs in the pricing table.
func KnownModels() []string {
	ids := make([]string, 0, len(providerPricing))
	for id := range providerPricing {
		ids = append(ids, id)
	}
	return ids
}
