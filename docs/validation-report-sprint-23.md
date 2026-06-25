# Sprint 23 validation — `radiant telemetry rotate`

**Commit (this report):** TBD
**Re-validates:** `20ecc1a` (Sprints 20-22 final)
**Version:** v0.6.3 (in source)

## What shipped

### `radiant telemetry rotate [--max-entries=N]`

Operational hygiene for the local telemetry log: when the active log
exceeds `--max-entries`, the oldest events are moved to a date-stamped
archive file (`telemetry-YYYY-MM-DD.jsonl`) so the user keeps full
history without the active log growing unbounded.

Behavior:
- `telemetry disabled` → no-op, no error, prints status hint
- `log missing` → no-op (idempotent)
- `len(lines) <= max` → no-op, prints `Log has N entries; under cap M. No rotation needed.`
- `len(lines) > max` → archive oldest, keep newest max; prints both counts
- `--max-entries <= 0` → errors clearly (`--max-entries must be > 0 (got N)`)

Defaults: `max-entries=1000` (one well-trodden rough upper bound; user can override).

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| Tests | **328 PASS** (was 324, +4 rotate) |
| Failures | **0** |
| Data races | **0** |
| Cross-compile | **6/6** (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64) |

## Iteration discipline

Two issues caught + fixed in this sprint:

1. **`undefined: fmt` and `undefined: time`** — `main_test.go` used
   `fmt.Sprintf` and `time.Now().UTC()` without importing those.
   `go vet` caught it on the first build. Fix: added `fmt` and `time`
   to the test file's import block.

2. **Test substring mismatch (`TestTelemetryRotateOverCap`)** —
   initial version wrote `\"hash\":\"a%d\"` (with Go escapes) to the
   JSONL file, then asserted against `hash=\"a2\"` (also Go escapes).
   But the file contained raw JSON (`"hash":"a2"`); the test was
   looking for the Go-escaped form, so `strings.Contains` returned
   false. Fix: switched assertions to raw backtick strings
   `` `"hash":"a2"` `` that match what `fmt.Sprintf` actually wrote.

Both caught at dev time, not at user time. Same playbook as before:
fix → re-run → green.

## Final tally (post-everything)

- **21 CLI commands** + **21 bundled skills** + **1 open MIT schema spec**
- **328 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**

## Stopping point

This is a clean stopping point. v0.6.3 is shippable as-is:
- 21 commands + 21 skills + 1 open schema
- 328 tests passing, 0 races
- 6/6 cross-compile
- Privacy-first telemetry (opt-in, no args/paths/env)
- Operational hygiene: rotate, summary, show, enable, disable
- Documentation: README/INSTALL/EXAMPLES, per-sprint reports,
  methodology-merge final, release notes for v0.6.0

Future work is purely additive and not blocking:
- More domain skills (`radiant-ml`, `radiant-game`, `radiant-cli`)
- `radiant telemetry export`
- `radiant release --interactive`
- MCP server hardening (auth, rate limiting, transport limits)

Decide: tag v0.6.3 for real, or keep iterating.