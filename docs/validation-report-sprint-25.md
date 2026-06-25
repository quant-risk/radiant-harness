# Sprint 25 validation — `radiant release --interactive`

**Commit (this report):** TBD
**Re-validates:** `a063c52` (Sprint 24 — export)
**Version:** v0.6.3 (in source)

## What shipped

### `radiant release --interactive`

Adds a final-confirmation prompt between quality gates and the
destructive steps (version bump + commit + tag). When the flag is
set:

1. The pipeline runs normally: pre-flight, tag check, quality gates.
2. After all checks pass, the user sees:

```
  ────────────────────────────────────────────
  About to bump version → 0.6.4
  Then commit 'release: cut v0.6.4'
  Then create tag v0.6.4
  ────────────────────────────────────────────
  Continue? [Y/n]:
```

3. Default (Enter / `y` / `yes`) → proceed.
4. `n` / `no` → abort cleanly with `✗ Aborted by user. No changes made.`
5. Anything else → error (`invalid answer "X" — expected y/yes/n/no`)

## Safety properties

- **CI-safe:** `--interactive` is silently skipped when stdin is not
  a terminal (`isTerminal(os.Stdin)` returns false). CI environments
  pipe stdin, so the prompt never blocks.
- **`--dry-run` safe:** `--interactive` is a no-op when combined with
  `--dry-run` (no destructive steps to confirm).
- **Pre-bump placement:** the prompt fires AFTER all checks pass and
  BEFORE the version bump. If the user aborts, the source is
  unchanged.
- **No partial state:** on abort, returns nil (clean exit), no error.
  The user can re-run the command after fixing whatever they wanted.

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| Tests | **337 PASS, 0 FAIL** |
| Data races | **0** |
| Cross-compile | **6/6** |

## Iteration discipline

No issues caught in this sprint (rare!). First-pass green across
all 6 new tests. The earlier `runRelease` signature change rippled
through 4 existing tests (TestReleaseRejectsInvalidVersion,
TestReleaseAcceptsSemver, TestReleaseAcceptsVPrefix,
TestReleaseAcceptsPreRelease) — all updated to pass the new
`interactive bool` argument and re-verified green.

## Final tally (post-everything)

- **21 CLI commands** + **21 bundled skills** + **1 open MIT schema spec**
- **337 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**
- **`radiant release --interactive` shipped** — final-confirmation prompt

## Stopping point

v0.6.3 is shippable as-is. The release pipeline now has:
- Pre-flight (clean tree)
- Tag existence check
- Quality gates (build/vet/fmt/test-race)
- **Interactive confirmation** ← NEW
- Version bump
- Cross-compile (6/6)
- Commit
- Git tag
- Telemetry hook (if opt-in, on tag success)

This is the complete, safe, dogfooded release flow. The MCP server
also wraps this pipeline with `radiant_release` hard-coded to
`--dry-run` for safety — so any external caller must use the CLI
for a real release, and now the CLI also asks for confirmation.

Remaining candidates (purely additive):
- More domain skills (`radiant-ml`, `radiant-game`, `radiant-cli`)
- Tag v0.6.3 for real via `radiant release v0.6.3`