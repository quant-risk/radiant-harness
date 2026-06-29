# Tool Use — Operator Guide (Sprint 69 / v2.38.0)

> **Status:** Shipped in v2.38.0 (Sprint 69)
> **Audience:** Operators using `radiant run` against LLM agents that emit
> structured tool calls instead of (or in addition to) code blocks.
> **Scope:** This guide covers the wire format, dispatcher, and operator
> controls. Tool authors adding new tools should read
> `internal/tools/tools.go` and the `annotator` interface comment in
> `internal/engine/engine.go`.

---

## TL;DR

In v2.38.0 the executor can route LLM-emitted tool calls through a
**typed registry** instead of relying solely on regex-extracted
code blocks. The first concrete tool, **`write_file`**, replaces
the legacy `os.WriteFile`-based code-block emission for any LLM
output that includes structured tool calls. Backwards compatibility
is preserved: pure code-block responses still work exactly as before.

```text
$ radiant tools list
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
run_gate        Run a quality gate command (go test, go vet, etc.). Retur... 1
read_file       Read the contents of a file at the given path. Path must ... 1
write_file      Write content to a file at the given path. Creates parent... 2
search_code     Search the project for a regex code pattern. Returns matc... 2

$ radiant tools list --real
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
write_file      Write content to a file at the given path (project-relati... 2
```

The `--real` flag shows the registry that v2.38.0 actually wires into
the executor. `--list` (default) shows the v2.37.0 stub registry for
back-compat inspection of the advertised surface area.

---

## Why structured tool calls

The legacy executor path parses LLM responses looking for markdown
code fences with `// File:` headers:

````markdown
```go
// File: internal/foo.go
package foo

func Bar() string { return "bar" }
```
````

This works but has limitations that compound with agent autonomy:

1. **No argument validation at the boundary.** A malformed `// File:`
   path that slips past the regex still produces a write.
2. **No per-tool tracing.** Each write is invisible to the tracer —
   the loop sees only "wrote file X" at the end of the iteration.
3. **No retry semantics per failure.** If a write fails because the
   parent dir doesn't exist, the executor doesn't know to create it
   inline.
4. **Tool surface not enumerable.** Operators couldn't inspect
   available tools before this release — `radiant tools list` didn't
   exist.
5. **The verifier can't audit a structured trace** — it only sees a
   final `executor_output` blob and has to guess what happened.

Structured tool calls address each of these by replacing the
regex-based code-block emission with a **typed, traceable,
re-playable dispatch table**.

---

## Wire format

The LLM emits a structured call inside a fenced markdown block with
language tag `tool_call`:

````markdown
```tool_call
{"name": "write_file", "args": {"path": "internal/foo.go", "content": "package foo\n"}}
```
````

The format was chosen because it:

- **Needs no SDK.** The current `internal/llm/` package talks raw
  HTTP and doesn't parse function-call responses; adding SDK-level
  parsing is a Sprint 72 effort.
- **Is symmetric with the legacy code-block parser.** Both live
  inside markdown fences, so `extractToolCalls` is structurally
  identical to `extractCodeBlocks`.
- **Is human-readable.** A developer can paste the same payload into
  a markdown file and see exactly what the LLM emitted.
- **Aligns with MCP at the registry level.** The `tools.Tool` struct
  mirrors MCP's tool definition (`name`, `description`, `inputSchema`),
  so when Sprint 71 adds the MCP bridge, the registry stays
  unchanged.

Multiple tool calls per response are supported. Emit one `tool_call`
fence per call. They execute in emission order.

### Behaviour matrix

| LLM output | Executor behaviour |
|------------|-------------------|
| No `tool_call` fences | Legacy code-block path (identical to v2.37.0) |
| Only `tool_call` fences | Tool calls dispatched, code blocks ignored |
| Mixed (both) | **Tool calls win.** Code blocks silently ignored. |
| Malformed `tool_call` fence | Skipped (not crashed), rest of the response processed |

The contract is deterministic: tool calls are an **alternative**,
not an addition. A confused LLM that emits both produces a
predictable outcome.

---

## Tools registered in v2.38.0

### `write_file` (concrete, wired)

