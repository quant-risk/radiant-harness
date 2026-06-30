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

// Budget tracks token, iteration, time, and cost consumption for a loop run.
// All methods are safe for concurrent use.
type Budget struct {
	mu          sync.Mutex
	maxTokens   int
	maxIter     int
	warnRatio   float64
	maxDuration time.Duration
	maxCostUSD  float64
	costPer1K   float64
	usedTokens  int
	usedIter    int
	startedAt   time.Time
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
	// MaxDuration is the wall-clock time limit per run. Zero means unlimited.
	MaxDuration time.Duration
	// MaxCostUSD is the dollar ceiling for the run. Zero means unlimited.
	MaxCostUSD float64
	// CostPer1K is the provider's output-token price per 1K tokens (USD).
	// Used to compute EstimatedCostUSD. Zero disables cost tracking.
	CostPer1K float64
}

// ProfileDefaults captures the full per-profile budget matrix
// (token, iter, wall-clock, dollar). The implicit defaults in
// NewBudget only wire MaxTokens per profile; this struct is the
// single source of truth that the CLI consults when the user
// passes `--profile X` without any explicit budget cap.
//
// Sizes are conservative for late-2026 / 2027 model context
// windows. Bump if your host gets a larger default context.
type ProfileDefaults struct {
	MaxTokens   int
	MaxIter     int
	MaxDuration time.Duration
	MaxCostUSD  float64
}

// profileDefaultsTable is the per-profile budget matrix. The key
// is the BudgetProfile string; an unknown profile falls back to
// ProfileStandard (the framework's default).
var profileDefaultsTable = map[BudgetProfile]ProfileDefaults{
	ProfileLean: {
		MaxTokens: 10_000, MaxIter: 5,
		MaxDuration: 1 * time.Minute, MaxCostUSD: 0.50,
	},
	ProfileStandard: {
		MaxTokens: 50_000, MaxIter: 20,
		MaxDuration: 10 * time.Minute, MaxCostUSD: 2.00,
	},
	ProfileThorough: {
		MaxTokens: 200_000, MaxIter: 50,
		MaxDuration: 30 * time.Minute, MaxCostUSD: 8.00,
	},
}

// DefaultsForProfile returns the per-profile budget matrix for
// the given profile. Unknown profiles resolve to Standard so a
// typo doesn't silently enable unbounded runs.
func DefaultsForProfile(p BudgetProfile) ProfileDefaults {
	if def, ok := profileDefaultsTable[p]; ok {
		return def
	}
	return profileDefaultsTable[ProfileStandard]
}

// ProfileBudgets reports the full table for `radiant budget list`
// and CHANGELOG / docs surfaces. Order is stable so consumers can
// range over the slice without worrying about map iteration.
func ProfileBudgets() []ProfileBudget {
	order := []BudgetProfile{ProfileLean, ProfileStandard, ProfileThorough}
	out := make([]ProfileBudget, 0, len(order))
	for _, p := range order {
		def := profileDefaultsTable[p]
		out = append(out, ProfileBudget{
			Profile:     p,
			MaxTokens:   def.MaxTokens,
			MaxIter:     def.MaxIter,
			MaxDuration: def.MaxDuration,
			MaxCostUSD:  def.MaxCostUSD,
		})
	}
	return out
}

// ProfileBudget carries a row of the per-profile budget table for
// `radiant budget list`-style surfaces.
type ProfileBudget struct {
	Profile     BudgetProfile `json:"profile"`
	MaxTokens   int           `json:"max_tokens"`
	MaxIter     int           `json:"max_iter"`
	MaxDuration time.Duration `json:"-"`
	MaxCostUSD  float64       `json:"max_cost_usd"`
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
		maxIter = 20 // sane default; the profile matrix below overrides this per-profile
	}
	warnRatio := cfg.WarnRatio
	if warnRatio <= 0 {
		warnRatio = DefaultWarnRatio
	}

	// v3.7.x: per-profile defaults. When the user passes --profile
	// (or it's resolved from config) and any of the budget caps
	// are zero, fill them from the profile matrix. Existing knobs
	// (--budget, --max-cost, --max-time, --max-iter) still win
	// when set explicitly — only the zero/limbo fields are filled.
	if cfg.Profile != "" {
		def := DefaultsForProfile(cfg.Profile)
		if maxTokens <= 0 {
			maxTokens = def.MaxTokens
		}
		if maxIter <= 0 {
			maxIter = def.MaxIter
		}
		if cfg.MaxDuration <= 0 {
			cfg.MaxDuration = def.MaxDuration
		}
		if cfg.MaxCostUSD <= 0 {
			cfg.MaxCostUSD = def.MaxCostUSD
		}
	}

	return &Budget{
		maxTokens:   maxTokens,
		maxIter:     maxIter,
		warnRatio:   warnRatio,
		maxDuration: cfg.MaxDuration,
		maxCostUSD:  cfg.MaxCostUSD,
		costPer1K:   cfg.CostPer1K,
		startedAt:   time.Now().UTC(),
		phaseTokens: map[Phase]int{},
	}
}

