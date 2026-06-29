# Examples

The Light binary exposes **one** MCP tool: `radiant_run`. Every example below
is a real walkthrough of that single entry point.

---

## Example 1 — Add a `/healthz` endpoint (the canonical demo)

This is the smallest task that exercises the full loop. You should be able to
reproduce it in under five minutes against any Go HTTP service.

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

### Wire radiant into your agent

```bash
# from a real release (or `go install` from source)
radiant setup-mcp
# → writes .mcp.json (or whatever your agent uses)
```

Restart your agent (Claude Code, Cursor, Hermes, …).

### Ask your agent

From inside the agent:

> *"use radiant-harness to add a /healthz endpoint that returns 200 OK with
> a JSON body {"status":"ok"} and write a table-driven test that covers
> status code and content-type. Add a Makefile target `make test` that runs
> it."*

The agent will call:

```text
radiant_run(
  goal="Add a /healthz endpoint to the Go HTTP server in main.go. Returns
        200 OK with JSON body {\"status\":\"ok\"}. Add a table-driven test
        covering status code and Content-Type. Add a Makefile target
        `make test` that runs the tests. Keep the change minimal.",
  profile="standard",
  max_iter=20,
  max_cost="1.00",
  max_time="5m"
)
```

The harness drives the loop inside your agent's process. When it's done, you
get a JSONL trace at `.radiant-harness/traces/<run-id>.jsonl`.

---

## Example 2 — Run with budgets

The four budgets are independent — set whichever matters for the task:

```text
radiant_run(
  goal="refactor internal/loop/state.go to use generics instead of interface{}",
  profile="thorough",       # lean | standard | thorough (default: standard)
  max_iter=50,              # cap iterations (default 20)
  max_cost="3.00",          # dollar cap, must parse as float
  max_time="20m"            # wall-clock cap, accepts "30s", "5m", "1h"
)
```

What happens when a budget is hit:

- `max_iter` → loop exits with `budget_exhausted:iter` in the trace.
- `max_cost` → loop exits with `budget_exhausted:cost`.
- `max_time` → loop exits with `budget_exhausted:time`.

In all three cases, the trace is still written and the partial work is
preserved. You can re-run with bigger budgets and the harness will pick up
from where it stopped.

---

## Example 3 — Read a trace after the fact

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

## Example 4 — Wire a non-default agent

If `radiant setup-mcp` doesn't auto-detect your agent, force it:

```bash
radiant setup-mcp --agent=hermes --dry-run     # preview the config
radiant setup-mcp --agent=hermes --global      # write to ~/.config/hermes/
```

See the [supported agents table](README.md#-supported-agents) for the full
list of `--agent=` values.

---

## Example 5 — Verify the zero-API-key guarantee

This is what makes the Light binary auditable. Anyone can run it:

```bash
# Build the binary (or download from release)
make build

# 1. No HTTP-LLM client symbols in the binary
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# → (empty)

# 2. No provider names in the strings table
strings bin/radiant | grep -iE 'anthropic|openai|openrouter'
# → (empty)

# 3. The full smoke battery
make smoke
# → 17/17 OK
```

These three checks are the entire "no API key" story.

---

## What the Light binary does NOT do

The Light binary is intentionally minimal. The following are **not** in this
repo — they live in the Full binary at
[`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full):

- `radiant loop`, `radiant run`, `radiant fleet`
- `radiant run --resume` (crash-safe BoltDB resume)
- `radiant init`, `radiant spec`, `radiant product`, `radiant audit`, `radiant evals`
- `radiant doctor`, `radiant release`, `radiant setup-ci`, `radiant handoff`, `radiant state`
- `radiant config --provider=openrouter` (Light needs no provider config)
- `radiant views`, `radiant update`
- Additional MCP tools: `radiant_spec`, `radiant_adr`, `radiant_product`, `radiant_evals`, `radiant_audit`, `radiant_release`

If you need any of those, use the Full repo. The split is enforced at the
source level — there's no flag or config that toggles modes. See
[`docs/TWO-REPOS.md`](docs/TWO-REPOS.md).

---

## See also

- [README](README.md) — overview
- [INSTALL](INSTALL.md) — installation
- [`docs/TWO-REPOS.md`](docs/TWO-REPOS.md) — why there are two repos