# Validation Report — Sprint 14 first batch (v0.5.1)

**Date:** 2026-06-25
**Version:** 0.5.1
**Commit under validation:** (pending — this commit)
**Sprint:** 14 — Post-merge (first-class release command)
**Scope:** `radiant release v0.X.Y` + helpers + 9 tests.

---

## 1. Build hygiene

```
$ go build ./...
(clean)

$ go vet ./...
(clean)

$ gofmt -l .
(clean)
```

**Result:** ✅ Pass.

## 2. Race-detector tests

```
$ CGO_ENABLED=0 go test ./... -count=1 -race -timeout=180s
?   	github.com/quant-risk/radiant-harness/internal         [no test files]
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.589s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  2.595s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.918s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.982s
ok  	github.com/quant-risk/radiant-harness/internal/llm        7.137s
ok  	github.com/quant-risk/radiant-harness/internal/policy     3.330s
ok  	github.com/quant-risk/radiant-harness/internal/quality    4.230s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   4.464s
ok  	github.com/quant-risk/radiant-harness/internal/skill      2.955s
ok  	github.com/quant-risk/radiant-harness/internal/spec       2.901s
```

**Total:** 277 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build ... -o dist/radiant-linux-amd64
GOOS=linux   GOARCH=arm64 go build ... -o dist/radiant-linux-arm64
GOOS=darwin  GOARCH=amd64 go build ... -o dist/radiant-darwin-amd64
GOOS=darwin  GOARCH=arm64 go build ... -o dist/radiant-darwin-arm64
GOOS=windows GOARCH=amd64 go build ... -o dist/radiant-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build ... -o dist/radiant-windows-arm64.exe
✓ Release binaries in dist/
```

| Target | Status |
|---|---|
| linux/amd64 | ✅ |
| linux/arm64 | ✅ |
| darwin/amd64 | ✅ |
| darwin/arm64 | ✅ |
| windows/amd64 | ✅ |
| windows/arm64 | ✅ |

**Result:** ✅ 6/6 targets build clean.

## 4. End-to-end — `radiant release --dry-run`

```
$ radiant release v0.5.1 --dry-run
  → Cutting release v0.5.1

  [skip] pre-flight (--dry-run)
  [skip] tag existence check (--dry-run); would check v0.5.1

  → Running quality gates
  [skip] quality gates (--dry-run)

  → Bumping version
  [would-replace] cmd/radiant/main.go
        var version = "0.5.0"
      → var version = "0.5.1"

  → Cross-compiling (6 targets)
  [skip] cross-compile (--dry-run)

  → Committing version bump
  [skip] commit (--dry-run)

  → Tagging
  [skip] tag (--dry-run); would create v0.5.1

  ✓ Release v0.5.1 complete
    Next: git push origin main && git push origin v0.5.1
