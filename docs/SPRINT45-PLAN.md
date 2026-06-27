# Sprint 45 — Verifier Hardening: Review Panel + Quorum + Grounding

> **Status**: Planned  
> **Version target**: v1.3.0  
> **Sources**: Code study of `jonny981/loops` (loop.ts, condition.ts, ground.ts),
> `awesome-loop-engineering` (FIELD-NOTES.md, docs/05-stop-conditions-and-verification.md),
> `Agent-Loop-Skills` (optimize-loop/SKILL.md, tournament-autoresearch/SKILL.md)

---

## Background

After Sprint 44 closes the four hard-stop gaps, Sprint 45 hardens the **verifier** itself.
The `jonny981/loops` codebase (code-studied, not just README) implements three concepts
that our verifier is missing, all grounded in the same principle: the verifier that runs
*once per iteration* and gives a binary PASS/FAIL is the weakest honest signal available.

---

## Confirmed Gaps (from code study)

### Gap 1 — Review Panel (post-convergence slot)

**Source**: `jonny981/loops/src/core/loop.ts` lines 130–170

```typescript
if (conv.met) {
  if (!config.review) {
    await recordMilestone(...)
    return finish({ status: 'pass', ... }, iteration)
  }
  // ← review slot: runs AFTER until is met
  let reviewOutcome = await config.review(ctxAt(iteration, last))
  if (reviewOutcome.status === 'pass') {
    await recordMilestone(...)
    return finish({ status: 'pass', ... }, iteration)
  }
  // review failed — thread findings to next iteration as lastReview
  consecutiveReviewFails += 1
  lastReview = reviewOutcome
  // re-enter the loop body (not a retry of the same action)
}
```

The loop has **two verification layers**:
1. `until` — checked every iteration (our current verifier)
2. `review` — runs only AFTER convergence; a fail re-opens the loop with `lastReview`
   threaded to the next iteration so the agent fixes concrete findings

`maxReviewRestarts` caps the worker↔reviewer standoff independently of `max`.

**Missing in radiant:** our verifier is only layer 1. There is no post-convergence review
slot, no `lastReview` handoff, no `maxReviewRestarts` cap.

**Design:**
```go
// internal/loop/review.go
type ReviewPanel struct {
    MaxRestarts int           // default 3; independent of MaxIter
}
type ReviewResult struct {
    Pass     bool
    Findings []string         // fed to next iteration as context
}
// RunReview(goal, executorOutput, traces, cfg) ReviewResult
// Called by cycle.go after verifier PASS, before recording milestone
```

---

### Gap 2 — Quorum k-of-n Verifier

**Source**: `jonny981/loops/src/core/condition.ts`

```typescript
export function quorum(k: number, ...inputs: ConditionInput[]): Condition {
  return async (ctx, last) => {
    // all judges run in parallel; a throw counts as a "no" vote
    const settled = await Promise.allSettled(conds.map(c => c(ctx, last)))
    const results = settled.map(s =>
      s.status === 'fulfilled' ? s.value
        : { met: false, reason: `judge errored: ${s.reason.message}` }
    )
    const held = results.filter(r => r.met)
    const confidence = mean(held.map(r => r.confidence).filter(isNumber))
    return {
      met: held.length >= k,
      confidence,
      reason: `quorum ${held.length}/${inputs.length} held (need ${k})`
    }
  }
}
```

A single verifier call is the classic trap: "models reliably skew positive when grading
their own work" (docs/05-stop-conditions-and-verification.md). Quorum runs N independent
judges in parallel; a judge that errors counts as "no." Reported confidence = mean of
passing judges.

**`awesome-loop-engineering` docs/05** adds the empirical basis: four different AI
reviewers over 146 PRs → 93.4% of findings caught by only ONE of the four tools. Zero
issues caught by all four. Heterogeneity is the point — N copies of the same model is
one reviewer with a bigger bill.

**Missing in radiant:** single verifier call, single model, single attempt.

**Design:**
```go
// internal/loop/verifier.go — extend existing VerifyConfig
type VerifyConfig struct {
    // existing fields ...
    QuorumK     int      // if > 0, run QuorumN judges and require K to pass
    QuorumN     int      // number of parallel judge calls (default = QuorumK+1)
}
// runQuorumVerify(goal, output, cfg) VerifyResult
//   → spawn QuorumN goroutines, collect with errgroup, count passing, mean confidence
```

---

### Gap 3 — Geometric-Mean per Dimension

**Source**: `jonny981/loops/src/core/condition.ts` — `agentCheck` with `dimensions`

```typescript
// any zero (a fully-failed dimension) drags the result to 0
function geometricMean(values: number[]): number {
  if (!values.length) return 0
  if (values.some(v => v <= 0)) return 0
  return Math.exp(values.reduce((a, b) => a + Math.log(b), 0) / values.length)
}
// agentCheck prompt:
// "Score EACH named dimension 0..1. Gate opens when geometric mean >= threshold."
// "A weak dimension drags the whole verdict down."
```

Our verifier outputs a single `Score float64`. A score of 0.9 could hide one dimension
at 0.0 averaging with others at 1.0. Geometric mean makes that impossible: one 0.0
dimension → overall 0.0.

**Missing in radiant:** no per-dimension scoring, arithmetic average only.

