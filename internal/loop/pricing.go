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
	"claude-opus-4-8":           {CostPer1KOutput: 0.015},
	"claude-opus-4-7":           {CostPer1KOutput: 0.015},
	"claude-sonnet-4-6":         {CostPer1KOutput: 0.003},
	"claude-haiku-4-5":          {CostPer1KOutput: 0.00025},
	"claude-haiku-4-5-20251001": {CostPer1KOutput: 0.00025},

	// OpenAI
	"gpt-4o":      {CostPer1KOutput: 0.005},
	"gpt-4o-mini": {CostPer1KOutput: 0.00015},
	"o3":          {CostPer1KOutput: 0.060},
	"o4-mini":     {CostPer1KOutput: 0.0044},

	// Google
	"gemini-2.0-flash": {CostPer1KOutput: 0.00035},
	"gemini-2.5-pro":   {CostPer1KOutput: 0.005},

	// DeepSeek
	"deepseek-chat": {CostPer1KOutput: 0.00028},
	"deepseek-r1":   {CostPer1KOutput: 0.00219},
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