```

- All 8 steps clearly labelled ✓
- Dry-run shows exactly what would change ✓
- `would-replace` line shows old → new ✓

### Edge cases

| Case | Behaviour |
|------|-----------|
| Invalid version (`not-a-version`) | `Error: invalid version "not-a-version" — expected semver (e.g. 0.5.1 or v0.5.1)` |
| Missing version arg | `Error: accepts 1 arg(s), received 0` |
| `v` prefix | Accepted (strips to `0.5.1`) |
| Pre-release suffix (`0.5.0-rc.1`) | Accepted, tag becomes `v0.5.0-rc.1` |

**Result:** ✅ Dry-run + all edge cases work.

## 5. Iteration discipline recap

First attempt had a compile error:
```
cmd/radiant/main.go:1876:14: assignment mismatch: 1 variable but runGit returns 2 values
```
Caught by `go build`. Fix: discard the first return (`_, err := runGit(...)`).

Second issue surfaced at test time:
```
TestLooksLikeSemver failed: looksLikeSemver("v0.5.0") = false, want true
```
The function rejected the `v` prefix. Fix: `looksLikeSemver` now strips
`v` before validation, so both `0.5.1` and `v0.5.1` pass.

Third issue was test isolation: the `TestReleaseAcceptsPreRelease`
test failed because the CWD was changed by a previous test that
didn't restore properly. Fix: introduced a `chdirToTemp` helper
that uses `t.Cleanup` (LIFO order, runs after `t.TempDir()` cleanup).

## 6. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | 240 |
| Sprint 13 batch 1 | +5 | 245 |
| Sprint 13 batch 2 | +9 | 254 |
| Sprint 13 batch 3 | +6 | 260 |
| Sprint 13 batch 4 | +3 | 263 |
| Sprint 13 batch 5 | +5 | 268 |
| **Sprint 14 batch 1** | **+9** | **277** |

Sprint 14 tests:

- `TestLooksLikeSemver` — 12 cases: valid semver, v-prefix,
  pre-release suffix, build metadata, invalid forms.
- `TestReleaseRejectsInvalidVersion` — dry-run with bad version
  returns error before any file operations.
- `TestReleaseAcceptsSemver` — dry-run with `0.5.1` succeeds.
- `TestReleaseAcceptsVPrefix` — dry-run with `v0.5.1` succeeds.
- `TestReleaseAcceptsPreRelease` — dry-run with `0.5.0-rc.1` succeeds.
- `TestBumpVersionInSourceDryRun` — file unchanged under dry-run.
- `TestBumpVersionInSourceReal` — file updated, old line removed.
- `TestBumpVersionInSourceNoChange` — bumping to same version = no-op.
- `TestBumpVersionInSourceMissingFile` — returns error.

All 9 pass in `-race` mode.

## 7. Decisions

- ✅ Sprint 14 first batch is **READY TO MERGE** at v0.5.1.
- ✅ MVP is `--dry-run` first; user sees the full plan before any
  destructive step.
- ✅ Quality gates are NOT skipped by default — the user must
  explicitly pass `--skip-tests` if they want to bypass.
- ✅ Cross-compile is NOT skipped by default for the same reason.
- ✅ Git tag is the last step; user can `--skip-tag` to bump
  version + commit without tagging (e.g. for hotfix branches).
- ✅ `--skip-commit` allows local version-bump experimentation
  without committing.

## 8. End-to-end flow now complete (14 steps)

```
1. radiant product "..."          ← Lean Inception (v0.4.4)
2. radiant spec "<feature>"       ← AC→test mapping (v0.4.2)
3. radiant run specs/<NNNN>       ← implementation (v0.3.x)
4. radiant adr "<decision>"       ← Nygard ADR (v0.4.3)
5. radiant diagramar <level>      ← C4 Mermaid (v0.4.3)
6. radiant integrations list      ← MCP discovery (v0.4.5)
7. radiant handoff --feature=...  ← session pause (v0.4.2)
8. radiant update [--force]       ← skill refresh (v0.4.3)
9. radiant views --agent=<list>   ← native agent views (v0.4.6)
10. radiant review-pr <spec>      ← PR review scaffold (v0.4.7)
11. radiant setup-ci              ← CI workflow (v0.4.8)
12. radiant camada-agentica       ← agentic layer audit (v0.4.9)
13. radiant evals                 ← AC→test coverage (v0.5.0)
14. radiant release v0.X.Y        ← cut a release (v0.5.1) ← NEW
```

## 9. Next steps

After this commit, the next post-merge candidates are:

| Priority | Item | Rationale |
|----------|------|-----------|
| High | `radiant audit` CLI command | The `auditar` skill exists; wiring it as a CLI command lets it run in CI. |
| Medium | Unify AGENTS.md templates | Real drift exists between `scaffold`'s template and `generateAgentsMD()`. Audit caught it; fix it. |
| Medium | `since-last-release` scope for `radiant evals` | Git-state aware coverage; useful for "what changed since last tag". |
| Low | MCP `serve` command | Explicitly deferred per HARNESS-PLAN.md; only build when needed. |