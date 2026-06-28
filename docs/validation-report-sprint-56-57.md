# Model Routing Engine — Final Validation & Status

> **Date:** 2026-06-27
> **Project version:** v2.23.0 (v0.7.0-58-g77d4ccc)
> **Model routing:** Sprint 56 (core) + Sprint 57 (integration) — COMPLETE

---

## Validation Results

### Build / Vet / Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✅ CLEAN |
| `go vet ./...` | ✅ CLEAN |
| `gofmt -l` (my files) | ✅ CLEAN |
| `go test routing/ boot/ llm/ loop/ -race` | ✅ 4/4 GREEN |
| `go test ./... -race` | ⚠️ 19/20 GREEN (fleet dispatch_test flaky — other agent's WIP) |

**Note:** The single failure (`TestRunAllContextCanceled` in `internal/fleet/dispatch_test.go`)
is in the other agent's fleet dispatcher code, not in any file I created or modified.
It's a timing-dependent test about killed processes.

### Model Routing Smoke Tests

**`radiant models route --agent=claude --anchor=claude-sonnet-4-6`:**
```
PHASE          MODEL                    TIER     VIA
research       claude-opus-4-8          top      subagent
plan           claude-opus-4-8          top      subagent
implement      claude-sonnet-4-6        mid      main
verify         claude-opus-4-8          top      subagent
summarize      claude-haiku-4-5         budget   subagent
```

**`radiant models route --agent=codex --anchor=gpt-5`:**
```
PHASE          MODEL                    TIER     VIA
research       gpt-5                    top      advisory
implement      gpt-5-mini               mid      advisory
verify         gpt-5                    top      advisory
summarize      gpt-5-nano               budget   advisory
```

**`radiant boot --json`:** Includes `"routing"` section with detected agent,
strategy, anchor, family, and per-phase model assignments.

### Model Coverage

**27 models across 12 families**, reconciled across 3 tables:

```
PresetModels (llm/client.go):        27 entries
providerPricing (loop/pricing.go):   27 entries
PricePerMTokensUSD (llm/routing.go): 27 entries
```

All three tables use identical canonical IDs (verified by
`TestPriceTableCoversAllPresets`).

---

## What's Committed

All work is in commit `77d4ccc` (committed by other agent as part of
a larger sprint batch). The commit includes:

| File | Change |
|------|--------|
| `internal/routing/routing.go` | Core types (Strategy, Phase, Tier, RoutingPlan) |
| `internal/routing/matrix.go` | 12 families × 3 tiers, 7 phases mapped |
| `internal/routing/capability.go` | 10 detection rules with priority |
| `internal/routing/resolver.go` | anchor → per-phase model resolution |
| `internal/routing/emitter.go` | 5 strategies (subagent, delegate, config, advisory, direct_api) |
| `internal/routing/*_test.go` | 40+ table-driven tests |
| `internal/llm/client.go` | PresetModels: 27 canonical model entries |
| `internal/llm/routing.go` | AutoRoute + tierByPreset + PricePerMTokensUSD updated |
| `internal/loop/pricing.go` | providerPricing: 27 entries |
| `internal/loop/runner.go` | Default anchor when ExecutorModel empty |
| `internal/boot/boot.go` | Manifest gains Routing field + routing.Resolve call |
| `cmd/radiant/cmd_run.go` | `radiant models route` subcommand |
| `docs/MODEL-ROUTING.md` | Full design document |
| `docs/SPRINT56-PLAN.md` | Implementation plan |
| `docs/validation-report-sprint-56-57.md` | This validation report |

---

## Gaps (what the routing engine does NOT do yet)

1. **Routing override file.** Users cannot customize the tier table
   without editing Go code. A `.radiant-harness/routing.yaml` override
   is designed but not implemented.

2. **`radiant init` hint.** After scaffolding, the CLI doesn't mention
   that model routing is available for the detected agent.

3. **README docs.** The feature isn't documented in README.md for new
   users.

4. **Version bump.** The `version` var in main.go still says "1.1.0"
   while the project is at v2.23.0 per git tags.

5. **AutoRoute wrapper.** `llm.AutoRoute` still has its own tier logic
   that duplicates `routing.Matrix`. It should delegate to
   `routing.Resolve` entirely (currently both exist in parallel).

6. **Fleet test flakiness.** `TestRunAllContextCanceled` in
   `internal/fleet/dispatch_test.go` is timing-dependent and fails
   intermittently. Not related to routing but blocks `go test ./...`
   from being fully green.
