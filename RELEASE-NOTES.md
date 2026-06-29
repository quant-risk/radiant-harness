# Release Notes — v2.49.0 (`hostdetect` + `radiant host-info`)

> Foundational sprint for auto-detecting which agent is currently
> invoking radiant-harness. Detection is exposed today; automatic
> possession follow in Sprint 80.

## TL;DR

A new `internal/hostdetect/` package identifies at runtime which
agent host (if any) is running radiant-harness. The `radiant
host-info` subcommand surfaces the result so the user (or an LLM)
can verify what's happening.

```
$ CLAUDE_CODE_ENTRY=/entry/claude CLAUDE_CODE_SSE_PORT=8080 \
  radiant host-info
Detected host agent:  claude-code (High confidence)
Sampling supported:  yes
Detection source:    env
PID:                  63894  PPID: 63857

Host "claude-code" supports MCP sampling — possession is possible.

$ radiant host-info --json
{
  "agent": "claude-code",
  "confidence": 90,
  "supports_sampling": true,
  "sample_env_vars": ["CLAUDE_CODE_ENTRY", "CLAUDE_CODE_SSE_PORT"],
  ...
}
```

## Detection matrix

Nine agents are detected. Each has at least one distinguishing
env var. If multiple env vars match, confidence climbs.

| Agent             | Env signature                                              | PPID fallback       | Sampling |
|-------------------|------------------------------------------------------------|---------------------|----------|
| Claude Code       | CLAUDE_CODE_ENTRY, CLAUDE_CODE_SSE_PORT, CLAUDE_CODE_PID   | claude-code          | yes      |
| Cursor            | CURSOR_TRACE_ID, CURSOR_HOME, CURSOR_USER_DATA_DIR          | cursor               | yes      |
| Hermes            | HERMES_VERSION, HERMES_HOME, HERMES_AGENT_HOME              | hermes-agent         | yes      |
| Kimi CLI          | KIMI_SHARE_DIR, KIMI_VERSION, KIMI_CONFIG_DIR               | kimi                 | yes      |
| OpenClaw          | OPENCLAW_GATEWAY_URL, OPENCLAW_VERSION, OPENCLAW_WORKSPACE  | openclaw             | yes      |
| Codex             | CODEX_HOME, CODEX_THREAD_ID, CODEX_RUNTIME, CODEX_THREAD_ENV | codex                | yes      |
| Cline             | CLINE_USER, CLINE_VERSION, CLINE_WORKSPACE                  | cline                | yes      |
| OpenCode          | OPENCODE_HOME, OPENCODE_VERSION, OPENCODE_CONFIG            | opencode-cli         | yes      |
| VS Code Copilot   | VSCODE_PID, VSCODE_IPC_HOOK_CLI, VSCODE_CWD                 | Code Helper          | yes      |

All known agents support MCP sampling. (Sprint 80 will wire this.)

## What's NOT in this release (deferred to Sprint 80)

- **`PickBackend` with auto-possession** — currently `radiant
  loop`, `radiant run`, `radiant fleet` require an API key even
  if you're inside Claude Code. Sprint 80 introduces
  `internal/llm/pick.go` and applies it to every Full subcommand:
  ```
  1. Host detected + supports sampling → SamplingBackend (no key)
  2. Else API key set → HTTPBackend
  3. Else clear error
  ```
- **Configurable precedence** — user override of the order above
  (e.g. RADIANT_BACKEND_PREFERENCE=api-key to disable auto-possession).
- **Deep process-tree walk** on Windows / BSD. Sprint 79 is
  macOS/Linux focused.

## How to use right now

```bash
# Verify your agent is detected:
$ CLAUDE_CODE_ENTRY=... radiant host-info
# You should see "Detected host agent: claude-code (High confidence)"

# Or with JSON for tooling:
$ CLAUDE_CODE_ENTRY=... radiant host-info --json | jq .
```

If detection is wrong (false positive or false negative), the env
var list per agent lives in `internal/hostdetect/hostdetect.go` —
edit the `signatures` map and add a test in `hostdetect_test.go`.

## Stats

