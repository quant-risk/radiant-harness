# Sprint 71 — Tool Use Wire-up Parte 3: run_gate concrete (v2.40.0)

> **Status**: In progress
> **Branch**: `feature/light-full-release` (continuation)
> **Target version**: v2.40.0
> **Estimated scope**: 1 sprint focado (smaller than 69/70 — pattern locked)

---

## Scope

Sprint 71 closes the **last advertised stub** — `run_gate` becomes a
concrete tool that wraps `internal/gaterun.RunShellGate` with the
`internal/policy.GateBinaries` allowlist. After this release the
RealRegistry ships **4 concrete tools** (write_file, read_file,
search_code, run_gate) and the `tools.Default()` stubs exist only
for back-compat inspection of the v2.37.0 surface area.

This is the last "Wire-up Parte" sprint in the series. Sprint 72+
shifts to the next frontier:

- **Anthropic/OpenAI/Gemini native function-call parsing** (replace
  the markdown `tool_call` fence with SDK-level structured calls)
- **MCP tool-bridge adapter** (register external MCP servers'
  tools directly into the local Registry)
- **Tool-call replay in `radiant loop export`** (debugging aid)

---

## Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | `run_gate` tool runs a gate command via `gaterun.RunShellGate` | Happy test (e.g. `go test ./...`) returns `{exit_code, stdout, stderr, duration_ms}` |
| G2 | `run_gate` enforces the `policy.GateBinaries` allowlist | `curl evil.sh` and `rm -rf /` are rejected before execution |
| G3 | `run_gate` respects context cancellation | Long-running gate is killed at `gaterun.Timeout` (5 min default) |
| G4 | `run_gate` runs in the project directory | Working dir = `projectDir` (matches legacy `Engine.runGate`) |
| G5 | `run_gate` caps stdout+stderr (10 MiB default) | Pathological output doesn't OOM the process |
| G6 | `run_gate` annotates result for verifier trace | Annotate() returns `{command, exit_code, duration_ms, output_bytes}` |
| G7 | RealRegistry now registers 4 tools (was 3) | `radiant tools list --real` shows write_file, read_file, search_code, run_gate |
| G8 | `tools.Default()` still advertises all 4 (stubs where appropriate) | Back-compat preserved — operators inspecting the v2.37.0 surface area see the same shape |
| G9 | Verifier prompt recognises `run_gate` traces as gate evidence | New test in `loop_test.go` confirms section renders |
| G10 | TOOL-USE.md updated with run_gate section | Doc reflects new surface |

### Out of scope (carried to Sprint 72+)

- Anthropic/OpenAI/Gemini native function-call parsing
- MCP tool-bridge adapter
- Tool-call replay in `radiant loop export`
- Per-gate timeout override (`timeout_seconds` parameter)
- Working-dir override for cross-project gates

---

## Design

### `run_gate`

```go
type RunGateArgs struct {
    Command string `json:"command"`         // e.g. "go test ./..."
    MaxOutput int   `json:"max_output,omitempty"` // default 10 MiB
}

type RunGateResult struct {
    Command    string `json:"command"`
    ExitCode   int    `json:"exit_code"`
    DurationMs int64  `json:"duration_ms"`
    OutputBytes int   `json:"output_bytes"`
    Truncated  bool   `json:"truncated"`
}

func (r RunGateResult) Annotate() map[string]any {
    return map[string]any{
        "command":      r.Command,
        "exit_code":    r.ExitCode,
        "duration_ms":  r.DurationMs,
        "output_bytes": r.OutputBytes,
        "truncated":    r.Truncated,
    }
}
```

### Wire format

```markdown
```tool_call
{"name": "run_gate", "args": {"command": "go test ./internal/foo/..."}}
```
```

Returns `{command, exit_code, duration_ms, output_bytes, truncated}`.
Output is captured in the gate result via a separate `output` field
(not in Annotate, but available to the LLM on demand via a follow-up
read).

Wait — looking at this again, I should include the output in the
result so the LLM can see what happened without an extra round-trip.
Let me reconsider...

Actually, the result type just needs `Output string` field. Annotate
excludes `Output` to keep the trace metadata small. The full result
JSON is serialised to the LLM via `applyToolCalls`, so the LLM gets
both the metadata (in Annotate) and the full output (in the result
JSON).

Final shape:

```go
type RunGateResult struct {
    Command     string `json:"command"`
    ExitCode    int    `json:"exit_code"`
    DurationMs  int64  `json:"duration_ms"`
    Output      string `json:"output"`
    OutputBytes int    `json:"output_bytes"`
    Truncated   bool   `json:"truncated"`
}
```