```text
NAME        write_file
DESCRIPTION Write content to a file at the given path (project-relative).
            Creates parent directories as needed. Path must resolve inside
            the project directory after symlink resolution. Atomic: a crash
            mid-write leaves the previous file intact.
PARAMS      path     (string, required)  Project-relative path
            content  (string, required)  Full file contents (max 4 MiB)
RETURNS     {written, bytes, created, existed, project_ok}
```

#### Boundary enforcement

Every call runs through `fsutil.PathIsSafe(projectDir, path)`, which
resolves symlinks before checking. A symlink inside the project
that points outside is rejected — the same protection as the
legacy code-block path, applied earlier in the lifecycle.

#### Atomic writes

Files are written via temp file + fsync + rename (the same pattern
`internal/loop/cycle.go:persistLocked` uses for loop state). A
SIGKILL between the temp write and the rename leaves the original
file untouched and the temp file cleaned up by the next
`MkdirAll` or OS-level sweep.

#### Size cap

`MaxWriteBytes = 4 MiB`. A runaway model emitting a 50 MB file is
rejected before any filesystem side effect. The cap is generous
for source code and configuration; raise it in
`internal/tools/fs/fs.go` if a real workload needs more.

### `read_file` (stub, advertised)

```text
NAME        read_file
DESCRIPTION Read the contents of a file at the given path.
PARAMS      path (string, required)
RETURNS     Not yet wired (Sprint 70).
```

### `search_code` (stub, advertised)

```text
NAME        search_code
DESCRIPTION Search the project for a regex pattern.
PARAMS      pattern (string, required), path (string)
RETURNS     Not yet wired (Sprint 70).
```

### `run_gate` (stub, advertised)

```text
NAME        run_gate
DESCRIPTION Run a quality gate command (go test, go vet, etc.).
PARAMS      command (string, required)
RETURNS     Not yet wired (Sprint 71).
```

The three stubs are registered so the surface area is visible
before wiring. Calling them returns
`"tools: <name> is registered but not yet wired..."` — the
executor catches this in `applyToolCalls` and surfaces the error
to the verifier, the same way a real tool failure is handled.

---

## How the executor decides which path to take

`internal/engine/engine.go: applyLLMResponse` does:

```go
if e.ToolRegistry != nil {
    calls := extractToolCalls(response)
    if len(calls) > 0 {
        return e.applyToolCalls(response, calls)
    }
}
return e.applyCodeBlocks(response)
```

Three rules:

1. If `ToolRegistry` is nil, only the legacy code-block path runs.
   This is the v2.37.0 default for any caller that hasn't opted in.
2. If `ToolRegistry` is set but the LLM emits no `tool_call` fences,
   the legacy path runs. Back-compat is preserved.
3. If `ToolRegistry` is set AND `tool_call` fences exist, the tool
   path runs. Code blocks in the same response are ignored.

To **disable** tool-use entirely (force legacy only), pass
`--no-tools` on `radiant run`:

```bash
radiant run specs/0001-foo --no-tools
```

This sets `e.ToolRegistry = nil` before `Run()`, restoring the
v2.37.0 behaviour exactly.

---

## How the verifier sees the trace

When the executor dispatches tool calls, it accumulates a trace:

```go
type ToolCallRecord struct {
    Name      string          // "write_file"
    Args      json.RawMessage // the raw JSON the LLM emitted
    Result    json.RawMessage // the JSON the tool returned
    Err       string          // error if any
    Bytes     int             // populated by write_file
    Written   string          // populated by write_file
    Created   bool            // populated by write_file
    ProjectOK bool            // mirrors PathIsSafe result
}
```

`BuildVerifierPrompt` (in `internal/loop/verifier.go`) takes the
trace and adds a `TOOL CALLS OBSERVED` section to the verifier
prompt:

```
TOOL CALLS OBSERVED (in execution order):
1. write_file — internal/foo.go (1432 bytes, created)
2. write_file — internal/foo_test.go (892 bytes, created)

ANTI-CHEAT ADDENDUM:
- No tool call wrote outside the project boundary
- A tool call erroring is NOT grounds for rejection if the executor
  correctly surfaced the error and adjusted
```

When the trace is empty (legacy code-block path), the prompt is
byte-identical to v2.37.0. The verifier sees what the executor
actually did, not a string blob.