- Light: 9.9-11 MB across 5 platforms.
- Full: 14-15 MB across 5 platforms.
- Light: 0 HTTP-LLM symbols.
- Tests: 31 packages OK (Full), 29 OK (Light), 0 FAIL.
- 24 hostdetect tests cover 9 agents + fallback cases.

---

# Release Notes — v2.48.0 (Light vs Full: physical binary separation)

> The user's explicit ask, delivered. **Two physically separate binaries
> from one source tree**. Light = no API key infrastructure. Full =
> everything. Each can be published to its own repo.

## TL;DR

You can now build two distinct binaries from the same source:

```bash
# Full: every subcommand, HTTP LLM providers, requires API key.
go build -o /usr/local/bin/radiant ./cmd/radiant

# Light: only `setup-mcp` + `mcp serve`, no API key, host-agent only.
go build -tags light_only -o /usr/local/bin/radiant-light ./cmd/radiant
```

The Light binary **physically cannot talk to HTTP LLM providers** — the
Anthropic native client, the OpenAI-compatible adapter, and the
`HTTPBackend` struct are tag-excluded at compile time. There's no API
key to set because there's no HTTP LLM layer to authenticate against.
Inference in Light comes exclusively from the host agent via MCP
sampling/createMessage.

## Why

In the prior architecture (v2.47.0 and earlier), Light was just a
subcommand (`radiant mcp serve`) of the Full binary. The binary
shipped with all LLM provider code linked in, even if you didn't use
it. That made it harder to:
- Run the harness in environments where you don't want HTTP LLM
  code at all (CI, air-gapped, vendored binary review).
- Publish a "minimal host-agent-driven" build to one repo and a
  "vendor-neutral API-key-driven" build to another.

Sprint 78 fixes both.

## Build matrix

| Build command                                 | Size      | What it contains                          |
|-----------------------------------------------|-----------|-------------------------------------------|
| `go build ./cmd/radiant`                       | 14-15 MB  | Everything (Full); API key required       |
| `go build -tags light_only ./cmd/radiant`      | 9.8-11 MB | Only `mcp serve` + `setup-mcp` (Light)    |

Both cross-compile to linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64.

## Light binary contents (post-Sprint 78)

```
$ radiant-light --help
Light build of the harness. Inference comes exclusively from the
host agent via MCP sampling/createMessage. No API key required,
no HTTP LLM backend included. For the Full build, use the default
build.

Available Commands:
  completion    Generate the autocompletion script
  help          Help about any command
  mcp           MCP server commands (Light mode — MCP sampling, no API key)
  setup-mcp     Register radiant as an MCP server in your agent's config
```

That's it. No `loop`, no `run`, no `fleet`, no `audit`, no `init`,
no `validate`, no `evals`, no `release`, no scaffold commands. None.
Light is intentionally minimal.

## Full binary contents (post-Sprint 78)

```
$ radiant-full --help
Vendor-neutral harness for autonomous LLM-driven development. ...

Available Commands:
  adr                   Create an Architecture Decision Record
  audit                 Run the auditar skill
  autodata              Auto-author a skill via LLM
  bench                 Run h2026 comparisons (TLC, OpenSpec, ...)
  boot                  Print minimal project manifest
  budget                Token budget estimation
  camada-agentica       Audit the agentic layer
  causal-estimate       Scaffold a causal analysis
  completion            ...
  config                Configure LLM provider
  context               Manage project context
  diagramar             Generate C4 Mermaid diagram
  doctor                Diagnose radiant environment
  drift                 Scaffold drift monitoring
  eval                  Run a single prompt N times
  evals                 AC→test coverage
  evaluate              Scaffold evaluation plan
  fleet                 Multi-agent coordination
  handoff               ...
  ... (everything)
```

(no change from v2.47.0 — Full is the default build.)

## Symbol-verification proof

```bash
$ nm radiant-light | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend|api\.anthropic\.com|api\.openai\.com'
0
```

None. Light has zero HTTP LLM symbols. It is physically incapable of
making an HTTP call to an LLM provider. The only inference path is
MCP sampling.

