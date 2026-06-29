package routing

import "testing"

func TestTierForPhase(t *testing.T) {
	tests := []struct {
		phase Phase
		want  Tier
	}{
		{PhaseResearch, TierTop},
		{PhasePlan, TierTop},
		{PhaseImplement, TierMid},
		{PhaseCorrect, TierMid},
		{PhaseVerify, TierTop},
		{PhasePersist, TierBudget},
		{PhaseSummarize, TierBudget},
	}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := TierForPhase(tt.phase); got != tt.want {
				t.Errorf("TierForPhase(%s) = %s, want %s", tt.phase, got, tt.want)
			}
		})
	}
}

func TestModelForTier(t *testing.T) {
	tests := []struct {
		family string
		tier   Tier
		want   string
	}{
		{"claude", TierTop, "claude-opus-4-8"},
		{"claude", TierMid, "claude-sonnet-4-6"},
		{"claude", TierBudget, "claude-haiku-4-5"},
		{"openai", TierTop, "gpt-5"},
		{"openai", TierMid, "gpt-5-mini"},
		{"openai", TierBudget, "gpt-5-nano"},
		{"glm", TierTop, "glm-5.2"},
		{"glm", TierMid, "glm-5.2-air"},
		{"kimi", TierTop, "kimi-k2"},
		{"minimax", TierTop, "minimax-m1"},
		{"qwen", TierTop, "qwen-3-coder-plus"},
		{"unknown", TierTop, ""}, // unknown family
	}
	for _, tt := range tests {
		t.Run(tt.family+"-"+string(tt.tier), func(t *testing.T) {
			if got := ModelForTier(tt.family, tt.tier); got != tt.want {
				t.Errorf("ModelForTier(%s, %s) = %q, want %q",
					tt.family, tt.tier, got, tt.want)
			}
		})
	}
}

func TestFamilyOf(t *testing.T) {
	tests := []struct {
		preset string
		want   string
	}{
		{"claude-opus-4-8", "claude"},
		{"claude-sonnet-4-6", "claude"},
		{"claude-haiku-4-5", "claude"},
		{"gpt-5", "openai"},
		{"gpt-5-mini", "openai"},
		{"gpt-5-nano", "openai"},
		{"gpt-5-codex", "openai"},
		{"gemini-2.5-pro", "gemini"},
		{"mimo-v2.5-pro", "xiaomi"},
		{"deepseek-v4-pro", "deepseek"},
		{"glm-5.2", "glm"},
		{"glm-5.2-air", "glm"},
		{"kimi-k2", "kimi"},
		{"kimi-k2-flash", "kimi"},
		{"minimax-m1", "minimax"},
		{"abab-7", "minimax"},
		{"qwen-3-coder-plus", "qwen"},
		{"groq-llama-3.3-70b", "groq"},
		{"unknown-model", ""},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			if got := FamilyOf(tt.preset); got != tt.want {
				t.Errorf("FamilyOf(%q) = %q, want %q", tt.preset, got, tt.want)
			}
		})
	}
}

func TestEveryFamilyHasAllTiers(t *testing.T) {
	for _, family := range AllFamilies() {
		t.Run(family, func(t *testing.T) {
			ft, ok := FamilyTiers[family]
			if !ok {
				t.Fatalf("family %q not in FamilyTiers", family)
			}
			if ft.Top == "" {
				t.Errorf("family %q has empty Top", family)
			}
			if ft.Mid == "" {
				t.Errorf("family %q has empty Mid", family)
			}
			if ft.Budget == "" {
				t.Errorf("family %q has empty Budget", family)
			}
		})
	}
}

func TestAllPhasesHaveTiers(t *testing.T) {
	for _, phase := range AllPhases() {
		if TierForPhase(phase) == "" {
			t.Errorf("phase %q has no tier mapping", phase)
		}
	}
}

func TestFamilyOfGPT5CodexBeforeGPT5(t *testing.T) {
	// "gpt-5-codex" should match "openai", not be treated as
	// an unknown because "gpt-5" matched first.
	if got := FamilyOf("gpt-5-codex"); got != "openai" {
		t.Errorf("FamilyOf(\"gpt-5-codex\") = %q, want \"openai\"", got)
	}
}
