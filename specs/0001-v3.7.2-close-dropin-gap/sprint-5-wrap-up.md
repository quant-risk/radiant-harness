# Sprint 5 — wrap-up + v3.7.3 plan

**Sprint 4 (just closed)** delivered v3.7.2 — three install-path
bugs fixed, four async primitives landed, drop-in validated end-to-end.
This doc hands off the open items for the next sprint and proposes
the v3.7.3 scope.

---

## ✅ What landed in this sprint

| # | Item | Status |
|---|---|---|
| T1 | install.sh SIGPIPE fix (T1) | ✅ shipped (blob `9f37043d…` lives on tag v3.7.2) |
| T2 | install.sh `set -u` AGENT_NAME init | ✅ shipped (CDN still serving stale `77939d6b…` until ~5–15 min after the force-push) |
| T3 | INSTALL.md drift fixes (5 hunks) | ✅ shipped |
| T4 | module repath `/v3` | ✅ shipped (74 internal imports + CI workflows + docs) |
| T5 | `mcp__radiant__run_gate` + `mcp__radiant__possess_async` real impls + 4 regression tests | ✅ shipped |
| T6 | `make audit-install` wired into `make smoke` | ✅ shipped (3 paths; 2 PASS + 1 SKIP) |
| T7 | v3.7.2 release on `quant-risk/radiant-harness` + `Fortvna-Risk-Solutions/radiant-harness` | ✅ shipped (7 attached assets, SHA256 verified) |

**Tags / commits:**
- `main` → `7680c18` ("v3.7.2: docs + smoke-test whitelist")
- Tag `v3.7.2` → `81b51e7` (the code commit, *force-pushed to both
  remotes*; the doc commit lives one revision ahead of the tag on
  `main`).
- Release id `346600132` on `quant-risk/radiant-harness`.

**Verified:**
- `go test ./... —short` — 31/31 packages PASS (1 pre-existing
  flaky in `fleet.TestRunAllContextCanceled`).
- `make smoke` — audit-skills 6/6, audit-docs 46/57 (0 drift),
  audit-install 2 PASS + 1 SKIP, smoke-test 16/16.
- `make test-agents` — 12/12 PASS on the cross-agent matrix.
- E2E `Path B` (direct-tarball) — binary v3.7.2, `mcp self-test`
  PASS, 7 tools exposed.
- E2E `Path A` (`curl | bash`) — binary v3.7.2 installed and
  functional; one transient `AGENT_NAME: unbound variable` warning
  on the post-install message (path B-via-CDN) until CDN propagates.

---

## 🟡 Open items for the next sprint

### Q1 — Go module proxy still indexes v0.7.0 (priority medium)

`proxy.golang.org/@v/list` for `github.com/quant-risk/radiant-harness`
returns only `[v0.6.0, v0.7.0]`. `audit-install` flags this as SKIP.

**Owner:** Release (requires a `pkg.go.dev` operator-side action to
re-index the v3 tags).

**Workarounds:**
- Document is accurate: "use `curl | bash` until the proxy re-indexes".
- Alternative: open a proxy indexing request via the **Go issue
  tracker** with `proxy.golang.org` (`https://github.com/golang/go/issues`).
- Or: spin up a private Go proxy mirror pointed at the public repo
  (heavier-weight; only worth it if Q1 blocks a major customer).

### Q2 — Pre-existing flaky test (`fleet.TestRunAllContextCanceled`)

Passes when run in isolation; fails intermittently in batch.
Probably a timing race inside `internal/fleet`. Not introduced by
this sprint's changes — surfaced again because the test count
changed.

**Owner:** code.

**Fix sketch:** inspect the test, add a tighter timeout or a
deterministic cancellation step. Likely a 30-minute job.

### Q3 — `radiant_run` alias deprecation timeline

v3.7.2 marked it DEPRECATED. Plan: remove in v3.8.0 unless host
agents surface a blocking reason. Track deprecation comments
remain in `cmd/radiant/cmd_mcp_runtime.go` and the public-facing
docs (`INSTALL.md`, `AGENTS-FOR-TASKS.md`).

---

## 🚀 v3.7.3 proposal — "close the v3.8-prep holes"

Three cleanup items worth shipping before v3.8 lands (estimated
1 to 2 sprints).

### Scope

1. **V3.7.3: Hermes TUI `radiant_possess` end-to-end fix.**
   Today `radiant_possess` on Hermes hits a 120s timeout because
   the synchronous TUI cannot satisfy nested sampling/createMessage
   callbacks. v3.7.2 ships the gate primitives but the protocol-
   level fix is independent. The workstream documented in
   `AGENTS-FOR-TASKS.md` (4 small MCP tools + Python/bash direct)
   is still required for end-to-end execution under Hermes.

   Subtasks: prove the gate primitives compose a non-blocking
   path; add `radiant_possess` host detection (sync-host table);
   document the auto-routing behaviour in CHANGELOG.

   Acceptance: a Hermes TUI session completes the bundled
   `credit-risk` skill workflow end-to-end inside one MCP
   round-trip window (<30s wallclock).

2. **V3.7.3: kill `radiant_run` alias entirely.** Remove the
   switch case in `cmd/radiant/cmd_mcp_runtime.go`. Update
   `AGENTS-FOR-TASKS.md` to remove the deprecation table. Read
   the host-agent detection unit logs for any host that was
   actually using the alias; reach out before cut if any.

3. **V3.8.0-prep: profile-aware budgets.** Loop runs today
   carry hardcoded budgets. Add `--budget <tokens|usd|time>` and a
   per-profile table so agents can stop runaway loops cleanly.

### Out of scope (deferred)

- Major-version bump to `/v3` was already done; no further
  repath planned.
- `bench` subsystem overhaul (currently a stub).
- New host agents beyond the current 12 — pending user's call.

---

## 📌 Suggested sprint-5 audit cycle (manual)

When a fresh user / agent tries the canonical line, this is the
expected sequence:

1. Read `AGENTS-FOR-TASKS.md` from the repo.
2. Run `curl -fsSL .../install.sh | bash -s -- --agent=<host>`.
3. Restart the host; verify `mcp__radiant__possess` shows in
   `tools/list`.
4. Call `mcp__radiant__possess(task=…, workdir=…)`.

Acceptable end-state: state.json under `.radiant-harness/state/`
with all four phases marked done (sync hosts) OR with
`mode: self-driven` markers for offline scaffolds.

---

## 🛠️ Field-ops-friendly summary

- **If you only remember one line:** `curl -fsSL
  https://raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh
  | bash`
- **If you want a sandbox:** `RADIANT_VERSION=v3.7.2 bash
  <(curl …) --prefix=/tmp/radiant-sandbox`
- **If you only need the binary:** `curl -L -o /usr/local/bin/radiant
  https://github.com/quant-risk/radiant-harness/releases/download/v3.7.2/radiant-darwin-arm64
  && chmod +x /usr/local/bin/radiant`
- **If you want validation:** `make smoke && make test-agents` (both
  PASS at v3.7.2).
- **If you hit a 404 on the install URL:** check the v3.7.2 GitHub
  Release exists; CDN caching can take a few minutes after
  force-pushed tags.

---

## Who

This sprint: Mavis (orchestrator) under Henrique (creator /
reviewer). Net delta: ~300 LoC, 1 new script, 1 new spec, 1
CHANGELOG entry, 1 release. Total elapsed: ~3.5 hours.