```
$ nm radiant-full | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
00000001006f1000 T github.com/quant-risk/radiant-harness/internal/llm.HTTPBackend
00000001006f1100 T github.com/quant-risk/radiant-harness/internal/llm.NewHTTPBackend
0000000100c5a070 T github.com/quant-risk/radiant-harness/internal/llm.(*Client).chatAnthropic
0000000100c5b100 T github.com/quant-risk/radiant-harness/internal/llm.(*HTTPBackend).Chat
0000000100c5b1a0 T github.com/quant-risk/radiant-harness/internal/llm.(*HTTPBackend).ChatStream
```

Full has all of them.

## How the publishing flow works

The same source repo produces both artifacts. The user publishes each
to its own GitHub repo:

```
# Monorepo (this repo): quant-risk/radiant-harness
#   branches: feature/...
#   ./cmd/radiant/  -- source tree
#   ./docs/LIGHT-VS-FULL.md -- this publishing guide

# Target repo 1: <user>/radiant-harness-light
#   CI: go build -tags light_only -o radiant-light ./cmd/radiant
#   Release artifacts: radiant-light-{linux,darwin,windows}-{amd64,arm64}

# Target repo 2: <user>/radiant-harness-full
#   CI: go build -o radiant-full ./cmd/radiant
#   Release artifacts: radiant-full-{linux,darwin,windows}-{amd64,arm64}
```

The user can `git filter` (e.g. `git filter-repo`) the relevant subset
of files from this monorepo into each target repo, or use sparse
checkout, or copy files manually. CI in each repo applies the right
`go build` flag (light_only or default).

## File-level map of what's tagged

```
internal/llm/
  types.go                  untagged   shared types (Model, Message, ...)
  presets.go                untagged   PresetModels + GetPreset + ListPresets
  sampling.go               untagged   SamplingBackend (always available)
  backend.go                untagged   Backend interface
  backend_http.go           !light_only  HTTPBackend impl (Full only)
  anthropic.go              !light_only  Anthropic native client
  client.go                 !light_only  OpenAI-compatible HTTP client

cmd/radiant/
  main.go                   !light_only  Full entrypoint
  main_full.go              !light_only  ... (legacy name for consistency)
  ...
```

(Wrapping it up — see CHANGELOG.md for the exact file list.)

## What stays in the monorepo

This repo (radiant-harness) keeps the full source. The user forks or
mirrors each subset into its target repo. Code that lives in only
one binary is fine to delete in the other binary's mirror:

- Light repo's mirror: all untagged files + cmd_mcp_runtime.go +
  builder_light.go. **Delete all `!light_only` files.** Result: a
  ~600-LOC clean codebase that compiles to a minimal harness.
- Full repo's mirror: all files except cmd_mcp_runtime.go +
  builder_light.go (the Light-specific runtime). Keep everything
  else as-is.

## What's NOT in this release

- **Runtime host-agent detection.** `radiant-full run --in-sampling`
  doesn't yet detect the host agent — Full still requires explicit
  API key. Sprint 79 will add that.
- **More commands in Light.** Currently only `mcp serve` +
  `setup-mcp`. Could expose doctor/security/telemetry later.
- **Two separate goreleaser configs.** For now, publish from source
  per repo's CI; goreleaser unification is a future sprint.

## Stats

- Light: 9.8-11 MB; Full: 14-15 MB
- Light: 0 HTTP-LLM symbols; Full: all of them.
- Tests: 29 packages, 0 FAIL (Full build).
- Cross-compile: 5 platforms × 2 modes = 10 binary targets.

---

# Release Notes — v2.47.0 (helpers.go extraction: PR review block)

> Pure refactor. Pulls the PR review block (`radiant review-pr`
> worker functions) out of the 2948-line `cmd/radiant/helpers.go`
> into a new themed file. Same call sites, same behaviour, zero
> test edits.

## TL;DR

`cmd/radiant/helpers.go` grew back to 2948 LOC after Sprint 74 (it
had reached 3894, the Sprint-74 extractions trimmed it to 2948,
and ~1000 LOC has accumulated since). Sprint 77 extracts the
biggest single-themed block: **the PR review block (~278 LOC)**.

