# Sprint 44 — Loop Hardening: Human Checkpoint + Brakes

> **Status**: Planned  
> **Version target**: v1.2.0  
> **Source**: Loop Engineering audit against Osmani/Cherny/Steinberger playbook + Akshay Twitter images (June 2026)

---

## Background

After building Sprints 41–43 (Ontology, Worktree, Schedule), a final audit against all
loop-engineering source material — including two Twitter/X images that were missed in the
initial pass — identified four remaining gaps. These are concrete, named concepts from the
literature that have zero implementation in the codebase.

---

## Confirmed Gaps (post-image audit)

### Gap 1 — Human Checkpoint / "The Open Door" (Playbook, highest priority)

The Loop Engineering Playbook calls human-in-loop "the most important element" of the
First-Loop Checklist (Table VI). The loop must be able to pause at a configurable phase
and wait for human approval before continuing.

**Missing:**
- `awaiting-human` loop state
- `./inbox/` directory for uncertain findings the agent can't resolve autonomously
- `radiant loop review` command to display + approve/reject held items
- `PhaseReview` as a named phase in the state machine

**Current state:** loop runs fully autonomous; there is no mechanism to pause for human input.

---

### Gap 2 — No-Progress Brake (Akshay image, Q2)

> *"Know when to stop: no-progress — same call & args"*

The loop must detect when the agent is calling the same tool with the same arguments
repeatedly (stall) and halt rather than burning budget.

**Missing:** zero stall-detection logic. Confirmed by grep: no `no-progress`, `stall`,
`sameCall`, `repeatArgs` anywhere in the codebase.

**Design:**
```go
type CallRecord struct { Tool string; ArgsHash string }
// RingBuffer of last N calls; if last K are identical → stall
func DetectStall(history []CallRecord, window int) bool
```

---

### Gap 3 — Time + Cost Budget Dimensions (Akshay image, Q2)

> *"Know when to stop: budget + time — token/$/sec"*

Current `Budget` struct only tracks tokens and iterations:
```go
type Budget struct { MaxTokens int; MaxIter int }
```

Missing:
- Wall-clock time limit per run (`MaxDuration time.Duration`)
- Dollar-cost ceiling (`MaxCostUSD float64`) derived from provider price tables
- Both enforced as hard brakes, not advisory limits

---

### Gap 4 — Sample Review (Playbook)

The playbook describes periodic "comprehension rot" checks: a separate agent reads a
random sample of prior reasoning traces and flags if the agent's understanding has drifted.

**Missing:** no sampling mechanism, no comprehension checker.

This is lower priority than Gaps 1–3 and can be a sub-deliverable of the fleet layer.

---

## Deliverables

| # | Deliverable | Effort | Acceptance |
|---|-------------|--------|------------|
| 1 | `internal/loop/checkpoint.go` — `awaiting-human` state, `Inbox` (read/write/list) | M | `radiant loop review` lists held items; approve/reject advances or aborts loop |
| 2 | `internal/loop/brake.go` — `NoProgressBrake` (stall detector) | S | Halts after K identical `(tool, args-hash)` pairs; emits structured reason |
| 3 | `internal/loop/budget.go` — extend with `MaxDuration` + `MaxCostUSD` | S | Hard-stops loop when wall-clock or cost exceeds limit; `loop status` shows remaining |
| 4 | Provider price table (`internal/loop/pricing.go`) | S | Static map of provider→model→cost/1K-tokens; used by cost brake |
| 5 | `radiant loop review` command | M | List inbox items; `--approve <id>` / `--reject <id>` resumes or aborts loop |
| 6 | `radiant loop start` flags: `--max-time`, `--max-cost`, `--checkpoint=<phase>` | S | All three new brake dimensions available from CLI |
| 7 | Tests — checkpoint, stall, time+cost brakes | M | ≥20 new tests, pure (no wall-clock reads inside logic) |
| 8 | `docs/LOOP-ENGINE.md` update — document all 4 brakes and human checkpoint | S | Table of all exit conditions is complete |

**Version bump**: `1.1.0` → `1.2.0`

---

## Design Principles

- `Evaluate*` functions are **pure**: time and cost arrive as parameters, never read from
  wall clock inside logic. Same pattern as `schedule.Evaluate`.
- `Inbox` is just files in `.radiant-harness/inbox/<id>.json` — no database, inspectable
  with any editor, committed to git if the user wants a review trail.
- The stall detector uses a ring buffer of `(tool, sha256(args))` pairs; the window and
  threshold are policy parameters, not hardcoded.
- Cost brake requires a provider+model to be known at loop start (`--model`); if unknown,
  cost brake is disabled (not an error).

---

## Exit-Condition Table (post-Sprint-44)

| Brake | Trigger | Current | After Sprint 44 |
|-------|---------|---------|-----------------|
| Max iterations | `iter >= MaxIter` | ✅ | ✅ |
| Token budget | `tokens >= MaxTokens` | ✅ | ✅ |
| Completion check | verifier PASS | ✅ | ✅ |
| **No-progress (stall)** | same call+args K times | ❌ | ✅ |
| **Wall-clock time** | elapsed >= MaxDuration | ❌ | ✅ |
| **Dollar cost** | cost >= MaxCostUSD | ❌ | ✅ |
| **Human checkpoint** | phase == checkpoint | ❌ | ✅ |

---

## References

- Osmani / Cherny / Steinberger — *Loop Engineering Playbook* (June 2026), Table VI (First-Loop Checklist), §4 "The Open Door"
- Akshay (@akshay_pachaar) — "Loop Engineering Clearly Explained" (Twitter/X, June 2026), Q2: "Know when to stop"
- `internal/schedule/schedule.go` — template for pure `Evaluate()` pattern
- `internal/loop/budget.go` — existing budget struct to extend
