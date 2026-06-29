# Sprint 70 — Tool Use Wire-up Parte 2: read_file + search_code (v2.39.0)

> **Status**: In progress
> **Branch**: `feature/light-full-release` (continuation — single feature branch carries both sprints)
> **Target version**: v2.39.0
> **Estimated scope**: 1 sprint focado (smaller than Sprint 69 — pattern established)

---

## Scope

Sprint 69 wired `write_file` (the highest-value tool) through the
engine and surfaced tool-call traces in the verifier prompt.
Sprint 70 closes the **read** half of the read-write pair and adds
the first **search** tool. This brings the harness from "1 concrete
tool" to "3 concrete tools" — the LLM can now read state before
mutating it, and grep the project tree without round-tripping
through the shell.

### Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | `read_file` tool reads a project-relative file and returns its contents | Happy test returns `{content, bytes, lines}` |
| G2 | `read_file` enforces `fsutil.PathIsSafe` | Unsafe path → structured error |
| G3 | `read_file` caps content size (4 MiB default, matching write) | Oversize file → structured error |
| G4 | `read_file` annotates result (bytes/lines/path) for the verifier trace | Annotate() returns the trace-friendly map |
| G5 | `search_code` tool does regex search across project files | Happy test returns `[{file, line, col, content}]` matches |
| G6 | `search_code` rejects unsafe path (search root must be inside project) | Boundary check enforced |
| G7 | `search_code` compiles pattern via Go regexp, surfaces syntax errors | Bad regex → structured error |
| G8 | `search_code` caps results (default 1000) to prevent DoS | Cap enforced + reported |
| G9 | RealRegistry wires all 3 concrete tools; stubs removed from the real path | `radiant tools list --real` shows write_file + read_file + search_code |
| G10 | Default registry still advertises the 4 stubs for back-compat (run_gate remains) | `radiant tools list` (default) unchanged |
| G11 | `internal/loop/real_registry.go` registers new tools without breaking the engine ↔ tools/fs cycle | No new cycles |
| G12 | TOOL-USE.md updated with read_file/search_code sections | Doc reflects new surface |

### Out of scope (carried to Sprint 71+)

- `run_gate` concrete implementation (needs `internal/gaterun` wrapper + policy allowlist)
- MCP tool-bridge adapter
- Tool-call replay in `radiant loop export`
- Schema validation beyond JSON type-check
- Anthropic/OpenAI/Gemini native function-call parsing

---

## Design

### `read_file`

Wire-compatible with the existing stub; replaces its implementation
in `internal/tools/fs/`:

```go
type ReadArgs struct {
    Path string `json:"path"`
}

type ReadResult struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Bytes   int    `json:"bytes"`
    Lines   int    `json:"lines"`
}

func (r ReadResult) Annotate() map[string]any {
    return map[string]any{
        "path":  r.Path,
        "bytes": r.Bytes,
        "lines": r.Lines,
    }
}
```

Boundary check + size cap mirror write_file (4 MiB default).
Project-relative paths only. Symlinks resolved via `fsutil.PathIsSafe`.

### `search_code`

```go
type SearchArgs struct {
    Pattern    string `json:"pattern"`         // Go regexp
    Path       string `json:"path,omitempty"`  // search root; default project root
    MaxResults int    `json:"max_results,omitempty"` // default 1000
    Include    string `json:"include,omitempty"` // glob filter (e.g. "*.go")
}

type SearchMatch struct {
    File    string `json:"file"`
    Line    int    `json:"line"`
    Column  int    `json:"column"`
    Content string `json:"content"`
}

type SearchResult struct {
    Pattern    string        `json:"pattern"`
    Root       string        `json:"root"`
    MatchCount int           `json:"match_count"`
    Truncated  bool          `json:"truncated"`
    Matches    []SearchMatch `json:"matches"`
}

func (r SearchResult) Annotate() map[string]any {
    return map[string]any{
        "pattern":     r.Pattern,
        "root":        r.Root,
        "match_count": r.MatchCount,
        "truncated":   r.Truncated,
    }
}
```

Implementation uses `filepath.WalkDir` over the search root, skips
hidden directories (`.git`, `.radiant-harness`, `node_modules`) and
binary files (detected via `http.DetectContentType` on first 512
bytes). Each matched file is scanned line-by-line with `regexp.MatchString`.
Output is capped at `MaxResults` (default 1000); `Truncated=true` is
set when the cap is hit so the LLM knows to narrow the search.