```
BEFORE (post-Sprint 76)
─────────────────────────
cmd/radiant/helpers.go    2948 LOC  (everything)


AFTER (post-Sprint 77)
──────────────────────
cmd/radiant/helpers.go              2670 LOC  (−278, −9%)
  └── (everything else: spec helpers, MCP serve, telemetry, …)

cmd/radiant/cmd_pr_review.go        309 LOC  (NEW)
  ├── type gateResult
  ├── type acceptanceCriterion
  ├── runReviewPR                  ← body of `radiant review-pr`
  ├── parseAcceptanceCriteria
  ├── parseGatesFromTasks
  ├── countDiffFiles
  └── renderPRReview
```

## Caller unchanged

`cmd_spec.go:364` still calls `runReviewPR(...)` from inside the
`review-pr` subcommand's `RunE` closure. Nothing to change.

```go
// cmd_spec.go (UNCHANGED)
RunE: func(cmd *cobra.Command, args []string) error {
    ...
    return runReviewPR(args[0], diffPath, runGates, out)
},
```

The 9 existing PR review tests in `main_test.go` stay where they
are and pass with **zero edits**:

```
TestParseAcceptanceCriteriaBasic         PASS
TestParseAcceptanceCriteriaEmpty         PASS
TestParseAcceptanceCriteriaCaseInsensitive  PASS
TestParseGatesFromTasks                  PASS
TestParseGatesFromTasksEmpty             PASS
TestCountDiffFiles                       PASS
TestRenderPRReviewIncludesSections       PASS
TestRenderPRReviewGatePassFail           PASS
TestRenderPRReviewWithDiffStats          PASS
```

## Why this candidate

PR review was the **biggest single-themed contiguous block** in
helpers.go besides the MCP run-* chunk (which is even bigger but
spans multiple subcommands and would be over-eager to split). The
review block is also self-contained: it parses spec/tasks markdown,
optionally runs gates, and produces `pr-review.md`. Depends on
stdlib only.

## Remaining candidates for future sprints

`helpers.go` is now 2670 LOC. Smaller themed extractions still
inside:

- integrations (`runIntegrationsList`, `renderIntegrationsDoc` ~150 LOC)
- incident (`runIncident`, `renderIncidentDoc` ~150 LOC)
- autodata (`runAutodata`, `parseAutodataResponse` ~225 LOC)
- evals (`runEvals`, `computeFeatureCoverage`, `renderEvalsReport` ~225 LOC)
- runDoctor (~115 LOC)

Each will become its own themed file in a future sprint.

## Stats

- 1 file trimmed: `helpers.go` (−278 LOC).
- 1 file added: `cmd_pr_review.go` (+309 LOC including file header).
- Net: `cmd/radiant/` +31 LOC.
- 0 tests added (zero behaviour change).
- 0 deps added.
- 1189 PASS, 0 confirmed FAIL.

---

# Release Notes — v2.46.0 (cmd_setup_mcp split: main + per_agent)

> Pure refactor. Zero behaviour change, zero new agents, zero new
> features. Just split `cmd_setup_mcp.go` so the routing file stays
> under 400 LOC and the per-agent merges live in a flat reference
> file that's easy to extend.

## TL;DR

`cmd/radiant/cmd_setup_mcp.go` had grown to **781 LOC** by the end of
Sprint 75 — route, detect, generic JSON merges, plus six per-agent
merges (Codex TOML, OpenCode nested JSON, Hermes YAML, Kimi global
JSON, OpenClaw nested JSON, Cline JSON-with-fields). All 800+ lines
of that file lived in one place.

This release splits it:

- **`cmd_setup_mcp.go`** (375 LOC) — register, resolve, route, write.
- **`cmd_setup_mcp_per_agent.go`** (439 LOC) — six per-agent merges,
  one block each.

Both stay in `package main`. Same identifiers, same signatures, same
behaviour. The 18 setup-mcp tests pass with **zero edits**.

## File layout

```
cmd/radiant/
├── cmd_setup_mcp.go            (was 781 LOC; now 375 LOC)
│   ├── registerSetupMCPCmd
│   ├── radiantBinaryPath
│   ├── resolveMCPAgents
│   ├── type mcpEntry
│   ├── mcpConfigFor             ← single source of routing truth
│   ├── mergeClaudeSettings
│   ├── mergeMCPJSON             ← shared by Cursor/Windsurf/VSCode
│   ├── mergeZedSettings
│   └── writeMCPConfig
│
└── cmd_setup_mcp_per_agent.go  (NEW; 439 LOC)
    ├── Codex (TOML):            radiantBlockPattern,
    │                            tomlQuote, mergeCodexTOML
    ├── OpenCode (nested JSON):  openCodeServer,
    │                            openCodeConfig, mergeOpenCodeConfig
    ├── Hermes (YAML):           hermesEntry, mergeHermesConfig
    ├── Kimi (global JSON):      mergeKimiMCP
    ├── OpenClaw (nested JSON):  openClawServer,
    │                            mergeOpenClawJSONConfig
    └── Cline (JSON + fields):   clineEntry, mergeClineConfig
```

