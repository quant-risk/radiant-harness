# Validation Report — Sprint 16 (v0.6.1)

**Date:** 2026-06-25
**Version:** 0.6.1
**Commit under validation:** (pending — this commit)
**Sprint:** 16 — Post-release new command
**Scope:** `radiant security` CLI + 8 tests.

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
(all 10 packages pass)
```

**Total:** 306 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — 6/6 targets clean

```
$ make release
(6/6 targets clean)
```

## 4. End-to-end — `radiant security`

Test fixture: `app.go` containing AWS + GitHub + OpenAI keys; `.env`
with mode 0644.

```
$ radiant security --scope=secrets
  ✓ wrote docs/security-report.md
  Summary: 3 errors, 0 warnings, 0 info

$ radiant security --scope=perms
  ✓ wrote docs/security-report.md
  Summary: 0 errors, 1 warnings, 0 info

$ radiant security --fail-on-warning
  ✓ wrote docs/security-report.md
  Summary: 3 errors, 1 warnings, 0 info
  Error: security scan found 3 error(s) and 1 warning(s) — see docs/security-report.md
```

- AWS / GitHub / OpenAI secrets detected (3 errors) ✓
- `.env` mode 0644 detected as WARNING (chmod 600 recommended) ✓
- `--fail-on-warning` returns non-zero on warnings ✓

## 5. Iteration discipline recap

First attempt had a regex bug: the OpenAI pattern was
`sk-[A-Za-z0-9]{20,}` which doesn't allow dashes — but real
OpenAI keys look like `sk-proj-abc...` (with dashes). The test
caught it. Fix: `[A-Za-z0-9_-]{20,}`.

**Lesson:** when writing regex for real-world identifiers,
include dashes/underscores even if "the canonical format"
doesn't use them. Test fixtures with realistic examples
catches this; dry runs don't.

## 6. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Methodology merge (Sprints 10-13) | +188 | 268 |
| Sprint 14 (release + audit + AGENTS + evals-scope + MCP) | +30 | 298 |
| Sprint 15 (polish) | 0 (docs only) | 298 |
| **Sprint 16 (security)** | **+8** | **306** |

Sprint 16 tests:

- `TestScanSecretsDetectsAWSAccessKey` — single AWS key in `app.go`
- `TestScanSecretsDetectsMultiplePatterns` — 3 different secrets in one file
- `TestScanSecretsIgnoresTestFiles` — `*_test.go` skipped
- `TestScanSecretsNoFalsePositivesOnCleanCode` — clean code = 0 findings
- `TestScanPermsDetectsWorldReadableEnv` — `.env` mode 0644 → 1 WARNING
- `TestScanPermsIgnores0600Env` — `.env` mode 0600 → 0 findings
- `TestRenderSecurityReportEmpty` — "No findings" message
- `TestRenderSecurityReportWithFindings` — error/warning rendering

All 8 pass in `-race` mode.

## 7. Decisions

- ✅ Sprint 16 is **READY TO MERGE** at v0.6.1.
- ✅ MVP scope: hardcoded secrets + sensitive file permissions.
- ✅ Test files skipped (fixtures commonly contain fake secrets).
- ✅ `--fail-on-warning` is opt-in (default only fails on ERROR)
  so the command is safe to wire into pre-commit hooks.
- ✅ Dependency-CVE scanning deferred (would require network).
- ✅ Config-CORS / insecure-defaults checks deferred
  (more LLM-judgment territory).

## 8. End-to-end flow now complete (19 commands)

```
1-18. (all prior — see docs/METHODOLOGY-MERGE-FINAL.md)
19. radiant security               ← Sprint 16 (v0.6.1) ← NEW
```

## 9. What's next

After this commit, candidates for future sprints:

| Priority | Item | Notes |
|----------|------|-------|
| High | Wire `radiant security` into CI template | `setup-ci` should call it as a 5th gate |
| Medium | `radiant telemetry` | Usage stats; needs privacy-first design |
| Medium | More bundled skills | Domain-specific (e.g. `radiant-mobile`, `radiant-data`) |
| Low | Dependency-CVE scanning | Network access required |