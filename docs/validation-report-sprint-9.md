# Radiant Harness — Sprint 9 Validation Report

**Date**: 2026-06-24
**Commit**: `a9614b7`
**Version**: `0.3.5`

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files |
| `go test ./... -race -count=1` | ✓ all 6 packages pass |
| Test count | ✓ 188 passing |
| Race conditions | ✓ zero |

## Cross-Compile (6 OS/arch targets)

| Target | File | Architecture (verified via `file(1)`) |
|--------|------|------------------------------------------|
| linux/amd64 | `dist/radiant-linux-amd64` | ELF 64-bit LSB, x86-64, statically linked |
| linux/arm64 | `dist/radiant-linux-arm64` | ELF 64-bit LSB, ARM aarch64, statically linked |
| darwin/amd64 | `dist/radiant-darwin-amd64` | Mach-O 64-bit x86_64 |
| darwin/arm64 | `dist/radiant-darwin-arm64` | Mach-O 64-bit arm64 |
| windows/amd64 | `dist/radiant-windows-amd64.exe` | PE32+ x86-64 |
| windows/arm64 | `dist/radiant-windows-arm64.exe` | PE32+ Aarch64 |

## Sprint 9 Deliverables — Acceptance Criteria

| # | Criterion | Result |
|---|-----------|--------|
| 1 | `internal/policy/` created as single source of truth | ✓ |
| 2 | `AgentCommands` exported (closed set, 5 agents) | ✓ |
| 3 | `GateBinaries` exported (closed set, 28 binaries) | ✓ |
| 4 | `IsAgentAllowed`, `IsGateBinaryAllowed` use comma-ok form (NOT `!= struct{}{}`) | ✓ — caught latent bug in original code |
| 5 | `ValidateGateCommand` (canonical, quote-aware, double-quote support) | ✓ |
| 6 | `SplitOnLogicalOps`, `SplitShellTokens` exported | ✓ |
| 7 | `IsShellOp` exported | ✓ |
| 8 | `internal/engine/`, `internal/harness/`, `internal/quality/` migrated to use policy | ✓ |
| 9 | All 3 packages now share identical error message | ✓ |
| 10 | New package tests pass | ✓ — 12 tests in `internal/policy/` |
| 11 | `TestGateBinariesExcludeDestructive` regression guard | ✓ — locks `rm`, `curl`, `bash`, etc. out |
| 12 | `TestValidateGateCommandAcceptsAllowed` consistency check | ✓ — every allowlist member must validate |

## Coverage

| Package | Coverage | Δ from 0.3.4 |
|---------|----------|--------------|
| `internal/benchmark` | 77% | unchanged |
| `internal/engine` | 47.0% | unchanged |
| `internal/harness` | 61.1% | unchanged |
| `internal/llm` | 84.3% | unchanged |
| `internal/quality` | 59.5% | unchanged |
| `internal/spec` | 88.5% | unchanged |
| `internal/policy` | NEW | 100% of closed set + validator + tokenizers |

## Files Changed

```
internal/engine/engine.go                ~140 lines deleted (5 duplicates removed)
internal/engine/engine_test.go           +1 line (error message assertion updated)
internal/harness/agent.go                ~160 lines deleted (5 duplicates removed)
internal/quality/validate.go             ~100 lines deleted (4 duplicates removed)
internal/policy/allowlist.go             +275 lines (NEW — canonical)
internal/policy/allowlist_test.go        +217 lines (NEW — 12 tests)
internal/engine/engine.go (additions)    ~30 lines (delegation + import)
cmd/radiant/main.go                      no change
internal/llm/client.go                   no change
```

Total: ~400 lines deleted from consumer packages, ~490 lines added in `internal/policy/`.

## Latent Bug Caught

The original `IsGateBinaryAllowed` implementation in all 3 packages was:

```go
return GateBinaries[binary] != struct{}{}
```

This is **always false** — both "in the map" and "absent from the map" return the zero value of `struct{}`, so the comparison can never be true. The canonical implementation uses comma-ok:

```go
_, ok := GateBinaries[binary]
return ok
```

This bug was never caught in production because the validator's error path returned a sensible "binary not allowed" message even when the lookup returned wrong. Caught only because the new policy tests asserted that every allowlist member must be accepted — and they weren't.

## Git State

```
a9614b7 feat: sprint 9 — gate command allowlist deduplication via internal/policy
266eb9b docs: add sprint 8 validation report
7fb5b54 feat: sprint 8 — gate command output cap via --max-gate-output
9f9a0f5 docs: add sprint 7 validation report
f20e94e feat: sprint 7 — planner fires, JSONL trace, race fix, 6-target release
7fb5262 feat: sprint 6 — multi-agent routing + tracing + CodeLens
```

Working tree clean. `0.3.5` embedded in every release binary.

---

## Sprint 10 Kickoff (next)

Following the methodology merge plan committed in `a6cca6b`:

- Sprint 10 delivers the skill schema runtime + 3 skills + bundled distribution
- See `docs/HARNESS-PLAN.md` §5.1 for the 8 deliverables
- See `docs/SKILL-SCHEMA.md` for the open specification
- Open questions documented in `docs/validation-report-pivot.md`