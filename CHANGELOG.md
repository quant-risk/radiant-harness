# Changelog

All notable changes to `radiant-harness` (Light) are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

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