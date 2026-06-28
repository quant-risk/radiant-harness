# Sprint 56–57 — Model Routing Engine Validation Report

> **Date:** 2026-06-27
> **Versions:** Sprint 56 (core engine) + Sprint 57 (integration)
> **Test suite:** 20/20 packages green with `-race -count=1`

---

## Sprint 56 — Core Engine (internal/routing/)

### Delivered

| Component | File | Status |
|-----------|------|--------|
| Core types | routing.go | ✅ Strategy, Phase, Tier, RoutingPlan, PhaseRouting |
| Tier matrix | matrix.go | ✅ 12 families × 3 tiers, 7 phases mapped |
| Agent detection | capability.go | ✅ 10 detection rules with priority |
| Resolver | resolver.go | ✅ anchor → per-phase models |
| Emitter (5 strategies) | emitter.go | ✅ subagent_delegation, delegate_task, config_per_role, advisory, direct_api |
| Tests | *_test.go | ✅ 40+ table-driven tests |

### Model ID Reconciliation (Task 56.0)

Three previously out-of-sync tables now share canonical IDs:

| Table | Before | After |
|-------|--------|-------|
| PresetModels (llm/client.go) | 14 models, stale IDs (opus-4.1) | **27 models**, canonical IDs (opus-4-8) |
| providerPricing (loop/pricing.go) | 11 models, mixed IDs | **27 models**, canonical IDs |
| PricePerMTokensUSD (llm/routing.go) | 10 models, stale IDs | **27 models**, canonical IDs |

**12 families supported:**
claude, openai, gemini, xiaomi, deepseek, mistral, glm, kimi, minimax, qwen, groq

**New models added** (previously missing):
claude-haiku-4-5, gpt-5-mini, gpt-5-nano, gemini-2.5-flash, mimo-v2.5-lite,
deepseek-r1, glm-5.2, glm-5.2-air, kimi-k2, kimi-k2-flash, minimax-m1,
minimax-text-01, abab-7, qwen-3-coder-plus, qwen-2.5-coder-plus, groq-llama-3.3-8b

---

## Sprint 57 — Integration

### Task 57.1 — `radiant models route` command

**File:** cmd/radiant/cmd_run.go

Smoke test output (Claude Code detection):
```
Detected agent: claude
Strategy:       subagent_delegation
Anchor model:   claude-sonnet-4-6
Family:         claude

PHASE          MODEL                    TIER     VIA
------------------------------------------------------------
research       claude-opus-4-8          top      subagent
plan           claude-opus-4-8          top      subagent
implement      claude-sonnet-4-6        mid      main
correct        claude-sonnet-4-6        mid      main
verify         claude-opus-4-8          top      subagent
persist        claude-haiku-4-5         budget   main
summarize      claude-haiku-4-5         budget   subagent
```

Smoke test output (Codex CLI advisory):
```
research       gpt-5                    top      advisory
implement      gpt-5-mini               mid      advisory
verify         gpt-5                    top      advisory
summarize      gpt-5-nano               budget   advisory
```

Flags: `--agent`, `--anchor`, `--dry-run` (default), `--apply`, `--json`

### Task 57.2 — Boot manifest routing section

**File:** internal/boot/boot.go

Manifest now includes `routing` field populated via routing.DetectAgent +
routing.Resolve. Verified via `radiant boot --json` output.

### Task 57.3 — Loop runner auto-resolve

**File:** internal/loop/runner.go

When `ExecutorModel` is empty, defaults to `claude-sonnet-4-6` (mid-tier
anchor). The existing `AutoRoute` flag (added by other agent) already
derives verifier and planner from the anchor via the tier matrix.

---

## Test Results

```
20/20 packages green with -race -count=1
go build ./...   — clean
go vet ./...     — clean
gofmt            — clean (except internal/schedule/schedule.go, other agent's WIP)
```

---

## What Remains (Sprint 58 candidates)

- [ ] Routing override file (`.radiant-harness/routing.yaml`) for user
      custom tier mappings
- [ ] `radiant init` hint message after scaffolding
- [ ] README section documenting the feature
- [ ] ROADMAP-V2.md update with sprint entries
- [ ] Version bump to v2.16.0+ (project is at v2.15.0 from Sprint 67)
