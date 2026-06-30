# Release v3.7.2 — Close the drop-in install gap

**Tag:** `v3.7.2`
**Date:** 2026-06-30
**Type:** Bug fix + Feature

## TL;DR

Three install-path bugs reproduced on the 2026-06-29 drop-in rehearsal
are fixed:

1. `install.sh:resolve_latest` SIGPIPE on macOS arm64 → exit 141 silent.
2. `install.sh` post-install `set -u` AGENT_NAME unbound → exit 1.
3. INSTALL.md go install snippet broken (v3.2.0 path), table missing
   MiniMax row, `radiant_run` referenced instead of
   `mcp__radiant__possess`.

After v3.7.2 the canonical `curl | bash` line ends in a working `v3.7.2`
binary with all 7 MCP tools wired on every supported host agent.

Also in this release:

- **Module repath to `/v3`** — closes the v0.7.0-vs-v3 line
  contradiction for `go install`. (Note: the module proxy still mirrors
  v0.7.0 for the legacy path; tracked in Q1 — see Notes below.)
- **`mcp__radiant__run_gate` and `mcp__radiant__possess_async` are real** —
  the v3.7.2-prep stubs are gone. Synchronous-host agents (Hermes TUI)
  can now drive the 4-phase loop through 4 short MCP calls instead of
  the 120 s blocking one.
- **`make audit-install`** — new drift detector wired into `make smoke`.
- **AGENTS-FOR-TASKS.md alias normalisation** — `mavis-code` → `minimax`.

## Verified

- 31 packages `go test ./... —short` PASS (1 pre-existing flaky in
  fleet, runs clean in isolation).
- `make smoke` — audit-skills 6/6, audit-docs 46/57 (0 drift),
  audit-install 2 PASS + 1 SKIP, smoke-test 16/16.
- `make test-agents` — 12/12 PASS on the cross-agent matrix.
- `make release` — 6 cross-platform targets built, all 11–12 MB,
  zero HTTP-LLM symbols on every target.
- E2E rehearsal against public v3.7.2: Path A (`curl | bash`) PASS,
  Path B (direct-tarball) PASS.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh | bash
```

Verify:

```bash
RADIANT_VERSION=v3.7.2 bash <(curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh) --prefix=/usr/local/bin
/usr/local/bin/radiant --version   # → v3.7.2
/usr/local/bin/radiant mcp self-test   # PASS, 7 tools
```

## Notes

- **Q1 — Go module proxy.** `proxy.golang.org` still mirrors the
  v0.7.0 line for the un-versioned path. `go install
  github.com/quant-risk/radiant-harness/v3/cmd/radiant@latest`
  will work after the proxy re-indexes — a `pkg.go.dev` operator-side
  action. `make audit-install` flags this as SKIP. Workaround is the
  `curl | bash` install path.
- **Q2 — Hermes TUI synchronous-TUI deadlock at the MCP protocol
  layer.** v3.7.2 ships the gate primitives; the protocol-level
  deadlock is independent. Hosts that need end-to-end execution in a
  synchronous TUI use the 4-tool hybrid pattern documented in
  `AGENTS-FOR-TASKS.md`.
- **`radiant_run` alias** is now **DEPRECATED** (kept for back-compat).
  New code must call `mcp__radiant__possess`.

## Asset manifest

- `radiant-darwin-arm64` — 11 326 786 bytes
- `radiant-darwin-amd64` — 12 169 152 bytes
- `radiant-linux-arm64` — 11 075 746 bytes
- `radiant-linux-amd64` — 11 940 002 bytes
- `radiant-windows-arm64.exe` — 11 229 696 bytes
- `radiant-windows-amd64.exe` — 12 292 096 bytes
- `SHA256SUMS`

SHA256 hashes match the file listed at the end of the release.
Verify locally:

```bash
curl -fsSL https://github.com/quant-risk/radiant-harness/releases/download/v3.7.2/SHA256SUMS | shasum -a 256 -c
```