### Why these specific defaults

| Decision | Rationale |
|----------|-----------|
| Read size cap = 4 MiB (same as write) | Symmetric — LLM can read back what it wrote |
| Search result cap = 1000 | A typical project has <10k source files; 1000 matches is ~10 files × 100 lines, plenty for the LLM to identify the right spot |
| Skip hidden dirs | `.git`, `.radiant-harness`, `node_modules` would explode search time without useful results |
| Skip binary files | Regex on binary produces noise matches; `http.DetectContentType` is fast and reliable |
| `Include` glob filter | Lets the LLM narrow to "*.go" or "*.test.*" without paying the cost of irrelevant files |

---

## Files

| File | Change | LOC est. |
|------|--------|----------|
| `docs/SPRINT70-PLAN.md` | NEW — this file | 180 |
| `internal/tools/fs/read_file.go` | NEW — ReadFileTool + ReadResult + tests | 130 + 90 |
| `internal/tools/fs/search_code.go` | NEW — SearchCodeTool + SearchResult + tests | 200 + 130 |
| `internal/loop/real_registry.go` | MODIFY — register read_file + search_code | +3 |
| `internal/tools/tools.go` | MODIFY — bump Default() comment to reflect what's still stubbed (run_gate only) | +5 |
| `docs/TOOL-USE.md` | MODIFY — add read_file/search_code sections | +60 |
| `CHANGELOG.md` | MODIFY — v2.39.0 entry | +60 |
| `RELEASE-NOTES.md` | MODIFY — v2.39.0 entry | +50 |
| `cmd/radiant/main.go` | MODIFY — bump version to 2.39.0 | +1 |

**Total estimate: ~900 LOC** (300 new in `internal/`, ~220 tests,
~250 docs/misc).

---

## Test matrix

### read_file

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestReadFile_HappyPath` | Reads content, returns correct bytes/lines counts |
| 2 | `TestReadFile_CreatesParentDirs` | N/A — read doesn't create |
| 3 | `TestReadFile_MissingFile` | Returns "not found" structured error |
| 4 | `TestReadFile_RejectsUnsafeRelativeEscape` | `../escape.txt` → error |
| 5 | `TestReadFile_RejectsSymlinkedProjectSubdir` | Symlink escape caught |
| 6 | `TestReadFile_RejectsOversize` | File >4 MiB → structured error |
| 7 | `TestReadFile_Annotate` | Annotate() returns trace-friendly map |
| 8 | `TestReadFile_ViaRegistry` | Roundtrip through tools.Registry |

### search_code

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestSearchCode_FindsMatches` | Returns correct file/line/col/content |
| 2 | `TestSearchCode_NoMatches` | Returns empty matches, count=0 |
| 3 | `TestSearchCode_InvalidRegex` | Returns compile error |
| 4 | `TestSearchCode_RespectsScope` | Custom `path` arg limits search |
| 5 | `TestSearchCode_SkipsHiddenDirs` | `.git`, `.radiant-harness` not traversed |
| 6 | `TestSearchCode_SkipsBinaryFiles` | PNG/ELF not searched |
| 7 | `TestSearchCode_RespectsMaxResults` | Cap enforced, `truncated=true` |
| 8 | `TestSearchCode_RejectsUnsafeScope` | `../escape` search root → error |
| 9 | `TestSearchCode_Annotate` | Annotate() returns trace-friendly map |

---

## Risks

| Risk | Mitigation |
|------|------------|
| LLM emits regex like `.*` causing every line to match → massive output | `MaxResults` cap + `Truncated` flag |
| Binary file with `MatchString` matches noise | `http.DetectContentType` skip |
| Symlink in search root escapes project | `fsutil.PathIsSafe` on `path` arg |
| Read of huge log file DoSes the loop | 4 MiB size cap (symmetric with write) |
| Wire format conflict with future SDK-style function calls | `tool_call` fences are language-agnostic; future SDKs can map onto the same `Registry.Call` |

---

## Commit plan

Single commit on `feature/light-full-release`:

```
feat(tool-use): Sprint 70 — read_file + search_code concrete (v2.39.0)
```

Pass criteria: `go vet ./...` clean, `go test -count=1 -v ./...`
green, cross-compile 3/3 platforms.

---

**Status at plan write**: Sprint 69 (v2.38.0) committed at `3a0394d`.
Sprint 70 implementation in progress.