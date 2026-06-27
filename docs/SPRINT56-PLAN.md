# Sprint 56–58 — Model Routing Engine Implementation Plan

> **Design doc:** [`MODEL-ROUTING.md`](MODEL-ROUTING.md)
> **Target:** v2.1.0
> **Current state:** v2.0.0 shipped (Sprint 52), Sprint 55 in progress
> (Plan Phase with LLM)
> **WARNING:** Sprint numbering was corrected. Sprints 46–55 are already
> shipped/in-progress by another agent with different topics.
> This plan starts at Sprint 56.

---

## What the other agent already solved

These items from the original design discussion are DONE:

- [x] Plan Phase with LLM (Sprint 55) — RunConfig.Plan, PlannerModel,
      BuildPlannerPrompt, plannerClient
- [x] Streaming (Sprint 52) — simpleChatStream, StreamOut
- [x] Context injection (Sprint 51) — assembleContextBlock
- [x] estimateTokens UTF-8 accuracy (Sprint 52) — utf8.RuneCountInString
- [x] discover->discover bug (Sprint 52)
- [x] Trace integration (Sprint 50) — JSONL per LLM call
- [x] Budget/cost/time brakes (Sprint 44)
- [x] Review panel + quorum + grounding (Sprint 45)
- [x] Separate executor/verifier clients (Sprint 47)

## What remains unsolved

- [ ] Three model ID tables are out of sync (PresetModels vs pricing vs
      PricePerMTokensUSD)
- [ ] No GLM, Kimi, MiniMax, Qwen, Haiku, gpt-5-mini/nano presets
- [ ] AutoRoute uses stale model IDs (claude-opus-4.1 etc.)
- [ ] No agent-aware routing (only direct_api works)
- [ ] No `radiant models route` command
- [ ] No static config emission for Claude/Hermes/OpenCode/etc.

---

## Sprint 56 — Model ID Sync + Core Routing Package

### Task 56.0 — Reconcile model IDs (PREREQUISITE)

**WARNING:** This touches files the other agent may be editing.
Coordinate before applying.

**Files:**
- `internal/llm/client.go` (PresetModels — replace all entries)
- `internal/llm/routing.go` (PricePerMTokensUSD + tierByPreset)
- `internal/loop/pricing.go` (add missing models)

**Deliverable:** All three tables use canonical IDs. New families added.

**Canonical model list** (28 models, 12 families):

```
# Claude (3 tiers)
claude-opus-4-8         top     anthropic/claude-opus-4-8
claude-sonnet-4-6       mid     anthropic/claude-sonnet-4-6
claude-haiku-4-5        budget  anthropic/claude-haiku-4-5

# OpenAI (3 tiers + codex)
gpt-5                   top     openai/gpt-5
gpt-5-mini              mid     openai/gpt-5-mini
gpt-5-nano              budget  openai/gpt-5-nano
gpt-5-codex             code    openai/gpt-5-codex

# Gemini (2 tiers)
gemini-2.5-pro          top     google/gemini-2.5-pro
gemini-2.5-flash        mid     google/gemini-2.5-flash

# Xiaomi (2 tiers)
mimo-v2.5-pro           top     xiaomi/mimo-v2.5-pro
mimo-v2.5-lite          budget  xiaomi/mimo-v2.5-lite

# DeepSeek (3 models)
deepseek-v4-pro         top     deepseek/deepseek-v4-pro
deepseek-v4-flash       mid     deepseek/deepseek-v4-flash
deepseek-r1             reason  deepseek/deepseek-r1

# Mistral (2 tiers)
mistral-large-2         top     mistral-large-latest (native)
codestral-22b           mid     codestral-latest (native)

# GLM/Z.AI (2 tiers)
glm-5.2                 top     zhipuai/glm-5.2
glm-5.2-air             mid     zhipuai/glm-5.2-air

# Kimi (2 tiers)
kimi-k2                 top     moonshot/kimi-k2
kimi-k2-flash           budget  moonshot/kimi-k2-flash

# MiniMax (3 models)
minimax-m1              top     minimax/minimax-m1
minimax-text-01         mid     minimax/minimax-text-01
abab-7                  budget  minimax/abab-7

# Qwen (2 tiers)
qwen-3-coder-plus       top     qwen/qwen-3-coder-plus
qwen-2.5-coder-plus     budget  qwen/qwen-2.5-coder-plus

# Groq (2 tiers)
groq-llama-3.3-70b      top     llama-3.3-70b-versatile (native)
groq-llama-3.3-8b       budget  llama-3.3-8b-versatile (native)
```

