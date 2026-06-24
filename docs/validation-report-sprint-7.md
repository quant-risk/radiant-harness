# Radiant Harness — Sprint 7 Validation Report

Date: 2026-06-24
Commit: `f20e94e`
Version: `0.3.3`

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files |
| `go test ./... -race -count=1` | ✓ all 6 packages pass |
| Test count | ✓ 172 passing (up from 168 in 0.3.2) |
| Race conditions | ✓ zero (4-writer × 500-iteration stress on `currentTaskID`; 50-goroutine × 100-iteration stress on trace log + token accounting) |

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
console executables. Version string `f20e94e` is embedded via
`-ldflags "-X main.version=$(git describe --always)"`.

## Smoke Tests

| Check | Result |
|-------|--------|
| `./bin/radiant --version` | ✓ `0.3.3` |
| `./bin/radiant init .tmp-validation --all --yes` | ✓ scaffolded SDD layout |
| `./bin/radiant validate .tmp-validation` | ✓ Audit OK, Fidelity OK |
| `./bin/radiant doctor` | ✓ all checks pass; reports `radiant v0.3.3` |

## Sprint 7 Deliverables — Acceptance Criteria

### 7.1 — Race fix on `Engine.currentTaskID`

| Criterion | Result |
|-----------|--------|
| Read at `engine.go:308` locked under `e.mu` | ✓ |
| `TestCurrentTaskIDLockedRead` exercises 4-writer + locked-reader pattern | ✓ |
| Race detector silent under `-race -count=1` | ✓ |
| No regressions in other tests | ✓ 172 passing |

### 7.2 — `runPlannerAdvisory` (planner LLM actually fires)

| Criterion | Result |
|-----------|--------|
| `chatPlanner` called at least once during `Run` when `plannerModelName != ""` | ✓ — see `engine.go:178` |
| Output parsed into `Result.Warnings` (advisory, never blocking) | ✓ |
| Failure on planner call is non-fatal; run continues | ✓ — see `runPlannerAdvisory` error path |
| Planner event tagged `phase: "planner"` in trace | ✓ |
| Surfaces in post-run summary under `⚠ Planner raised N concern(s)` | ✓ — see `cmd/radiant/main.go:267-273` |
| Falls back to no-op when `plannerModelName == ""` (single-model users) | ✓ — `engine.go:171` guards the call |

### 7.3 — JSONL trace export (`--trace-out`)

| Criterion | Result |
|-----------|--------|
| `Engine.WriteTraceJSONL(io.Writer) error` implemented | ✓ |
| Atomic write via temp + fsync + rename in `writeTraceToFile` | ✓ |
| Failure to write is non-fatal; run still completes | ✓ — see `cmd/radiant/main.go:260-269` |
| `--trace-out <file>` flag registered | ✓ — see `run --help` |
| `TestWriteTraceJSONL` round-trips 2 events via `bytes.Buffer` | ✓ |
| `TestWriteTraceJSONLEmpty` confirms zero-byte output on empty trace | ✓ |
| JSON shape matches documented schema (`type`, `phase`, `task_id`, `model`, `input_tokens`, `output_tokens`, `latency_ms`, `ok`, `detail`) | ✓ |

### 7.4 — 6-target cross-compile

| Criterion | Result |
|-----------|--------|
| `linux/arm64` target added to Makefile `release` | ✓ |
| `windows/arm64` target added to Makefile `release` | ✓ |
| Each target documented in comment block | ✓ |
| All 6 binaries verified via `file(1)` for correct architecture | ✓ |
| Existing 4 targets still build (no regressions) | ✓ |

## Coverage

| Package | Coverage | Δ from 0.3.2 |
|---------|----------|--------------|
| `internal/benchmark` | 77% | unchanged |
| `internal/engine` | 45.5% | **+1.5pp** (race + JSONL tests) |
| `internal/harness` | 61.1% | unchanged |
| `internal/llm` | 84.3% | unchanged |
| `internal/quality` | 59.5% | unchanged |
| `internal/spec` | 88.5% | unchanged |

## Known Limitations Carried Forward

These remain on the backlog for future sprints; none are regressions:

- Output cap on gate commands (`io.LimitReader`) — **Sprint 8 candidate**
- Gate command allowlist duplicated in 3 files — drift risk
- `internal/scaffold/` has no tests
- Spec marketplace / shared registry — large product surface
- OS-level sandbox for gates (containers/gVisor) — needs design decision
- OpenTelemetry exporter (in-process trace exists; just needs OTLP sink)

## Git State

```
f20e94e feat: sprint 7 — planner fires, JSONL trace, race fix, 6-target release
7fb5262 feat: sprint 6 — multi-agent routing + tracing + CodeLens
653c51e feat: sprint 5 — Anthropic native client + eval suite + project move
313a591 feat: sprint 4 — cost display, rate-limit awareness, package manifests
a505b87 feat: sprint 3 — real cross-platform builds + auto model routing
```

Working tree clean. `0.3.3` embedded in every release binary.