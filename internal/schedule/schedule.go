// Package schedule implements the "Schedule" stage of the loop-engineering
// cycle (Discover → Plan → Execute → Verify → Persist → Schedule): the harness
// reads work signals from the repository and decides, under a policy, whether
// to re-dispatch an autonomous loop run — instead of only ever running when a
// human invokes it.
//
// Evaluate is a pure function of (policy, state, signals, now) so the decision
// is deterministic and testable; the detector and persistence pieces are kept
// separate from the policy logic.
package schedule

import (
	"fmt"
	"sort"
	"time"
)

// TriggerKind is a category of work signal that can justify a loop run.
type TriggerKind string

const (
	TriggerNewCommits  TriggerKind = "new-commits"  // commits since last run
	TriggerFailingGate TriggerKind = "failing-gate" // a gate is currently red
	TriggerPendingWork TriggerKind = "pending-work" // open tasks / TODO markers
	TriggerInterval    TriggerKind = "interval"     // enough time has elapsed
)

// Signal is a single observed reason to (maybe) run.
type Signal struct {
	Kind   TriggerKind `json:"kind"`
	Detail string      `json:"detail"`
	Value  int         `json:"value"` // magnitude: commit count, TODO count, …
}

// Policy declares which signals trigger a run and the rate limits.
type Policy struct {
	Triggers      []TriggerKind `json:"triggers"`        // enabled trigger kinds
	MinInterval   time.Duration `json:"min_interval"`    // floor between runs
	MaxRunsPerDay int           `json:"max_runs_per_day"` // 0 = unlimited
}

// DefaultPolicy fires on new commits, failing gates, or pending work, no more
// than once every 15 minutes and at most 20 times per day.
func DefaultPolicy() Policy {
	return Policy{
		Triggers:      []TriggerKind{TriggerNewCommits, TriggerFailingGate, TriggerPendingWork},
		MinInterval:   15 * time.Minute,
		MaxRunsPerDay: 20,
	}
}

// State is the persisted scheduler memory between evaluations.
type State struct {
	LastRunAt   time.Time `json:"last_run_at"`
	RunsToday   int       `json:"runs_today"`
	DayStamp    string    `json:"day_stamp"` // YYYY-MM-DD of RunsToday
	LastCommit  string    `json:"last_commit"`
}

// Decision is the output of Evaluate.
type Decision struct {
	ShouldRun bool     `json:"should_run"`
	Reason    string   `json:"reason"`
	Signals   []Signal `json:"signals"`
}

// enabled reports whether kind is in the policy's trigger set.
func (p Policy) enabled(kind TriggerKind) bool {
	for _, t := range p.Triggers {
		if t == kind {
			return true
		}
	}
	return false
}

// Evaluate decides whether to dispatch a run now. It is pure: all time-dependent
// inputs arrive via `state` and `now`.
//
// Order of checks:
//  1. Rate limit — MinInterval since LastRunAt (unless this is the first run).
//  2. Daily cap — MaxRunsPerDay (reset when the day changes).
//  3. Triggers — at least one enabled trigger must have a matching signal.
func Evaluate(p Policy, s State, signals []Signal, now time.Time) Decision {
	// 1. Rate limit.
	if !s.LastRunAt.IsZero() && now.Sub(s.LastRunAt) < p.MinInterval {
		return Decision{
			ShouldRun: false,
			Reason: fmt.Sprintf("rate-limited: %s since last run, need %s",
				now.Sub(s.LastRunAt).Round(time.Second), p.MinInterval),
			Signals: signals,
		}
	}

	// 2. Daily cap (counter resets when the calendar day changes).
	today := now.Format("2006-01-02")
	runsToday := s.RunsToday
	if s.DayStamp != today {
		runsToday = 0
	}
	if p.MaxRunsPerDay > 0 && runsToday >= p.MaxRunsPerDay {
		return Decision{
			ShouldRun: false,
			Reason:    fmt.Sprintf("daily cap reached: %d/%d runs today", runsToday, p.MaxRunsPerDay),
			Signals:   signals,
		}
	}

	// 3. Triggers — collect the enabled signals that actually fired.
	var fired []Signal
	for _, sig := range signals {
		if sig.Kind == TriggerInterval {
			// interval is implicit: it "fires" only if the policy enables it
			// AND the rate-limit check above already passed.
			if p.enabled(TriggerInterval) {
				fired = append(fired, sig)
			}
			continue
		}
		if p.enabled(sig.Kind) && sig.Value > 0 {
			fired = append(fired, sig)
		}
	}

	if len(fired) == 0 {
		return Decision{
			ShouldRun: false,
			Reason:    "no enabled trigger fired",
			Signals:   signals,
		}
	}

	// Build a stable, human-readable reason.
	sort.Slice(fired, func(i, j int) bool { return fired[i].Kind < fired[j].Kind })
	parts := make([]string, 0, len(fired))
	for _, f := range fired {
		parts = append(parts, fmt.Sprintf("%s(%d)", f.Kind, f.Value))
	}
	return Decision{
		ShouldRun: true,
		Reason:    "triggered by " + joinComma(parts),
		Signals:   fired,
	}
}

// RecordRun advances the state after a dispatch, returning the new state.
func RecordRun(s State, lastCommit string, now time.Time) State {
	today := now.Format("2006-01-02")
	runsToday := s.RunsToday
	if s.DayStamp != today {
		runsToday = 0
	}
	return State{
		LastRunAt:  now,
		RunsToday:  runsToday + 1,
		DayStamp:   today,
		LastCommit: lastCommit,
	}
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
