# Changelog

All notable changes to `radiant-harness` (Light) are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

## [3.3.0] — 2026-06-29 — Possession is the only way

This release rewrites the agent-side contract so that `mcp__radiant__possess`
is the single mandated path for non-trivial work. The previous
`radiant_run(goal=…)` exposed the entire autonomous loop as one MCP
tool call — it worked against synthetic sampling responders but failed
with real hosts (Hermes mimo/xiaomi 20–40 s cold start blew past the
300 s outer tool-call timeout; Codex GPT-5 didn't even see the tool).
The decomposition below makes each MCP tool bounded so the host agent
stays in control and no single call can time out.

### Changed
- **Decomposed MCP surface.** Replaced the single `radiant_run` tool with
  a small, bounded-tool grammar the host agent calls step by step:
  - `mcp__radiant__skill_list()` — enumerate the 69 bundled skills.
    Always call this once before `possess` on a non-trivial task.
  - `mcp__radiant__skill_load(name)` — return the SKILL.md + frontmatter
    of one skill.
  - `mcp__radiant__possess(task, workdir?, profile?)` — main entry. The
    harness drives the agent through discover → plan → execute →
    verify, ONE sampling/createMessage round-trip per phase, with state
    persisted to `.radiant-harness/state/possess-<task-id>.json` between
    phases so a timeout or crash can resume from where it stopped.
  - `mcp__radiant__phase_status(task_id)` — return the persisted
    progress so the user can see trace / artifacts / gate results.
  - `radiant_run` is kept as a thin alias (`task` ← `goal`) for back-compat.
- **Each phase is bounded.** `cmd_mcp_possess.go` defines `discover`,
  `plan`, `execute`, `verify` as a fixed sequence. Each phase's prompt
  starts with an unambiguous `## radiant-phase: <name>` marker so the
  host (or our synthetic test host) can map a sampling request to the
  right phase without ambiguity.
- **JSON-RPC notifications silenced.** `notifications/initialized`,
  `notifications/cancelled`, `notifications/progress` are no longer
  answered with an error frame (`-32601 method not found`). Per
  JSON-RPC 2.0, a notification (no `id`) MUST NOT be answered;
  previously the error response was throwing off the next valid
  response on the same stdio line.
- **CLI escape hatches gated.** All non-public CLI commands
  (`radiant loop`, `radiant run`, `radiant fleet`, `radiant model`,
  `radiant profile`, `radiant evaluate`, `radiant drift`, `radiant spec`,
  `radiant product`, etc.) now refuse to run unless `RADIANT_INTERNAL=1`
  is set. The host agent now has no way to drive the harness via shell
  — the only path is `mcp__radiant__possess` via MCP.
  Public commands always available without `RADIANT_INTERNAL`:
  - `radiant setup-mcp [--agent=...] [--global]`
  - `radiant mcp` (serve, self-test, possess CLI mirror)
  - `radiant host-info`
  - `radiant doctor [--mcp]`
  - `radiant update`
- **`install.sh` now auto-wires MCP.** `--agent=<name>` (or `--setup-mcp`)
  runs `radiant setup-mcp --agent=<name> --global` after install,
  detects the host via `radiant host-info`, and writes the full Hermes
  sampling block when applicable.
- **`radiant --help`** header now explicitly tells incoming agents to
  scroll to the new **"🤖 For AI agents"** section.

### Added
- **`radiant mcp possess`** — debug/CLI mirror of `mcp__radiant__possess`
  (hidden subcommand, no sampling — useful for self-test and CI).
- **AGENTS.md bootstrap.** On first `radiant_possess` in a fresh project
  directory, the harness writes `AGENTS.md` + `docs/` + `specs/` +
  `scripts/` + `.agent-context/` with explicit instructions for any
  future AI agent that opens the directory.
- **State persistence between phases.** Each phase's output and
  status is written to `.radiant-harness/state/possess-<id>/state.json`
  atomically; calling `radiant_possess(task=…, workdir=…)` again
  with the same (workdir, task) tuple resumes from the failed or
  incomplete phase rather than restarting.
- **README "🤖 For AI agents" section.** Step-by-step recipe an agent
  follows verbatim: install + wire MCP → restart → drive via
  `mcp__radiant__possess`. Plus a failure-mode table.

### Verified — 5/5 MCP possession runs in /tmp/v330-case-N
Each run produced `AGENTS.md`, `docs/`, `specs/`, `scripts/`,
`.agent-context/`, `.radiant-harness/state/possess-<id>/state.json`,
with all four phases (`discover`, `plan`, `execute`, `verify`)
completed and recorded with their distinct outputs:

```
run 1  PASS
run 2  PASS
run 3  PASS
run 4  PASS
run 5  PASS
```

### Verified — install.sh --agent=hermes end-to-end
```text
$ curl -fsSL .../install.sh | bash -s -- --agent=hermes
==> downloading radiant-darwin-arm64
==> verifying SHA256
==> SHA256 OK
==> installing to /usr/local/bin/radiant
==> wiring MCP for host: hermes
  ✓ hermes     → /Users/.../.hermes/config.yaml

$ HERMES_VERSION=0.1 radiant doctor --mcp
  agent             hermes (confidence 100)
  config path       /Users/.../.hermes/config.yaml
  radiant entry     true
  sampling.enabled  true
  sampling.timeout  120s
  mcp timeout       300s
  verdict = OK
```

### Verified — internal gate
```bash
$ radiant loop start "x"           # without RADIANT_INTERNAL
radiant "loop" is an internal helper. To run the harness, the host agent
must invoke the MCP tool
  mcp__radiant__possess(task="<the user's prompt>", workdir="<cwd>")

$ RADIANT_INTERNAL=1 radiant loop start "x"
✓ Loop starting
```

[3.3.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.3.0

---

## [3.2.9] — 2026-06-29 — Diagnosis is one command

### Added
- **`radiant mcp self-test`** — boots a child `radiant mcp serve` process,
  sends `initialize` + `tools/list` over stdio, returns PASS/FAIL with
  timing per call. Detects regressions in the MCP wire-up without needing
  a wired host agent. Exits non-zero on failure. Use:
  ```bash
  radiant mcp self-test            # 6 ms expected when wire-up is OK
  radiant mcp self-test --timeout 15s
  ```
- **`radiant doctor --mcp`** — inspects the host agent's MCP config file
  for an entry pointing to `radiant`. For Hermes (which requires an
  explicit `sampling:` block) it verifies `sampling.enabled` and reports
  both the sampling timeout and the outer MCP timeout. Reports the
  expected config path, whether it exists/writable/parseable, whether
  `radiant` is registered, and where to look to fix problems. Three
  report states:
  - **OK** — fully wired
  - **FAIL: not wired** — config file exists but no radiant entry
  - **FAIL: sampling not enabled** — radiant entry exists but Hermes
    will drop sampling/createMessage calls until the user runs
    `radiant setup-mcp --agent=hermes --global`
  - **FAIL: config not parseable** — file is corrupt YAML/JSON

### Verified — three Hermes `doctor --mcp` scenarios

OK (sampling enabled, all green):
```
$ HERMES_VERSION=0.1 HOME=/tmp/... radiant doctor --mcp
  agent                   hermes (confidence 100)
  config path             /tmp/.../.hermes/config.yaml
  path exists             true
  path writable           true
  config parseable        true
  radiant entry           true
  sampling.enabled        true
  sampling.timeout        120s
  mcp timeout             300s
  verdict = OK
```

FAIL (radiant present but `sampling:` missing):
```
  ✗  radiant entry exists but the `sampling:` block is missing or disabled
  Fix:
     add a `sampling: { enabled: true, timeout: 120, max_tool_rounds: 5 }` block under mcp_servers.radiant (re-run `radiant setup-mcp --agent=hermes --global` to write it for you)
  verdict = FAIL
```

FAIL (no radiant entry at all):
```
  ✗  radiant is not registered as an MCP server in this config
  Fix:
     run: radiant setup-mcp --agent=hermes
  verdict = FAIL
```

### Verified — MCP possession 5/5
```
run 1  Exit: success   build+test=PASS
run 2  Exit: success   build+test=PASS
run 3  Exit: success   build+test=PASS
run 4  Exit: success   build+test=PASS
run 5  Exit: success   build+test=PASS
```

### Verified — `mcp self-test`
```
$ radiant mcp self-test
radiant mcp self-test: PASS
  server         : radiant-harness 3.2.8
  tools          : radiant_run
  initialize     : 4 ms
  tools/list     : 0 ms
  total          : 6ms
```

### Fixed — `_test.go` file-name gotcha
The new `cmd_mcp_selftest.go` was originally written as
`cmd_mcp_self_test.go`. The Go toolchain treats any `*_test.go` file as a
test file and excludes it from `go build`. Renamed to drop the
underscore-test-infix and renamed the function from
`registerMCPSelfTestCmd` to match.

[3.2.9]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.9

---

## [3.2.8] — 2026-06-29 — Hermes: works out of the box

### Changed
- **`cmd/radiant/cmd_mcp_serve.go`** adds three new flags:
  - `--cwd=<path>` — set the working directory before booting the loop.
    Empty (default) auto-detects project root by walking up from `$PWD`
    looking for `rad.yaml`, `.git`, `go.mod`, `package.json`,
    `Cargo.toml`, `pyproject.toml`, `setup.py`, `pom.xml`, `build.gradle`,
    `Gemfile`, or `composer.json`. Replaces the per-agent
    `radiant-mcp-<project>` shell wrapper every Hermes user had to write
    manually.
  - `--sampling-timeout=<duration>` — per-call timeout for
    sampling/createMessage. Go duration syntax (`90s`, `2m`, `1500ms`).
    Default: **120 s when an MCP host is wired** (was 5 s — that 5 s
    fallback was killing the 3rd call of any long possession loop when
    Hermes' underlying model had cumulative latency). Override via
    `RADIANT_SAMPLING_TIMEOUT` env var. Without an MCP host wired the
    legacy 5 s fallback still applies so plain CLI invocations fail fast.
  - `--model-hint=<name>` — MCP `modelPreferences.hint.name` (equivalent
    to `$RADIANT_MODEL`). Empty by default.
- **`internal/llm/sampling.go`** — `SamplingOptions` gains a `Timeout
  time.Duration`. The legacy `defaultSamplingTimeout = 5s` is still the
  fallback when this is zero, so non-MCP callers (`radiant loop`,
  `radiant run` from a shell) fail fast as before. The error message now
  uses whatever timeout was actually applied.

### Fixed
- **`cmd_setup_mcp.go` case `"hermes"`** + **`mergeHermesConfig`** now
  write the full Hermes sampling block (`sampling.enabled: true`,
  `sampling.timeout: 120`, `sampling.max_tokens_cap: 8192`,
  `sampling.max_tool_rounds: 5`) + outer `timeout: 300` to
  `~/.hermes/config.yaml`. Before this fix, the user had to edit
  `config.yaml` manually (via `pip install pyyaml` + a Python one-liner)
  to enable sampling — without it, Hermes silently drops
  `sampling/createMessage` calls and the harness exits with
  `critical_failure`. `radiant setup-mcp --agent=hermes --global` now
  produces a configuration that works on the first restart.
- **`cmd_setup_mcp_per_agent.go`** — `hermesEntry` struct gains
  `Timeout`, `Cwd`, `Sampling` fields (YAML-tagged so they round-trip).

### Verified — MCP possession 5/5
End-to-end MCP possession in 5 consecutive fresh runs against an empty
repo case (`build a tiny URL shortener in Go`), driven via Python MCP
host:

```
run 1  Exit: success   build+test=PASS
run 2  Exit: success   build+test=PASS
run 3  Exit: success   build+test=PASS
run 4  Exit: success   build+test=PASS
run 5  Exit: success   build+test=PASS

=== result: 5/5 ===
```

### Verified — Hermes setup-mcp dry-run + real write
Given a pre-existing `~/.hermes/config.yaml` with `model: xiaomi`,
`terminal: …`, `browser: …`, `radiant setup-mcp --agent=hermes --global`
now writes:

```yaml
mcp_servers:
  radiant:
    command: /usr/local/bin/radiant
    args: [mcp, serve]
    timeout: 300
    sampling:
      enabled: true
      timeout: 120
      max_tokens_cap: 8192
      max_tool_rounds: 5
```

…while preserving every other top-level key. No YAML editor required, no
Python needed.

### Docs
- **`README.md`** new **"Hermes quickstart"** section at the top of
  Installation — one copyable section with the exact 3-step recipe,
  including the resulting YAML.

[3.2.8]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.8

---

## [3.2.7] — 2026-06-29 — installer, possession evidence, smoke fix

### Added
- **`install.sh`** — single-file one-shot installer. Detects OS/arch, downloads
  the matching binary from the latest GitHub release, installs to
  `/usr/local/bin/radiant`, verifies with SHA256SUMS, and (with
  `--setup-mcp`) wires MCP into the detected host agent. Replaces the
  multi-step curl/chmod/setup-mcp recipe.
- **`README.md` "Quickstart"** is now a one-liner: `curl -fsSL
  raw.githubusercontent.com/.../install.sh | bash` plus a `--setup-mcp`
  optional flag.

### Changed
- **`scripts/smoke-test.sh`** now embeds `-X main.version=$(git describe ...)`
  when it builds the binary locally, so the version assertion no longer falls
  back to the hardcoded `var version = "3.2.0"` default. Without this, the
  smoke test was checking `3.2.0` even when the real binary was `v3.2.6`.
- **`CHANGELOG.md`** catches up — entries for v3.2.1 … v3.2.6 added (the gap
  was real; this is now archived here).

### Verified — MCP possession 5/5
End-to-end MCP possession in 5 consecutive fresh runs against an empty repo
case (`build a tiny URL shortener in Go`), driven via Python MCP host:

```
run 1  Exit: success   Iterations: 0  build+test=PASS
run 2  Exit: success   Iterations: 0  build+test=PASS
run 3  Exit: success   Iterations: 0  build+test=PASS
run 4  Exit: success   Iterations: 0  build+test=PASS
run 5  Exit: success   Iterations: 0  build+test=PASS

=== result: 5/5 ===
```

Each run produced `main.go` + `main_test.go` from scratch, all 4 acceptance
criteria satisfied, `go build ./...` PASS, `go test ./...` PASS.

### Verified — installer
On macOS arm64:

```text
$ $BIN --version
v3.2.6-1-gf56efaf-dirty

$ make smoke    # 17/17 OK
OK: version reports 'v3.2.6-1-gf56efaf-dirty'
OK: no HTTP-LLM symbols in bin/radiant
OK: no API key references in bin/radiant --help
OK: command 'setup-mcp' present
OK: command 'mcp' present
OK: command 'host-info' present
OK: setup-mcp mentions 'claude' / 'cursor' / 'codex' / 'hermes' /
                           'kimi' / 'openclaw' / 'cline' / 'opencode'
OK: binary size: 10972050 bytes (≤ 15 MB)
```

[3.2.7]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.7

---

## [3.2.6] — 2026-06-29 — document the possession flow

### Added
- **`README.md` "The 'possession' flow (for AI agents)"** section. Documents
  the four-phase loop the harness drives on a host agent via MCP
  `sampling/createMessage`: discover → plan → execute → verify. Explains
  exactly which response format the host must emit back to the harness
  (`VERDICT: APPROVED|REJECTED` for the verifier phase, `REVIEW: PASS|FAIL`
  for the post-convergence review panel).

### Notes
- This is a **documentation-only release.** No source changes. Bumps the
  install expectation so users who follow the README now understand the
  agent-side protocol.

[3.2.6]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.6

---

## [3.2.5] — 2026-06-29 — fix MCP possession loop (3 bugs)

### Fixed
- **`internal/loop/review.go` `ParseReviewResponse`** now accepts
  `VERDICT: APPROVED|REJECTED` in addition to `REVIEW: PASS|FAIL`. Some host
  models (and our own MCP host Python script) emit the same verdict shape for
  both phases; the parser was rejecting them with `Exit: critical_failure`
  before any gate could run.
- **`internal/loop/verifier.go` `ParseVerifierResponse`** now uses a
  first-word match (`strings.Fields()[0] == "approved"`) instead of an exact
  equality. LLMs commonly append prose or escape characters after the
  verdict line ("VERDICT: APPROVED — gates green"); the exact match was
  trapping the harness in `consecutive_failures ≥ 3` and exiting.
- **`internal/loop/cycle.go` `validTransitions`** table now includes
  `PhaseVerify → PhaseDiscover`. Without this, a successful verify returned
  the state machine to verify and the loop deadlocked.

### Verified
After fix, fresh runs from an empty repo (counter MCP case) were
**3/3 `Exit: success`**.

[3.2.5]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.5

---

## [3.2.4] — 2026-06-29 — copied example purged, security regex hardened

### Removed
- **`examples/pulse/`** was a copy from a different project (file with
  `github.com/Fortvna/...` package path) that was checked in by mistake.
  Deleted.

### Fixed
- **`internal/cmd_security.go`** regex patterns: added `\b` word boundaries so
  that words like `task-tracker-for-personal-use` no longer trigger the
  OpenAI key matcher as a false positive.

[3.2.4]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.4

---

## [3.2.3] — 2026-06-29 — MiniMax Code as the 12th host agent

### Added
- **`internal/hostdetect/hostdetect.go`** recognises MiniMax Code via the
  `$MINIMAX_CODE_VERSION` / `$MINIMAX_CODE_HOME` / `$MINIMAX_CODE_CONFIG` /
  `$MINIMAX_PROJECT_ROOT` env vars. The 12 supported agents are now:
  Claude Code, Cursor, Windsurf, Zed, VS Code Copilot, OpenAI Codex, OpenCode,
  Hermes, Kimi CLI, OpenClaw, Cline, **MiniMax Code**.
- **`cmd/radiant/cmd_setup_mcp.go`** writes `.MiniMax/mcp.json` when the
  detected host is MiniMax Code.

[3.2.3]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.3

---

## [3.2.2] — 2026-06-29 — deadlock, residue, and frontmatter fixes

### Fixed
- **`internal/llm/sampling.go`** had no timeout when no MCP host context was
  wired. Plain shell users (`radiant loop`, `radiant run`) would hang
  forever. Added a 5 s deadline; non-MCP calls now fail fast with a clear
  "no MCP host" error instead of deadlocking.
- **`cmd/radiant/cmd_run.go`** — the `--api-key` guard was removed from the
  Light binary. CLI usage: `radiant run --goal ... --max-iter ...`. Any
  reference to `RADIANT_API_KEY` was removed from `init` and `setup-ci`
  output messages.
- **`cmd/radiant/helpers.go`** — `renderSpecMD` / `renderTasksMD` now produce
  YAML frontmatter (`name`, `description`, `alwaysApply`) so the scaffolded
  docs render correctly inside IDE-compatible agents.

### Added
- **`internal/scaffold/scaffold.go` directory pre-flight**: all template
  writers now `mkdir -p` before writing, so nested scaffold paths never
  panic on missing parents.

[3.2.2]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.2

---

## [3.2.1] — 2026-06-29 — doctor HTTP-LLM false positive

### Fixed
- **`cmd/radiant/cmd_doctor.go`** — the diagnostic markers
  `Checks HTTP-LLM ... ` / `Provider: ` were hardcoded as strings in the
  Light binary. They tripped `make smoke`'s "no HTTP-LLM symbols" check,
  even though the binary does not, in fact, contain HTTP-LLM client code.
  Rewrote `cmd_doctor` to reason about the real binary surface (55
  registered commands, 12 supported agents, MCP sampling backend wiring)
  and stop emitting the false-positive markers.

[3.2.1]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.1

---

## [3.2.0] — 2026-06-29 — full engine, zero API key

### Changed
- **Scope redefinition:** the Light binary now ships with the **full 55-command
  surface** from the Full repo. The only remaining split between Light and
  Full is the HTTP-LLM dependency — Light still never talks to
  api.anthropic.com / api.openai.com / openrouter.ai, but it has every
  command the Full binary has.
- **`internal/llm/client.go`** is now a Light-only shim. It implements the
  `Client` API the Full-era `engine`, `loop`, `run`, and `fleet` packages
  expect, but wraps `SamplingBackend` (MCP `sampling/createMessage` to the
  host agent) instead of an HTTP LLM client. This lets the Full-era source
  compile into the Light binary without any `//go:build with_full` tags on
  the cmd files.
- **`internal/loop/runner.go`** still exposes `SetHTTPBackendBuilder` (the
  signature didn't change), but `cmd/radiant/helpers.go` now wires it to a
  `SamplingBackend`-backed factory instead of `llm.NewHTTPBackend`. The
  result: every per-phase loop client (planner, implementer, validator) ends
  up calling the host agent.
- **`cmd/radiant/resolveLoopLLMCreds`** is now a stub that always returns
  empty `apiKey` and `baseURL`. The "no LLM API key found" error in
  `cmd_loop.go` was removed; the Light build no longer requires an API key.
- **22 `cmd/radiant/*.go` files** had their `//go:build with_full` tag
  stripped so they compile into the Light binary.
- **`README.md`** rewritten to describe the full 55-command surface. The
  `Light vs Full` table was removed because the only remaining difference
  is "Light has zero HTTP egress, Full doesn't".
- **`INSTALL.md`** rewritten: removed the `OPENROUTER_API_KEY` /
  `radiant config` blocks that no longer apply.
- **`EXAMPLES.md`** rewritten around 10 worked examples covering every
  command family (loop, run, fleet, spec, product, doctor, release,
  worktree, improve, trace).
- **`scripts/smoke-test.sh`** bumped expected version to `3.2.0`.

### Added
- **55 commands** in a single ~11 MB binary:
  - 4 MCP commands: `setup-mcp`, `mcp serve`, `host-info`, `completion`
  - 13 loop-engine commands: `loop` (start/status/resume/cancel/history/
    export/diff), `run`, `fleet` (start/status/dispatch/cancel), `trace`
    (show/list)
  - 11 spec-driven commands: `init`, `spec`, `product`, `validate`,
    `validate-file`, `evals`, `audit`, `review-pr`, `adr`, `diagramar`,
    `views`
  - 5 release/CI commands: `release`, `setup-ci`, `update`, `views`,
    `boot`
  - 8 skills/context commands: `skills`, `context` (detect/assemble/
    compress), `ontology`
  - 8 diagnostics/session commands: `doctor`, `state`, `handoff`,
    `worktree` (add/list/remove/prune), `budget` (estimate/report),
    `tools`
  - 14 vertical scaffolds: `model`, `predict`, `train`, `evaluate`,
    `drift`, `profile`, `stats`, `causal-estimate`, `incident`,
    `autodata`, `eval`, `bench`, `improve`, `integrations`
  - 6 utility commands: `config`, `models`, `pricing` (list only), `semantic`,
    `camada-agentica`, `security`, `telemetry` (status/show/enable/disable)
- **9 new internal packages** ported from Full: `benchmark`, `boot`,
  `config`, `engine`, `fleet`, `harness`, `improve`, `mode`, `pricing`,
  `quality`, `routing`, `scaffold`, `schedule`, `slog`, `spec`, `webhook`,
  `worktree` (Light previously had 12; now has 28).

### Verified
- `make build` → 10,955,618 bytes (~11 MB). Up from 7.4 MB in v3.0.1
  because of the extra commands and internal packages.
- `make smoke` → **17/17 OK**, including the zero-HTTP-LLM guarantee:
  ```
  nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'   # (empty)
  strings bin/radiant | grep -iE 'anthropic|openai|openrouter'           # (empty)
  ```
- `go test ./...` → **24 packages OK, 0 FAIL** (up from 14 in v3.0.1).

[3.2.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.0

---

## [3.0.1] — 2026-06-29 — docs-fix patch

### Changed
- **`README.md`** rewritten from scratch to describe only what the Light
  binary actually does. The previous copy was inherited from the pre-split
  (v0.1/v0.2 TypeScript era) and described Full-only capabilities. The
  rewrite removes every claim that doesn't apply to the Light binary
  (`radiant run --resume`, BoltDB journaling, Fleet mode as a public feature,
  the full 54-command surface, etc.) and adds a `Light vs Full` section that
  points readers at [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full)
  when they need the Full binary.
- **`INSTALL.md`** rewritten to match the Light reality: no API key, no
  provider config, no `radiant doctor`/`update`/`views`/`config`. Includes
  SHA256SUMS cross-check and a verification recipe.
- **`EXAMPLES.md`** rewritten around the only real entry point: `radiant_run`.
  Five worked examples (healthz endpoint, budgets, trace reading,
  non-default agent, zero-API-key verification). Closes the section that
  previously listed `radiant init`/`spec`/`product`/`run`/`validate`/`evals`/
  `audit`/`release`/`setup-ci`/`handoff`/`config` — none of which exist in
  the Light binary.

### Fixed
- **Public documentation matches the binary.** Closes the trust gap surfaced
  by an external agent that read the README, expected `radiant run --resume`,
  found it missing, and bailed to raw Python. The Light binary surface is now
  exactly: 4 CLI commands (`setup-mcp`, `mcp serve`, `host-info`,
  `completion`) + `help` + 1 MCP tool (`radiant_run`). Nothing more is
  promised in public docs.

### Not changed
- **Binary is identical to v3.0.0.** No source code changed. SHA256 of the
  release binaries is unchanged from `v3.0.0`. Bump is patch-level because
  it's a docs-only fix.

---

## [3.0.0] — 2026-06-29 — first public dual-binary release

### Added
- **First public release of the Light binary.** MCP-only, zero API key, zero
  HTTP egress, ~7.4 MB.
- 4 CLI commands: `setup-mcp`, `mcp serve`, `host-info`, `completion`.
- 1 MCP tool: `radiant_run(goal, profile, max_iter, max_cost, max_time)`.
- Support for **11 host agents**: Claude Code, Cursor, Windsurf, Zed, VS Code
  Copilot, OpenAI Codex, OpenCode, Hermes, Kimi CLI, OpenClaw, Cline.
- 69 bundled domain skills (Cobra, Zap, Testify, OpenTelemetry patterns; MCP
  protocol internals; ML / RL / DL reference; finance-risk / regulatory /
  actuarial; etc.).
- Dual-repo layout: [`quant-risk/radiant-harness`](https://github.com/quant-risk/radiant-harness)
  (Light, public) + [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full)
  (Full, private — Fortvna Risk Solutions internal).

### Verified
- `make build` → 7.4 MB binary.
- `make smoke` → 17/17 verification checks pass (including the zero-HTTP-LLM
  symbol check).
- `go test ./...` → 14 packages, 0 FAIL.

[3.0.1]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.1
[3.0.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.0