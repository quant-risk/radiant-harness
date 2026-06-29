# Sprint 69 — Tool Use Wire-up Parte 1: fs_write concreto (v2.38.0)

> **Status**: In progress
> **Branch**: `feature/tool-use-wire-up` (to be created from `feature/light-full-release`)
> **Target version**: v2.38.0
> **Owner**: solo (radiant-harness dev)
> **Estimated scope**: 1 sprint focado

---

## Motivation

In v2.37.0 (Sprint 68), `internal/tools/` shipped as a **scaffold**: the
`Tool`, `Registry`, and `Call` machinery work and are tested, but every
built-in tool is a stub that returns "not yet wired". The executor in
`internal/engine/engine.go` still extracts code blocks from markdown
fences (` ```go ` blocks with `// File: <path>` headers) and writes
them via `os.WriteFile` after a `pathIsSafe` boundary check.

That's brittle:

1. **No argument validation at the boundary.** A malformed `// File:`
   path that passes the regex can still produce a write. Symlink
   detection partially mitigates but is reactive.
2. **No per-tool tracing.** Each write is invisible to the tracer —
   the loop sees only "wrote file X" at the end of the iteration.
3. **No retry semantics per failure.** If a write fails because the
   parent dir doesn't exist, the executor doesn't know to create it.
4. **Tool surface is not enumerable.** `radiant tools list` returns
   "unknown command" because there is no CLI for it.
5. **The verifier can't audit a structured trace** — it only sees a
   final `executor_output` blob and has to guess what happened.

Sprint 69 fixes the **first half** of this: make `write_file` (the
highest-value tool) actually work, wire it through `applyLLMResponse`,
and teach the verifier to surface tool-call traces in its prompt.
Sprints 70-71 close the rest (`read_file`, `search_code`, `run_gate`,
plus the MCP tool-bridge adapter).

---

## Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | `write_file` tool implements the actual filesystem write | Happy path test: writes content, creates parent dirs, returns `{written: <path>, bytes: <n>}` |
| G2 | `write_file` enforces the same `pathIsSafe` boundary check | Unsafe path test: rejects `../etc/passwd` and symlink escapes with structured error |
| G3 | Engine detects tool calls in LLM output and dispatches via Registry | Test: mixed output (tool call + code block) routes correctly; tool-only output skips the code-block path |
| G4 | Back-compat: code-block emission still works when LLM doesn't emit tool calls | Test: legacy fixture produces identical file outputs to v2.37.0 |
| G5 | `PathIsSafe` is exported from `internal/engine` so non-engine packages can reuse it without re-implementation | Test: `PathIsSafe` matches `pathIsSafe` for all 6 existing fixtures |
| G6 | Verifier prompt includes "Tool calls observed" section when present | Test: prompt string contains the section iff any tool calls occurred in the iteration |
| G7 | `radiant tools list` CLI shows registered tools with their schemas | Manual smoke test in validation report |
| G8 | Documentation: `docs/TOOL-USE.md` explains the new contract | Doc PR'd |

---

## Design

### 1. Tool call syntax

The LLM emits structured tool calls inside a fenced code block with
language tag `tool_call`:

````markdown
```tool_call
{"name": "write_file", "args": {"path": "internal/foo.go", "content": "package foo\n"}}
```
````

Why this format (not the standard JSON function-call protocol used by
Anthropic/OpenAI/Gemini APIs):

- **No SDK dependency.** The current `internal/llm/` package talks raw
  HTTP and doesn't parse function-call responses — adding that would
  be a Sprint 70 effort.
- **Symmetric with the legacy code-block parser.** Both code blocks
  and tool calls live inside markdown fences, so `extractToolCalls`
  is structurally identical to `extractCodeBlocks`. One regex pass
  finds both.
- **MCP-aligned at the registry level.** The `tools.Tool` struct
  mirrors MCP's tool definition (`name`, `description`, `inputSchema`),
  so when Sprint 70 adds the MCP bridge, the wire format changes but
  the registry doesn't.

Multiple tool calls per response are supported — emit one `tool_call`
fence per call. They execute in emission order.

### 2. fs_write implementation

`internal/tools/fs/fs.go` provides:

```go
// WriteFileTool returns the write_file tool wired to the given project dir.
// The tool's args are validated against the project boundary using
// engine.PathIsSafe (which resolves symlinks before checking).
func WriteFileTool(projectDir string) *tools.Tool

// Args passed by the LLM:
//   {
//     "path":    "<project-relative path>",
//     "content": "<full file contents>"
//   }
//
// Returns:
//   {
//     "written": "<resolved path>",
//     "bytes":   <int>,
//     "created": <bool>  // true if the file didn't exist before
//   }
```

Writes are **atomic** (temp file + fsync + rename, same pattern as
`cycle.go:persistLocked`). This prevents a partial write from leaving
the project in a half-updated state if the loop is killed mid-write.

### 3. Engine switch in `applyLLMResponse`

```go
func (e *Engine) applyLLMResponse(response string, specDir string) error {
    // New: try tool calls first.
    calls := extractToolCalls(response)
    if len(calls) > 0 {
        return e.applyToolCalls(calls)
    }
    // Legacy fallback: code blocks.
    return e.applyCodeBlocks(response)
}
```

If the LLM emits **both** tool calls and code blocks in the same
response, the tool calls win (executed) and the code blocks are
ignored with a debug log. This keeps the contract unambiguous: tool
calls are an alternative, not an addition.

The Engine gains an optional field:

```go
type Engine struct {
    // ...
    // ToolRegistry is the structured tool-use registry. When non-nil,
    // applyLLMResponse routes tool calls through this registry instead
    // of (or before) the legacy code-block emission path. nil = legacy
    // only (default in v2.37.0; opt-in for v2.38.0).
    ToolRegistry *tools.Registry
}
```

The CLI command `radiant run` populates `ToolRegistry` with
`tools.Default()` plus `fs.WriteFileTool(projectDir)`.

### 4. Verifier prompt addition

After Sprint 69, `BuildVerifierPrompt` takes an extra optional arg:

```go
func BuildVerifierPrompt(goal, executorOutput string, cfg VerifierConfig, toolTrace []ToolCallRecord) string
```

When `toolTrace` is non-empty, the prompt gains a section:

```
TOOL CALLS OBSERVED (in execution order):
1. write_file — internal/foo.go (1432 bytes, created=true)
2. write_file — internal/foo_test.go (892 bytes, created=true)

ANTI-CHEAT ADDENDUM:
- A tool call that wrote outside the project boundary is grounds for rejection
- A tool call whose result was an error is NOT grounds for rejection if the
  executor correctly surfaced the error and adjusted (verifier: check the trace)
```

When `toolTrace` is empty (legacy code-block path), the prompt is
unchanged. `ToolCallRecord` is exported from `internal/loop`.

### 5. `radiant tools list`

New CLI subcommand:

```
$ radiant tools list
NAME        DESCRIPTION                                                PARAMS
read_file   Read the contents of a file at the given path.             1
write_file  Write content to a file at the given path.                 2
search_code Search the project for a regex pattern.                    2
run_gate    Run a quality gate command (go test, go vet, etc.).        1
```

Implemented as `cmd/radiant/cmd_tools.go`, mirroring the style of
`cmd_semantic.go` (50-80 LOC).

---

## Files to add / modify

| File | Change | LOC estimate |
|------|--------|--------------|
| `docs/SPRINT69-PLAN.md` | NEW — this file | 230 |
| `docs/TOOL-USE.md` | NEW — operator guide | 180 |
| `internal/tools/fs/fs.go` | NEW — WriteFileTool implementation | 130 |
| `internal/tools/fs/fs_test.go` | NEW — happy + unsafe + atomic + size cap | 140 |
| `internal/tools/tools.go` | MODIFY — add `Real()` builder that uses `fs.WriteFileTool` | +20 |
| `internal/tools/tools_test.go` | MODIFY — add `TestReal_IncludesConcreteWriteFile` | +15 |
| `internal/engine/engine.go` | MODIFY — add `PathIsSafe` public wrapper, add `ToolRegistry` field, switch `applyLLMResponse` | +60 |
| `internal/engine/engine_test.go` | MODIFY — back-compat + tool-call-path tests | +80 |
| `internal/loop/toolcall.go` | NEW — `extractToolCalls`, `applyToolCalls`, `ToolCallRecord` type | 110 |
| `internal/loop/toolcall_test.go` | NEW — extractor + dispatcher tests | 130 |
| `internal/loop/verifier.go` | MODIFY — `BuildVerifierPrompt` accepts `toolTrace`, adds section | +35 |
| `internal/loop/loop_test.go` | MODIFY — verifier prompt contains tool trace test | +30 |
| `cmd/radiant/cmd_tools.go` | NEW — `radiant tools list` CLI | 70 |
| `cmd/radiant/main.go` | MODIFY — register `tools` subcommand | +3 |
| `cmd/radiant/cmd_run.go` | MODIFY — pass ToolRegistry to Engine | +5 |
| `README.md` | MODIFY — add Tool Use section | +20 |
| `CHANGELOG.md` | MODIFY — v2.38.0 entry | +50 |
| `RELEASE-NOTES.md` | MODIFY — v2.38.0 entry | +60 |

**Total estimate: ~1,370 LOC** (350 new in `internal/`, ~270 tests,
~110 CLI, ~340 docs, ~300 misc).

---

## Test matrix

| # | Test | What it asserts |
|---|------|-----------------|
| 1 | `TestWriteFile_HappyPath` | File is created with correct content, returns `{written, bytes, created:true}` |
| 2 | `TestWriteFile_CreatesParentDirs` | `internal/foo/bar.go` works when `internal/foo/` doesn't exist |
| 3 | `TestWriteFile_OverwritesExisting` | Returns `created:false`, content replaced atomically |
| 4 | `TestWriteFile_RejectsUnsafePath` | `../etc/passwd` → structured error, no write |
| 5 | `TestWriteFile_RejectsSymlinkEscape` | Symlink inside project pointing outside → rejected |
| 6 | `TestWriteFile_Atomic` | SIGKILL mid-write doesn't leave partial file |
| 7 | `TestExtractToolCalls_Single` | One `tool_call` fence → one ToolCall |
| 8 | `TestExtractToolCalls_Multiple` | Three fences → three ToolCalls in order |
| 9 | `TestExtractToolCalls_Malformed` | Missing name or args → skipped, not crashed |
| 10 | `TestApplyLLMResponse_ToolCall` | Mixed response → tool call path, code block ignored |
| 11 | `TestApplyLLMResponse_LegacyFallback` | Pure code block → code block path (identical to v2.37.0) |
| 12 | `TestApplyLLMResponse_PureToolCalls` | No code blocks → only tool calls executed |
| 13 | `TestBuildVerifierPrompt_IncludesToolTrace` | Non-empty trace → prompt contains "TOOL CALLS OBSERVED" |
| 14 | `TestBuildVerifierPrompt_NoTrace` | Empty trace → prompt identical to v2.37.0 |
| 15 | `TestPathIsSafe_PublicMatchesPrivate` | `PathIsSafe` == `pathIsSafe` for 6 fixtures |
| 16 | `TestReal_IncludesConcreteWriteFile` | `tools.Real()` registers a `write_file` whose Invoke is not the stub |

---

## Out of scope (carried to Sprint 70+)

| Item | Sprint |
|------|--------|
| `read_file` concrete impl | 70 |
| `search_code` concrete impl | 70 |
| `run_gate` concrete impl | 71 |
| MCP tool-bridge adapter (register MCP server tools into Registry) | 71 |
| Switch `internal/loop/runner.go` to also accept tool calls in the executor phase (currently only engine) | 71 |
| Anthropic/OpenAI/Gemini function-call native protocol parsing | 72 |
| Tool-call replay in `radiant loop export` (debugging aid) | 72 |
| Schema validation beyond JSON type-check (min/max length, regex, etc.) | 73 |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Tool calls introduce a new attack surface (e.g. write_file to sensitive paths) | Reuse `pathIsSafe` + allowlist (only project-relative paths); explicit `PathIsSafe` test |
| LLM emits both tool calls and code blocks in one response | Documented contract: tool calls win, code blocks ignored; tested |
| Verifier rejects tool-call traces it doesn't understand | New "ANTI-CHEAT ADDENDUM" section guides the verifier; empty trace path is unchanged |
| `internal/engine` depending on `internal/tools/fs` would be cyclic | `fs.WriteFileTool` is a factory function — engine holds a `*tools.Registry`, not a concrete dep |

---

## Commit plan

Three commits on `feature/tool-use-wire-up`:

1. `feat(tools-fs): concrete write_file implementation + PathIsSafe wrapper`
   — `internal/tools/fs/*` + `internal/engine/engine.go` wrapper + 16 tests
2. `feat(loop-toolcall): extract + dispatch tool calls from LLM output`
   — `internal/loop/toolcall.go` + engine switch + verifier prompt + 6 tests
3. `feat(cli-tools): radiant tools list + docs + v2.38.0 release notes`
   — `cmd/radiant/cmd_tools.go` + `docs/TOOL-USE.md` + README + CHANGELOG

Each commit must pass: `go vet`, `go test ./...`, cross-compile 3 platforms.

---

**Status at plan write**: Sprint 68 (v2.37.0) validated and committed
(`4c38b0a`). Sprint 69 implementation in progress.