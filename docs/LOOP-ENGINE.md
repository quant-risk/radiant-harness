# Loop Engine

The Loop Engine (`internal/loop/`) implements the autonomous feedback cycle that powers `radiant loop start`.

## State Machine

```
idle
  └─→ discover               # project detection, context assembly
        ├─→ plan             # decompose goal into tasks
        │     ├─→ execute    # implement tasks
        │     │     ├─→ verify           # adversarial check
        │     │     │     ├─→ persist    # checkpoint + token snapshot
        │     │     │     │     ├─→ done     (success)
        │     │     │     │     └─→ discover  (next iteration)
        │     │     │     ├─→ execute    (retry on rejection)
        │     │     │     └─→ failed
        │     │     └─→ failed
        │     └─→ failed
        └─→ failed
              ├─→ idle
              └─→ discover  (retry)
```

Invalid transitions are rejected with an error. The state machine is encoded in
`validTransitions` in `cycle.go` — no ad-hoc if-chains.

## Components

### Budget (`budget.go`)
- Thread-safe token + iteration counter
- Three profiles: `lean` (10K), `standard` (50K), `thorough` (200K)
- States: `ok` → `warning` (>70%) → `exceeded` (>100%)
- Per-phase consumption tracking for reporting

### Cycle (`cycle.go`)
- Manages state transitions with crash-safe atomic persistence
- `loop.json` written via temp+fsync+rename after every transition
- Tracks consecutive failures; halts after 3 (`ExitCritical`)
- `ShouldContinue()` checks budget + iter limit before each phase

### Trace (`trace.go`)
- Append-only JSONL file per run: `.radiant-harness/traces/<run-id>.jsonl`
- One event per tool call (via `post-tool.mjs` hook)
- Fields: phase, action, result, evidence, tokens_in/out, meta

### Verifier (`verifier.go`)
The verifier is always a **separate** agent call. The executor never grades its own work.

```
BuildVerifierPrompt(goal, executorOutput, cfg)
  → "Assume the work is BROKEN until you find concrete evidence otherwise..."
  → "Default to REJECTED if uncertain" (strict mode)

ParseVerifyResponse(response, cfg)
  → VERDICT: APPROVED|REJECTED
  → SCORE: 0.0–1.0
  → EVIDENCE: <specific proof>
  → ISSUES: - <list>

Score below MinScore (0.70 default) forces rejection even if verdict says APPROVED.
```

## CLI Commands

```bash
# Start a new autonomous loop
radiant loop start "implement payment webhook" [--profile=lean|standard|thorough]
                                               [--budget=N] [--max-iter=N]

# Check current phase + budget
radiant loop status

# Resume after interrupt (picks up from last committed phase)
radiant loop resume

# Inspect reasoning trace
radiant trace show <run-id>
radiant trace show <run-id> --json
radiant trace list
```

## Persistence Files

```
.radiant-harness/
  loop.json              # current cycle state (atomic writes)
  traces/
    <run-id>.jsonl       # append-only JSONL event log
```

## Exit Conditions

| Condition | Exit Reason | Version |
|-----------|-------------|---------|
| All tasks verified + persisted | `success` | ✅ v1.1.0 |
| Token budget exceeded | `budget_exhausted` | ✅ v1.1.0 |
| Max iterations reached | `budget_exhausted` (iter limit) | ✅ v1.1.0 |
| 3 consecutive failures | `critical_failure` | ✅ v1.1.0 |
| User cancellation | `canceled` | ✅ v1.1.0 |
| **No-progress stall** (N fruitless turns) | `stalled` | 📋 v1.2.0 |
| **Wall-clock time exceeded** | `time_exhausted` | 📋 v1.2.0 |
| **Dollar cost exceeded** | `cost_exhausted` | 📋 v1.2.0 |
| **Verifier escalated to human** | `needs_human` | 📋 v1.2.0 |
| **Review panel failed** (post-convergence) | re-opens loop | 📋 v1.3.0 |

> Sources: `awesome-loop-engineering/engine.py` (stall + escalate), `jonny981/loops/loop.ts`
> (review panel). See `docs/SPRINT44-PLAN.md` and `docs/SPRINT45-PLAN.md`.

## Planned Components (Sprints 44–45)

### StallBrake (`internal/loop/brake.go`) — v1.2.0
Tracks action hashes across iterations. After N fruitless turns (no observable change),
halts with `stalled`. Pure: no wall-clock inside logic. Policy: `stall_patience` (default 3).

### Pricing + Cost Brake (`internal/loop/pricing.go`) — v1.2.0
Static `provider→model→$/1K-token` table. `Budget.EstimatedCostUSD()` reports live spend.
`MaxCostUSD` is a hard brake enforced before each iteration.

### Escalate / Inbox (`internal/loop/verifier.go` + cycle) — v1.2.0
`VerifyResult.Escalate bool` signals the loop to stop with `needs_human` and write the
finding to `.radiant-harness/inbox/<id>.json`. `radiant loop review` lists/resolves items.

### Review Panel (`internal/loop/review.go`) — v1.3.0
Post-convergence second layer. Runs after `verifier` passes. A fail re-opens the loop
body with `lastReview` findings threaded as context. Capped by `MaxRestarts` (default 3),
independent of `MaxIter`. Pattern from `jonny981/loops:config.review()`.

### Quorum Verifier — v1.3.0
N parallel judge goroutines; K must pass. A goroutine that errors counts as "no."
Confidence = mean of passing judges. Prevents a single biased model from rubber-stamping.

### Commit-Log Grounding (`internal/loop/ground.go`) — v1.3.0
`GroundingBlock()` injects recent branch commits into each iteration's prompt.
Prevents the agent from re-walking dead ends across fresh-context turns.
Pattern from `jonny981/loops:ground.ts:groundingText()`.

## Example Run

```bash
$ radiant loop start "add rate limiting to /api/users endpoint"
✓ Loop started
  Run ID: run-1719396000
  Goal:   add rate limiting to /api/users endpoint
  Budget: tokens 0/50000 (0%) | iter 0/20 | status: ok

# Loop autonomously executes:
# discover → plan → execute → verify (REJECTED: missing tests)
# → execute (retry: add tests) → verify (APPROVED: 0.92)
# → persist → done

$ radiant loop status
Run:   run-1719396000
Goal:  add rate limiting to /api/users endpoint
Phase: done
Iter:  2 / 20

$ radiant trace show run-1719396000
15:04:01 [discover] ✓ detect_domain — ok (domain: backend)
15:04:02 [plan]     ✓ decompose — ok
15:04:05 [execute]  ✓ write_file — ok (internal/api/ratelimit.go)
15:04:10 [verify]   ✗ review — failed (missing tests)
15:04:15 [execute]  ✓ write_file — ok (internal/api/ratelimit_test.go)
15:04:22 [verify]   ✓ review — ok (score 0.92, 4/4 ACs verified)
15:04:23 [persist]  ✓ checkpoint — ok
```
