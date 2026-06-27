package llm

import (
	"testing"
)

func TestPresetFamily(t *testing.T) {
	cases := []struct {
		preset string
		family string
	}{
		// Vendor-level grouping: any Anthropic preset maps to "claude",
		// any OpenAI to "openai", etc. This is what AutoRoute uses to
		// find a top-tier sibling (e.g. opus) when the anchor is mid
		// tier (e.g. sonnet).
		{"claude-sonnet-4-6", "claude"},
		{"claude-opus-4-8", "claude"},
		{"gpt-5", "openai"},
		{"gpt-5-codex", "openai"},
		{"gemini-2.5-pro", "gemini"},
		{"deepseek-v4-pro", "deepseek"},
		{"groq-llama-3.3-70b", "groq"},
		{"unknown-model", ""},
	}
	for _, c := range cases {
		got := presetFamily(c.preset)
		if got != c.family {
			t.Errorf("presetFamily(%q) = %q, want %q", c.preset, got, c.family)
		}
	}
}

func TestTierByPreset(t *testing.T) {
	cases := []struct {
		preset string
		tier   ModelTier
	}{
		{"claude-opus-4-8", TierTop},
		{"claude-sonnet-4-6", TierMid},
		{"claude-haiku-4-5", TierBudget},
		{"gpt-5", TierTop},
		{"gpt-5-mini", TierMid},
		{"gpt-5-nano", TierBudget},
		{"gemini-2.5-pro", TierTop},
		{"gemini-2.5-flash", TierMid},
		{"deepseek-v4-pro", TierTop},
		{"deepseek-v4-flash", TierMid},
		{"mistral-large-2", TierTop},
		{"codestral-22b", TierMid},
	}
	for _, c := range cases {
		got := tierByPreset(c.preset)
		if got != c.tier {
			t.Errorf("tierByPreset(%q) = %q, want %q", c.preset, got, c.tier)
		}
	}
}

func TestAutoRouteUnknownFamilyIsNoop(t *testing.T) {
	got := AutoRoute("totally-unknown-model", PhaseResearch)
	if got != "totally-unknown-model" {
		t.Errorf("expected no-op for unknown family, got %q", got)
	}
}

func TestAutoRouteAnthropicFamily(t *testing.T) {
	// Anchor is Sonnet 4-6; research should pick a top-tier model
	// (Opus), plan and implement stay mid-tier (Sonnet).
	research := AutoRoute("claude-sonnet-4-6", PhaseResearch)
	plan := AutoRoute("claude-sonnet-4-6", PhasePlan)
	implement := AutoRoute("claude-sonnet-4-6", PhaseImplement)

	if research == "claude-sonnet-4-6" {
		t.Errorf("research should NOT stay on anchor when top-tier available, got %q", research)
	}
	if !hasPrefix(research, "claude-opus") {
		t.Errorf("research should pick claude-opus, got %q", research)
	}
	if !hasPrefix(plan, "claude-sonnet") {
		t.Errorf("plan should pick a sonnet variant, got %q", plan)
	}
	if !hasPrefix(implement, "claude-sonnet") {
		t.Errorf("implement should pick a sonnet variant, got %q", implement)
	}
}

func TestAutoRouteGPT5Family(t *testing.T) {
	research := AutoRoute("gpt-5", PhaseResearch)
	// No TierTop sibling exists in the openai family, so AutoRoute falls
	// back to the anchor.
	if research != "gpt-5" {
		t.Errorf("gpt-5 research should fall back to anchor (no top-tier sibling), got %q", research)
	}
	// But plan/implement should pick mid-tier (the anchor itself if
	// it's already mid).
	plan := AutoRoute("gpt-5", PhasePlan)
	if !hasPrefix(plan, "gpt-") {
		t.Errorf("gpt-5 plan should stay in gpt- family, got %q", plan)
	}
}

func TestAutoRouteBudgetFamilyStaysCheap(t *testing.T) {
	// Groq budget model (8b) routing to research now finds the
	// top-tier sibling (70b) — so it does NOT stay on anchor.
	research := AutoRoute("groq-llama-3.3-8b", PhaseResearch)
	if research != "groq-llama-3.3-70b" {
		t.Errorf("expected upgrade to 70b (TierTop), got %q", research)
	}
}

func TestCostUSDFallsBackToZeroOnUnknown(t *testing.T) {
	if c := CostUSD("never-existed", 1000, 1000); c != 0 {
		t.Errorf("expected 0 for unknown model, got %f", c)
	}
}

func TestCostUSDSimpleCalculation(t *testing.T) {
	// claude-sonnet-4-6 is $3 / 1M tokens. 1M input + 1M output = $6.
	got := CostUSD("claude-sonnet-4-6", 1_000_000, 1_000_000)
	if got < 5.99 || got > 6.01 {
		t.Errorf("expected ~$6 for 2M tokens on sonnet-4-6, got %f", got)
	}
}

func TestCostUSDScales(t *testing.T) {
	// 1K tokens on sonnet-4-6: 1000 * 3 / 1M = $0.003
	got := CostUSD("claude-sonnet-4-6", 500, 500)
	if got > 0.01 || got < 0 {
		t.Errorf("expected ~$0.003 for 1K tokens, got %f", got)
	}
}

func TestPriceTableCoversAllPresets(t *testing.T) {
	// Any preset that callers can pass should have a known price so
	// `radiant run` can show a cost estimate.
	missing := []string{}
	for name := range PresetModels {
		if _, ok := PricePerMTokensUSD[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("presets missing from PricePerMTokensUSD: %v", missing)
	}
}

func TestFormatCost(t *testing.T) {
	cases := []struct {
		usd  float64
		want string
	}{
		{0.0, "<$0.01"},
		{0.005, "<$0.01"},
		{0.42, "$0.42"},
		{1.5, "$1.50"},
		{12.345, "$12.35"},
	}
	for _, c := range cases {
		got := FormatCost(c.usd)
		if got != c.want {
			t.Errorf("FormatCost(%f) = %q, want %q", c.usd, got, c.want)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
