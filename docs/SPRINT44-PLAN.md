# Sprint 44 — Loop Hardening: Human Checkpoint + Brakes

> **Status**: Shipped ✅ — commit `a2f232e`  
> **Version target**: v1.2.0  
> **Sources**: Loop Engineering Playbook (Osmani/Cherny/Steinberger), Akshay Twitter/X images Q2,
> `awesome-loop-engineering/examples/runnable/loop_cookbook/engine.py` (code verified)

---

## Background

After Sprints 41–43 (Ontology, Worktree, Schedule), an audit against the full loop-engineering
corpus — playbook, Twitter/X images, and code study of 6 community repos cloned to
`~/Downloads/forks-github/` — confirmed four gaps with zero implementation in the codebase.

Sprint 44 covers the three highest-priority gaps. The fourth (review panel
post-convergence) goes to Sprint 45 because it requires a deeper verifier redesign.

---

## Confirmed Gaps

### Gap 1 — Human Checkpoint / `escalate` signal

**Source**: `awesome-loop-engineering/examples/runnable/loop_cookbook/verifier.py`

```python
@dataclass
class Verdict:
    passed: bool
    evidence: str = ""
    tokens: int = 0
    escalate: bool = False   # ← when True, status = "needs_human", not retry
```

```python
# engine.py — the loop checks escalate before deciding to retry:
if verdict.escalate:
    status = "needs_human"
    break
```

The verifier decides whether to escalate ambiguous or high-risk changes to a human rather
than retrying. The loop stops with `status = "needs_human"` — a **success state**, not a
failure. The playbook calls this "the open door."

**Missing in radiant:**
- `Verdict.Escalate bool` field in `internal/loop/verifier.go`
- `awaiting-human` loop exit reason in `internal/loop/cycle.go`
- `./inbox/` directory for items the verifier escalated
- `radiant loop review` — list/approve/reject escalated items

---

### Gap 2 — No-Progress Brake (stall)

**Source**: `awesome-loop-engineering/examples/runnable/loop_cookbook/engine.py`

```python
before = dict(self.workspace)
self.apply(self.workspace, action)
made_change = self.workspace != before or bool(action.patch)
# ...
no_progress_streak = 0 if made_change else no_progress_streak + 1
if no_progress_streak >= self.stall_patience:
    status = "stalled"
    break
```

The loop tracks consecutive iterations with no observable change. After `stall_patience`
fruitless turns it halts with `status = "stalled"`. The Akshay image names the same
concept as *"same call & args"* — the harness variant is identical in intent.

**Missing in radiant:** zero stall detection. Confirmed by grep: no `stall`, `noProgress`,
`progress_streak` anywhere in `internal/loop/`.

**Design for Go:**
```go
// internal/loop/brake.go
type StallBrake struct {
    Patience int           // consecutive no-change turns before halt; default 3
    history  []string      // ring of sha256(action) for last N turns
}
func (b *StallBrake) Record(actionHash string) (stalled bool)
func (b *StallBrake) Reset()
```

The action hash covers the tool name + args (for tool-call loops) or the workspace diff
(for code loops). Pure function: `stalled` is returned, not read from a clock.

---

### Gap 3 — Time + Cost Budget

**Source**: `awesome-loop-engineering/examples/runnable/loop_cookbook/budget.py`

```python
@dataclass
class Budget:
    max_iterations: int = 25
    max_tokens: int | None = None
    cost_per_1k_tokens: float = 0.0   # ← missing in radiant
    tokens_used: int = 0

    @property
    def estimated_cost_usd(self) -> float:
        return round((self.tokens_used / 1000.0) * self.cost_per_1k_tokens, 6)
```

Current radiant `Budget` struct only has `MaxTokens` and `MaxIter`. Missing:
- `MaxDuration time.Duration` — wall-clock limit per run
- `MaxCostUSD float64` — dollar ceiling
- `CostPer1KTokens float64` — provider price, enables cost tracking
- `EstimatedCostUSD()` — live cost estimate for `radiant loop status`

`jonny981/loops` `budget.ts` confirms the same: `limit` (tokens) only, no time or cost.

