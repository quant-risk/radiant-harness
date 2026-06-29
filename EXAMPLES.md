# Examples

radiant exposes **55 commands**. The examples below cover the ones you'll
actually use day-to-day. Every example assumes you've installed the binary
and, where the command needs an LLM, wired up a host agent via
`radiant setup-mcp`.

---

## Example 1 — Add a `/healthz` endpoint (the canonical demo)

The smallest task that exercises the full loop. Reproducible in under five
minutes against any Go HTTP service.

### Setup

```bash
mkdir healthz-demo && cd healthz-demo
go mod init example.com/healthz
```

Add a minimal `main.go`:

```go
package main

import "net/http"

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    http.ListenAndServe(":8080", nil)
}
```

### Drive the loop from your shell

```bash
radiant loop start "add a /healthz endpoint that returns 200 OK with a JSON body {\"status\":\"ok\"} and write a table-driven test covering status code and Content-Type"
```

The harness drives the loop and calls `sampling/createMessage` to whatever
agent you wired in via `radiant setup-mcp`. When it's done, you get a JSONL
trace at `.radiant-harness/traces/<run-id>.jsonl`.

### Inspect progress

```bash
radiant loop status               # list running loops
radiant loop status <run-id>      # one specific loop
radiant trace show <run-id>       # reasoning trace
```

### Cancel or resume

```bash
radiant loop cancel <run-id>      # abort
radiant loop resume <run-id>      # resume from journaled state
```

---

## Example 2 — Run with budgets

The four budgets are independent — set whichever matters for the task:

```bash
radiant loop start "refactor internal/loop/state.go to use generics instead of interface{}" \
  --profile=thorough \
  --max-iter=50 \
  --max-cost=3.00 \
  --max-time=20m
```

What happens when a budget is hit:

- `max-iter` → loop exits with `budget_exhausted:iter`.
- `max-cost` → loop exits with `budget_exhausted:cost`.
- `max-time` → loop exits with `budget_exhausted:time`.

In all three cases, the trace is still written and the partial work is
preserved. Re-run with bigger budgets and the harness picks up where it stopped.

---

## Example 3 — Spec-driven dev

Cut a feature as `spec.md` + `tasks.md`, then run it.

```bash
# 1. scaffold a spec
radiant spec "rate-limit middleware" \
  --tier=feature \
  --ac="AC1: 100 requests/min per IP" \
  --ac="AC2: returns HTTP 429 with Retry-After header when over quota" \
  --task="1: add token-bucket algorithm in internal/ratelimit" \
  --task="2: wire as middleware in cmd/server/main.go" \
  --task="3: write table-driven test covering under/at/over quota" \
  --gate="go test ./internal/ratelimit/..." \
  --gate="go build ./..." \
  --covers="1:AC1" \
  --covers="2:AC1,AC2" \
  --covers="3:AC2"
```

This produces `specs/0001-rate-limit/spec.md` + `tasks.md` with the AC ↔ task
mapping.

```bash
# 2. run it (one-shot, not multi-step)
radiant run specs/0001-rate-limit

# 3. validate AC↔test coverage
radiant evals

# 4. review the PR
radiant review-pr specs/0001-rate-limit --run-gates -o pr-review.md
```

---

## Example 4 — Lean Inception

Start a product discovery workflow from a one-liner.

```bash
radiant product "API observability for small dev teams" --mvp-weeks=6
```

Produces `docs/product/inception.md` with the 6-phase Lean Inception
template, plus 3 persona slots to fill in. After filling, choose 3-5
features for the MVP and continue with `radiant spec ...`.

---

## Example 5 — Fleet mode (multi-agent)

Parallel agent coordination: Planner + Implementer + Verifier + Summarizer
work concurrently on the same goal.

```bash
# Start a fleet
radiant fleet start "migrate auth from session cookies to JWT"

# Inspect progress
radiant fleet status <run-id>
radiant fleet watch <run-id>      # tail logs

# Add tasks mid-flight
radiant fleet dispatch <run-id> --task="rotate signing keys"

# Retry failed tasks
radiant fleet retry <run-id> --task=<task-id>

# Cancel
radiant fleet cancel <run-id>
```

Fleet runs land in `.radiant-harness/fleets/<run-id>/` with per-agent traces.

---

## Example 6 — Doctor + verification

Diagnose the wire-up and prove the zero-API-key guarantee.

```bash
radiant doctor
```

Expected output covers:

- **Agent host detection** — which agent is calling radiant, confidence score
- **MCP wiring** — which config files were written, whether the binary path is correct
- **Zero-HTTP-LLM guarantee** — runs `nm | grep` and `strings | grep` to prove no provider symbols
- **Binary self-check** — version, command count, bundle size

To verify the zero-API-key guarantee manually:

```bash
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# → (empty)

strings bin/radiant | grep -iE 'anthropic|openai|openrouter'
# → (empty)

ls -lh bin/radiant
# → ≈ 11 MB

make smoke
# → 17/17 OK
```

These three checks are the entire "no API key" story.

---

## Example 7 — Release pipeline

Cut a release end-to-end:

```bash
# 1. dry-run (preview only)
radiant release v3.3.0 --dry-run

# 2. real release (runs gates, cross-compiles 6 targets, commits, tags)
radiant release v3.3.0
```

`radiant release` runs the full chain:

1. Pre-flight (clean tree)
2. Quality gates (build / vet / fmt / test-race)
3. Version bump in `cmd/radiant/main.go`
4. Cross-compile to `bin/radiant-{linux,darwin,windows}-{amd64,arm64}`
5. Generate SHA256SUMS
6. Commit + git tag

---

## Example 8 — Worktrees for parallel agents

Isolated git worktrees for parallel agent work without branch conflicts.

```bash
# create a worktree for agent A
radiant worktree add agent-a --branch=feat/auth-migration

# create one for agent B
radiant worktree add agent-b --branch=feat/rate-limit

# list active worktrees
radiant worktree list

# prune stale ones
radiant worktree prune
```

Each worktree gets its own `.radiant-harness/` directory; traces don't
collide between agents.

---

## Example 9 — Trace analysis + self-improvement

After several loop runs, analyse the traces and propose skill edits.

```bash
# show the last 10 traces
radiant trace list --last=10

# inspect one
radiant trace show <run-id>

# run the self-improvement engine
radiant improve analyze --last=20
radiant improve apply --dry-run     # preview skill edits
radiant improve apply               # commit them
```

`radiant improve` uses MCP sampling to ask the host agent to look at recent
traces and propose updates to bundled skills.

---

## Example 10 — Read a trace after the fact

```bash
ls .radiant-harness/traces/
# run-2026-06-29-14-23-01-7f3a.jsonl
# run-2026-06-29-14-31-44-91be.jsonl

# pretty-print one step
jq '.' .radiant-harness/traces/run-2026-06-29-14-23-01-7f3a.jsonl | less
```

Each line is one step. Common fields:

| Field            | Meaning                                              |
|------------------|------------------------------------------------------|
| `phase`          | `discover` / `plan` / `execute` / `verify`           |
| `step`           | Sequential step number within the phase              |
| `tool`           | Which host-agent tool was called (if any)            |
| `tokens_in`      | Tokens sent to the host agent this step              |
| `tokens_out`     | Tokens returned                                      |
| `cost_usd`       | Cost attributed to this step                         |
| `verdict`        | On verify phase: `pass` / `fail` / `inconclusive`    |

---

## See also

- [README](README.md) — overview + full command list
- [INSTALL](INSTALL.md) — installation
- [`docs/TWO-REPOS.md`](docs/TWO-REPOS.md) — why there are two repos