# v3.7.2 — Close the drop-in install gap

**Slug:** `v3.7.2-close-dropin-gap`
**Tier:** architecture (CLI installer + Go module + release automation)
**Status:** draft → implementing

---

## Context

A 2026-06-29 drop-in rehearsal reproduced **3 distinct breakages** on the
install path that AGENTS-FOR-TASKS.md recommends as drop-in:

1. The canonical line `curl .../install.sh | bash` exits with **rc 141
   (SIGPIPE)** silently — no "downloading", no SHA256, no install. Caused
   by `tr -d '\r' | grep -m1` race on a 20KB GitHub API body under
   `set -euo pipefail`.
2. `go install .../radiant@v3.2.0` (the line INSTALL.md shows) errors with
   `invalid version: module contains a go.mod file, so module path must
   match major version`. INSTALL.md never updated to a v3-path-compatible
   form.
3. `go install .../radiant@latest` returns **v0.7.0** — the legacy
   TypeScript-era product line still indexed on the Go module proxy
   (`proxy.golang.org/github.com/quant-risk/radiant-harness/@v/list` lists
   only v0.6.0 and v0.7.0). v3.7.1 exists as a GitHub Release but is NOT
   published to the Go proxy. v0.7.0 is a fundamentally different
   product (Full, requires API keys, contains `chatAnthropic` symbols).

A second-order gap found by code audit: AGENTS-FOR-TASKS.md routes
Hermes-TUI users to `radiant_run_gate` + `radiant_possess_async` to
unblock the synchronous deadlock. Those tools exist **only in HEAD**
(v3.7.2-prep work). `git tag` has v3.7.0/v3.7.1 only — `go install`
or `curl` users at the v3.7.1 tag will hit the documented 120s
deadlock they were promised a workaround for.

## Scope

In scope:

- T1 — Fix SIGPIPE in `install.sh:resolve_latest`.
- T2 — Fix `set -u` post-install `AGENT_NAME` unbound warning.
- T3 — Fix INSTALL.md go install snippet (path + version drift).
- T4 — Sync Go module proxy: either publish v3.x tags to the proxy or
  freeze v0.x as the proxy tag. v3 becomes the canonical release.
- T5 — Finish `radiant_run_gate` + `radiant_possess_async` so they are
  usable end-to-end, not just stubs.
- T6 — Add `make audit-install` that exercise-tests every install path
  on a fresh sandbox and chains it into `make smoke`.
- T7 — Cut v3.7.2 release: build, cross-compile all 6 OS/arch targets,
  publish GitHub Release, verify the canonical install line reaches a
  working v3.7.2 binary.

Out of scope:

- Adding new host agents to hostdetect.
- Re-naming `mavis-code` ↔ `MiniMax-code` aliases (separate concern).
- Async plumbing work inside the agentic driver (the v3.7.2 primitives
  are stubs that gate MCP work and emit the documented "in-development"
  response — that contract is signed off; replacing it is the next spec).

## Acceptance Criteria

### AC1 — Canonical install line succeeds end-to-end on macOS arm64

Given a fresh Darwin/arm64 box with `bash ≥ 5.x` and `curl`,
running

```
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh \
  | bash -s -- --agent=claude --global --force --self-for-agent
```

must produce (within ≤ 90 s including network):

- exit code 0
- output containing exactly the milestone lines:
  `detected target`, `resolved latest`, `downloading`, `SHA256 OK`,
  `installed`, `setup-mcp --agent=claude`, `restart your agent`
- `radiant --version` reports `v3.7.2`
- `~/.claude/settings.json` exists and contains a `mcpServers.radiant`
  entry pointing at the installed binary
- `radiant doctor --mcp` reports `verdict = OK` (or `MCP_WIRE_NOT_LOADED`
  documented pre-restart, with instructions printed)

Evidence: a hand-rolled script in `scripts/audit-install.sh` runs the
canonical line in a sandbox `$HOME` + sandbox `proj/`, asserts each
milestone line and reads back `radiant --version` and the agent's MCP
config.

### AC2 — go install @latest returns the v3 line, not v0.x

`go install github.com/quant-risk/radiant-harness/v3/cmd/radiant@latest`
must install ≥ v3.7.2 within 30 s and produce a binary that contains
**zero** of `chatAnthropic`, `HTTPBackend`, `NewHTTPBackend`,
`api.anthropic.com`, `api.openai.com`, `openrouter.ai` symbols/strings.

If the team decides to repath the module, the canonical install
line in AGENTS-FOR-TASKS.md and INSTALL.md **must** match the actual
proxy-published path. The audit-install script asserts this.

### AC3 — INSTALL.md has zero command-doc drift

`make audit-docs` continues to pass (already wired into `make smoke`).
Drift list to close:

