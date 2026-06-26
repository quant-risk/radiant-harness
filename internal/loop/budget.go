// Package loop implements the autonomous feedback cycle:
// Discover → Plan → Execute → Verify → Persist → Repeat
//
// Key design principles:
//   - Adversarial verification: the verifier is always a separate agent call
//     with an explicitly skeptical prompt. The executor never grades its own work.
//   - Budget-first: every phase checks remaining budget before starting.
//     When budget is exhausted the loop exits cleanly, not mid-write.
//   - Crash-safe: loop state is persisted atomically after every phase
//     transition. Resume picks up exactly where it left off.
//   - Deterministic exit: the loop always terminates — on success, budget
//     exhaustion, max iterations, or consecutive critical failures.
package loop

import (
	"fmt"
	"sync"
	"time"
)

// BudgetProfile is a named token-budget preset.
type BudgetProfile string

const (
	ProfileLean     BudgetProfile = "lean"     // ~10K tokens total
	ProfileStandard BudgetProfile = "standard" // ~50K tokens total
	ProfileThorough BudgetProfile = "thorough" // ~200K tokens total
)

// profileDefaults maps profile names to token budgets.
var profileDefaults = map[BudgetProfile]int{
	ProfileLean:     10_000,
	ProfileStandard: 50_000,
	ProfileThorough: 200_000,
}

// DefaultWarnRatio is the fraction at which the budget manager emits warnings.
const DefaultWarnRatio = 0.70

// BudgetStatus describes the current budget state.
type BudgetStatus int

const (
	BudgetOK       BudgetStatus = iota // below warn threshold
	BudgetWarning                      // above warn threshold, below limit
	BudgetExceeded                     // at or above limit
)

func (s BudgetStatus) String() string {
	switch s {
	case BudgetOK:
		return "ok"
	case BudgetWarning:
		return "warning"
	case BudgetExceeded:
		return "exceeded"
	default:
		return "unknown"
	}
}

// Budget tracks token and iteration consumption for a loop run.
// All methods are safe for concurrent use.
type Budget struct {
	mu          sync.Mutex
	maxTokens   int
	maxIter     int
	warnRatio   float64
	usedTokens  int
	usedIter    int
	phaseTokens map[Phase]int // tokens consumed per phase
}

// BudgetConfig configures a new Budget.
type BudgetConfig struct {
	// MaxTokens is the hard token limit. Zero means unlimited.
	MaxTokens int
	// MaxIter is the maximum number of cycle iterations. Zero means unlimited.
	MaxIter int
	// WarnRatio is the fraction at which warnings are emitted (default 0.70).
	WarnRatio float64
	// Profile is a named preset — overrides MaxTokens if MaxTokens is 0.
	Profile BudgetProfile
}

// NewBudget creates a Budget from a BudgetConfig.
func NewBudget(cfg BudgetConfig) *Budget {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 && cfg.Profile != "" {
		if preset, ok := profileDefaults[cfg.Profile]; ok {
			maxTokens = preset
		}
	}
	maxIter := cfg.MaxIter
	if maxIter <= 0 {
		maxIter = 20 // sane default
	}
	warnRatio := cfg.WarnRatio
	if warnRatio <= 0 {
		warnRatio = DefaultWarnRatio
	}
	return &Budget{
		maxTokens:   maxTokens,
		maxIter:     maxIter,
		warnRatio:   warnRatio,
		phaseTokens: map[Phase]int{},
	}
}

// Consume records token usage. phase may be PhaseUnknown.
func (b *Budget) Consume(tokens int, phase Phase) {
	if tokens <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.usedTokens += tokens
	b.phaseTokens[phase] += tokens
}

// IncrIter increments the iteration counter.
func (b *Budget) IncrIter() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.usedIter++
}

// Status returns the current budget status.
func (b *Budget) Status() BudgetStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.statusLocked()
}

func (b *Budget) statusLocked() BudgetStatus {
	// Check token budget
	if b.maxTokens > 0 {
		ratio := float64(b.usedTokens) / float64(b.maxTokens)
		if ratio >= 1.0 {
			return BudgetExceeded
		}
		if ratio >= b.warnRatio {
			return BudgetWarning
		}
	}
	// Check iteration budget
	if b.maxIter > 0 && b.usedIter >= b.maxIter {
		return BudgetExceeded
	}
	return BudgetOK
}

// Remaining returns the number of tokens remaining, or -1 if unlimited.
func (b *Budget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxTokens <= 0 {
		return -1
	}
	rem := b.maxTokens - b.usedTokens
	if rem < 0 {
		return 0
	}
	return rem
}

// UsedTokens returns total tokens consumed so far.
func (b *Budget) UsedTokens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usedTokens
}

// UsedIter returns the current iteration count.
func (b *Budget) UsedIter() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usedIter
}

// MaxTokens returns the configured token limit (0 = unlimited).
func (b *Budget) MaxTokens() int {
	return b.maxTokens
}

// MaxIter returns the configured iteration limit.
func (b *Budget) MaxIter() int {
	return b.maxIter
}

// Summary returns a human-readable budget summary.
func (b *Budget) Summary() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.maxTokens > 0 {
		pct := float64(b.usedTokens) / float64(b.maxTokens) * 100
		return fmt.Sprintf("tokens %d/%d (%.0f%%) | iter %d/%d | status: %s",
			b.usedTokens, b.maxTokens, pct,
			b.usedIter, b.maxIter,
			b.statusLocked())
	}
	return fmt.Sprintf("tokens %d/∞ | iter %d/%d | status: %s",
		b.usedTokens, b.usedIter, b.maxIter, b.statusLocked())
}

// PhaseBreakdown returns tokens consumed per phase.
func (b *Budget) PhaseBreakdown() map[Phase]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[Phase]int, len(b.phaseTokens))
	for k, v := range b.phaseTokens {
		out[k] = v
	}
	return out
}

// Snapshot is a point-in-time copy of budget state (for persistence).
type Snapshot struct {
	MaxTokens   int            `json:"max_tokens"`
	MaxIter     int            `json:"max_iter"`
	UsedTokens  int            `json:"used_tokens"`
	UsedIter    int            `json:"used_iter"`
	PhaseTokens map[string]int `json:"phase_tokens"`
	Status      string         `json:"status"`
	CapturedAt  time.Time      `json:"captured_at"`
}

// Snapshot returns a point-in-time copy for JSON persistence.
func (b *Budget) Snapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	pt := make(map[string]int, len(b.phaseTokens))
	for k, v := range b.phaseTokens {
		pt[string(k)] = v
	}
	return Snapshot{
		MaxTokens:   b.maxTokens,
		MaxIter:     b.maxIter,
		UsedTokens:  b.usedTokens,
		UsedIter:    b.usedIter,
		PhaseTokens: pt,
		Status:      b.statusLocked().String(),
		CapturedAt:  time.Now().UTC(),
	}
}
