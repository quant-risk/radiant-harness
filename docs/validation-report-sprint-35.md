# Validation Report — Sprint 35: Loop Engine

**Date:** 2026-06-26
**Sprint:** 35 of 40
**Status:** PASSED

---

## Deliverables

| File | Lines | Purpose |
|------|-------|---------|
| `internal/loop/budget.go` | 246 | Thread-safe token+iteration budget manager |
| `internal/loop/trace.go` | 171 | Append-only JSONL reasoning trace recorder |
| `internal/loop/cycle.go` | 299 | Loop state machine with crash-safe persistence |
| `internal/loop/verifier.go` | 185 | Adversarial verification (prompt builder + parser) |
| `internal/loop/loop_test.go` | 350 | 37 tests covering all four components |

**CLI additions (cmd/radiant/main.go):**
- `radiant loop start "<goal>" [--budget=N] [--max-iter=N] [--profile=lean|standard|thorough]`
- `radiant loop status`
- `radiant loop resume`
- `radiant trace show <run-id> [--json]`
- `radiant trace list`

---

## Test Results

```
ok  github.com/quant-risk/radiant-harness/internal/loop  0.429s
```

**37/37 tests pass.** Coverage by component:

### Budget (9 tests)
- `TestBudget_NewFromProfile` — standard profile yields 50K tokens
- `TestBudget_NewFromExplicit` — explicit values override profile
- `TestBudget_ConsumeAndStatus` — OK → Warning at 70% → Exceeded at 100%
- `TestBudget_IterLimit` — BudgetExceeded when MaxIter reached
- `TestBudget_Remaining` — returns tokens left, 0 when negative
- `TestBudget_UnlimitedRemaining` — -1 when no token limit set
- `TestBudget_PhaseBreakdown` — per-phase consumption tracking
- `TestBudget_Snapshot` — point-in-time copy for JSON persistence
- `TestBudget_Summary` — human-readable string contains used tokens and status

### Cycle (11 tests)
- `TestCycle_NewAndTransition` — starts idle, transitions to discover
- `TestCycle_InvalidTransition` — idle→execute rejected (must go through discover→plan)
- `TestCycle_Persistence` — loop.json written atomically; LoadCycle restores phase and goal
- `TestCycle_LoadMissing` — error returned for empty directory
- `TestCycle_ConsecFailures` — loop halts with ExitCritical after 3 consecutive failures
- `TestCycle_ShouldContinue_BudgetExceeded` — stops when token budget exceeded
- `TestCycle_ShouldContinue_MaxIter` — stops when iteration limit reached
- `TestCycle_SetExit` — records exit reason in persisted state
- `TestCycle_LogPreservation` — phase log accumulates entries
- `TestCycle_FormatStatus` — output contains run ID, goal, phase
- `TestCycle_FormatStatus_Empty` — "No active loop" for zero-value state

### Trace (8 tests)
- `TestTracer_RecordAndRead` — 3 events written, read back with correct RunID and Action
- `TestTracer_JSONValidity` — each line is valid JSON
- `TestTracer_TimestampSet` — Timestamp set within bounded wall-clock window
- `TestListTraces` — finds all 3 trace files by run ID
- `TestListTraces_Empty` — nil for empty directory
- `TestFormatTrace` — ✓ for ok, ✗ for failed, evidence included
- `TestFormatTrace_Empty` — "(empty trace)" message

### Verifier (9 tests)
- `TestBuildVerifierPrompt_ContainsGoal` — goal and output appear in prompt
- `TestBuildVerifierPrompt_StrictMode` — "Default to REJECTED" in strict prompt
- `TestParseVerifyResponse_Approved` — score 0.92, evidence extracted
- `TestParseVerifyResponse_Rejected` — 2 issues parsed correctly
- `TestParseVerifyResponse_ScoreBelowThreshold` — score 0.65 < 0.80 minimum → forced rejected
- `TestParseVerifyResponse_MalformedDefault` — strict mode defaults to rejected
- `TestShouldRetry` — retry on rejected-with-issues; no retry on approved
- `TestFormatVerifyResult` — REJECTED + issue text in output

---

## Architecture Correctness

### State Machine Transitions
```
idle → discover
discover → plan | failed
plan → execute | failed
execute → verify | failed
verify → persist | execute (retry) | failed
persist → done | discover (next iteration)
done → idle
failed → idle | discover
```
Invalid transitions (e.g., `idle → execute`) are rejected with an error.

### Adversarial Verifier Design
The verifier uses an explicitly skeptical prompt: *"Assume the work is BROKEN until you find concrete evidence otherwise."* This prevents the common failure mode where an LLM confirms its own output without scrutiny. Key properties:
- Structured output format (VERDICT / SCORE / EVIDENCE / ISSUES)
- MinScore enforcement: score below threshold forces rejection even if verdict says APPROVED
- StrictMode: malformed responses default to REJECTED
- ShouldRetry: returns true only when rejected with actionable issues

### Crash Safety
All state writes go through `persistLocked()`:
1. Marshal to JSON
2. `os.CreateTemp()` in same directory
3. `Write()` + `Sync()`
4. `os.Rename()` (atomic on POSIX)

Resume after crash reads `loop.json` and picks up from the last committed phase.

---

## Build

```
$ go build ./cmd/radiant/...   # exit 0
$ go vet ./internal/loop/...   # exit 0
$ go vet ./cmd/radiant/...     # exit 0
$ gofmt -l ./internal/loop/... # clean after auto-format
```

---

## Pre-existing Issues (not Sprint 35)

`internal/harness`, `internal/engine`, `internal/llm` fail with `dyld: missing LC_UUID load command` on Darwin 25.5.0. Confirmed pre-existing via git history — not caused by Sprint 35 changes.

---

## Next: Sprint 36

Enhanced Hooks + IDE Adapters:
- `post-tool.mjs` / `pre-tool.mjs` — context-aware hook scripts
- `load-context.mjs v2` — lazy skill loading from registry
- Cursor MDC format support
