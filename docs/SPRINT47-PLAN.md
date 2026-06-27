# Sprint 47 — Loop Runner: LLM Integration (v1.5.0)

> **Status**: Shipped ✅  
> **Version target**: v1.5.0

---

## Background

Sprints 44–46 built a complete loop harness — state machine, brakes, budget,
verifier, review panel, grounding, CLI — but `radiant loop start` didn't actually
call any LLM. The loop managed state but never ran inference. Sprint 47 closes that
gap with `loop.Run()`: the central integration function that connects every prior
sprint into a real autonomous loop.

---

## What was built

### `internal/loop/runner.go` — `Run()` and `RunConfig`

```go
func Run(ctx context.Context, projectDir, runID, goal string, cfg RunConfig) (*RunResult, error)
```

The main loop. Per iteration:

1. **Boundary checks** — `ShouldContinue`, `CheckTime`, `CheckCost`, `ctx.Err()`
2. **Discover → Plan** — state transitions (no LLM, lightweight)
3. **Execute** — `execClient.SimpleChat()` with optional `GroundingBlock()` injection and prior review findings
4. **Stall check** — `StallBrake.Record(execOutput)` → `ExitStalled` if triggered
5. **Verify** — separate `verClient.SimpleChat()` with `BuildVerifierPrompt()`
6. **Escalation** — if `result.Escalate`, writes inbox item, exits `ExitNeedsHuman`
7. **Rejection** — if `!result.Approved`, increments iteration, re-enters loop
8. **Post-convergence review** — `BuildReviewPrompt()` → `ParseReviewResponse()`; findings fed back to next execute; `MaxRestarts` caps standoff
9. **Persist** — `UpdateBudget`, stall reset, `ExitSuccess`

### Design invariants

- **Executor ≠ Verifier**: separate `llm.Model` configs; verifier defaults to executor model only when not set explicitly; the maker never grades own work
- **Nil-safe stall brake**: when `StallPatience == 0`, stall is `nil`; all `stall.*` calls go through a nil-safe wrapper
- **Fail-open review**: if the reviewer LLM errors, the loop continues (avoid blocking on infra failure)
- **Token estimation**: `estimateTokens(prompt, response)` uses 4-chars/token when real counts are unavailable

### `RunConfig` fields

| Field | Source | Description |
|-------|--------|-------------|
| `ExecutorModel` | `--model` (Sprint 46) | LLM that implements each iteration |
| `VerifierModel` | `--model` (future flag) | Separate verifier LLM; falls back to executor |
| `Budget` | `BudgetConfig` (Sprint 44) | Token/iter/time/cost brakes |
| `StallPatience` | `--stall-patience` | 0 = disabled |
| `Verifier` | `VerifierConfig` | MinScore, quorum config |
| `Review` | `ReviewPanel` | MaxRestarts for post-convergence slot |
| `Ground` | `--ground` | Inject commit log each iteration |
| `MaxGroundCommits` | internal | 0 → `GroundingBlock` default (10) |

### `RunResult` fields

```go
type RunResult struct {
    RunID, Goal  string
    ExitReason   ExitReason
    Iterations   int
    FinalPhase   Phase
    Elapsed      time.Duration
    TokensUsed   int
    CostUSD      float64
}
```

---

## Tests — `sprint47_test.go` (21 new tests)

- `RunConfig` verifier defaults and overrides
- `ReviewPanel.maxRestarts()` — zero value and custom
- `estimateTokens` — empty, approximate, rounding
- `StallBrake.reset()` — nil-safe, state cleared after reset
- `buildExecutorPrompt` — goal only, grounding block, findings, no spurious sections
- System prompt content — non-empty, verifier defaults to REJECTED
- `buildResult` — field mapping, elapsed time
- `RunConfig` verifier model fallback chain

---

## References

- `internal/llm/client.go` — `SimpleChat(ctx, system, user)`
- `internal/loop/cycle.go` — `Cycle`, state machine, `WriteInboxItem`
- `internal/loop/budget.go` — `Budget.CheckTime`, `CheckCost`, `UpdateBudget`
- `internal/loop/brake.go` — `StallBrake.Record`, `Reset`
- `internal/loop/verifier.go` — `BuildVerifierPrompt`, `ParseVerifyResponse`
- `internal/loop/review.go` — `BuildReviewPrompt`, `ParseReviewResponse`, `DefaultReviewPanel`
- `internal/loop/ground.go` — `GroundingBlock`
