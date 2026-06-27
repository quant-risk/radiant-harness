package routing

// FamilyTiers maps family names to their tier models.
// This is the single source of truth for which model is used at each tier.
// Model IDs match the canonical IDs in llm.PresetModels.
//
// When a family has only one viable model for multiple tiers (e.g.
// Xiaomi's top and mid are the same), that's fine — the resolver
// will use the same model for both.
var FamilyTiers = map[string]FamilyTier{
	"claude": {
		Top:    "claude-opus-4-8",
		Mid:    "claude-sonnet-4-6",
		Budget: "claude-haiku-4-5",
	},
	"openai": {
		Top:    "gpt-5",
		Mid:    "gpt-5-mini",
		Budget: "gpt-5-nano",
	},
	"gemini": {
		Top:    "gemini-2.5-pro",
		Mid:    "gemini-2.5-flash",
		Budget: "gemini-2.5-flash",
	},
	"xiaomi": {
		Top:    "mimo-v2.5-pro",
		Mid:    "mimo-v2.5-pro",
		Budget: "mimo-v2.5-lite",
	},
	"deepseek": {
		Top:    "deepseek-v4-pro",
		Mid:    "deepseek-v4-flash",
		Budget: "deepseek-v4-flash",
	},
	"mistral": {
		Top:    "mistral-large-2",
		Mid:    "codestral-22b",
		Budget: "codestral-22b",
	},
	"glm": {
		Top:    "glm-5.2",
		Mid:    "glm-5.2-air",
		Budget: "glm-5.2-air",
	},
	"kimi": {
		Top:    "kimi-k2",
		Mid:    "kimi-k2",
		Budget: "kimi-k2-flash",
	},
	"minimax": {
		Top:    "minimax-m1",
		Mid:    "minimax-text-01",
		Budget: "abab-7",
	},
	"qwen": {
		Top:    "qwen-3-coder-plus",
		Mid:    "qwen-3-coder-plus",
		Budget: "qwen-2.5-coder-plus",
	},
	"groq": {
		Top:    "groq-llama-3.3-70b",
		Mid:    "groq-llama-3.3-70b",
		Budget: "groq-llama-3.3-8b",
	},
}

// phaseTiers maps each phase to the tier it should use.
var phaseTiers = map[Phase]Tier{
	PhaseResearch:  TierTop,
	PhasePlan:      TierTop,
	PhaseImplement: TierMid,
	PhaseCorrect:   TierMid,
	PhaseVerify:    TierTop,
	PhasePersist:   TierBudget,
	PhaseSummarize: TierBudget,
}

// TierForPhase returns the tier a phase should use.
func TierForPhase(phase Phase) Tier {
	if t, ok := phaseTiers[phase]; ok {
		return t
	}
	return TierMid
}

// ModelForTier returns the model preset name for a given family and tier.
// Returns empty string if the family is unknown.
func ModelForTier(family string, tier Tier) string {
	ft, ok := FamilyTiers[family]
	if !ok {
		return ""
	}
	switch tier {
	case TierTop:
		return ft.Top
	case TierMid:
		return ft.Mid
	case TierBudget:
		return ft.Budget
	}
	return ""
}

// FamilyOf extracts the family name from a model preset name.
// E.g. "claude-sonnet-4-6" -> "claude", "gpt-5-mini" -> "openai".
func FamilyOf(preset string) string {
	// Order matters: check longer prefixes first to avoid
	// "gpt-5-codex" matching "gpt-5" before "gpt-5-codex".
	families := []struct{ prefix, family string }{
		{"claude-opus", "claude"},
		{"claude-sonnet", "claude"},
		{"claude-haiku", "claude"},
		{"gpt-5-codex", "openai"},
		{"gpt-5-mini", "openai"},
		{"gpt-5-nano", "openai"},
		{"gpt-5", "openai"},
		{"gpt-4o", "openai"},
		{"gemini-2.5-pro", "gemini"},
		{"gemini-2.5-flash", "gemini"},
		{"gemini", "gemini"},
		{"mimo-v2.5-pro", "xiaomi"},
		{"mimo-v2.5-lite", "xiaomi"},
		{"mimo", "xiaomi"},
		{"deepseek-v4", "deepseek"},
		{"deepseek-r1", "deepseek"},
		{"deepseek", "deepseek"},
		{"mistral-large", "mistral"},
		{"codestral", "mistral"},
		{"glm-5.2", "glm"},
		{"glm", "glm"},
		{"kimi-k2", "kimi"},
		{"kimi", "kimi"},
		{"minimax", "minimax"},
		{"abab", "minimax"},
		{"qwen", "qwen"},
		{"groq", "groq"},
	}
	for _, f := range families {
		if len(preset) >= len(f.prefix) && preset[:len(f.prefix)] == f.prefix {
			return f.family
		}
	}
	return ""
}

// AllPhases returns all phases in canonical order.
func AllPhases() []Phase {
	return []Phase{
		PhaseResearch,
		PhasePlan,
		PhaseImplement,
		PhaseCorrect,
		PhaseVerify,
		PhasePersist,
		PhaseSummarize,
	}
}

// AllFamilies returns all supported family names.
func AllFamilies() []string {
	return []string{
		"claude", "openai", "gemini", "xiaomi", "deepseek",
		"mistral", "glm", "kimi", "minimax", "qwen", "groq",
	}
}
