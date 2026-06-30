# Sprint-7 wrap-up — v3.7.3 final

**Sprint-7 closed at:** commit `f5afd84` (smoke allow-list) +
release tag `v3.7.3` → `f5afd84`. Local-only — pushed by Henrique
on a separate op when ready.

Sprint-6 shipped the carrier-scope (T1–T3) without unblocking the
read-only-when-broken symptom on the CLI surface. Sprint-7 picks up
the open observation from sprint-6 kickoff § "New observation from
validation re-run": a fresh sandbox invoking the README-promoted
commands exits `critical_failure` with no artefacts.

## The trap (re-stated)

From a fresh shell (`/tmp/scratch`) with `radiant-harness`
installed via `curl … | bash`:

  - `radiant loop start "ship X"` → exits `critical_failure:
    resolveBackends: HTTP fallback unavailable` after emitting
    nothing into the workdir.
  - `radiant run <spec-dir>` → exits `Error: no API key provided`.
  - `radiant fleet start "ship X"` → exits `0` and prints
    `✓ Fleet started` but writes a `tasks: []` placeholder JSON
    into `.radiant-harness/fleet/<runID>.json`.

Three shapes of the same failure: the contract was implemented
**only** for the Full-mode + API-key deployment. Light mode from a
shell has no MCP transport, so every sampling path silently
degrades. The README never said "requires `RADIANT_FORCE_SAMPLING=1`
or a wired MCP host". The audit-docs pass was a false positive
because the commands exist — they just don't produce the artefacts
the prose implies.

## What changed

### Code

| File                                       | Lines | What                                                   |
|--------------------------------------------|-------|--------------------------------------------------------|
| `cmd/radiant/main.go`                      | +50/-18 | `publicCommands` extended to include `loop`, `run`, `fleet`, `worktree`, `state`, `handoff`, `improve`. The header comment now documents exactly why each command is on or off the public list. |
| `cmd/radiant/cmd_loop.go`                  | +122    | New `loopStartCLIDropIn` + `runLoopCLILight`. `loopStartCmd` and `loopResumeCmd` route through them when no API key + no MCP host are present. |
| `cmd/radiant/cmd_fleet.go`                 | +29/-2  | `fleetStartCmd` short-circuits to `runSelfDrivenPossess` under the same conditions; legacy Coordinator path remains for callers with API keys. |
| `cmd/radiant/cmd_run.go`                   | +17     | `runCmd` mirrors the drop-in so `radiant run <spec-dir>` no longer fails with "no API key provided". |
| `scripts/smoke-test.sh`                    | +1/-1   | Version allow-list tracks the v3.7.x release. |

### Behaviour

`RADIANT_FORCE_SAMPLING=1` and explicit `RADIANT_OPENROUTER_API_KEY`
/ `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` continue to bypass the
drop-in so callers who want the real `loop.Run` error path get
that exact error (no silent downgrade).

The drop-in makes `loop start` / `run` / `fleet start` produce
the canonical self-driven scaffold (mirroring what `radiant mcp
possess` already does on a fresh agent wiring):

  `.radiant-harness/CONTEXT.md`
  `specs/0001-<slug>/spec.md` (templated, with `[host-agent: fill
    in …]` markers for the next agent to complete)
  `specs/0001-<slug>/tasks.md` (templated)
  `scripts/run.sh`
  `docs/README.md`
  `.radiant-harness/{state.md, handoff.md, verify.md}`
  `state=/…/.radiant-harness/state/possess-<id>/state.json` with
    `current_phase: done` and all 4 phases marked `done`.

## Validation log (the rehearsals that proved the fix)

