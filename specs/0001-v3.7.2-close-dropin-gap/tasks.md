# v3.7.2 — Close the drop-in install gap — Tasks

Each task maps to one or more ACs in `spec.md`. A task is **closed**
only when its gates pass.

## T1 — Fix install.sh SIGPIPE in resolve_latest

**Owner:** code
**Maps to:** AC1
**File:** `install.sh` → `resolve_latest()` (≈ line 90-100)

Change:

```bash
echo "$body" | tr -d '\r' | grep -m1 '"tag_name"' | sed -E 's/.../\1/'
```

to a pipe that does not race `grep -m1` against `tr`'s writer side:

```bash
# Option A (preferred — minimal change, drops the tr step that
# wasn't doing anything useful on POSIX JSON anyway)
grep -m1 '"tag_name"' <<< "$body" | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
```

**Accept:** on this Darwin/arm64 box the canonical line exits 0 and
prints all six milestone lines.

**Gate:** `bash ./scripts/audit-install.sh` (defined in T6) exits 0
on the canonical install path.

## T2 — Fix AGENT_NAME unbound warning in install.sh post-install

**Owner:** code
**Maps to:** AC1, AC5
**File:** `install.sh` line ~202 (post-install section that prints the
"after the agent launches" message block)

Change `AGENT_NAME` references from "${AGENT_NAME-…}" to use
`: "${AGENT_NAME:=}"` at the top of the script (or guard every read).
Even when `--agent=` isn't passed, the post-install block must not
exit non-zero.

**Accept:** `RADIANT_VERSION=v3.7.2 bash install.sh --prefix=/tmp/x`
exits 0 with no `unbound variable` warning.

**Gate:** `make audit-install` exit 0.

## T3 — Fix INSTALL.md drift

**Owner:** docs
**Maps to:** AC3
**File:** `INSTALL.md`

Patches needed (verified by `make audit-docs` re-run + manual review):

- L9: replace `go install ... @v3.2.0` with `@latest`. Mark the path
  as the recommended one; mention that `go install @v3.7.2` will
  work **after** T4 lands, until then use the install script.
- L100-112 "If you use…" table: add a MiniMax row.
- L101: Claude Code target → `.claude/settings.json`.
- L122 ("discover `radiant_run`") → `mcp__radiant__possess`.
- L222 ("My agent doesn't see `radiant_run`") → rewrite to
  `mcp__radiant__possess` and link to AGENTS-FOR-TASKS.md § MCP tools.
- L224 ("Restart the agent after …") stays; fix the example command.
- install.sh `argv` docstring mentions `mavis-code` — replace with
  `MiniMax` (canonical short name).

**Accept:** `make audit-docs` passes and a manual grep over INSTALL.md
returns no `mavis-code` / no `.mcp.json` for Claude / no
`radiant_run` references outside the deprecation table.

**Gate:** `make audit-docs` exit 0.

## T4 — Sync v3.x to the Go module proxy (or freeze cleanly)

**Owner:** release
**Maps to:** AC2, AC5

Investigate first: `git ls-remote --tags origin`, then
`go env GONOSUMCHECK` / module proxy behavior. Two acceptable outcomes:

- **Outcome A — repath: change `go.mod` to
  `module github.com/quant-risk/radiant-harness/v3`, tag as `v3.7.2`,
  push tag. AGENTS-FOR-TASKS.md and INSTALL.md both update.
- **Outcome B — keep path, push v3.x to proxy via `git tag v3.7.2 &&
  go mod tidy` plus asking the go.sum DB to re-index.

Decision: lean **Outcome A** (it's the Go-idiomatic path and matches
the install script's expectation that the module root matches the
major version). Estimate: 30 min plus docs touch-up.

**Accept:** `go install github.com/quant-risk/radiant-harness/v3/cmd/radiant@latest`
(on a fresh `GOPATH`) installs a binary that is Light (zero
chatAnthropic symbols) and reports version ≥ `v3.7.2-prep` (whatever
the new tag is).