**Acceptance:**
- `ListPresets()` returns all 28 models
- `PriceFor("claude-opus-4-8")` returns a price
- `PriceFor("glm-5.2")` returns a price
- `CostUSD("claude-opus-4-8", 1000, 500)` > 0
- `CostUSD("kimi-k2", 1000, 500)` > 0
- No duplicate IDs across the three tables
- `AutoRoute("claude-sonnet-4-6", PhaseResearch)` returns "claude-opus-4-8"
- All existing tests pass (update model IDs in test fixtures)

### Task 56.1 — Package skeleton + core types

**Files:** `internal/routing/routing.go`

### Task 56.2 — Tier matrix

**Files:** `internal/routing/matrix.go`, `matrix_test.go`

`FamilyTiers` map for all 12 families. Phase->tier for all 7 phases.

### Task 56.3 — Capability detection

**Files:** `internal/routing/capability.go`, `capability_test.go`

### Task 56.4 — Resolver

**Files:** `internal/routing/resolver.go`, `resolver_test.go`

### Task 56.5 — Emitter (5 strategies)

**Files:** `internal/routing/emitter.go`, `emitter_test.go`

### Sprint 56 exit criteria

- [ ] `go build ./...` clean
- [ ] `go test ./... -count=1 -race` all green
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` clean
- [ ] 28 models in all three tables
- [ ] `internal/routing/` standalone, no existing test broken

---

## Sprint 57 — Integration

### Task 57.1 — AutoRoute wrapper

`llm.AutoRoute` delegates to `routing.Resolve` via function pointer
(no import cycle).

### Task 57.2 — Loop runner auto-resolve

When `RunConfig.ExecutorModel` is empty, runner uses resolver to pick
executor, verifier, and planner from the anchor.

### Task 57.3 — Boot manifest routing section

### Task 57.4 — Scaffold init hint

### Task 57.5 — CLI: `radiant models route`

---

## Sprint 58 — Polish + Release

### Task 58.1 — E2E integration test
### Task 58.2 — Routing override file (.radiant-harness/routing.yaml)
### Task 58.3 — README + docs finalize
### Task 58.4 — Version bump v2.1.0 + release

---

## Risk Register

| Risk | L | I | Mitigation |
|------|---|---|------------|
| Task 56.0 conflicts with other agent | H | H | Coordinate; do 56.0 only when other agent is not editing llm/*.go |
| Import cycle routing <-> llm | M | H | routing receives familyLookup func pointer |
| OpenRouter IDs differ from native | H | M | PresetModels maps canonical -> OpenRouter path |
| 7111-line main.go | H | L | Self-contained subcommand block; no main.go refactor |
| Multi-agent detection ambiguity | H | L | Priority order; --agent= overrides; --dry-run shows all |

---

## File Impact

| File | Sprint | Change |
|------|--------|--------|
| internal/routing/*.go | 56 | NEW (6 files + tests) |
| internal/llm/client.go | 56.0 | MODIFY (PresetModels) |
| internal/llm/routing.go | 56.0/57 | MODIFY (prices, AutoRoute wrapper) |
| internal/loop/pricing.go | 56.0 | MODIFY (add models) |
| internal/loop/runner.go | 57 | MODIFY (auto-resolve) |
| internal/boot/boot.go | 57 | MODIFY (routing section) |
| internal/scaffold/scaffold.go | 57 | MODIFY (init hint) |
| cmd/radiant/main.go | 57 | MODIFY (models route command) |
| internal/routing/e2e_test.go | 58 | NEW |
| internal/routing/override.go | 58 | NEW |
| README.md | 58 | MODIFY |
| docs/ROADMAP-V2.md | 58 | MODIFY |

**New:** 14 files | **Modified:** 9 files | **New package:** internal/routing/
