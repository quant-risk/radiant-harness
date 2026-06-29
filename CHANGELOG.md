# Changelog

All notable changes to `radiant-harness` (Light) are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

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