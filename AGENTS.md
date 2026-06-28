# radiant-harness — Agent Onboarding

> You are operating inside a **radiant-harness** project.
> **Do not write code, run tests, or modify files directly.**
> All work must go through the `radiant` CLI. This is non-negotiable.

---

## Why

`radiant` enforces a crash-safe Discover→Plan→Execute→Verify→Persist cycle.
Every LLM call is traced, cost-tracked, and checkpointed.
Bypassing it means no tracing, no retry, no cost control, and no audit trail.

---

## Step 0 — Orient yourself (always first)

```bash
radiant doctor          # verify environment (API key, git, model)
radiant boot            # emit ≤500-token manifest with project context
```

Read the boot output before doing anything else. It tells you the active domain,
loaded skills, and any open loop runs.

---

## Step 1 — Single-goal task (Loop Engine)

```bash
# Start an autonomous loop
radiant loop start "<your goal here>" --profile=standard

# Examples
radiant loop start "add input validation to the /api/users POST handler"
radiant loop start "fix the race condition in internal/fleet/dispatch.go"
radiant loop start "write unit tests for internal/config/config.go"

# Monitor progress
radiant loop status                  # latest run
radiant loop status <run-id>         # specific run
radiant loop list                    # all runs
radiant loop history                 # summary table

# If the loop stalls, resume it
radiant loop resume <run-id>

# Cancel if needed
radiant loop cancel <run-id>
```

### Profiles

| Profile | Use when |
|---------|----------|
| `lean` | Trivial fix, ≤3 files touched |
| `standard` | Default — most tasks |
| `thorough` | Complex feature, cross-cutting change |

### Cost / time guards (optional but recommended)

```bash
radiant loop start "<goal>" --max-cost=2.00 --max-time=10m
```

---

## Step 2 — Multi-agent parallel task (Fleet Engine)

Use Fleet when the goal can be decomposed into independent sub-tasks.

```bash
# Plan → dispatch agents in parallel
radiant fleet start "<high-level goal>"
radiant fleet status <run-id>
radiant fleet summary <run-id>

# Resume failed tasks without restarting the whole fleet
radiant fleet resume <run-id>

# Retry a specific task
radiant fleet retry <run-id> <task-id>

# Cancel everything
radiant fleet cancel <run-id>

# History
radiant fleet history
```

---

## Step 3 — After the loop finishes

```bash
radiant loop export <run-id>              # full trace as Markdown
radiant loop export <run-id> --format=json  # machine-readable
radiant loop diff <run-id>               # git diff of what changed
```

---

## Rules

1. **Never** edit files manually before starting a loop — the loop does it.
2. **Never** run `go test ./...` or `go build` directly — the loop's Verify phase does it.
3. If you need to check project state, use `radiant boot` or `radiant loop status`.
4. If a loop fails, use `radiant loop resume` before starting a new one.
5. If you're unsure which profile to use, use `standard`.
6. Check `radiant doctor` if anything feels wrong — it diagnoses the environment.

---

## Config (`.radiant.yaml`)

If the project has a `.radiant.yaml`, its defaults are applied automatically:

```yaml
model: claude-sonnet-4-6
max_iter: 20
profile: standard
webhook_url: ""
fleet_concurrency: 4
fleet_max_retries: 2
auto_route: true
```

You can override any field with CLI flags.

---

## Structured logging

```bash
radiant loop start "<goal>" --log-json     # emit JSONL per LLM call to stdout
```

Each line is a JSON object:
```json
{"time":"...","level":"info","event":"loop.llm_call","run_id":"r1","phase":"execute","model":"claude-sonnet-4-6","tokens":200,"cost_usd":0.0009}
```

---

## Quick reference

```
radiant doctor
radiant boot
radiant loop start "<goal>" [--profile=lean|standard|thorough] [--max-cost=N] [--max-time=Nm] [--log-json]
radiant loop status [<run-id>]
radiant loop list
radiant loop history [--json]
radiant loop resume <run-id>
radiant loop cancel <run-id>
radiant loop export <run-id> [--format=json|md]
radiant loop diff <run-id> [--base=main] [--stat]
radiant fleet start "<goal>"
radiant fleet status <run-id> [--json]
radiant fleet summary <run-id> [--json]
radiant fleet resume <run-id>
radiant fleet retry <run-id> <task-id>
radiant fleet cancel <run-id>
radiant fleet history [--json]
```
