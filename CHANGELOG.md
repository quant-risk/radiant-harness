# Changelog

All notable changes to `radiant-harness` (Light) are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

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