- "If you use…" table lists 12 agents (currently lists 11, missing MiniMax).
- "Claude Code → `.mcp.json`" → fix to `.claude/settings.json`.
- "discover `radiant_run`" → fix to `mcp__radiant__possess`.
- "Troubleshooting: My agent doesn't see `radiant_run`" → fix to
  `mcp__radiant__possess`.
- `go install ...@v3.2.0` → either `@latest` or the correct v3.x path.

### AC4 — Hermes-TUI workstream works on the v3.7.2 tag

After `radiant setup-mcp --agent=hermes --global`, restarting Hermes and
querying its MCP server's `tools/list` returns **at least**:

- `mcp__radiant__possess`
- `mcp__radiant__possess_async` (returns a populated response, not a
  "v3.7.2 in-development" stub)
- `mcp__radiant__run_gate` (returns a populated response)
- `mcp__radiant__skill_list`
- `mcp__radiant__skill_load`
- `mcp__radiant__phase_status`

Tools must NOT require `RADIANT_INTERNAL=1` to be set by the agent.
Both `radiant_run_gate` and `radiant_possess_async` must perform the
job their description promises, not just echo a stub body.

### AC5 — v3.7.2 release is reachable through every documented path

End-to-end. A fresh sandbox with no prior setup, the user runs **any
one** of these and reaches a working v3.7.2 binary + MCP wire-up:

```
# Path A — canonical, what AGENTS-FOR-TASKS.md hands out
curl -fsSL .../v3.7.2/install.sh | bash -s -- --self-for-agent

# Path B — explicit agent
RADIANT_VERSION=v3.7.1 curl -fsSL .../main/install.sh | bash -s -- \
  --agent=claude --global

# Path C — go install (if Go module sync is in scope)
go install github.com/quant-risk/radiant-harness/v3/cmd/radiant@v3.7.2

# Path D — direct tarball
curl -L -o /usr/local/bin/radiant .../v3.7.2/radiant-darwin-arm64
chmod +x /usr/local/bin/radiant
radiant --version  # → v3.7.2
```

`make audit-install` exercises path A + path D in CI/sandbox and emits
a pass/fail table per path.

### AC6 — `make audit-install` is wired into `make smoke`

`make smoke` runs:

```
smoke: build audit-skills audit-docs audit-install
```

A failure in any one of them breaks the build. `audit-install` is a
script at `scripts/audit-install.sh` that:

- creates a sandbox `HOME` + sandbox `proj/`,
- runs the canonical install line pointing at the repo's own
  `dist/radiant-darwin-arm64` (no GitHub hop in CI),
- asserts exit 0 + milestone lines + version report,
- runs `radiant doctor --mcp` against the sandbox and asserts the
  verdict is `OK`,
- cleans up the sandbox.

## Gates

Each task closes when:

1. `go test ./...` → 0 FAIL.
2. `make smoke` → 0 FAIL (smoke now includes `audit-install`).
3. `make test-agents` → 12/12 PASS (matrix doesn't regress).
4. Manual install rehearsal on this Darwin/arm64 box reaches v3.7.2
   via ≥ 1 of the 4 paths and `radiant doctor --mcp` OK.
5. `git tag v3.7.2` pushed; GitHub Release v3.7.2 published; cross-
   platform bins attached.

## Risks

- **R1 — `RADIANT_VERSION=v3.7.2 -name "agent not found". The v3.7.0
  → v3.7.1 transition demonstrated one of these; every release slot
  burns a 30-min debugging session per host agent that didn't move
  forward. Mitigation: AC4 forces Hermes TUI explicit pass, then we
  re-run `test-agents` and trust the matrix.
- **R2 — Go module repath.** If we switch to `/v3` in `go.mod`, every
  consumer that hard-coded the old path breaks. Mitigation: keep the
  module path stable; rely on `RADIANT_VERSION=` + the install script
  path until we have signal to repath.
- **R3 — Hermes TUI sync still has the deadlock at the MCP protocol
  layer (per AGENTS-FOR-TASKS.md:80-136). v3.7.2 plugs `run_gate` /
  `possess_async`; if those keep racing the 120s outer timeout, we
  ship v3.7.2 with workstream documented but still bounded; fall back
  to subprocess decoupling in v3.7.3.

## Open Questions (will resolve during implement)

- Q1 — Is the Go module tag conflict legit (v3.x tagged but not on
  proxy) or a settings typo? `git push origin v3.7.2 && go mod tidy`
  must be confirmed.
- Q2 — `radiant_run_gate` in HEAD is a 50-line stub returning
  "v3.7.2 in-development" — the real work is flag-by-flag plumbing
  inside `cmd_mcp_runtime.go`. Estimate: half a day.

---
