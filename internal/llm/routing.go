// Package llm — model routing and cost estimation.
//
// AutoRoute() picks a model per RPI phase (Research / Plan / Implement)
// based on a per-family budget strategy: cheapest viable model for
// implementation, mid-tier for planning, top-tier for research. The
// strategy is opinionated and overridable per-model.
//
// CostUSD() estimates USD cost from a token count and a model name.
// Prices are per-million-tokens (input + output blended); the function
// uses a conservative 1:3 input/output ratio since the harness prompts
// are mostly input-heavy (specs, tasks, conventions) with shorter
// generated outputs (code, fixes).
package llm

import (
	"fmt"
	"strings"
)

// Phase identifies which RPI phase a model call belongs to.
type Phase string

const (
	PhaseResearch  Phase = "research"
	PhasePlan      Phase = "plan"
	PhaseImplement Phase = "implement"
)

// ModelTier classifies a model by relative cost/capability.
type ModelTier string

const (
	TierBudget ModelTier = "budget" // cheap, fast, weaker
	TierMid    ModelTier = "mid"    // balanced
	TierTop    ModelTier = "top"    // expensive, strongest
)

// tierByPreset returns the default tier for a known preset name.
func tierByPreset(presetName string) ModelTier {
	switch {
	case strings.HasPrefix(presetName, "claude-opus"):
		return TierTop
	case strings.HasPrefix(presetName, "claude-sonnet"),
		strings.HasPrefix(presetName, "gpt-5"),
		strings.HasPrefix(presetName, "gemini-2.5-pro"):
		return TierMid
	default:
		return TierBudget
	}
}

// AutoRoute returns the preset name to use for the given phase, given
// the operator's "anchor" preset (the one they would have used by
// default). The anchor's family drives the routing — if the anchor is a
// Sonnet, research uses Opus and implementation stays on Sonnet; if
// the anchor is a Haiku-class model, all phases stay on the cheap side.
//
// If the anchor's family isn't recognized, or the family has no model
// at the requested tier (e.g. DeepSeek family has no Top tier), the
// anchor is returned as a fallback. Callers can override per-phase
// with --model per run, which takes precedence over this function.
func AutoRoute(anchor string, phase Phase) string {
	family := presetFamily(anchor)
	if family == "" {
		return anchor
	}

	var tier ModelTier
	switch phase {
	case PhaseResearch:
		tier = TierTop
	case PhasePlan, PhaseImplement:
		tier = TierMid
	default:
		return anchor
	}

	pick := pickFromFamily(family, tier)
	if pick == "" {
		return anchor // no sibling at this tier — stay on anchor
	}
	return pick
}

// presetFamily extracts a model family from a preset name. Used by
// AutoRoute to pick siblings across the same vendor — e.g. from
// "claude-sonnet-4.5" the family is "claude" so research can be routed
// to "claude-opus-4.1" (top tier) while implement stays on Sonnet.
func presetFamily(preset string) string {
	switch {
	case strings.HasPrefix(preset, "claude"):
		return "claude"
	case strings.HasPrefix(preset, "gpt-5"),
		strings.HasPrefix(preset, "gpt-4o"):
		return "openai"
	case strings.HasPrefix(preset, "gemini"):
		return "google"
	case strings.HasPrefix(preset, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(preset, "mistral"),
		strings.HasPrefix(preset, "codestral"):
		return "mistral"
	case strings.HasPrefix(preset, "groq"):
		return "groq"
	case strings.HasPrefix(preset, "grok"):
		return "xai"
	case strings.HasPrefix(preset, "mimo"):
		return "xiaomi"
	}
	return ""
}

// pickFromFamily returns the first preset in `family` matching `tier`.
// Empty string if no match — caller should fall back to the anchor.
func pickFromFamily(family string, tier ModelTier) string {
	for name := range PresetModels {
		if !strings.HasPrefix(name, family) {
			continue
		}
		if tierByPreset(name) == tier {
			return name
		}
	}
	return ""
}

// PricePerMTokensUSD is the blended input+output cost per million tokens
// for known models. Prices were last verified 2026-06-24 — update when
// vendors change their pricing. Outdated prices silently inflate or
// deflate the cost estimate; check vendor pages quarterly.
//
// The values are input prices (output is typically 3x–5x but we average
// to keep the table simple). For exact accounting, callers should
// record the input and output token counts separately and apply the
// vendor's published per-direction pricing.
var PricePerMTokensUSD = map[string]float64{
	// Anthropic
	"claude-opus-4.1":   15.0,
	"claude-sonnet-4.5": 3.0,
	"claude-sonnet-4":   3.0,
	// OpenAI
	"gpt-5":       5.0,
	"gpt-5-codex": 5.0,
	"gpt-4o":      2.5,
	// Google
	"gemini-2.5-pro": 1.25,
	// DeepSeek
	"deepseek-v4-pro":   0.27,
	"deepseek-v4-flash": 0.07,
	// Mistral
	"mistral-large-2": 2.0,
	"codestral-22b":   0.30,
	// Groq (cheap because Groq's infra is cheap)
	"groq-llama-3.3-70b": 0.59,
	"groq-mixtral-8x7b":  0.27,
	// xAI
	"grok-2": 2.0,
	// Xiaomi
	"mimo-v2.5-pro": 0.30,
}

// CostUSD estimates the USD cost of generating `outputTokens` tokens
// after reading `inputTokens` tokens, using the model's price per
// million tokens. Returns 0 when the model is unknown — we don't make
// up a cost.
func CostUSD(model string, inputTokens, outputTokens int) float64 {
	price, ok := PricePerMTokensUSD[model]
	if !ok {
		return 0
	}
	return float64(inputTokens+outputTokens) * price / 1_000_000
}

// FormatCost returns a human-readable USD string (e.g. "$0.42",
// "<$0.01"). Used by `radiant run` to show the cost of each feature.
func FormatCost(usd float64) string {
	if usd < 0.01 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", usd)
}