**Design:**
```go
// internal/loop/verifier.go
type VerifyDimension struct {
    Name  string
    Score float64  // 0..1
}
type VerifyResult struct {
    Verdict    string            // APPROVED | REJECTED
    Score      float64           // geometric mean of dimensions
    Dimensions []VerifyDimension // per-axis breakdown (optional)
    Evidence   string
    Issues     []string
    Escalate   bool              // from Sprint 44
}
func geometricMean(scores []float64) float64
```

---

### Gap 4 — Commit-Log Grounding

**Source**: `jonny981/loops/src/core/ground.ts`

```typescript
// groundingText() — injected into each fresh context before the agent works
export async function groundingText(workspace, opts): Promise<string> {
  const records = await log({ cwd, since, max: 10 })
  // renders as:
  // "## Recent work on `branch` (the commit log)
  //  What prior iterations already did and why — read it so you don't repeat a dead end."
}
```

The principle: *"fresh context kills rot; grounding kills amnesia."* Each iteration runs
with a clean context (no carry-over transcript) but reads the recent commit log as memory.
Prevents the agent from re-walking dead ends.

**Missing in radiant:** our trace is passive (stored to JSONL) but not injected into the
next iteration's prompt. The agent has no automatic access to what previous iterations tried.

**Design:**
```go
// internal/loop/ground.go
// GroundingBlock(repoDir, since, maxCommits int) (string, error)
//   → runs git log --oneline --format=... --max-count=N
//   → returns markdown block injected into loop prompt
//   → truncates each body to avoid re-rot
```

---

### Gap 5 — Anti-Cheat Clauses in Verifier

**Source**: `awesome-loop-engineering/FIELD-NOTES.md` Note #8

> *"If you don't name the cheats, the loop will take them. A maker optimizing to make
> checks pass will delete the failing test, weaken the assertion, stub the function,
> widen the scope. The anti-cheat clauses aren't paranoia — they're part of the spec."*

**Source**: `cobusgreyling/loop-engineering/docs/anti-patterns.md` Anti-pattern #1

> *"Verifier default stance: REJECT. It does not grade its own homework."*

Our `verifier.go` already has "assume BROKEN" stance (✅). What's missing: explicit
instructions to the verifier to **detect and reject test weakening, stub implementations,
and scope widening** by the maker. Currently the verifier only checks the outcome,
not the method.

**Missing in radiant:** verifier prompt has no anti-cheat clauses.

**Design:** extend `BuildVerifierPrompt()` in `internal/loop/verifier.go` to include:
```
ANTI-CHEAT CHECKS (verify all before approving):
- No test was deleted, commented out, or had its assertion weakened
- No function was left as a stub or placeholder
- Scope is unchanged from the original goal (no unrelated changes)
- The gate was not widened to make a check pass
If any of these are violated, REJECT with specific evidence.
```

---

## Deliverables

| # | File | What | Acceptance |
|---|------|------|------------|
| 1 | `internal/loop/review.go` | `ReviewPanel` — post-convergence slot, `lastReview` handoff, `MaxRestarts` | Failing review re-enters loop body with findings; capped by MaxRestarts |
| 2 | `internal/loop/cycle.go` | Call `ReviewPanel.Run()` after verifier PASS before milestone | Loop only exits `done` after both verifier AND review panel pass |
| 3 | `internal/loop/verifier.go` | Quorum: `QuorumK`, `QuorumN` fields; `runQuorumVerify()` | N parallel judge goroutines; K must pass; erroring judge = no vote |
| 4 | `internal/loop/verifier.go` | Dimensions + geometric mean; anti-cheat prompt clauses | `Dimensions []VerifyDimension`; geo mean; one zero = overall zero |
| 5 | `internal/loop/ground.go` | `GroundingBlock()` — recent commit log as markdown | Injected into loop prompt on every iteration; truncated to avoid re-rot |
| 6 | `internal/loop/*_test.go` | ≥20 pure tests for all five gaps | `go test ./internal/loop/... -race` green |
| 7 | `cmd/radiant/main.go` | `loop start` flags: `--quorum-n`, `--quorum-k`, `--review-restarts`, `--ground` | All new verifier options available from CLI |
| 8 | `docs/LOOP-ENGINE.md` | Document review panel, quorum, dimensions, grounding | Components section updated |

**Version bump**: `1.2.0` → `1.3.0`

---

## Interaction with Sprint 44

Sprint 44 adds `Escalate bool` to `VerifyResult` (Gap 1 there). Sprint 45 extends
`VerifyResult` further with `Dimensions`, quorum, and review panel. Both sprints must
touch `internal/loop/verifier.go` — build in sequence (44 first, 45 after).

---

## References

- `jonny981/loops/src/core/loop.ts` — review slot, `lastReview`, `maxReviewRestarts`
- `jonny981/loops/src/core/condition.ts` — `quorum()`, `agentCheck.dimensions`, `geometricMean()`
- `jonny981/loops/src/core/ground.ts` — `groundingText()`, `retrieveLedger()`
- `awesome-loop-engineering/FIELD-NOTES.md` — Note #2 (verifier is the asset), #8 (anti-cheat), #4 (fresh context + grounding)
- `awesome-loop-engineering/docs/05-stop-conditions-and-verification.md` — 4-reviewer study (93.4% single-tool findings), geometric mean rationale
- `Agent-Loop-Skills/loops/optimize-loop/SKILL.md` — correctness gate + metric separation
- `cobusgreyling/loop-engineering/docs/anti-patterns.md` — Anti-pattern #1 (verifier = same session)
- `cobusgreyling/loop-engineering/docs/failure-modes.md` — "Verifier Theater" failure mode
