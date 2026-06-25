# Validation Report — Sprints 20-22 (v0.6.3)

**Date:** 2026-06-25
**Version:** 0.6.3
**Commit under validation:** (pending — this commit)
**Sprint:** 20 + 21 + 22 — telemetry wiring + summary + 3 domain skills

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
(all 10 packages pass)

Total: 324 PASS, 0 FAIL, 0 data races detected
```

**Result:** ✅ Pass.

## 3. Cross-compilation — 6/6 targets clean

```
$ make release
(6/6 targets clean)
```

## 4. Sprint 20 — telemetry wired into release

When telemetry is enabled, a successful `radiant release` records
an event. Implementation:

```go
// In runRelease, after the success line:
if !skipTag {
    recordTelemetry("release")
}
```

`recordTelemetry` is a no-op when telemetry is disabled (per
`isTelemetryEnabled()`). Same privacy guarantees — only command
name + timestamp + 8-char hash + CLI version.

## 5. Sprint 21 — `radiant telemetry summary`

Test fixture: 4 events across 2 days, 3 distinct commands.

```
$ radiant telemetry summary
  Total events: 4
  Distinct commands: 3
  Distinct days: 2

  Top commands:
    spec                 2
    audit                1
    release              1

  Daily counts:
    2026-06-24  1
    2026-06-25  3
```

Aggregates computed locally; no network. Same privacy guarantees.

## 6. Sprint 22 — 3 domain skills

| Skill | Purpose |
|-------|---------|
| `mobile` (19th) | iOS/Android/cross-platform apps: platform decision, offline strategy, App Store checklist |
| `data` (20th) | Data pipelines: warehouses, lakes, streams; schema evolution; lineage; quality |
| `frontend` (21st) | Web apps: framework decision, rendering strategy, Core Web Vitals, a11y |

All 3 validate cleanly via `TestAllBundledSkillsValidateCleanly`
(which checks the full schema: 10 rules per skill).

## 7. Iteration discipline recap

Three issues caught and fixed in this commit:

1. **Compile error**: missing `crypto/sha256` import for `shortHash`.
   Fix: added the import.

2. **Test assertion misframing**: the first version of
   `TestRecordTelemetryWritesWhenEnabled` checked for the
   literal substring `"command":"release"` which works because
   `json.Marshal` produces compact output. Verified.

3. **Stdout capture in `TestTelemetrySummaryCountsAndGroups`**:
   initial version used `fmt.Println` and didn't capture stdout,
   so assertions ran against an empty buffer. Fix: use
   `os.Pipe()` to redirect stdout for the duration of the call,
   then read + assert against the captured bytes.

**All caught at dev time, not at user time.**

## 8. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| ... (prior) | ... | 319 |
| Sprint 20 (recordTelemetry) | +2 | 321 |
| Sprint 21 (summary) | +3 | **324** |

## 9. Decisions

- ✅ Sprints 20-22 are **READY TO MERGE** at v0.6.3.
- ✅ `recordTelemetry` is best-effort — never fails the user's
  command. Errors (file system, malformed JSON) are silently
  ignored; telemetry is observability, not a hard requirement.
- ✅ Summary shows top-10 commands + all days; bounded for
  very long logs (10 most-frequent commands; full daily history).
- ✅ 3 new skills are top-of-line: schema-compliant, full SKILL.md
  structure (Decision tree, Workflow, Examples, Anti-patterns,
  Failure modes, Related skills).

## 10. End-to-end flow (21 commands, 21 skills)

```
21 CLI commands:
1-19. (all prior)
20. radiant telemetry {status|enable|disable|show|summary}  ← Sprint 21
21. radiant incident <severity> <summary>                   ← Sprint 19

21 bundled skills (18 prior + 3 new):
mobile (Sprint 22), data (Sprint 22), frontend (Sprint 22)
```

## 11. What's next

After this commit, the project is at v0.6.3 with 21 commands,
21 skills, 324 tests, all gates green. The roadmap of "post-merge
+ polish + domain" is now substantially complete.

Future candidates (lower priority):

| Priority | Item | Notes |
|----------|------|-------|
| Medium | More domain skills: `radiant-ml`, `radiant-game`, `radiant-cli` | Vertical specialisation |
| Low | `radiant telemetry export` | Allow user to share local stats if they want |
| Low | `radiant telemetry rotate` | Cap log size; archive older events |
| Low | `radiant release` interactive prompts | Confirm version, etc. (currently all flags) |