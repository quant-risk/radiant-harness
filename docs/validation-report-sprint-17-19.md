# Validation Report — Sprints 17-19 (v0.6.2)

**Date:** 2026-06-25
**Version:** 0.6.2
**Commit under validation:** (pending — this commit)
**Sprint:** 17 + 18 + 19 — three post-merge additions in one cycle.

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

Total: 319 PASS, 0 FAIL, 0 data races detected
```

**Result:** ✅ Pass.

## 3. Cross-compilation — 6/6 targets clean

```
$ make release
(6/6 targets clean)
```

## 4. Sprint 17 — wire `radiant security` into setup-ci

The setup-ci templates now run 5 gates:

```
1. radiant validate (spec/code alignment)
2. radiant audit (project layout)
3. radiant security --fail-on-warning  ← NEW
4. go test ./... -count=1 -race
5. go build ./...
```

All 3 providers (GitHub Actions / GitLab CI / CircleCI) updated.
Verified by `TestCITemplatesIncludeSecurityGate`.

## 5. Sprint 18 — `radiant telemetry`

Privacy-first. Default = OFF. User must explicitly enable.

```
$ radiant telemetry status
  Telemetry: disabled (opt-in)
  Log path:  .radiant-harness/telemetry.jsonl

  NEVER recorded (privacy-first):
    - command arguments
    - file paths
    - project names or git SHAs
    - environment variables
    - network endpoints

$ radiant telemetry enable
  ✓ Telemetry enabled. Log: .radiant-harness/telemetry.jsonl

$ radiant telemetry show
  Last N events:
    {"timestamp":"...","command":"spec","hash":"...","radiant_ver":"0.6.2"}

$ radiant telemetry disable
  ✓ Telemetry disabled. Removed .radiant-harness/telemetry.jsonl.
```

All 4 subcommands verified end-to-end.

## 6. Sprint 19 — `radiant incident` + `incident` skill

```
$ radiant incident sev1 "API outage from bad deploy"
  ✓ created docs/incidents/0001-api-outage-from-bad-deploy.md

$ radiant incident bogus "test"
Error: invalid severity "bogus" — expected sev1 | sev2 | sev3 | sev4
```

Severity validated. Output file has 7 sections (header, severity,
date, duration, impact, commander, author, timeline, root cause,
contributing factors, what went well, action items).

The `incident` skill (18th bundled skill) is the canonical
incident response playbook. It validates cleanly:

```
$ go test ./internal/skill/ -run TestAllBundledSkillsValidateCleanly
PASS — all 18 skills (17 prior + incident) validate
```

Iteration: caught missing `## Failure modes` section on first
attempt; added it; CI guard `TestAllBundledSkillsValidateCleanly`
passed on second attempt.

## 7. Iteration discipline recap

Three issues caught and fixed in this commit:

1. **`gofmt`** flagged `cmd/radiant/main.go` (formatting drift
   after multi-edit). Fix: `gofmt -w`.
2. **`## Failure modes` missing from `incident` skill** —
   caught by `TestAllBundledSkillsValidateCleanly`. Fix: added
   the section following the pattern of every other skill.
3. **`TestRenderIncidentDocIncludesSections` substring mismatch**
   — the body has `'incident' skill` (with quotes) but the test
   looked for `incident skill` (without). Fix: changed the test
   to check for `radiant incident` (the CLI command name in
   the footer) which is unambiguously present.

**All caught at dev time, not at user time.**

## 8. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| ... (prior) | ... | 306 |
| Sprint 17 (security gate) | +1 | 307 |
| Sprint 18 (telemetry) | +7 | 314 |
| Sprint 19 (incident) | +5 | **319** |

## 9. Decisions

- ✅ Sprints 17-19 are **READY TO MERGE** at v0.6.2.
- ✅ Security as a CI gate (5th) catches issues BEFORE merge —
  closes the gap between "ran audit locally" and "PR is green".
- ✅ Telemetry is opt-in by default; the log file's existence
  IS the flag (no separate config file to keep in sync).
- ✅ Telemetry NEVER records args, paths, env vars, or network
  endpoints — privacy-first by design.
- ✅ Incident scaffold creates the file with the post-mortem
  template pre-filled; the engineer fills in the timeline + RCA.

## 10. End-to-end flow now complete (21 commands)

```
1-19. (all prior — see docs/METHODOLOGY-MERGE-FINAL.md + sprint reports)
20. radiant telemetry {status|enable|disable|show}  ← Sprint 18 (v0.6.2) ← NEW
21. radiant incident <severity> <summary>            ← Sprint 19 (v0.6.2) ← NEW
```

Plus 18 bundled skills (incident added in Sprint 19).

## 11. What's next

After this commit, the project is at v0.6.2 with 21 commands,
18 skills, 319 tests, all gates green. The CI template now
includes 5 gates (validate, audit, security, tests, build).

Future candidates (in priority order):

| Priority | Item | Notes |
|----------|------|-------|
| High | Wire telemetry into the release pipeline | `radiant release` should record an event when it tags |
| Medium | `radiant telemetry summary` | Aggregate counts (commands run per day) instead of raw events |
| Medium | Domain skills: `radiant-mobile`, `radiant-data`, `radiant-frontend` | Vertical specialisation |
| Low | `radiant telemetry export` | Allow user to share local stats if they want |