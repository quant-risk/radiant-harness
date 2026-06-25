# Sprint 24 validation — `radiant telemetry export`

**Commit (this report):** TBD
**Re-validates:** `3ef46f8` (Sprint 23 — rotate)
**Version:** v0.6.3 (in source)

## What shipped

### `radiant telemetry export [--format=json|csv] [--output=path] [--since=YYYY-MM-DD]`

Share your local telemetry log with the team or analyze externally.
Default: JSON to stdout (pipe-friendly). Flags:

- `--format json|csv` (default `json`) — pretty-printed array or CSV
- `--output <path>` (default stdout) — write to file
- `--since YYYY-MM-DD` (default empty) — only events on or after this date

Behavior:
- `telemetry disabled` → no-op, prints status hint
- `log missing` → no-op (idempotent)
- `log empty` → no-op
- `--format` invalid → errors clearly

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean (after `gofmt -w`) |
| `go test ./... -race` | 10 packages OK |
| Tests | **335 PASS** (was 328, +7 export) |
| Failures | **0** |
| Data races | **0** |
| Cross-compile | **6/6** (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64) |

## Iteration discipline

Two issues caught + fixed in this sprint:

1. **`os.WriteFile` to non-existent subdirectory** — `TestTelemetryExportToFile`
   wrote to `filepath.Join(dir, "subdir", "out.json")` without creating `subdir/`.
   `os.WriteFile` doesn't create parent dirs. Fix: use a flat file
   (`filepath.Join(dir, "out.json")`).

2. **`gofmt` drift in `cmd/radiant/main.go`** — gofmt detected whitespace
   drift after multiple edits. Fix: `gofmt -w cmd/radiant/main.go`. Caught
   at the `gofmt -l .` gate in the validation cycle, not at user time.

Both caught at dev time. Same playbook as before.

## Privacy invariants (preserved)

The exporter emits ONLY the 4 fields already recorded locally:
- `timestamp` (ISO-8601 UTC)
- `command` (CLI command name)
- `hash` (8-char SHA-256 prefix of the working tree)
- `radiant_ver` (CLI version)

A privacy-invariants test (`TestTelemetryExportPrivacyFields`) scans
the exported JSON for any field names that would suggest leaked data
(`"args"`, `"path"`, `"env"`, `"secret"`, `"token"`, `"key"`).
Green.

The export command never opens a network connection. The user must
explicitly pipe or write to a file, then handle the output themselves.

## Telemetry surface (post-Sprint 24)

| Subcommand | Purpose |
|---|---|
| `radiant telemetry status` | Show enabled/disabled + log path + recorded fields |
| `radiant telemetry enable` | Opt in; creates log file |
| `radiant telemetry disable` | Opt out; deletes log file |
| `radiant telemetry show` | Print last 20 events (cat -n style) |
| `radiant telemetry summary` | Aggregate counts (total, distinct commands, distinct days) |
| `radiant telemetry rotate` | Cap log size; archive old events |
| `radiant telemetry export` | Share log (JSON or CSV) |

**Complete.** Privacy-first telemetry trio + operational hygiene + portability.

## Final tally (post-everything)

- **21 CLI commands** + **21 bundled skills** + **1 open MIT schema spec**
- **335 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**

## Stopping point

This is a strong stopping point. v0.6.3 is shippable as-is:
- Telemetry is complete: status, enable, disable, show, summary,
  rotate, export
- All 7 telemetry subcommands tested (was 4 in Sprint 23, +3 now)
- Privacy-first throughout: opt-in, no args/paths/env, network-free

Remaining candidates (purely additive, not blocking):
- `radiant release --interactive` — prompt before tagging
- More domain skills (`radiant-ml`, `radiant-game`, `radiant-cli`)
- Tag v0.6.3 for real via `radiant release v0.6.3` — pipeline is dogfooded