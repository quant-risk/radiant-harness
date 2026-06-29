# Release Notes — v2.39.0 (Tool Use Wire-up Parte 2)

> "Read before you write." The LLM can now inspect state and grep
> the project tree through the structured tool registry, without
> round-tripping through the shell.

## Headlines

### 1. `read_file` tool

```markdown
```tool_call
{"name": "read_file", "args": {"path": "internal/foo.go"}}
```
```

Returns `{path, content, bytes, lines}`. Boundary-checked via
`fsutil.PathIsSafe` (symlink-aware), capped at 4 MiB
(symmetric with `write_file`).

### 2. `search_code` tool

```markdown
```tool_call
{"name": "search_code", "args": {"pattern": "TODO", "include": "*.go", "max_results": 100}}
```
```

Returns `[{file, line, column, content}]` matches. Skips hidden
directories (`.git`, `.radiant-harness`, `node_modules`, `vendor`,
`.idea`, `.vscode`) and binary files. Default cap 1000 matches;
`Truncated=true` indicates the cap was hit.

### 3. `radiant tools list --real` now shows 3 tools

```
$ radiant tools list --real
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
write_file      Write content to a file at the given path (project-relati... 2
read_file       Read the contents of a file at the given path (project-re... 1
search_code     Search the project for a regex pattern. Returns matching ... 4
```

`run_gate` remains a stub (lands in Sprint 71 with the `gaterun`
wrapper).

## Stats

- 2 new concrete tools (`read_file`, `search_code`).
- **969 tests passing across 28 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 5 new files, 1 modified. ~600 LOC.

## Compatibility

- No breaking changes. New tools are opt-in via the existing
  `Engine.ToolRegistry` wiring.
- LLM outputs that contain only `write_file` keep working
  unchanged.
- `--no-tools` still forces the legacy code-block path.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.39.0
# or:
git pull
make build
./bin/radiant --version             # should report 2.39.0
./bin/radiant tools list --real     # 3 tools (was 1 in v2.38.0)
```

See [`docs/TOOL-USE.md`](docs/TOOL-USE.md) for the full operator
guide.

---

# Release Notes — v2.38.0 (Tool Use Wire-up Parte 1)

> "Stop regex-parsing code blocks." The first concrete structured
> tool replaces the legacy `os.WriteFile` path for any LLM that
> emits `tool_call` fences. The verifier sees the trace, not a
> string blob.

## Headlines

### 1. Structured `write_file` tool

LLMs can now emit a structured call inside a `tool_call` fenced
block:

```markdown
```tool_call
{"name": "write_file", "args": {"path": "internal/foo.go", "content": "package foo\n"}}
```
```

The executor dispatches it through `internal/tools/Registry`,
which calls `internal/tools/fs.WriteFileTool` — atomic write
(temp + fsync + rename), `fsutil.PathIsSafe` boundary check,
4 MiB size cap. The legacy code-block path is untouched and
runs whenever the LLM doesn't emit tool calls.

### 2. Verifier sees the trace

`BuildVerifierPrompt` now accepts a `toolTrace` slice. When
non-empty, the prompt gains a `TOOL CALLS OBSERVED` section:

```
TOOL CALLS OBSERVED (in execution order):
1. write_file — internal/foo.go (1432 bytes, created)
2. write_file — internal/foo_test.go (892 bytes, created)
```

plus two anti-cheat clauses about boundary violations and
tool-call error handling. When the trace is empty (legacy
code-block path), the prompt is byte-identical to v2.37.0.

### 3. CLI: `radiant tools list`

```
$ radiant tools list
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
run_gate        Run a quality gate command (go test, go vet, etc.). Retur... 1
read_file       Read the contents of a file at the given path. Path must ... 1
write_file      Write content to a file at the given path. Creates parent... 2
search_code     Search the project for a regex pattern. Returns matching ... 2

