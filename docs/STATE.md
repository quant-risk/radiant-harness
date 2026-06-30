---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 by Codex after final validation pass

## Current sprint / active feature

- Active: final validation of docs/backlog consolidation after v3.7.5.
- Sprint goal: remove stale task scaffolds, align docs with shipped behavior,
  and keep the repo ready for another agent to install and use.
- Progress: MenuFlex noise removed; self-driven drop-in flow validated;
  placeholder specs closed into explicit status records; final validation
  passed.

## Next concrete action

- Review the remaining roadmap item with the highest value: broader
  host-agent matrix coverage or release tagging for the cleanup build.

## Latest validation

2026-06-30 final pass:

- `./scripts/run.sh` — PASS.
- `radiant doctor` — PASS.
- `radiant mcp self-test` — PASS, 6 MCP tools listed.
- `go test ./cmd/radiant ./internal/...` — PASS.
- `go test ./...` — PASS.
- `make test-dropin` — PASS against `v3.7.5`.
- `make test-agents` — PASS; matrix regenerated under local
  `.radiant-harness/`.
- `make audit-install` — PASS for reachable paths; canonical `curl|bash`
  path skipped only because the local tree was dirty before this validation
  commit, so no matching release tag existed for that transient build.

## Decisions log

- 2026-06-30: keep `radiant_possess` as the primary path for hosts with
  sampling and use self-driven scaffolds for hosts without sampling.
- 2026-06-30: keep `radiant_run_gate` and `radiant_possess_async` as real
  offline MCP primitives for synchronous hosts.
- 2026-06-30: remove unrelated external user cases from this repository;
  keep only a minimal audit trail when a removal itself was harness-guided.

## Blockers

- None known for the validated drop-in path.

## Context bookmarks

- `README.md` — public install and usage entrypoint.
- `AGENTS-FOR-TASKS.md` — instructions for third-party host agents.
- `cmd/radiant/cmd_mcp_runtime.go` — MCP tool registration.
- `cmd/radiant/cmd_mcp_possess_self_driven.go` — self-driven fallback.
- `scripts/e2e/dropin_self_driven_e2e.py` — public install E2E.
- `docs/ROADMAP.md` — remaining backlog.

## Deferred ideas / backlog

- Add true background subprocess execution if the current offline
  self-driven primitives prove insufficient for a specific synchronous host.
- Add broader host-agent matrix coverage when new host CLIs are available.
