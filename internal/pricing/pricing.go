// Package pricing is the single source of truth for LLM model pricing.
//
// One YAML file (`data/pricing.yaml`) holds the canonical rates. At build
// time, embedded data is generated into three views used elsewhere in the
// codebase:
//
//   - PresetModels — provider+modelID+maxTokens per preset alias
//   - Models       — flat per-model rates (input/output USD per 1K tokens)
//   - PricePerMTokensUSD — blended rates used by llm/routing.go
//
// All three must stay in lockstep. This package owns that consistency.
//
// To update rates: edit data/pricing.yaml and run `radiant pricing refresh`.
// The CLI also exposes `radiant pricing list` and `radiant pricing stale`
// for visibility.
package pricing

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"
)

//go:embed data/pricing.yaml
var embeddedPricingYAML string

// Source describes the provenance of a price quote.
type Source string

const (
	SourceBuiltin Source = "builtin"   // shipped in the binary
	SourceCustom  Source = "custom"    // user override at ~/.config/radiant/pricing.yaml
	SourceStale   Source = "stale"     // older than configured threshold
)

// ModelRate is the canonical rate card for one model.
type ModelRate struct {
	Model          string  `yaml:"model" json:"model"`                                   // canonical ID used by providers
	Provider       string  `yaml:"provider" json:"provider"`                             // openrouter, openai, anthropic, ...
	Preset         string  `yaml:"preset,omitempty" json:"preset,omitempty"`             // short alias (claude-sonnet-4-6)
	InputPer1K     float64 `yaml:"input_per_1k_usd" json:"input_per_1k_usd"`             // USD per 1K input tokens
	OutputPer1K    float64 `yaml:"output_per_1k_usd" json:"output_per_1k_usd"`           // USD per 1K output tokens
	MaxTokens      int     `yaml:"max_tokens" json:"max_tokens"`                         // model's documented output cap
	VerifiedAt     string  `yaml:"verified_at,omitempty" json:"verified_at,omitempty"` // YYYY-MM-DD
}

// Catalog is the loaded set of rates.
type Catalog struct {
	mu       sync.RWMutex
	rates    map[string]ModelRate   // key = preset or model id (normalized)
	source   Source
	loadedAt time.Time
}

// Default returns the built-in catalog shipped with the binary.
func Default() *Catalog {
	c := &Catalog{
		rates:    parseEmbedded(),
		source:   SourceBuiltin,
		loadedAt: time.Now(),
	}
	return c
}

// Get returns the rate for a given preset or model id.
// Lookup is case-insensitive on the preset name.
func (c *Catalog) Get(id string) (ModelRate, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id = strings.ToLower(strings.TrimSpace(id))
	r, ok := c.rates[id]
	return r, ok
}

// List returns all known rates sorted by preset name.
func (c *Catalog) List() []ModelRate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := make(map[string]bool)
	out := make([]ModelRate, 0, len(c.rates))
	for _, r := range c.rates {
		key := strings.ToLower(r.Preset)
		if seen[key] || r.Preset == "" {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	// Insertion sort by preset (small N, no need for sort.Slice).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Preset > out[j].Preset; j-- {
			tmp := out[j-1]
				out[j-1] = out[j]
				out[j] = tmp
		}
	}
	return out
}

// Source returns where the catalog was loaded from.
func (c *Catalog) Source() Source {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.source
}

// LoadedAt returns when the catalog was loaded.
func (c *Catalog) LoadedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadedAt
}

// Stale returns true if any rate has a verified_at older than threshold.
// Returns false if all rates are within threshold or have no verified_at.
func (c *Catalog) Stale(threshold time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cutoff := time.Now().Add(-threshold)
	for _, r := range c.rates {
		if r.VerifiedAt == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", r.VerifiedAt)
		if err != nil {
			return true
		}
		if t.Before(cutoff) {
			return true
		}
	}
	return false
}

// EstimateCost returns USD cost for a given number of input/output tokens.
// Returns 0 when the model is unknown.
func (c *Catalog) EstimateCost(model string, tokensIn, tokensOut int) float64 {
	r, ok := c.Get(model)
	if !ok {
		return 0
	}
	return float64(tokensIn)/1000*r.InputPer1K + float64(tokensOut)/1000*r.OutputPer1K
}

// parseEmbedded parses the embedded pricing YAML into a map.
// Best-effort: malformed lines are skipped silently rather than failing
// the build — pricing is non-critical for runtime correctness.
func parseEmbedded() map[string]ModelRate {
	out := make(map[string]ModelRate)
	for _, line := range strings.Split(embeddedPricingYAML, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// YAML format we accept:
		//   preset: claude-sonnet-4-6
		//     model: anthropic/claude-sonnet-4-6
		//     provider: openrouter
		//     input_per_1k_usd: 0.003
		//     output_per_1k_usd: 0.015
		//     max_tokens: 32000
		//     verified_at: 2026-06-24
		//
		// Implemented as flat-key entries for simplicity:
		//   claude-sonnet-4-6|anthropic/claude-sonnet-4-6|openrouter|0.003|0.015|32000|2026-06-24
		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			continue
		}
		rate := ModelRate{
			Preset:      strings.TrimSpace(parts[0]),
			Model:       strings.TrimSpace(parts[1]),
			Provider:    strings.TrimSpace(parts[2]),
			VerifiedAt:  strings.TrimSpace(parts[6]),
		}
		if v, err := parseFloat(parts[3]); err == nil {
			rate.InputPer1K = v
		}
		if v, err := parseFloat(parts[4]); err == nil {
			rate.OutputPer1K = v
		}
		if v, err := parseInt(parts[5]); err == nil {
			rate.MaxTokens = v
		}
		// Index by preset AND by model id (so callers can use either).
		out[strings.ToLower(rate.Preset)] = rate
		if rate.Model != "" {
			out[strings.ToLower(rate.Model)] = rate
		}
	}
	return out
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}