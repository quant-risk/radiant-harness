# Release v0.6.0 — Dogfood Notes

**Date:** 2026-06-25
**Tag:** v0.6.0
**Commit:** 213996c (CHANGELOG prep) + the new tag commit

---

## What this release contains

v0.6.0 closes the entire methodology merge + post-merge roadmap.

### Methodology merge (Sprints 10-13)

- **24 CLI commands** shipped (foundation + discovery + governance + PR/views)
- **17 bundled skills** authored and validated
- **1 open MIT schema spec** published
- **0 vendor-centrism** — works with Claude Code, Cursor, Codex,
  Copilot, Gemini CLI, Windsurf

### Post-merge additions (Sprints 14-15)

- `radiant audit` — project layout audit (AC traceability, ADR
  status, doc frontmatter)
- AGENTS.md unification — single source of truth via
  `scaffold.GenerateAgentsMD()`
- `--scope=since-last-release` for `radiant evals` — git-state aware
  coverage
- `radiant mcp serve` — Model Context Protocol server over stdio
  (release tool hard-coded to dry-run for safety)
- README, INSTALL, EXAMPLES — first-class user-facing docs

### Quality

- **298 tests passing**, 0 data races, 6/6 cross-compile targets
- **0 vendor-centrism**
- **0 hardcoded secrets** (verified by
  `TestNoHardcodedSecretsInCITemplates`)
- **0 global git config mutations** (per-commit identity via
  `-c user.name/email`)

---

## Dogfood experience: `radiant release v0.6.0`

This release was cut BY the release command itself — the
canonical end-to-end test of the methodology merge.

### Pipeline that ran (in order)

```
[1/7] Pre-flight        ✓ working tree clean
[2/7] Tag check         ✓ tag v0.6.0 does not exist yet
[3/7] Quality gates     ✓ build / vet / fmt / test (-race) all green
[4/7] Version bump      = cmd/radiant/main.go (no change — already 0.6.0)
[5/7] Cross-compile     ✓ 6/6 targets built (see dist/)
[6/7] Commit            [skip] (polish + CHANGELOG prep already committed)
[7/7] Tagging           ✓ tagged v0.6.0
```

### What we learned

1. **Version bump is idempotent**: when the source already has the
   target version, `bumpVersionInSource` correctly returns a no-op
   rather than corrupting the file. This is the right behaviour
   for re-runs.

2. **The dirty-tree guard is essential**: if the polish commit hadn't
   been made first, the release would have refused to run. This
   prevents accidental cross-contamination of unrelated work into
   a release.

3. **`--skip-commit` is a valid escape hatch**: when the bump is a
   no-op but you still want to exercise the rest of the pipeline
   + create the tag, `--skip-commit` lets the pre-committed content
   ride along.

### Dist artefacts (committed-as-tag, not-in-git)

The `dist/` directory contains 6 binaries:

- `radiant-linux-amd64` (8.0 MB)
- `radiant-linux-arm64` (7.6 MB)
- `radiant-darwin-amd64` (8.2 MB)
- `radiant-darwin-arm64` (7.7 MB)
- `radiant-windows-amd64.exe` (8.1 MB)
- `radiant-windows-arm64.exe` (7.6 MB)

These are not checked into git (Makefile generates them locally;
CI generates them per-tag).

---

## How to install this release

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v0.6.0
```

Or download from the
[releases page](https://github.com/quant-risk/radiant-harness/releases/tag/v0.6.0).

---

## Next steps after this tag

- `git push origin main && git push origin v0.6.0`
- Wait for CI to publish artefacts to the GitHub release
- Move to Sprint 16+ (new commands: `radiant security`,
  `radiant telemetry`, or domain-specific skills)
- Continue polishing per user feedback