$ radiant tools list --real
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
write_file      Write content to a file at the given path (project-relati... 2
```

`--real` shows the v2.38.0 wired registry; the default shows
the v2.37.0 stub registry for back-compat inspection of the
advertised surface area.

## Other changes

- **Internal: `internal/fsutil/`** — neutral package hosts
  `PathIsSafe` so `engine` and `tools/fs` can both depend on it
  without an import cycle.
- **Internal: `RealRegistry` indirection** — `internal/loop`
  wires the concrete builder through `tools.SetRealRegistryBuilder`,
  called automatically at init time.
- **`radiant run --no-tools`** — opt-out flag for operators who
  want v2.37.0 behaviour exactly.

## Stats

- 3 new packages: `internal/tools/fs/`, `internal/fsutil/`,
  `internal/loop/real_registry.go`.
- 1 new CLI subcommand: `radiant tools list` (+ `--real`, `--json`).
- 1 new flag: `--no-tools` on `radiant run`.
- 1 concrete tool wired: `write_file` (replaces the v2.37.0 stub).
- **947 tests passing across 28 packages, 0 failures** (validated
  with `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

## Compatibility

- **No breaking changes.** Default behaviour is to wire
  `RealRegistry` automatically. `--no-tools` restores v2.37.0.
- **Back-compat preserved:** LLM outputs that contain only
  code blocks keep working unchanged. Mixed outputs (tool calls
  + code blocks) → tool calls win, code blocks ignored.
- **Engine.PathIsSafe** retained as a wrapper for any caller
  that imported it directly.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.38.0
# or:
git pull
make build
./bin/radiant --version       # should report 2.38.0
./bin/radiant tools list --real
./bin/radiant run specs/0001-foo --no-tools   # opt out
```

See [`docs/TOOL-USE.md`](docs/TOOL-USE.md) for the full operator
guide.

---

# Release Notes — v2.37.0 (Light/Full + Semantic + Lazy-Executor)

> "Make it closed" — the release that turns radiant-harness from a
> working prototype into a complete, vendable product.

## Headlines

### 1. Two operating modes (Light / Full)

The harness can now be deployed two ways. The choice is a runtime
decision, not a build-time one — same binary, same loop engine, same
state machine, same verifier. What differs is who pays for the tokens.

| Mode | Inference path | Setup |
|------|---------------|-------|
| **Light** | Harness calls MCP `sampling/createMessage` on the host agent | `radiant setup-mcp --agent=claude` |
| **Full**  | Harness calls LLM HTTP endpoints directly | `export OPENROUTER_API_KEY=…` |

`radiant mode show` reports the resolved mode and the source (flag,
env, config, auto-detect). Auto-detect: presence of MCP config →
Light; presence of API key → Full; default → Light (safe).

### 2. Semantic model layer (credit-risk domain)

The "what it means here" layer that fixes the failure mode described
in the post that inspired this release: "instructions scale poorly,
context drifts, answers go wrong".

`internal/semantic/metrics/credit-risk.yaml` ships 7 metrics with
formulas, scopes, and regulation references:

- **PD** (Probability of Default) — CMN 4.966 §4.2.1
- **LGD** (Loss Given Default) — CMN 4.966 §4.2.3
- **EAD** (Exposure at Default) — CMN 4.966 §4.2.2
- **RWA** (Risk-Weighted Assets) — CMN 4.966 §4.2.1.4
- **ExpectedLoss** — IFRS 9 §5.5
- **provision_min_ifrs9** — CMN 4.966 §4.4
- **capital_required** — CMN 4.966 §4.1.1

The loop runner auto-detects the project domain and injects the
matching model's full markdown into the executor system prompt.
`radiant semantic resolve credit-risk RWA` returns the formula and
regulation inline.

### 3. Lazy-executor skill

Port of the [ponytail ladder](https://github.com/DietrichGebert/ponytail)
in PT-BR, adapted to the radiant context where the verifier already
cuts code that doesn't satisfy ACs. Three intensities:

- `lite` — build what was asked, suggest lazy alt in one line
- `full` — ladder enforced (default)
- `ultra` — YAGNI extremist, challenge the request itself

`--intensity=lite|full|ultra` on `radiant loop start`. Default `full`
so the skill is always injected unless explicitly off.

## Other changes

- **Pricing catalog** — `internal/pricing/data/pricing.yaml` consolidates
  the three duplicated rate tables. `radiant pricing list|stale|refresh`.
- **pathIsSafe security fix** — resolves symlinks before the boundary
  check. A symlink inside the project pointing outside is now rejected
  (was a TOCTOU hole that a confused or hostile LLM could exploit).
- **Documentation** — `docs/MODES.md` (full operator guide),
  `docs/IMPLEMENTATION-PLAN.md` (the plan this release executed),
  README updated, CHANGELOG with full diff.

## Stats

- 9 commits on branch `feature/light-full-release` (8 features + 1 plan).
- 5 new packages: `internal/mode/`, `internal/pricing/`,
  `internal/semantic/`, `internal/tools/` (scaffold), plus extensions
  to `internal/skill/`, `internal/engine/`, `internal/loop/`, `cmd/radiant/`.
- 4 new CLI subcommands: `mode`, `pricing`, `semantic`, plus
  `--intensity` flag on `radiant loop start`.
- 1 new skill: `lazy-executor` (PT-BR, port of the ponytail ladder).
- 7 new metrics in `credit-risk.yaml` — references CMN 4.966 / IFRS 9 / Basileia.
- **921 tests passing across 26 packages, 0 failures** (validated with
  `go test -count=1 -v ./...`). `go vet ./...` clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB), windows/amd64 (15 MB).
- 37 files changed: +4,747 / −1,050 LOC.

## Compatibility

- No breaking changes. `--mode` and `--intensity` default to auto/safe
  values when not specified. `radiant mode show` and `radiant pricing
  list` are pure read commands.
- Existing `.radiant.yaml` files keep working — `mode:` and `intensity:`
  are optional fields with sensible defaults.
- Embed-based semantic YAML is read-only at runtime; user overrides
  go in `<projectDir>/metrics/<domain>.yaml` and win over embedded.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.37.0
# or:
git pull
make build
./bin/radiant --version   # should report 2.37.0
./bin/radiant doctor      # new mode check, new pricing freshness check
./bin/radiant mode show   # see your active mode
./bin/radiant pricing list # see the new canonical rates table
```

---

# Release Notes — 0.2.0 (Go rewrite)

> Vendor-neutral, multi-platform, multi-LLM. No agent is privileged.

## What's new

### Security hardening

- **Agent binary allowlist** — `internal/harness/agent.go`. Refuses to spawn
  anything outside `{claude, codex, copilot, cursor, gemini}`. Adding a
  new adapter is an explicit code edit, not a config knob.
- **Gate command allowlist** — tasks.md gates are tokenized and each
  binary must be in the closed set (`node`, `npm`, `pnpm`, `yarn`, `go`,
  `make`, `pytest`, `python`, `cargo`, `jest`, …). `rm`, `curl`, `wget`
  are rejected by name.
- **Path sandboxing** — code blocks emitted by the LLM are checked
  against the project directory before being written.

### Crash safety

- **Atomic state persistence** — temp-file + fsync + rename, so a crash
  mid-write never leaves a half-written `progress.json`.
- **Advisory flock** — concurrent `radiant run` invocations on the same
  project serialize instead of corrupting state.
- **Timeouts everywhere** — 10 min per agent, 5 min per gate, with
  context cancellation propagating.

### Vendor neutrality (this release)

The CLI no longer treats Claude as the default agent:

- `radiant init` with `--yes` (no `--agent=`) now scaffolds **all** agents
  instead of silently picking Claude.
- `radiant init` without flags refuses to guess — the operator must
  declare which agent(s) they want. The error message lists all six
  supported vendors in alphabetical order.
- `DetectAgent()` scans `$PATH` alphabetically; no agent is privileged.
- The `claude` example in the README is one of many, not the first.
- All `--agent=` examples in the README and Makefile smoke test now
  exercise `--all` to assert multi-vendor behavior.

### LLM client

- **Provider-agnostic** — OpenRouter, OpenAI, Anthropic (via OpenRouter
  proxy or custom BaseURL), or any OpenAI-compatible endpoint.
- **10 curated presets** spanning Anthropic, OpenAI, Google, DeepSeek,
  Xiaomi. Add new vendors by editing `PresetModels` in
  `internal/llm/client.go` — no spec/format change needed.
- **Retry with backoff** — exponential + full jitter on 5xx, fail-fast on
  4xx. Capped at 5 attempts (initial + 4 retries).
- **Streaming** — SSE-aware with backpressure-friendly scan buffer.
- **32k default `MaxTokens`** — matches the size of real SDD specs; per-
  preset override available.

### Engine consolidation

- Parallel tasks within a phase are capped by a semaphore (4 by default)
  so we don't burst provider rate limits.
- Engine now actually runs gates (was a no-op stub before this release).
- Engine validates emitted code-block paths against the project boundary.

### Spec parser

- AC IDs in any of `AC-1`, `AC1`, `AC_1`, `AC 1`, `ac-1` are normalized
  to `AC-1`. Tasks and spec can mix forms.
- "And" clauses in Given/When/Then are appended to the most recent
  non-empty clause instead of being silently dropped.
- Tasks parser handles 5- and 6-column rows; tolerates `·` as a column
  separator inside "Covers AC".
- `groupPhases` now correctly groups consecutive parallel tasks into a
  single parallel phase.

### Quality scripts

- `radiant validate --gates` actually executes the task gates found in
  tasks.md (was static-only before). Each gate is validated against the
  allowlist and run with a 5-minute timeout. Skipped gates (binary not
  in allowlist) are reported but don't block.

### Build & distribution

- Single binary via `go build` (8.9 MB on Linux amd64).
- Cross-platform via goreleaser: linux/darwin/windows × amd64/arm64.
- Docker multi-stage Alpine build (Go 1.22 runtime).
- **GitHub Actions CI** on Go 1.22 and 1.24: gofmt, go vet, build, test,
  smoke, cross-build, coverage.

### VS Code extension

- Specs / Tasks / Progress tree views now populated (Tasks and Progress
  were empty stubs before).
- Status bar shows live state, feature, current/total tasks, and % done.
- File watcher on `.radiant-harness/progress.json` keeps the UI live.
- "Run gate" command available from the tasks.md context menu.

## Compatibility

- Templates and skills are reused from 0.1.0 (TypeScript) — no spec/tasks
  changes needed.
- Manifest format unchanged.
- Existing `.radiant-harness/progress.json` files load transparently.

## Known limitations

- **No auto model routing.** Pick your model explicitly per run. Future
  feature.
- **`internal/plugin/`** is a stub — the plugin system is documented
  but not implemented. Either wire it up or remove the package.
- **`internal/benchmark/`** has 138 lines no caller uses. Audit and
  either expose or remove.
- **Engine path uses OpenAI-compatible API only.** Direct Anthropic
  Messages API (which has a different shape) requires an OpenRouter
  proxy or a custom BaseURL.

## Migration from 0.1.0 (TypeScript)

1. Replace `npx @igoruehara/spec-driven init` with `radiant init --all`.
2. Replace `npx spec-driven validate` with `radiant validate --gates`.
3. Replace `npx spec-driven run` with `radiant run specs/NNNN-…`.
4. Existing specs and tasks.md files are unchanged.
