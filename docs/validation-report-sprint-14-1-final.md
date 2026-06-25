# Validation Report — Sprint 14 first batch FINAL (v0.5.1)

**Date:** 2026-06-25
**Version:** 0.5.1
**Commit under validation:** `e55414a`
**Sprint:** 14 — Post-merge (first-class release command; final pass)
**Scope:** `radiant release v0.X.Y` + helpers + 9 tests.

---

## 1. Build hygiene

```
$ go build ./...
$ go vet ./...
$ gofmt -l .
(all clean)
```

**Result:** ✅ Pass.

## 2. Race-detector tests

```
$ CGO_ENABLED=0 go test ./... -count=1 -race -timeout=180s
(10 packages, all ok)

Total: 277 PASS, 0 FAIL, 0 data races detected
```

**Result:** ✅ Pass.

## 3. Cross-compilation — 6/6 targets clean

```
$ make release
linux/amd64 ✓    linux/arm64 ✓
darwin/amd64 ✓   darwin/arm64 ✓
windows/amd64 ✓  windows/arm64 ✓
```

**Result:** ✅ Pass.

## 4. Iteration discipline recap

3 issues caught and fixed in this commit, each before any binary shipped:

1. **Compile error** — `assignment mismatch: 1 variable but runGit returns 2 values`. Caught by `go build`.
2. **Semver rejects v prefix** — `looksLikeSemver("v0.5.0") = false`. Caught by unit test.
3. **Test CWD isolation** — `TestReleaseAcceptsPreRelease` failed because earlier test left CWD in unexpected dir. Caught by `go test`. Fixed via `chdirToTemp` helper using `t.Cleanup`.

**Result:** ✅ All caught at dev time, not at user time.

## 5. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Methodology merge (Sprints 10-13) | +188 | 268 |
| **Sprint 14 batch 1** | **+9** | **277** |

All 9 Sprint 14 tests pass in `-race` mode.

## 6. Decisions

- ✅ Sprint 14 first batch is **READY TO MERGE** at v0.5.1.
- ✅ MVP is `--dry-run` first; user sees the full plan before any
  destructive step.
- ✅ Quality gates + cross-compile + tag are NOT skipped by default.
- ✅ Per-commit identity via `-c user.name/... -c user.email/...`
  (no global git config mutation).

## 7. End-to-end flow now complete (14 steps)

```
1-13. (all prior commands — see docs/METHODOLOGY-MERGE-FINAL.md)
14. radiant release v0.X.Y  ← cut a release (v0.5.1) ← THIS COMMIT
```

See `docs/METHODOLOGY-MERGE-FINAL.md` for the full history.

## 8. Next steps (Sprint 14.2+)

| Priority | Item |
|----------|------|
| High | `radiant audit` CLI (wire the `auditar` skill) |
| Medium | Unify AGENTS.md templates (scaffold vs `generateAgentsMD`) |
| Medium | `since-last-release` scope for `radiant evals` |
| Low | `radiant mcp serve` (MCP server for agents that prefer it) |