## How to add a 12th agent now

```bash
# 1. Add one switch case in cmd_setup_mcp.go:
case "myagent":
    target := filepath.Join(cwd, ".myagent", "config.json")
    content, err := mergeMyAgentConfig(target, entry)
    return target, content, err

# 2. Add the merge function in cmd_setup_mcp_per_agent.go:
func mergeMyAgentConfig(path string, entry mcpEntry) (string, error) {
    // ...
}

# 3. (Optional) Add detection in resolveMCPAgents above.
# 4. Add tests in cmd_setup_mcp_test.go.
```

Before this sprint, step 1 and 2 lived in the same 800-line file; now
they live in two themed files.

## Stats

- 1 file trimmed: `cmd_setup_mcp.go` (−406 LOC, −52%).
- 1 file added: `cmd_setup_mcp_per_agent.go` (+439 LOC).
- Net: `cmd/radiant/` +33 LOC (file-level header).
- 0 tests added (zero behaviour change).
- 0 deps added.
- 1190 PASS, 0 confirmed FAIL.

---

# Release Notes — v2.45.0 (setup-mcp: Hermes + Kimi + OpenClaw + Cline)

> `radiant setup-mcp` now covers **11 agents across 4 config formats**.
> Each new agent was added after researching its real published config
> shape — we don't fabricate "not supported" claims for major
> MCP-capable agents.

## Headlines

### 1. Hermes Agent (NousResearch)

```bash
radiant setup-mcp        # auto-detects if .hermes/ or ~/.hermes/ exists
radiant setup-mcp --agent=hermes
radiant setup-mcp --global   # writes ~/.hermes/config.yaml
```

Writes YAML under `mcp_servers:` while preserving every other
top-level key (model, terminal, browser, agent, ...) byte-for-byte.
Implementation uses `gopkg.in/yaml.v3` (already a project dep).

```yaml
mcp_servers:
  radiant:
    command: /usr/local/bin/radiant
    args:
      - mcp
      - serve
```

Hermes relaunches with the new entry the next time you start it
(it watches `~/.hermes/config.yaml`).

### 2. Kimi CLI (Moonshot AI)

```bash
radiant setup-mcp        # auto-detects if ~/.kimi/ exists
radiant setup-mcp --agent=kimi
```

Writes JSON to the global config (Kimi has no project-level MCP
support, so `--global` is always implicit):

```json
{
  "mcpServers": {
    "radiant": {
      "command": "/usr/local/bin/radiant",
      "args": ["mcp", "serve"]
    }
  }
}
```

`~/.kimi/mcp.json`. Pre-existing servers (e.g. `context7`) preserved.

### 3. OpenClaw

```bash
radiant setup-mcp        # auto-detects if .openclaw/ or ~/.openclaw/ exists
radiant setup-mcp --agent=openclaw
radiant setup-mcp --global   # writes ~/.openclaw/openclaw.json
```

OpenClaw's MCP config is a nested object under `mcp.servers`, sitting
alongside many siblings (`sessionIdleTtlMs`, future feature flags) and
unrelated top-level keys (`channels`, `gateway`, …). All siblings
and top-level keys are preserved byte-for-byte; only the `radiant`
entry under `mcp.servers` is added/replaced.

```json
{
  "channels": { "telegram": { ... } },
  "gateway": { "port": 18789 },
  "mcp": {
    "sessionIdleTtlMs": 600000,
    "servers": {
      "context7": { "command": "uvx", "args": ["context7-mcp"] },
      "radiant": {
        "command": "/usr/local/bin/radiant",
        "args": ["mcp", "serve"]
      }
    }
  }
}
```

