package pricing

import (
	"strings"
	"testing"
)

func TestDefault_HasKnownPresets(t *testing.T) {
	c := Default()
	known := []string{
		"claude-opus-4-8",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"gpt-5",
		"gpt-5-mini",
		"gemini-2.5-pro",
		"deepseek-v4-pro",
		"mistral-large-2",
	}
	for _, id := range known {
		if _, ok := c.Get(id); !ok {
			t.Errorf("expected preset %q in default catalog", id)
		}
	}
}

func TestGet_CaseInsensitive(t *testing.T) {
	c := Default()
	r1, ok1 := c.Get("claude-sonnet-4-6")
	r2, ok2 := c.Get("CLAUDE-SONNET-4-6")
	r3, ok3 := c.Get("  Claude-Sonnet-4-6  ")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("expected all case variants to find preset")
	}
	if r1.Model != r2.Model || r2.Model != r3.Model {
		t.Errorf("case-insensitive lookup returned different models: %q vs %q vs %q",
			r1.Model, r2.Model, r3.Model)
	}
}

func TestEstimateCost(t *testing.T) {
	c := Default()
	// claude-sonnet-4-6: $0.003/1K input, $0.015/1K output
	// 1000 in + 1000 out = 0.003 + 0.015 = $0.018
	cost := c.EstimateCost("claude-sonnet-4-6", 1000, 1000)
	if cost < 0.017 || cost > 0.019 {
		t.Errorf("estimate cost for 1k+1k sonnet: got %.4f want ~0.018", cost)
	}
}

func TestEstimateCost_Unknown(t *testing.T) {
	c := Default()
	if got := c.EstimateCost("nonexistent-model", 1000, 1000); got != 0 {
		t.Errorf("unknown model should return 0, got %.4f", got)
	}
}

func TestList_Sorted(t *testing.T) {
	c := Default()
	list := c.List()
	if len(list) == 0 {
		t.Fatal("expected non-empty catalog")
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Preset > list[i].Preset {
			t.Errorf("list not sorted: %q > %q", list[i-1].Preset, list[i].Preset)
			break
		}
	}
}

func TestStale_FreshCatalog(t *testing.T) {
	c := Default()
	if c.Stale(90 * 24 * 3600 * 1e9) { // 90 days
		t.Errorf("fresh catalog (verified 2026-06-29) should not be stale within 90 days")
	}
}

func TestStale_OldThreshold(t *testing.T) {
	c := Default()
	// Threshold = 1ns forces staleness
	if !c.Stale(1) {
		t.Errorf("1ns threshold should mark everything stale")
	}
}

func TestSource_Builtin(t *testing.T) {
	c := Default()
	if c.Source() != SourceBuiltin {
		t.Errorf("expected SourceBuiltin, got %q", c.Source())
	}
}

func TestParseEmbedded_RoundTrip(t *testing.T) {
	// Sanity: every line we wrote should produce a valid rate.
	c := Default()
	lines := strings.Split(embeddedPricingYAML, "\n")
	parsed := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			continue
		}
		preset := strings.TrimSpace(parts[0])
		if _, ok := c.Get(preset); !ok {
			t.Errorf("embedded line for preset %q not parseable", preset)
		}
		parsed++
	}
	if parsed < 20 {
		t.Errorf("expected at least 20 parsed rates, got %d", parsed)
	}
}