**Gate:** in a fresh sandbox,
`GOBIN=/tmp/x go install ... @latest && /tmp/x/radiant --version`
returns `v3.7.2`.

## T5 — Land async primitives so they actually work

**Owner:** code
**Maps to:** AC4
**Files:** `internal/possess/async.go`, `cmd/radiant/cmd_mcp_possess_async.go`,
`cmd/radiant/cmd_mcp_run_gate.go`, `internal/mcpbridge/` (gateway)

The "v3.7.2 in-development" stubs currently:

- `radiant_run_gate(phase, task)` — must exec the chosen phase from
  the harness's existing library (`internal/possess/driver.go`),
  persist `.radiant-harness/state/possess-<task-id>/state.json`
  between calls, and return stdout-only text (no blocking on
  sampling back to the host).
- `radiant_possess_async(task, workdir, profile)` — same Discover →
  Plan → Execute → Verify as `radiant_possess` but each call returns
  immediately with a `task_id`, and progress is observable via
  `radiant_phase_status(task_id)`. The host can poll.

**Accept:** calling `radiant_run_gate(phase="discover", task=…)` in
the `mcp self-test` exit 0 with a populated state.json under
`.radiant-harness/state/possess-*/`.

**Gate:** the test `cmd_mcp_possess_test.go::TestRunGateExecutesPhase`
PASS; `make smoke` PASS; `cmd_mcp_run_gate.go` exits 0 when invoked
via the public tool surface (not just RADIANT_INTERNAL).

## T6 — `make audit-install` + chain into `make smoke`

**Owner:** code
**Maps to:** AC1, AC2, AC5
**Files:** new `scripts/audit-install.sh`, `Makefile`

Script design:

- Create a sandbox `HOME` + sandbox `proj/` (use `mktemp -d`).
- Run the canonical install line pointing at this repo's
  `bin/radiant` (no GitHub hop in CI; the test is "does the script
  write the right things", not "does GitHub return bytes").
- Assert: exit 0, all milestone lines present, `radiant --version`
  reports the expected version, `~/.claude/settings.json` (or the
  relevant agent's config) contains a `radiant` MCP entry.
- Run `radiant doctor --mcp` and assert verdict OK.
- For path D: download `dist/radiant-darwin-arm64` and verify
  `radiant --version` reports the expected version.
- Print a Markdown table at end with PASS/FAIL per path.

Makefile change:

```
smoke: build audit-skills audit-docs audit-install
```

**Accept:** `make smoke` runs `audit-install.sh` and exits 0; on a
broken install the same `make smoke` exits non-zero.

**Gate:** `make smoke` exit 0 on this box.

## T7 — Cut v3.7.2 release

**Owner:** release
**Maps to:** AC1-6

Steps:

1. `make release` — cross-compile all 6 OS/arch targets into `dist/`.
2. `git tag v3.7.2 -m "v3.7.2 — close the drop-in install gap"`.
3. `git push origin v3.7.2` AND `git push fortvna v3.7.2` (Henrique
   maintains both remotes).
4. Build a `radiant-v3.7.2.tar.gz` and compute `SHA256SUMS`.
5. `gh release create v3.7.2 dist/* SHA256SUMS --title "v3.7.2 — close
   the drop-in install gap" --notes-file CHANGELOG.md[3.7.2]`.
   (If `gh` is not authed on this box, fall back to the
   `git credential fill` workaround documented in the Mavis memory
   for Henrique's machine.)
6. Confirm via the audit-install script that the canonical line
   pointing at the **tagged** install.sh and **pinned**
   RADIANT_VERSION=v3.7.2 reaches a working v3.7.2 binary.

**Accept:** the GitHub Release v3.7.2 exists with all 6 OS/arch
binaries, the SHA256SUMS file, and a tarball; the canonical install
line on a fresh Darwin/arm64 sandbox exits 0 and prints v3.7.2.

**Gate:** AC1 + AC5 hit end-to-end against the **tagged** artifacts,
not just local source.

---