**Design for Go:**
```go
// internal/loop/budget.go — extend existing struct
type Budget struct {
    MaxTokens    int
    MaxIter      int
    MaxDuration  time.Duration  // 0 = unlimited
    MaxCostUSD   float64        // 0 = unlimited
    CostPer1K    float64        // provider price per 1K tokens
}

// internal/loop/pricing.go — static table
var ProviderPricing = map[string]float64{
    "claude-opus-4-8":    15.0 / 1000,  // per 1K output tokens
    "claude-sonnet-4-6":   3.0 / 1000,
    "claude-haiku-4-5":    0.25 / 1000,
    "gpt-4o":              5.0 / 1000,
    "gpt-4o-mini":         0.15 / 1000,
}
```

---

## Deliverables

| # | File | What | Acceptance |
|---|------|------|------------|
| 1 | `internal/loop/verifier.go` | Add `Escalate bool` to `VerifyResult`; emit `ExitNeedsHuman` | Verifier returning `escalate=true` stops loop with reason, not retry |
| 2 | `internal/loop/cycle.go` | Add `ExitNeedsHuman` exit reason; write item to inbox on escalate | `.radiant-harness/inbox/<id>.json` created; loop status shows `needs-human` |
| 3 | `internal/loop/brake.go` | `StallBrake` — ring buffer, `Patience` policy, `Record()`/`Reset()` | Halts after N fruitless turns; pure (no wall-clock inside) |
| 4 | `internal/loop/budget.go` | Add `MaxDuration`, `MaxCostUSD`, `CostPer1K`; `EstimatedCostUSD()` | `loop status` shows `$0.042 / $1.00`; hard-stops on breach |
| 5 | `internal/loop/pricing.go` | Static provider→model→cost/1K table | `radiant loop start --model claude-sonnet-4-6` enables cost brake |
| 6 | `cmd/radiant/main.go` | `radiant loop review` — list/approve/reject inbox | `--approve <id>` resumes loop; `--reject <id>` aborts |
| 7 | `cmd/radiant/main.go` | `loop start` flags: `--max-time`, `--max-cost`, `--stall-patience` | Flags pass through to Budget + StallBrake |
| 8 | `internal/loop/*_test.go` | ≥20 pure tests for all three gaps | `go test ./internal/loop/... -race` green |
| 9 | `docs/LOOP-ENGINE.md` | Update exit-condition table; document all 7 brakes | No undocumented exit reason |

**Version bump**: `1.1.0` → `1.2.0`

---

## Design Principles

- All `Evaluate*` and `Record*` functions are **pure**: time arrives as `now time.Time`,
  cost is computed from tokens × price, never read from wall clock inside logic.
  Same pattern as `schedule.Evaluate`.
- `Inbox` = files in `.radiant-harness/inbox/<id>.json` — no database, git-committable,
  inspectable with any editor.
- `StallBrake` uses content hash of the action (workspace diff or tool+args), not
  iteration count — two different failed actions don't count as "same."
- Cost brake is disabled (not an error) when `--model` is unknown or not in pricing table.

---

## Exit-Condition Table (post-Sprint-44)

| Brake | Trigger | v1.1.0 | v1.2.0 |
|-------|---------|--------|--------|
| Max iterations | `iter >= MaxIter` | ✅ | ✅ |
| Token budget | `tokens >= MaxTokens` | ✅ | ✅ |
| Completion check | verifier PASS | ✅ | ✅ |
| Critical failures | 3× consecutive fail | ✅ | ✅ |
| **No-progress (stall)** | N fruitless turns | ❌ | ✅ |
| **Wall-clock time** | elapsed >= MaxDuration | ❌ | ✅ |
| **Dollar cost** | cost >= MaxCostUSD | ❌ | ✅ |
| **Human checkpoint** | verifier escalates | ❌ | ✅ |
| Review panel | post-convergence panel | ❌ | Sprint 45 |

---

## References

- `awesome-loop-engineering/examples/runnable/loop_cookbook/engine.py` — stall + escalate
- `awesome-loop-engineering/examples/runnable/loop_cookbook/budget.py` — cost tracking
- `jonny981/loops/src/core/budget.ts` — token-only budget (confirms our gap)
- `cobusgreyling/loop-engineering/docs/failure-modes.md` — "Infinite Fix Loop" (stall)
- Akshay image Q2 — "budget + time — token/$/sec", "no-progress — same call & args"
- Osmani/Cherny/Steinberger playbook — "The Open Door" (human checkpoint)
