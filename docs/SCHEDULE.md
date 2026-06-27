# Schedule Stage (Sprint 43)

The **Schedule** stage closes the loop-engineering cycle:

```
Discover → Plan → Execute → Verify → Persist → Schedule ↺
```

Before this, the loop only ran when a human invoked `radiant loop start`. The
scheduler lets the harness read work signals from the repository and decide,
under a policy, whether to re-dispatch an autonomous run — the "S" the senior
Anthropic engineer's loop framework calls out (Discover → … → Schedule).

## Design

`schedule.Evaluate(policy, state, signals, now)` is a **pure function** — every
time-dependent input arrives via `state` and `now`, so the decision is
deterministic and fully testable (no wall-clock reads inside the logic).

### Signals (reasons to run)

| Kind | Source |
|------|--------|
| `new-commits` | commits on HEAD since the last recorded run (`git rev-list`) |
| `pending-work` | count of TODO/FIXME markers in tracked files (`git ls-files`) |
| `failing-gate` | supplied by the caller (`--gate-failing`) when a gate is red |
| `interval` | always present; gated by the policy + rate limit |

### Policy

```go
DefaultPolicy() = {
  Triggers:      [new-commits, failing-gate, pending-work],
  MinInterval:   15 * time.Minute,   // floor between runs
  MaxRunsPerDay: 20,                 // daily cap (0 = unlimited)
}
```

### Decision order

1. **Rate limit** — skip if `now - LastRunAt < MinInterval` (first run exempt).
2. **Daily cap** — skip if `RunsToday >= MaxRunsPerDay` (counter resets when the
   calendar day changes).
3. **Triggers** — run only if at least one *enabled* trigger has a signal with
   `Value > 0`.

State persists atomically to `.radiant-harness/schedule.json` (temp + rename).

## CLI

```bash
# Report only — does not advance state
radiant loop schedule --check

# Evaluate and, on RUN, advance + persist scheduler state
radiant loop schedule

# Signal a red gate from your CI wrapper
radiant loop schedule --gate-failing

# Override policy
radiant loop schedule --min-interval=30m --max-per-day=10
```

### Example

```
$ radiant loop schedule --check
● RUN — triggered by pending-work(2)
  signals:
    - pending-work  TODO/FIXME markers (2)

$ radiant loop schedule          # advances state
● RUN — triggered by pending-work(2)
Dispatch: `radiant loop start "<goal>"` — scheduler state advanced (run 1 today).

$ radiant loop schedule --check  # immediately after
○ SKIP — rate-limited: 0s since last run, need 15m0s
```

## Wiring it into CI / cron

The scheduler decides *whether* to run; dispatching the actual loop is left to
the surrounding automation (cron, CI cron, a systemd timer), e.g.:

```bash
# crontab: every 15 min, let the harness decide
*/15 * * * * cd /repo && radiant loop schedule && radiant loop start "$(cat .radiant-harness/goal.txt)"
```

`radiant loop schedule` exits 0 with a RUN/SKIP report; the caller checks the
output (or a future `--exit-code` flag) to decide whether to invoke the loop.
