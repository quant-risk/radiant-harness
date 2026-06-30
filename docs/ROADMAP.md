# Roadmap — radiant-harness

## Roadmap objective

Make radiant-harness a reliable drop-in governance layer for host agents:
installable from GitHub, usable through MCP, auditable through persisted
state, and clear enough for another agent to complete real project work.

## Shipped in v3.7.6 (2026-06-30)

- **Host matrix broadened.** Google Gemini CLI added as the 13th
  Light-mode host; `setup-mcp --agent=gemini` writes
  `~/.gemini/settings.json` with the standard `mcpServers` JSON
  shape. Detection fingerprint is documented.
- **Status UX improved.** `radiant_phase_status` returns a
  structured `summary` (next step, resume command, pending files,
  marker count, last gate, clear error/cancel state). Five new
  contract tests pin the four phases × {done, in_progress, error,
  cancelled} matrix.
- **Validation entrypoint extended.** `scripts/run.sh` now covers
  the full install/test/audit matrix (was 4 commands); doctor
  steps surfaced as SKIP, not FAIL, in a host-less shell.
- **Doc/backlog consolidated.** Spec placeholders closed,
  `docs/ROADMAP.md` and `docs/STATE.md` re-organised as living
  memory, `radiant_run_gate` and `radiant_possess_async` finally
  documented in `AGENTS-FOR-TASKS.md` § MCP tools.
- **External user case removed.** MenuFlex spec purged from the
  harness repo (did not belong here).

## Now

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Add Gemini restart hint to `install.sh` | Consistent post-install UX | XS | Maintainers | v3.7.6 hostdetect already detects `gemini` | `--agent=gemini` prints a one-line restart instruction like the other 12 hosts |

## Next

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| True background subprocess async | Better long-running sync-host support | L | Maintainers | Concrete host need | `radiant_possess_async` runs phases in a detached subprocess; pid + state observable cross-process |
| Async gate pid/liveness probe | Cross-process cancel/inspect | M | Maintainers | Subprocess path | `radiant_phase_status` distinguishes "alive" from "crashed" without re-running the gate |
| Fleet async primitives | More predictable parallel orchestration | L | Maintainers | Stable loop async | Fleet has the same status/retry guarantees as loop |

## Later

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Richer ontology tooling | Better scope discovery and skill routing | M | Maintainers | Glossary/ontology adoption | Ontology can be validated against specs and skills |
| Per-host skill bundles | Smaller drop-in for host-specific stacks | M | Maintainers | Skill catalog | Each host has a default skill bundle surfaced on first run |
