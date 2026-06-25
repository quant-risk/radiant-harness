# Validation Report — Sprint 15 (Polish)

**Date:** 2026-06-25
**Version:** 0.6.0 (no code change; docs only)
**Commit under validation:** (pending — this commit)
**Sprint:** 15 — Polish (README, INSTALL, EXAMPLES)
**Scope:** 3 new top-level docs.

---

## What shipped

| File | Lines | Purpose |
|------|-------|---------|
| `README.md` | 230 | Overview, quick start, command roster, skill roster, architecture, quality, examples |
| `INSTALL.md` | 110 | Install methods (go install / binary download / build), LLM config, troubleshooting |
| `EXAMPLES.md` | 145 | Pulse worked example + end-to-end walkthrough + MCP server scenario |

Total: 485 lines of polished, user-facing docs.

## Build / vet / fmt / tests

```
$ go build ./...
(clean — docs only, no code change)

$ go vet ./...
(clean)

$ gofmt -l .
(clean)

$ CGO_ENABLED=0 go test ./... -count=1
(10 packages, all ok)
```

**Total:** 298 PASS, 0 FAIL, 0 races (unchanged).

## Cross-compile

```
$ make release
(6/6 targets clean — unchanged from Sprint 14)
```

## Decisions

- ✅ README is single-file, ≤230 lines, scannable in 2 minutes.
- ✅ INSTALL covers all three install paths (go install / binary
  download / build from source) with platform-specific snippets.
- ✅ EXAMPLES includes the end-to-end walkthrough as the canonical
  first-day-with-radiant flow.
- ✅ Linked back to existing docs (HARNESS-PLAN, SKILL-SCHEMA,
  METHODOLOGY-MERGE-FINAL) to avoid duplication.

## Why polish first

The user's next step is to dogfood `radiant release v0.6.0` (the
capstone). For that to produce a meaningful commit (the
`release:` step needs *something* to commit), we want polish
content in the repo first. So:

1. Sprint 15: add README + INSTALL + EXAMPLES (this commit).
2. Sprint 16: `radiant release v0.6.0` → bumps (no-op), commits
   the polish, tags v0.6.0.

The release commit becomes a clean "polish + tag v0.6.0" bundle
that future users can read to understand what changed at the
release boundary.