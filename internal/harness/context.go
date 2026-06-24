package harness

import (
	"fmt"
	"math"
	"strings"
)

// ContextWindow manages context window budget for AI agents.
type ContextWindow struct {
	MaxTokens int
	UsedTokens int
	SmartZone float64
	DumbZone  float64
}

// NewContextWindow creates a new context window with sensible defaults.
func NewContextWindow(maxTokens int) *ContextWindow {
	return &ContextWindow{
		MaxTokens: maxTokens,
		SmartZone: 0.40,
		DumbZone:  0.60,
	}
}

// defaultEstimator is the package-level token estimator used by the global
// EstimateTokens helper. The harness calls EstimateTokens on every prompt,
// every output line, and every agent invocation, so the helper is the hot
// path — allocating a fresh TokenEstimator per call would dominate
// allocations for long sessions.
var defaultEstimator = NewTokenEstimator()

// EstimateTokens estimates token count from text using word-aware heuristics.
// Falls back to the simpler char/4 ratio for very short inputs where the
// overhead of word-splitting isn't worth it.
func EstimateTokens(text string) int {
	if len(text) < 16 {
		return int(math.Ceil(float64(len(text)) / 4.0))
	}
	return defaultEstimator.EstimateTokens(text)
}

// AddTokens adds tokens to the used count.
func (cw *ContextWindow) AddTokens(n int) {
	cw.UsedTokens += n
}

// AddText estimates and adds tokens from text.
func (cw *ContextWindow) AddText(text string) {
	cw.AddTokens(EstimateTokens(text))
}

// Usage returns the current usage as a percentage (0.0 to 1.0).
func (cw *ContextWindow) Usage() float64 {
	if cw.MaxTokens == 0 {
		return 0
	}
	return float64(cw.UsedTokens) / float64(cw.MaxTokens)
}

// IsSmartZone returns true if usage is within the smart zone (< 40%).
func (cw *ContextWindow) IsSmartZone() bool {
	return cw.Usage() < cw.SmartZone
}

// IsDumbZone returns true if usage exceeds the dumb zone (> 60%).
func (cw *ContextWindow) IsDumbZone() bool {
	return cw.Usage() > cw.DumbZone
}

// ShouldSplit returns true if adding estimatedTokens would exceed smart zone.
func (cw *ContextWindow) ShouldSplit(estimatedTokens int) bool {
	return float64(cw.UsedTokens+estimatedTokens) > float64(cw.MaxTokens)*cw.SmartZone
}

// Status returns a human-readable status string.
func (cw *ContextWindow) Status() string {
	pct := cw.Usage() * 100
	zone := "smart"
	if cw.IsDumbZone() {
		zone = "DUMB - open new context!"
	} else if !cw.IsSmartZone() {
		zone = "warning"
	}
	return fmt.Sprintf("%.0f%% (%d/%d tokens) [%s]", pct, cw.UsedTokens, cw.MaxTokens, zone)
}

// Reset resets the token counter.
func (cw *ContextWindow) Reset() {
	cw.UsedTokens = 0
}

// RPIBudget tracks token budget across Research, Plan, Implement phases.
type RPIBudget struct {
	Total     *ContextWindow
	Research  *ContextWindow
	Plan      *ContextWindow
	Implement *ContextWindow
}

// NewRPIBudget creates a budget split across RPI phases.
func NewRPIBudget(totalTokens int) *RPIBudget {
	return &RPIBudget{
		Total:     NewContextWindow(totalTokens),
		Research:  NewContextWindow(int(float64(totalTokens) * 0.30)),
		Plan:      NewContextWindow(int(float64(totalTokens) * 0.20)),
		Implement: NewContextWindow(int(float64(totalTokens) * 0.50)),
	}
}

// Status returns a summary of all phases.
func (b *RPIBudget) Status() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Total:     %s\n", b.Total.Status()))
	sb.WriteString(fmt.Sprintf("  Research:  %s\n", b.Research.Status()))
	sb.WriteString(fmt.Sprintf("  Plan:      %s\n", b.Plan.Status()))
	sb.WriteString(fmt.Sprintf("  Implement: %s\n", b.Implement.Status()))
	return sb.String()
}