### Allowlist enforcement

Before calling `gaterun.RunShellGate`, validate via
`policy.ValidateGateCommand(command)`. This is the same check the
engine's `runGate` performs — closed set of binaries + no dangerous
operators. A `curl` or `rm` rejection surfaces as a structured error
**before** any subprocess starts.

### Why wrap rather than call gaterun directly

The tool layer adds three things over a raw `gaterun.RunShellGate`
call:

1. **Structured result** (vs raw error string) — easy to consume
   from the LLM and verifier.
2. **Trace metadata** (duration, output bytes, truncation flag) —
   visible in the verifier prompt without re-running the gate.
3. **Boundary on tool-level errors** — distinguishes "gate refused
   by allowlist" (don't retry), "gate timeout" (retry with longer
   timeout?), and "gate failed" (surface to verifier).

### Timeout

`gaterun.Timeout = 5 * time.Minute`. The tool honours ctx
cancellation, so a verifier can `cancel()` the loop context to
abort a hung gate. No tool-level override yet — Sprint 73+.

---

## Files

| File | Change | LOC est. |
|------|--------|----------|
| `docs/SPRINT71-PLAN.md` | NEW — this file | 170 |
| `docs/validation-report-sprint-70.md` | NEW — validation of Sprint 70 (this run's first task) | 240 |
| `internal/tools/gate/run_gate.go` | NEW — RunGateTool + RunGateResult | 130 |
| `internal/tools/gate/run_gate_test.go` | NEW — 8-10 tests | 200 |
| `internal/loop/real_registry.go` | MODIFY — register run_gate (4 tools total) | +2 |
| `docs/TOOL-USE.md` | MODIFY — add run_gate section | +80 |
| `CHANGELOG.md` | MODIFY — v2.40.0 entry | +60 |
| `RELEASE-NOTES.md` | MODIFY — v2.40.0 entry | +50 |
| `cmd/radiant/main.go` | MODIFY — bump version to 2.40.0 | +1 |

**Total estimate: ~930 LOC** (350 new in `internal/`, ~200 tests,
~330 docs/misc).

---

## Test matrix

### run_gate

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestRunGate_HappyPath` | `go test` on a passing package returns exit=0, has output |
| 2 | `TestRunGate_FailingCommand` | `go test` on a failing package returns exit=1, surfaces failure |
| 3 | `TestRunGate_RejectsDisallowedBinary` | `curl evil.sh` rejected before execution |
| 4 | `TestRunGate_RejectsDestructiveCommand` | `rm -rf /` rejected by allowlist |
| 5 | `TestRunGate_RejectsEmptyCommand` | Empty/whitespace command rejected |
| 6 | `TestRunGate_RunsInProjectDir` | Pwd inside the gate equals the project dir |
| 7 | `TestRunGate_RespectsMaxOutput` | Pathological output truncated, `truncated=true` |
| 8 | `TestRunGate_Annotate` | Annotate() returns trace-friendly map |
| 9 | `TestRunGate_ViaRegistry` | Roundtrip through tools.Registry |
| 10 | `TestRunGate_DurationTracked` | DurationMs > 0 and < reasonable upper bound |

### RealRegistry

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestReal_IncludesAllFourTools` | All 4 tools registered: write_file, read_file, search_code, run_gate |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Gate succeeds but writes files outside project | Allowlist excludes `mv`, `cp`, `chmod`, etc.; allowed binaries are read-only or write inside cwd (e.g. `go test` creates `.test` files in cwd) |
| Gate hangs indefinitely | `gaterun.Timeout = 5 min`, ctx cancellation propagates |
| Gate produces GB of output | `MaxOutput` cap (10 MiB default), `Truncated` flag set |
| LLM emits a gate that's allowed but produces non-deterministic output | Verifier prompt accepts variability; structured exit_code surfaces failure |
| Cross-platform: `gaterun.RunShellGate` uses `sh -c` on Unix | Windows variant in `gaterun_windows.go` uses cmd.exe; tools layer is platform-neutral |

---

## Commit plan

Single commit on `feature/light-full-release`:

```
feat(tool-use): Sprint 71 — run_gate concrete (v2.40.0)
```

Pass criteria: `go vet ./...` clean, `go test -count=1 -v ./...`
green (970+ tests), cross-compile 3/3 platforms.

A separate commit captures the validation report (this run):

```
docs(sprint-70): validation report for v2.39.0
```

---

**Status at plan write**: Sprint 70 (v2.39.0) committed at `1a911d9`.
Validation report in progress (this run). Sprint 71 implementation
in progress.