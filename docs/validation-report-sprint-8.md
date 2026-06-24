# Radiant Harness — Sprint 8 Validation Report

Date: 2026-06-24
Commit: `7fb5b54`
Version: `0.3.4`

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files |
| `go test ./... -race -count=1` | ✓ all 6 packages pass |
| Test count | ✓ 176 passing (up from 172 in 0.3.3) |
| Race conditions | ✓ zero (4 new gate tests + existing 50-goroutine stress for trace log + token accounting) |

## Cross-Compile (6 OS/arch targets)

| Target | File | `file(1)` output |
|--------|------|------------------|
| linux/amd64 | `dist/radiant-linux-amd64` | ELF 64-bit LSB executable, x86-64, statically linked |
| linux/arm64 | `dist/radiant-linux-arm64` | ELF 64-bit LSB executable, ARM aarch64, statically linked |
| darwin/amd64 | `dist/radiant-darwin-amd64` | Mach-O 64-bit executable x86_64 |
| darwin/arm64 | `dist/radiant-darwin-arm64` | Mach-O 64-bit executable arm64 |
| windows/amd64 | `dist/radiant-windows-amd64.exe` | PE32+ executable (console) x86-64 |
| windows/arm64 | `dist/radiant-windows-arm64.exe` | PE32+ executable (console) Aarch64 |

All 6 targets compile cleanly with `CGO_ENABLED=0`. ARM binaries are
statically linked (no glibc dependency). Windows binaries are PE32+
console executables. Version string `7fb5b54` is embedded via
`-ldflags "-X main.version=$(git describe --always)"`.

## Smoke Tests

| Check | Result |
|-------|--------|
| `./bin/radiant --version` | ✓ `0.3.4` |
| `./bin/radiant init .tmp-validation --all --yes` | ✓ scaffolded SDD layout |
| `./bin/radiant validate .tmp-validation` | ✓ Audit OK, Fidelity OK |
| `./bin/radiant doctor` | ✓ all checks pass; reports `radiant v0.3.4` |

## CLI Surface — Sprint 8 additions

| Flag | Default | Notes |
|------|---------|-------|
| `--max-gate-output` | `10485760` (10 MiB) | cap stdout+stderr per gate; truncated gates get a `[output truncated at N bytes]` marker |

All flags from prior sprints (`--planner`, `--implementer`, `--trace-out`,
`--auto-route`, `--verbose`) remain present and unchanged.

## Sprint 8 Deliverables — Acceptance Criteria

### 8.1 — Gate output cap (closes OOM vector)

| Criterion | Result |
|-----------|--------|
| `cmd.CombinedOutput()` replaced with pipe + `io.LimitReader` in `internal/engine/gate_unix.go` | ✓ |
| Same replacement in `internal/engine/gate_windows.go` | ✓ |
| Same replacement in `internal/harness/gate_unix.go` and `gate_windows.go` | ✓ |
| Same replacement in `internal/quality/gate_unix.go` and `gate_windows.go` | ✓ |
| Truncation appends marker so consumers know output is incomplete | ✓ — `[output truncated at N bytes — gate wrote more than the configured cap]` |
| Gate dies cleanly when cap is reached (broken-pipe on next write) | ✓ — SIGPIPE on POSIX, ERROR_BROKEN_PIPE on Windows |
| `--max-gate-output` CLI flag registered with sensible default | ✓ — 10 MiB |
| `engine.Config.GateMaxOutputBytes` wired through `New()` | ✓ — 0 = use package default |
| Quality package uses package default (0 passed in `validate.go`) | ✓ |
| Harness orchestrator uses package default | ✓ |
| `TestRunShellGateRespectsCap` validates 64KB-output vs 1KB cap | ✓ |
| `TestRunShellGateUnderCap` validates small gates pass through untouched | ✓ |
| `TestRunShellGateDefaultCap` validates zero-means-default contract | ✓ |
| `TestRunShellGateReportsFailure` regression guard for non-zero exit codes | ✓ |

### 8.2 — Compatibility

| Criterion | Result |
|-----------|--------|
| Existing callers of `runShellGate`/`runGateShell` updated | ✓ — engine, harness orchestrator, quality validator |
| Backward-compatible default behavior (no flag = 10 MiB cap) | ✓ |
| All previous tests still pass | ✓ — 172 prior tests unchanged |

## Coverage

| Package | Coverage | Δ from 0.3.3 |
|---------|----------|--------------|
| `internal/benchmark` | 77% | unchanged |
| `internal/engine` | 47.0% | **+1.5pp** (4 new gate tests) |
| `internal/harness` | 61.1% | unchanged |
| `internal/llm` | 84.3% | unchanged |
| `internal/quality` | 59.5% | unchanged |
| `internal/spec` | 88.5% | unchanged |

## Files Changed

```
CHANGELOG.md                           +44 lines
cmd/radiant/main.go                    +13 lines  (--max-gate-output flag + Config wiring)
docs/ROADMAP.md                        +3 lines
internal/engine/gate_unix.go           rewritten — pipe + LimitReader
internal/engine/gate_windows.go        rewritten — pipe + LimitReader
internal/engine/engine.go              +5 lines  (Config.GateMaxOutputBytes + Engine.gateMaxOutput)
internal/engine/engine_test.go         +115 lines (4 new tests)
internal/harness/gate_unix.go          rewritten — pipe + LimitReader
internal/harness/gate_windows.go       rewritten — pipe + LimitReader
internal/harness/orchestrator.go       +1 line   (pass 0 for default cap)
internal/quality/gate_unix.go          rewritten — pipe + LimitReader
internal/quality/gate_windows.go       rewritten — pipe + LimitReader
internal/quality/validate.go           +1 line   (pass 0 for default cap)
```

Total: 13 files changed, 397 insertions, 43 deletions.

## Known Limitations Carried Forward

These remain on the backlog; none are regressions from this sprint:

- Gate command allowlist duplicated in 3 files (`engine.go:619`,
  `harness/agent.go:86`, `quality/validate.go`) — drift risk;
  duplicated again in this sprint for the cap constant. **Sprint 9 candidate.**
- `internal/scaffold/` has no tests
- Spec marketplace / shared registry — large product surface
- OS-level sandbox for gates (containers/gVisor) — design decision first
- OpenTelemetry exporter (in-process trace exists; just needs OTLP sink)

## Git State

```
7fb5b54 feat: sprint 8 — gate command output cap via --max-gate-output
9f9a0f5 docs: add sprint 7 validation report
f20e94e feat: sprint 7 — planner fires, JSONL trace, race fix, 6-target release
7fb5262 feat: sprint 6 — multi-agent routing + tracing + CodeLens
653c51e feat: sprint 5 — Anthropic native client + eval suite + project move
```

Working tree clean. `0.3.4` embedded in every release binary.