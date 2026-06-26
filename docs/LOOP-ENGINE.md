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

| Condition | Exit Reason |
|-----------|-------------|
| All tasks verified + persisted | `success` |
| Token budget exceeded | `budget_exhausted` |
| Max iterations reached | `budget_exhausted` (iter limit) |
| 3 consecutive failures | `critical_failure` |
| User cancellation | `canceled` |

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