```bash
# Replay of the literal "I just installed radiant in a fresh
# sandbox; what does the README command do?" test:

$ cd /tmp && mkdir scratch && cd scratch
$ radiant loop start "ship a Go HTTP server with /healthz"
→ routing `radiant loop start` to the offline self-driven scaffold.
  Reason: no MCP-wired host agent reachable from this shell.
  ✓ discover
  ✓ plan
  ✓ execute
  ✓ verify
✓ Loop finished
  Exit:       success
  Iterations: 4
  Elapsed:    0s

$ ls -la
AGENTS.md  CONVENTIONS.md  settings.json  goal.md
.DS_Store  .agent-context/  .github/  .gitkeep  .radiant-harness/
docs/  hooks/  scripts/  specs/  src/

$ ls .radiant-harness/
CONTEXT.md  handoff.md  manifest.json  skills/  state/  state.md  verify.md

$ ls specs/
0001-ship-a-go-http-server-with-healthz-endpoint/  _templates/
$ ls specs/0001-ship-a-go-http-server-with-healthz-endpoint/
spec.md  tasks.md
```

### Cross-host sweep (12 hosts, Force-Sampling bypass)

Each host with `OPENROUTER_API_KEY=fake` + `RADIANT_FORCE_SAMPLING=1`
correctly bypasses the drop-in and runs `loop.Run`:

```
claude-code, cursor, hermes, codex, opencode, kimi, openclaw,
cline, windsurf, zed, vscode, MiniMax-code → ✓ Loop starting
```

### Mixed case (host detected + no API key)

`cursor`, `hermes`, `windsurf`, `opencode`, `codex` with no API
key all correctly route to the offline drop-in:

```
→ routing `radiant loop start` to the offline self-driven scaffold.
  AGENTS.md=yes  specs/0001-*=yes  CONTEXT.md=yes
```

### Numeric checks

```
make smoke        → 17 OK checks, 0 FAIL
make test-agents  → 12/12 PASS (claude, cursor, hermes, codex,
                                 opencode, kimi, openclaw, cline,
                                 windsurf, zed, vscode, MiniMax)
```

## Lessons (compactly)

1. **Audit skill level: walk the README from a fresh sandbox and
   assert workdir side effects.** That's the only check that
   catches both the gate-on-public-command bug and the
   `tasks: []` hollow-stub bug at the same time. audit-docs
   catches the first; audit-install catches the second; only an
   end-to-end walkthrough catches the third (silent print-done-do-
   nothing).

2. **`RADIANT_INTERNAL=1` is not a release gate.** It was used as
   one (every Light-mode harness command was hidden behind it),
   which meant the README's headline examples were literally
   untrue. Either a command is in the public list (`publicCommands`)
   or it shouldn't be referenced in the README. There is no
   middle ground.

3. **`fleet store start wrote `tasks: []`** but printed "started"
   — the worst kind of hollow-stub (success-shape with no
   payload). Any skeleton command needs an explicit self-check:
   "did I write something useful? If not, print the stub
   disclaimer or fail."

4. **Drop-in > silent failure.** Returning an error from a CLI is
   fine when the caller can act on it (e.g. "no API key provided,
   set OPENROUTER_API_KEY"). It's disastrous when the only path to
   avoid the error is a different surface (`radiant mcp possess`)
   the casual reader won't discover. The fix here is auto-route to
   the offline self-driven scaffold — same artefacts, same
   contract, no surprise.

5. **The harness audit pattern** (build → walk every public
   command → assert observable workdir side effects) should be
   a Makefile target added in v3.8.0. Today the smoke test
   covers 17 binary-level checks; running 4 fresh-shell
   rehearsals on top of that surfaced 3 bugs in 5 minutes.

## What's NOT done in this sprint (carry-over to v3.8.0)

1. **`make release-shell-rehearsal`** target that does the
   actual "curl … | bash" flow + walks the README. Without it,
   this trap will re-emerge every time someone adds a new
   harness CLI command.

2. **`radiant fleet status` worklog is still placeholder.** The
   drop-in route successfully bypasses the empty-store, but
   `radiant fleet status <runID>` on a self-driven run prints
   nothing useful. v3.8.0.

3. **`radiant loop start` produces a `run-<timestamp>` ID, not
   the same `possess-<id>` used by the MCP path.** Cross-surface
   resume will not work. v3.8.0.

4. **Push + Release.** v3.7.3 is local-only. To ship, push the
   tag to both remotes (`fortvna` private + `quant-risk` public)
   and re-create the GitHub Release artefacts via API. The `gh`
   auth workaround from sprint-5 (`gh auth status` reports "not
   logged in" on Henrique's machine; use git credential fill)
   still applies.