### 4. Cline (CLI)

```bash
radiant setup-mcp        # auto-detects if ~/.cline/ exists
radiant setup-mcp --agent=cline
```

Writes JSON to `~/.cline/mcp.json` with the official Cline shape
(including `disabled` and `autoApprove` fields for parity with
their examples):

```json
{
  "mcpServers": {
    "local-server": {
      "command": "node",
      "args": ["/path/to/server.js"],
      "env": { "API_KEY": "..." },
      "disabled": false,
      "autoApprove": []
    },
    "radiant": {
      "command": "/usr/local/bin/radiant",
      "args": ["mcp", "serve"],
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

VS Code-extension users manage their config through the Cline UI
panel; that file lives at a separate path and is intentionally NOT
addressed by `radiant setup-mcp`.

## Coverage matrix

```
$ radiant setup-mcp --agent=claude|cursor|windsurf|zed|vscode|
                       codex|opencode|hermes|kimi|openclaw|cline
```

11 agents, 4 config formats (JSON-std, TOML, YAML, JSON-nested).

## Why these four

- **Hermes** — 205k stars, MCP-native, 18+ providers, the major
  open-weights CLI agent of the NousResearch ecosystem.
- **Kimi CLI** — 9.1k stars (and the commercial Kimi Code product
  builds on top of this CLI). MCP support is first-class.
- **OpenClaw** — 250k stars, embedded MCP runtimes, gateway-style
  architecture. The MCP registry under `mcp.servers` is the
  authoritative source.
- **Cline** — Battle-tested VSCode MCP client; the CLI form writes
  to a stable `~/.cline/mcp.json`.

## Why NOT LangChain / LangGraph

We explicitly skipped the two Python frameworks — they are
**not** MCP hosts, they are libraries for building MCP clients /
agents. Users wanting LangChain integration wrap `radiant mcp serve`
from inside their LangChain agent (it's just a regular stdio MCP
server, accessible from any framework).

## Stats

- 1 file extended: `cmd_setup_mcp.go` (+240 LOC).
- 1 file extended: `cmd_setup_mcp_test.go` (+440 LOC, 18 new tests).
- 1 plan doc added: `docs/SPRINT75-PLAN.md`.
- 0 deps added.
- **1189 tests passing** (counted across all packages).

---

# Release Notes — v2.43.0 (setup-mcp: Codex + OpenCode)

> Two more agents in `radiant setup-mcp`. Codex (OpenAI) and
> OpenCode (sst/opencode) are now auto-detected and configured.

## Headlines

### 1. Codex (OpenAI CLI) support

```bash
radiant setup-mcp        # auto-detects if .codex/ or ~/.codex/ exists
radiant setup-mcp --agent=codex  # explicit
```

Writes TOML config:

```toml
[mcp_servers.radiant]
command = "/usr/local/bin/radiant"
args = ["mcp", "serve"]
```

Project: `.codex/config.toml`. Global (`--global`): `~/.codex/config.toml`.

### 2. OpenCode (sst/opencode) support

```bash
radiant setup-mcp        # auto-detects if .opencode/ exists
radiant setup-mcp --agent=opencode
```

Writes JSON config:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "radiant": {
      "type": "local",
      "command": ["/usr/local/bin/radiant", "mcp", "serve"]
    }
  }
}
```

Project: `.opencode/config.json`. Global: `~/.config/opencode/config.json`.

## What was NOT added (and why)

User requested 8 agents. 2 added (Codex, OpenCode). 6 skipped:

| Agent | Skipped because |
|-------|-----------------|
| **hermes** | NousResearch Hermes is a model family, not an MCP host. Unclear which agent was meant. |
| **MiniMax code** | Not a recognised public MCP host. |
| **kimi code** | `kimi-cli` MCP support not stable in upstream yet. |
| **open claw** | Not a recognised public MCP host. |
| **lang chain** | Framework, not MCP host. Wrap the harness as an MCP tool from inside the LangChain agent. |
| **lang graph** | Same — framework. |

For any of these, the user can:
- Open an issue with the agent's MCP config schema and path, and we'll add it.
- Build a custom wrapper that calls `radiant mcp serve` from the agent.

## Stats

