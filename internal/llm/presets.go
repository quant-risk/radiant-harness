package llm

// ── Model Presets ──
//
// Presets use OpenRouter model IDs so a single API key covers everything.
// MaxTokens is set per-model to the model's documented output limit; using
// a higher cap silently truncates, using a lower one underutilizes.
//
// PresetModels contains commonly used model configurations. The list is
// intentionally small: a few well-curated options beat a sprawling menu of
// half-broken aliases. Add to this list when a model proves its worth over
// at least a sprint of real SDD workloads.
//
// All presets use OpenRouter by default (one API key covers everything),
// but a preset can be redirected to its native provider by editing the
// `Provider` field — e.g. set `mistral-large-2` to `Provider: ProviderMistral`
// if the operator wants the Mistral-native endpoint with its own API key.
//
// PresetModels is a pure data table; always available for listing and
// validation regardless of which backend is wired up.

var PresetModels = map[string]Model{
	// ── Anthropic (3 tiers) ──
	"claude-opus-4-8": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-opus-4-8",
		MaxTokens: 32000,
	},
	"claude-sonnet-4-6": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-sonnet-4-6",
		MaxTokens: 32000,
	},
	"claude-haiku-4-5": {
		Provider:  ProviderOpenRouter,
		Model:     "anthropic/claude-haiku-4-5",
		MaxTokens: 16000,
	},

	// ── OpenAI (3 tiers + codex) ──
	"gpt-5": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5",
		MaxTokens: 32000,
	},
	"gpt-5-mini": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5-mini",
		MaxTokens: 32000,
	},
	"gpt-5-nano": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5-nano",
		MaxTokens: 16000,
	},
	"gpt-5-codex": {
		Provider:  ProviderOpenRouter,
		Model:     "openai/gpt-5-codex",
		MaxTokens: 32000,
	},

	// ── Google Gemini (2 tiers) ──
	"gemini-2.5-pro": {
		Provider:  ProviderOpenRouter,
		Model:     "google/gemini-2.5-pro",
		MaxTokens: 32000,
	},
	"gemini-2.5-flash": {
		Provider:  ProviderOpenRouter,
		Model:     "google/gemini-2.5-flash",
		MaxTokens: 32000,
	},

	// ── Xiaomi (2 tiers) ──
	"mimo-v2.5-pro": {
		Provider:  ProviderOpenRouter,
		Model:     "xiaomi/mimo-v2.5-pro",
		MaxTokens: 32000,
	},
	"mimo-v2.5-lite": {
		Provider:  ProviderOpenRouter,
		Model:     "xiaomi/mimo-v2.5-lite",
		MaxTokens: 16000,
	},

	// ── DeepSeek (3 models) ──
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
	"deepseek-r1": {
		Provider:  ProviderOpenRouter,
		Model:     "deepseek/deepseek-r1",
		MaxTokens: 16000,
	},

	// ── Z.AI / GLM (2 tiers) ──
	"glm-5.2": {
		Provider:  ProviderOpenRouter,
		Model:     "zhipuai/glm-5.2",
		MaxTokens: 32000,
	},
	"glm-5.2-air": {
		Provider:  ProviderOpenRouter,
		Model:     "zhipuai/glm-5.2-air",
		MaxTokens: 16000,
	},

	// ── Kimi / Moonshot (2 tiers) ──
	"kimi-k2": {
		Provider:  ProviderOpenRouter,
		Model:     "moonshot/kimi-k2",
		MaxTokens: 32000,
	},
	"kimi-k2-flash": {
		Provider:  ProviderOpenRouter,
		Model:     "moonshot/kimi-k2-flash",
		MaxTokens: 16000,
	},

	// ── MiniMax (3 models) ──
	"minimax-m1": {
		Provider:  ProviderOpenRouter,
		Model:     "minimax/minimax-m1",
		MaxTokens: 32000,
	},
	"minimax-text-01": {
		Provider:  ProviderOpenRouter,
		Model:     "minimax/minimax-text-01",
		MaxTokens: 16000,
	},
	"abab-7": {
		Provider:  ProviderOpenRouter,
		Model:     "minimax/abab-7",
		MaxTokens: 16000,
	},

	// ── Qwen / Alibaba (2 tiers) ──
	"qwen-3-coder-plus": {
		Provider:  ProviderOpenRouter,
		Model:     "qwen/qwen-3-coder-plus",
		MaxTokens: 32000,
	},
	"qwen-2.5-coder-plus": {
		Provider:  ProviderOpenRouter,
		Model:     "qwen/qwen-2.5-coder-plus",
		MaxTokens: 16000,
	},

	// ── Mistral (2 tiers, native provider) ──
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

	// ── Groq (2 tiers, ultra-low latency) ──
	"groq-llama-3.3-70b": {
		Provider:  ProviderGroq,
		Model:     "llama-3.3-70b-versatile",
		MaxTokens: 16000,
	},
	"groq-llama-3.3-8b": {
		Provider:  ProviderGroq,
		Model:     "llama-3.3-8b-versatile",
		MaxTokens: 16000,
	},
}


// GetPreset returns a preset model configuration, optionally overriding
// the API key with one supplied by the caller (e.g. from --api-key or env).
func GetPreset(name string, apiKey string) (Model, bool) {
	m, ok := PresetModels[name]
	if ok {
		m.APIKey = apiKey
	}
	return m, ok
}

// ListPresets returns all available preset names in sorted order for
// stable output.
func ListPresets() []string {
	names := make([]string, 0, len(PresetModels))
	for name := range PresetModels {
		names = append(names, name)
	}
	// Tiny sort: insertion sort is fine for ~25 entries and avoids
	// pulling in `sort` for a one-line helper.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}
