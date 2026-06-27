# Sprint 49 — Status Cost + Resume Wiring (v1.7.0)

> **Status**: Shipped ✅  
> **Version target**: v1.7.0

---

## Background

After Sprint 48 wired `loop start` to real LLM calls, two commands were still
incomplete: `loop status` showed no cost information, and `loop resume` only printed
state without re-entering the loop. Sprint 49 closes both gaps.

---

## What was built

### `internal/loop/cycle.go` — `FormatStatus` extended

`FormatStatus` now renders a budget line when any cost/token data is present:

```
Run:   run-1751234567
Goal:  fix the race condition in scheduler
Phase: persist
Iter:  2 / 20
Since: 2026-06-27 14:05 UTC
Budget: tokens 12450/50000 | cost $0.0374/$1.00

Recent transitions:
  14:05:12  execute → verify — calling verifier
  14:05:18  verify → persist — checkpointing
```

The budget line appears only when `MaxTokens > 0`, `MaxCostUSD > 0`, or
`EstimatedCostUSD > 0` — silent when budget was not configured.

### `cmd/radiant/main.go` — `loopResumeCmd` rewritten

`radiant loop resume` now:

1. Loads persisted `LoopState` via `loop.LoadCycle()`
2. Guards against resuming a finished run (exits with clear error unless `needs_human`)
3. Resolves LLM credentials via `resolveLoopLLMCreds()` (same as `start`)
4. Restores `BudgetConfig` from the persisted `Snapshot` (tokens, iter, cost ceiling)
5. Calls `loop.Run()` — resumes real inference from current phase
6. Prints `RunResult` on completion, same format as `start`

New flags on `resume` (mirrors `start`):
- `--model <id>` — executor model
- `--verifier-model <id>` — separate verifier model
- `--base-url <url>` — endpoint override
- `--dry-run` — load state and print without any LLM calls

Guard: if `ExitReason` is set and is not `needs_human`, resume returns an error:
```
loop already finished with exit=success — start a new one with `radiant loop start`
```

---

## Usage

```bash
# Check status with cost (after a real run)
radiant loop status

# Resume after interruption or needs_human escalation
radiant loop resume --model claude-opus-4-8

# Preview what would resume without calling LLM
radiant loop resume --dry-run
```

---

## References

- `internal/loop/cycle.go:FormatStatus` — budget line added
- `internal/loop/budget.go:Snapshot` — `UsedTokens`, `MaxTokens`, `EstimatedCostUSD`, `MaxCostUSD`
- `cmd/radiant/main.go:loopResumeCmd` — full rewrite calling `loop.Run()`