- 2 new concrete agents supported.
- **997 tests passing across 30 packages, 0 failures**.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 1 modified file, 1 new test file, 1 new doc.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.43.0
# or:
git pull
make build
./bin/radiant --version                  # should report 2.43.0
./bin/radiant setup-mcp --help           # lists codex, opencode in agent flag
cd /your/project/with/.codex/  # or create .opencode/
./bin/radiant setup-mcp --dry-run       # preview the config writes
./bin/radiant setup-mcp                 # actually write
```

---

# Release Notes — v2.42.0 (Light/Full by subcommand, not by flag)

> "Behaviour emerges from the subcommand." v2.42.0 collapses the
> Light/Full mode resolution chain into a single, unambiguous rule:
> `radiant mcp serve` is always Light; everything else is always Full.

## Headlines

### 1. No more `--mode` flag

v2.37.0 introduced Light and Full as runtime modes with a 4-level
resolution chain (flag > env > config > auto-detect). Operators
struggled with:
- Forgetting to pass `--mode=light` when using Claude Code
- Setting `RADIANT_MODE=full` on a host that expected sampling
- Trying to run `radiant loop start --mode=light` (nonsensical — loop is Full)

v2.42.0 fixes all of this by removing the choice entirely:

```bash
# Light — MCP sampling from host agent. No API key needed.
radiant mcp serve

# Full — autonomous HTTP to LLM providers. API key needed.
radiant loop start "..."
radiant run specs/...
radiant fleet start "..." --agents 5
```

That's it. Two entry points, two behaviours, zero ambiguity.

### 2. `radiant mcp serve` is always Light

Removed the `--sampling` flag. The MCP server always uses sampling
now — that was the only point of `mcp serve`. The harness emits
`sampling/createMessage` to the host agent (Claude Code, Hermes,
etc.) for every inference.

If you run `radiant mcp serve` from a TTY (terminal), you get a
warning that the server expects to be invoked from an MCP host.
The process still runs (useful for debugging).

### 3. Every other subcommand is always Full

`radiant loop start`, `radiant run`, `radiant fleet start`, `radiant init`,
`radiant validate`, ... all run in Full mode by default. They call
LLM HTTP endpoints directly with the operator's API key.

## Removed in v2.42.0

| v2.37.0–v2.41.0 | v2.42.0 status |
|----------------|----------------|
| `radiant mode show` | **removed** — use `radiant --help` |
| `radiant mode set light\|full` | **removed** — subcommand defines it |
| `--mode` flag on `loop start` | **removed** — `loop` is always Full |
| `--mode` flag on `fleet start` | **removed** — `fleet` is always Full |
| `--sampling` flag on `mcp serve` | **removed** — `mcp serve` is always sampling |
| `RADIANT_MODE` env var | **removed** — ignored if set |
| `mode:` field in `.radiant.yaml` | **removed** — ignored if set |
| `internal/mode.Resolve()` chain | **removed** — replaced by simple constants |

## Stats

- 1 file deleted (`cmd/radiant/cmd_mode.go`).
- 1 file rewritten (`internal/mode/mode.go` — 215 → 50 LOC).
- 4 files modified.
- **982 tests passing across 30 packages, 0 failures**.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.42.0
# or:
git pull
make build
./bin/radiant --version                # should report 2.42.0
./bin/radiant mcp serve --help         # Light path documented
./bin/radiant loop start --help        # no --mode flag anymore
./bin/radiant doctor                   # simplified mode check
```

If you were using `--mode=light` on `loop start` or `fleet start`:
those subcommands are **always** Full now. Use `radiant mcp serve`
for the Light path.

If you were using `RADIANT_MODE=light` or `mode: light` in
`.radiant.yaml`: silently ignored. Use `radiant mcp serve` instead.

See [`docs/MODES.md`](docs/MODES.md) for the full guide.

---

# Release Notes — v2.41.0 (MCP Tool-Bridge Adapter)

> "Any MCP server, any tool." Operators can now register external
> MCP servers as tool sources. Tools from those servers appear in
> the local registry alongside the built-in four.

## Headlines

### 1. `radiant run --mcp-bridge`

Register an MCP server as a tool source. Repeatable.