// CheckTime returns ExitTimeLimit if wall-clock elapsed >= MaxDuration.
// now is injected so the check is pure and testable.
func (b *Budget) CheckTime(now time.Time) (exceeded bool, elapsed time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxDuration <= 0 {
		return false, 0
	}
	elapsed = now.Sub(b.startedAt)
	return elapsed >= b.maxDuration, elapsed
}

// EstimatedCostUSD returns the current estimated cost in USD.
// Returns 0 if CostPer1K is not set.
func (b *Budget) EstimatedCostUSD() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.costPer1K <= 0 {
		return 0
	}
	return float64(b.usedTokens) / 1000.0 * b.costPer1K
}

// CheckCost returns true if estimated cost >= MaxCostUSD.
func (b *Budget) CheckCost() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxCostUSD <= 0 || b.costPer1K <= 0 {
		return false
	}
	cost := float64(b.usedTokens) / 1000.0 * b.costPer1K
	return cost >= b.maxCostUSD
}

// MaxDuration returns the configured time limit.
func (b *Budget) MaxDuration() time.Duration {
	return b.maxDuration
}

// MaxCostUSD returns the configured dollar ceiling.
func (b *Budget) MaxCostUSD() float64 {
	return b.maxCostUSD
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

	var s string
	if b.maxTokens > 0 {
		pct := float64(b.usedTokens) / float64(b.maxTokens) * 100
		s = fmt.Sprintf("tokens %d/%d (%.0f%%) | iter %d/%d | status: %s",
			b.usedTokens, b.maxTokens, pct,
			b.usedIter, b.maxIter,
			b.statusLocked())
	} else {
		s = fmt.Sprintf("tokens %d/∞ | iter %d/%d | status: %s",
			b.usedTokens, b.usedIter, b.maxIter, b.statusLocked())
	}
	if b.costPer1K > 0 {
		cost := float64(b.usedTokens) / 1000.0 * b.costPer1K
		if b.maxCostUSD > 0 {
			s += fmt.Sprintf(" | cost $%.4f/$%.2f", cost, b.maxCostUSD)
		} else {
			s += fmt.Sprintf(" | cost $%.4f", cost)
		}
	}
	return s
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
	MaxTokens        int            `json:"max_tokens"`
	MaxIter          int            `json:"max_iter"`
	MaxDurationSec   float64        `json:"max_duration_sec,omitempty"`
	MaxCostUSD       float64        `json:"max_cost_usd,omitempty"`
	UsedTokens       int            `json:"used_tokens"`
	UsedIter         int            `json:"used_iter"`
	EstimatedCostUSD float64        `json:"estimated_cost_usd,omitempty"`
	PhaseTokens      map[string]int `json:"phase_tokens"`
	Status           string         `json:"status"`
	CapturedAt       time.Time      `json:"captured_at"`
}

// Snapshot returns a point-in-time copy for JSON persistence.
func (b *Budget) Snapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	pt := make(map[string]int, len(b.phaseTokens))
	for k, v := range b.phaseTokens {
		pt[string(k)] = v
	}
	var estimatedCost float64
	if b.costPer1K > 0 {
		estimatedCost = float64(b.usedTokens) / 1000.0 * b.costPer1K
	}
	var maxDurSec float64
	if b.maxDuration > 0 {
		maxDurSec = b.maxDuration.Seconds()
	}
	return Snapshot{
		MaxTokens:        b.maxTokens,
		MaxIter:          b.maxIter,
		MaxDurationSec:   maxDurSec,
		MaxCostUSD:       b.maxCostUSD,
		UsedTokens:       b.usedTokens,
		UsedIter:         b.usedIter,
		EstimatedCostUSD: estimatedCost,
		PhaseTokens:      pt,
		Status:           b.statusLocked().String(),
		CapturedAt:       time.Now().UTC(),
	}
}
