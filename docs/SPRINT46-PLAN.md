# Sprint 46 — CLI Wiring: Loop Brakes + Review Command (v1.4.0)

> **Status**: Shipped ✅ — commit follows  
> **Version target**: v1.4.0

---

## Background

Sprints 44–45 built all the loop hardening logic inside `internal/loop/` but left
the CLI (`cmd/radiant/main.go`) untouched. The new flags and commands existed only as
Go API — unreachable from the terminal. Sprint 46 wires everything to the user.

---

## Deliverables

### `radiant loop start` — new flags

| Flag | Type | Maps to | Description |
|------|------|---------|-------------|
| `--max-time` | `duration` | `BudgetConfig.MaxDuration` | Wall-clock limit (e.g. `30m`, `2h`) |
| `--max-cost` | `float64` | `BudgetConfig.MaxCostUSD` | Dollar ceiling (e.g. `0.50`) |
| `--model` | `string` | `PriceFor(modelID)` → `CostPer1K` | Enables cost tracking for known models |
| `--stall-patience` | `int` | `StallBrake.Patience` | Halt after N identical actions |
| `--quorum-k` | `int` | `QuorumConfig.K` | Min passing judges (0 = disabled) |
| `--quorum-n` | `int` | `QuorumConfig.N` | Total judges (default = K+1) |
| `--ground` | `bool` | `GroundingBlock()` | Inject commit log into each iteration |
| `--review-restarts` | `int` | `ReviewPanel.MaxRestarts` | Post-convergence max restarts |

Active limits are printed at startup:
```
✓ Loop started
  Run ID: run-1751234567
  Goal:   add rate limiting
  Budget: tokens 0/50000 (0%) | iter 0/20 | status: ok | cost $0.0000/$0.50
  Time limit:  30m
  Cost limit:  $0.50
  Stall brake: 3 fruitless turns
  Quorum:      2-of-3 judges
  Grounding:   commit-log injection enabled
```

### `radiant loop review` — new subcommand

```bash
# List all items waiting for human review
radiant loop review

# Approve an item (marks resolved; loop can resume)
radiant loop review --approve <id>

# Reject an item (marks resolved; loop does not resume)
radiant loop review --reject <id>
```

Reads from `.radiant-harness/inbox/<id>.json` written by `cycle.WriteInboxItem()`
when the verifier sets `Escalate: true`. Calls `loop.ListInboxItems()`,
`loop.ResolveInboxItem()` — no new package-level code needed.

---

## What was NOT done (intentional)

- No goroutine orchestration for quorum/review panel — those configs are stored and
  passed to `internal/loop/` functions; the actual LLM calls happen inside the loop
  engine (future Sprint when agent runner is wired end-to-end).
- No `--ground` auto-injection into prompts yet — flag is parsed and stored;
  `GroundingBlock()` is called at the engine layer when the loop runs for real.

---

## References

- `internal/loop/brake.go` — `StallBrake`, `NewStallBrake`
- `internal/loop/budget.go` — `BudgetConfig.MaxDuration`, `MaxCostUSD`, `CostPer1K`
- `internal/loop/pricing.go` — `PriceFor(modelID)`
- `internal/loop/review.go` — `ReviewPanel`, `QuorumConfig`, `RunQuorum`
- `internal/loop/ground.go` — `GroundingBlock`
- `internal/loop/cycle.go` — `ListInboxItems`, `ResolveInboxItem`, `WriteInboxItem`