```bash
radiant run specs/0001-foo \
  --mcp-bridge "github:npx -y @modelcontextprotocol/server-github" \
  --mcp-bridge "fs:npx -y @modelcontextprotocol/server-filesystem ."
```

The bridge dials the server, performs the `initialize` handshake,
discovers the advertised tools via `tools/list`, and converts each
into a `tools.Tool` bound to the local registry. Tools are
namespaced as `<bridge>__<tool>` (e.g. `github__create_issue`).

### 2. JSON-RPC 2.0 over stdio

`internal/mcpbridge/` implements the MCP spec's stdio transport —
the same wire format any MCP client speaks. The client:

- Performs the `initialize` handshake on connect
- Tracks pending responses by ID via `sync.Map`
- Honours context cancellation and per-RPC timeouts
- Surfaces `isError=true` results as structured errors
- Closes gracefully (SIGTERM, then SIGKILL after 2s)

### 3. MCP tool → tools.Tool conversion

JSON Schema `inputSchema` is flattened into `tools.Param` slices.
Type, description, and `required` are propagated. Complex nested
schemas pass through as opaque `object` params so the LLM still
sees the raw structure in the description.

## Stats

- 1 new package: `internal/mcpbridge/` (client + registry +
  bridge + mock + tests, ~900 LOC).
- **989 tests passing across 29 packages, 0 confirmed failures**
  (1 pre-existing flaky documented).
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 1 new CLI flag: `--mcp-bridge` (repeatable).

## Compatibility

- No breaking changes. Built-in tools keep working unchanged.
- `loop.RealRegistry` signature changed to `(*Registry, error)` —
  callers via `tools.RealRegistry()` indirection are unaffected.
- `--mcp-bridge` is opt-in. Default behaviour unchanged.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.41.0
# or:
git pull
make build
./bin/radiant --version                          # should report 2.41.0
./bin/radiant run specs/0001-foo \
  --mcp-bridge "github:npx -y @modelcontextprotocol/server-github"
```

See [`docs/TOOL-USE.md`](docs/TOOL-USE.md) for the full operator
guide.

---

# Release Notes — v2.40.0 (Tool Use Wire-up Parte 3: run_gate)

> "Close the trio." `run_gate` is now concrete. The RealRegistry
> ships 4 structured tools; LLM can read, write, search, and gate
> through a uniform `tool_call` interface.

## Headlines

### 1. `run_gate` tool

```markdown
```tool_call
{"name": "run_gate", "args": {"command": "go test ./..."}}
```
```

Returns `{command, exit_code, duration_ms, output, output_bytes,
truncated}`. Allowlist-enforced via `policy.ValidateGateCommand`
(closed set of binaries + no dangerous operators). Runs in the
project directory; 5-minute timeout via `gaterun.Timeout`; output
capped at 10 MiB (or per-call `max_output`).

### 2. 4 concrete tools in the registry

```
$ radiant tools list --real
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
write_file      Write content to a file at the given path (project-relati... 2
read_file       Read the contents of a file at the given path (project-re... 1
search_code     Search the project for a regex pattern. Returns matching ... 4
run_gate        Run a quality gate command (go test, go vet, etc.).          3
```

The trio of read/write/gate is closed. Future tools (Sprint 72+)
move to the next frontier: SDK-level function-call parsing, MCP
tool-bridge adapter.

## Stats

- 1 new concrete tool (`run_gate`).
- **~995 tests passing across 29 packages, 0 confirmed failures**
  (validated with `go test -count=1 -v ./...`). `go vet ./...`
  clean.
- Cross-compile OK: linux/amd64 (15 MB), darwin/arm64 (14 MB),
  windows/amd64 (15 MB).
- 4 new files, 2 modified. ~830 LOC.

## Compatibility

- No breaking changes. `run_gate` is opt-in via the existing
  `Engine.ToolRegistry` wiring.
- LLM outputs that contain only `write_file`/`read_file`/`search_code`
  keep working unchanged.
- `--no-tools` still forces the legacy code-block path.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.40.0
# or:
git pull
make build
./bin/radiant --version             # should report 2.40.0
./bin/radiant tools list --real     # 4 tools (was 3 in v2.39.0)
```

See [`docs/TOOL-USE.md`](docs/TOOL-USE.md) for the full operator
guide.

---

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