### Example: write_file rejection surfaced to verifier

LLM emits:

````markdown
```tool_call
{"name": "write_file", "args": {"path": "../etc/passwd", "content": "nope"}}
```
````

Dispatcher rejects (`fsutil.PathIsSafe` catches the `..` escape).
Trace records:

```json
{
  "name": "write_file",
  "args": {"path": "../etc/passwd", "content": "nope"},
  "err": "write_file: refusing path \"../etc/passwd\" — resolves outside project"
}
```

Verifier prompt now contains:

```
TOOL CALLS OBSERVED (in execution order):
1. write_file [ERROR: write_file: refusing path "../etc/passwd" — resolves outside project]
```

The verifier can now base its verdict on a concrete, structured
action instead of guessing from prose.

---

## Adding a new tool (for tool authors)

Three steps:

1. **Define the tool in `internal/tools/<area>/<name>.go`.** Use the
   pattern in `internal/tools/fs/fs.go`:

   ```go
   func MyTool(projectDir string) *tools.Tool {
       return &tools.Tool{
           Name: "my_tool",
           Description: "...",
           Params: []tools.Param{
               {Name: "input", Type: "string", Required: true},
           },
           Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
               // parse `raw` per the schema above
               // do the work
               // return a result that satisfies annotator (optional)
           },
       }
   }
   ```

2. **Make the result type satisfy the `annotator` duck-typed interface**
   (in `internal/engine/engine.go`) so the verifier sees structured
   metadata:

   ```go
   func (r MyResult) Annotate() map[string]any {
       return map[string]any{
           "key1": r.Field1,
           "key2": r.Field2,
       }
   }
   ```

   The engine type-switches against `Annotate() map[string]any`. No
   import of the engine package is required from your tool — duck
   typing is enough.

3. **Register in `RealRegistry` (in `internal/loop/real_registry.go`):**

   ```go
   func RealRegistry(projectDir string) *tools.Registry {
       r := tools.NewRegistry()
       r.Register(fs.WriteFileTool(projectDir))
       r.Register(myarea.MyTool(projectDir))
       return r
   }
   ```

4. **Tests.** At minimum: happy path + boundary rejection (if the
   tool takes a path) + atomicity (if it writes). The `internal/tools/fs/fs_test.go`
   pattern is the canonical example.

5. **Document in this file.** Add a section under "Tools registered
   in vN.N.N" with name, params, returns, and any non-obvious
   behaviour.

---

## Migration from code-block emission

For LLM authors updating prompts:

**Before (v2.37.0):**

````markdown
```go
// File: internal/foo.go
package foo

func Bar() string { return "bar" }
```
````

**After (v2.38.0):**

````markdown
```tool_call
{"name": "write_file", "args": {"path": "internal/foo.go", "content": "package foo\n\nfunc Bar() string { return \"bar\" }\n"}}
```
````

Both work simultaneously — the executor detects which the LLM emitted
and routes accordingly. Existing prompts that emit code blocks keep
working unchanged.

**Tip for prompt authors:** when introducing tool calls, instruct
the LLM to emit tool calls only when path safety matters (writing
new files, large refactors) and fall back to code blocks for trivial
diff-style edits. The executor handles both seamlessly.

---

## Operator controls summary

| Flag / Command | Effect |
|----------------|--------|
| `radiant tools list` | List all registered tools (default registry) |
| `radiant tools list --real` | List the concrete (wired) registry |
| `radiant tools list --json` | Machine-readable JSON output |
| `radiant run ... --no-tools` | Disable structured tool-use; force legacy |
| (default) | Tool registry is wired automatically |

---

## See also

- `docs/SPRINT69-PLAN.md` — the implementation plan that produced this release
- `internal/tools/tools.go` — registry, Tool/Param types, dispatcher
- `internal/tools/fs/fs.go` — concrete write_file implementation
- `internal/engine/engine.go` — `applyLLMResponse` switch + `annotator` interface
- `internal/loop/verifier.go` — `BuildVerifierPrompt` with tool-trace section
- `internal/loop/real_registry.go` — concrete tool registration
- `internal/fsutil/fsutil.go` — symlink-aware `PathIsSafe`