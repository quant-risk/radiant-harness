# Sprint-6 kickoff — v3.7.3 scope

**Sprint-5 closed at:** commit `b9d70a0` (audit-install regression
fix). Local-only commit, no push. v3.7.2 release tag v3.7.2 →
`81b51e` is unchanged.

This is a planning doc only; no new spec.md / tasks.md generated
yet. Sprint-6 picks up the open items already proposed in
`specs/0001-v3.7.2-close-dropin-gap/sprint-5-wrap-up.md`.

## Scope (carry-over, not changed)

1. **Hermes TUI `radiant_possess` end-to-end** — gate primitives
   shipped (v3.7.2); protocol-level sync-host detection is the
   missing piece. Acceptance: a Hermes TUI session completes the
   bundled `credit-risk` skill workflow end-to-end in <30 s
   wallclock.
2. **Kill the `radiant_run` alias** — currently DEPRECATED;
   remove the `case "radiant_run":` switch in
   `cmd/radiant/cmd_mcp_runtime.go::callMCPToolLight` and the
   row in `AGENTS-FOR-TASKS.md` § MCP tools. v3.8.0.
3. **Profile-aware budgets** — `--budget <tokens|usd|time>` + a
   per-profile table under `internal/loop/budget.go` so runaway
   loops can be capped cleanly.

## New observation from validation re-run

The Go module proxy has finally re-indexed the v3 path — Path C
of `make audit-install` went from `SKIP ("module proxy still
mirrors v0.x")` to actually succeeding in 7-8 s. **Q1 from sprint
5 is essentially resolved.**

The audit-install script needed a one-line fix to recognise a
v3 build that reports `3.7.2` (no leading `v`) instead of `v3.7.2`
(which would have been the case if `make release`'s ldflags were
reused). That's the `b9d70a0` commit.

After this:
- `make audit-install` summary now reads `2 PASS + 1 SKIP + 0 FAIL`
- `make smoke` continues green

This means the canonical drop-in for a fresh agent still works end
to end on macOS arm64 today (it was probably also working last
night, just not yet visible to our gate).

## Open items (carry over from sprint-5 wrap-up)

- **Q2** `fleet.TestRunAllContextCanceled` — flaky 1/5 × isolated
  runs, fail message `expected non-zero exit for killed process`.
  Probability: sleep-based race in `fleet.dispatch_test.go:327`.
  ~30 min fix. Not blocking release.
- **Q3** `radiant_run` deprecation window — counted down on the
  next cut.

## Tricky bits to flag for the next session

- The binary version reported by `radiant --version` differs
  across install paths: `make release` reports `v3.7.2` (the
  ldflags set it explicitly); `go install` from proxy reports
  `3.7.2` (no ldflags). The audit gate now accepts either form,
  but operators building ad-hoc should know that `go install`
  without `-ldflags "-X main.version=v3.7.2"` will produce a
  binary that looks like it's "ahead" of the tag.
- A side-effect of the recent force-pushed `v3.7.2` tag is that
  the `git ls-remote` history of the public mirror briefly
  exposed the *old* SHA before the API delete-then-recreate. The
  release id changed (346600132 vs the original 346597081) so if
  anyone scripted around the old release id it will need
  updating. This is unlikely but noted.

## Tag/release state at sprint-5 close

- `main` — `b9d70a0` (audit-install regression fix).
- Tag `v3.7.2` (force-pushed) — points to `81b51e7` on **both**
  remotes (the `_origin_` (`quant-risk/radiant-harness`) public
  one was force-recreated via GitHub API after a transient 404
  window caught during the release-id swap).
- Release on `_origin_` — id `346600132`, all 7 assets re-uploaded,
  `download URLs return HTTP 200` confirmed.
- Release on `_fortvna_` — id `346596288` (original, still alive).
- CDN — `raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh`
  was cached for >2 min after the force-push. Eventually returned
  the new blob (`9f37043d…`).

## Sprint-6 hooks

The session can resume from any context. To restart
`v3.7.3 — close the v3.8-prep holes`:

```
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/v3.7.2/install.sh | bash
/usr/local/bin/radiant --version  # → v3.7.2
cd <repo>
make smoke                       # rc=0 expected
```

Then open the new session with prompt:

> "Sprint-6: open `specs/0001-v3.7.2-close-dropin-gap/sprint-6-kickoff.md`,
> pick up the three scope items, and ship v3.7.3."

## Who

- Sprint-5 (this thread): Mavis (Mavis) under Henrique.
- Net delta since previous close: 0 new spec, 0 new code; 1
  one-line audit-install fix; 1 new wrap-up doc; 1 commit (local).
