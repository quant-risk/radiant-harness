# AGENTS-FOR-TASKS.md

> **If you are an AI agent that just received a task and a link to this
> repository, this file is for you. Read it before anything else.**
>
> **If you are a human, this file is not for you.** The human-facing
> documentation lives in [`README.md`](README.md). This file is exclusively
> the contract an AI agent follows when handed a task inside this repo.

---

## TL;DR — the 4-step contract

1. **Install + wire MCP into yourself.** See [§ Install](#install--wire-mcp).
2. **Reload yourself.** See [§ Reload](#reload-yourself).
3. **Drive the task through the bounded MCP primitives.** See [§ MCP tools](#mcp-tools).
   The harness takes over from there via four bounded `sampling/createMessage`
   round-trips: `discover → plan → execute → verify`.
4. **Report back.** Show the user the trace, the artifact list, and the gate
   results.

**Do not** call `radiant loop`, `radiant run`, `radiant fleet`, `radiant model`,
or any other CLI subcommand from a shell. They are gated behind
`RADIANT_INTERNAL=1` (intended for CI, not for agents). The only path that
works for you is the MCP tool surface below — see [§ Why this is the only
path](#why-this-is-the-only-path).

---

### Heads-up: self-driven scaffold mode (v3.6.0)

If your host does NOT implement `sampling/createMessage` (Codex GPT-5
first; Cline / OpenCode / Kimi / OpenClaw / VSCode / `mavis-code`
next in line), `radiant_possess` will route to **self-driven scaffold
mode** automatically — the same four phases run, but the harness
emits deterministic Markdown templates (one per phase) with
`[host-agent: fill in ...]` markers instead of relying on LLM
sampling. The project scaffold (`docs/`, `specs/`, `scripts/`,
`.radiant-harness/`) is built either way; only the **content** of
each file comes from you instead of from sampling.

When that happens, the next step is: read every templated file,
replace the markers with real content using your own tools, and run
`./scripts/run.sh` (or whatever the templated entrypoint says). Run
`radiant_phase_status(task_id="…")` to see exactly which files are
still templated.

**Do not** try `RADIANT_INTERNAL=1 radiant possess` to bypass this —
internal mode is for CI. The self-driven path is the supported
behaviour on hosts that don't sample.

### Heads-up: when the agentic driver hits `-32601` (v3.7.1)

v3.7.0 introduced an agentic tool-calling driver — the
host's model can call `read_file`, `write_file`,
`search_code`, `run_gate` natively via the MCP
sampling wire format. **If your host advertises tools but
its first sampling call returns JSON-RPC `-32601` mid-run**
(this happens when the MCP subprocess can't see the agent's
env vars, e.g. Codex CLI doesn't propagate `CODEX_HOME`),
v3.7.1 automatically falls back to the self-driven scaffold
mode. The workdir still lands with `spec.md`, `tasks.md`,
`scripts/run.sh`, `docs/README.md`,
`.radiant-harness/{CONTEXT.md, handoff.md, verify.md}`
populated — no more empty folders when the host rejects
sampling.

If the self-driven fallback triggers, you'll see
`mode: self-driven (sampling unsupported mid-run (driver
fallback v3.7.1))` in the harness output. Same code path
as `mcp__radiant__possess` driven by a host agent with
sampling — you'll get `[host-agent: fill in ...]`
markers that you (or your next agent) replaces with real
content.

If you see this fall back fire on a host that *should*
support sampling, check the host's MCP subprocess env first
(it's a host integration issue, not a harness bug).

### Heads-up: when the host is Hermes TUI synchronous (v3.7.2-prep)

Hermes TUI implements **synchronous tool calls** — it
`wait_for_tool_result` before processing anything else. That
includes nested `sampling/createMessage` requests. The full
sequence is:

```
Hermes TUI                          radiant mcp serve
  │                                       │
  │── tool_call: radiant_possess(task) ───►
  │   (TUI blocked waiting here)            │
  │                                       │── discover (offline) ✓
  │                                       │── plan (offline) ✓
  │                                       │── execute needs sampling
  │                                       │── sampling/createMessage ──► ??? 
  │   TUI can't process it;               │     (callback is dropped)
  │   still waiting on possess ─────►     │
  │                                       │
  ▼                                       ▼
120s timeout on the tool call → fail
```

**This deadlock happens regardless of v3.7.1.** v3.7.1 closes the
*Codex* hollow-stub case (driver surfaces -32601 mid-run → falls
back to self-driven scaffold). It does NOT change the Hermes TUI
flow because the deadlock is at the MCP protocol layer, before
the driver even starts. The release notes framed the fix
broader than the actual scope; v3.7.2-prep already exposes
`radiant_run_gate` and `radiant_possess_async` in `tools/list`
(returning structured "v3.7.2 in-development" stubs today), and
PR-B/PR-C will wire real subprocess plumbing + auto-routing on
synchronous hosts.

**Until then, the supported workstream for Hermes TUI is:**

```
1. mcp__radiant__skill_list                 # enumerate (no round-trip)
2. mcp__radiant__skill_load(name="...")     # read SKILL.md (no round-trip)
3. mcp__radiant__init / radiant_create_spec # scaffold (no round-trip)
4. Python / bash directly                  # fill the [host-agent: ...] markers
```

Each MCP call is small and returns fast — none of them trap the
TUI. The harness becomes your **research / spec-writer /
skill-loader**; the actual code execution lives in the same chat
where Python and bash already run. This is the same hybrid pattern
that resolved the iFood Pago MenuFlex case end-to-end. See
`CHANGELOG.md` `[3.7.2-prep]` for full context and operational
notes.

**Do NOT** call `mcp__radiant__possess` from a synchronous TUI host
expecting a populated result. The harness will only land you with
templated scaffolds and 120 s of waiting. If you need end-to-end
execution via `radiant_possess`, use an async-capable host
(Claude Code, or terminal `radiant run` with a configured LLM
provider).

### Heads-up: skill names changed (v3.7.2-prep)

`credit-risk-modeling`, `ml-modeling`, `regulatory-compliance`,
`risk-management` are **gone**. Real names now:

- `credit-risk` (PD/LGD/EAD, IFRS 9, Basileia, scorecards)
- `ml` (machine-learning workflow)
- `regulatory` (Basileia, IFRS, compliance)
- `fraud-detection`, `nova-feature`, `camada-agentica` — unchanged

If you have cached skill names from prior runs, call
`mcp__radiant__skill_list` and re-pick. `mcp__radiant__skill_load`
on a ghost name returns an empty content block.



---

## Install + wire MCP

```bash
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh \
  | bash -s -- --agent=hermes --self-for-agent
```

Replace `hermes` with whichever agent you are running inside:
`claude` · `codex` · `cursor` · `hermes` · `mavis-code` · `opencode` ·
`kimi` · `openclaw` · `windsurf` · `zed` · `cline` · `vscode-copilot`.

If you cannot tell which one you are, pass `--setup-mcp` to autodetect:

```bash
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh \
  | bash -s -- --setup-mcp
```

The installer downloads the matching binary, verifies the SHA256, writes the
host agent's MCP-server entry into its config file, and (for Hermes only)
also writes the `sampling:` block so you don't have to. **One step.**

---

## Reload yourself

The MCP server entry is invisible until your host agent process picks up the
new config:

| Host agent            | How to apply                                              |
|-----------------------|-----------------------------------------------------------|
| **Hermes**            | Send `/reload-mcp` in chat.                               |
| **Claude Code**       | The shell session has to restart. `claude` does not hot-reload MCP. |
| **Cursor / Windsurf / OpenCode / VS Code** | Restart the editor.                          |
| **Codex CLI**         | Exit and re-launch with `--agent=<x>`.                    |
| **Kimi / Cline / MiniMax / OpenClaw / Zed** | Restart the process / editor.            |

The installer prints the exact restart command for the agent it detected.
After reload you should see `radiant_possess`, `radiant_phase_status`,
`radiant_skill_list`, and `radiant_skill_load` in your tool list. Confirm with
`tools/list` against the radiant MCP server.

---

## MCP tools

The harness exposes **four bounded primitives + one legacy alias** as MCP
tools. The loop is decomposed into bounded calls on purpose — see
[§ Why this is the only path](#why-this-is-the-only-path) for the production
post-mortem that led to this design.

| Tool | When | Parameters |
|------|------|------------|
| `radiant_skill_list` | Always call once on non-trivial work. | `filter?: string` (substring against name + description) |
| `radiant_skill_load` | Read one bundled skill's `SKILL.md` + `frontmatter.yaml`. | `name: string` (required) |
| **`radiant_possess`** | The main call: drives the user's task through discover → plan → execute → verify. | `task: string` (required, verbatim from user) · `workdir?: string` (absolute path, default = agent CWD) · `profile?: "lean" \| "standard" \| "thorough"` (default `standard`) |
| `radiant_phase_status` | Inspect / resume tracking of a `radiant_possess` run. | `task_id: string` (16-char prefix from the trace) · `workdir?: string` |
| `radiant_run` | **DEPRECATED alias.** Same as `radiant_possess(task=goal)`. Kept for older hosts. Do not call this in new code. | same as above, plus deprecated `max_iter` / `max_cost` / `max_time` (currently ignored) |

### Typical workflow

```text
1. mcp__radiant__skill_list(filter="credit-risk")           # see what's bundled
2. mcp__radiant__skill_load(name="nova-feature")             # read SKILL.md if relevant
3. mcp__radiant__possess(
       task     = "<the user's original prompt, verbatim>",
       workdir  = "<absolute path of the project directory>",
       profile  = "standard",                                # lean | standard | thorough
   )
4. (optional) mcp__radiant__phase_status(task_id="…")        # read the trace mid-run
5. report the trace + artifacts + gate results back to the user
```
| Tool | When | Parameters |
|------|------|------------|
| `radiant_skill_list` | Always call once on non-trivial work. | `filter?: string` (substring against name + description) |
| `radiant_skill_load` | Read one bundled skill's `SKILL.md` + `frontmatter.yaml`. | `name: string` (required) |
| **`radiant_possess`** | The main call: drives the user's task through discover → plan → execute → verify. | `task: string` (required, verbatim from user) · `workdir?: string` (absolute path, default = agent CWD) · `profile?: "lean" \| "standard" \| "thorough"` (default `standard`) |
| `radiant_phase_status` | Inspect / resume tracking of a `radiant_possess` run. | `task_id: string` (16-char prefix from the trace) · `workdir?: string` |
| `radiant_run` | **DEPRECATED alias.** Same as `radiant_possess(task=goal)`. Kept for older hosts. Do not call this in new code. | same as above, plus deprecated `max_iter` / `max_cost` / `max_time` (currently ignored) |

### Typical workflow

```text
1. mcp__radiant__skill_list(filter="credit-risk")           # see what's bundled
2. mcp__radiant__skill_load(name="nova-feature")             # read SKILL.md if relevant
3. mcp__radiant__possess(
       task     = "<the user's original prompt, verbatim>",
       workdir  = "<absolute path of the project directory>",
       profile  = "standard",                                # lean | standard | thorough
   )
4. (optional) mcp__radiant__phase_status(task_id="…")        # read the trace mid-run
5. report the trace + artifacts + gate results back to the user
```

The harness owns:

- **Discover** — reads `CONTEXT.md`, sniffs the repo, picks 3–10 bundled skills.
- **Plan** — decomposes the goal into acceptance criteria + tasks with explicit gates.
- **Execute** — drives the host agent through the tasks (writes files, runs gates).
- **Verify** — a *fresh* LLM round-trip judges the diff vs the AC. The same model
  never approves its own work.

State is persisted to `.radiant-harness/state/possess-<task-id>/state.json` between
phases, so a timeout or crash does not lose progress. Re-calling
`radiant_possess(task=…, workdir=…)` with the same pair resumes from the last
checkpoint instead of restarting.

### What the host agent must emit

Your `sampling/createMessage` response **MUST** end with one of these blocks,
exactly. Anything else and the harness retries.

```
VERDICT: APPROVED
SCORE: 1.00
EVIDENCE: <one sentence>
ESCALATE: false
ISSUES:
```

(per-iteration reviewer check)

```
REVIEW: PASS
SCORE: 1.00
EVIDENCE: <one sentence>
FINDINGS:
```

(post-convergence review panel)

Trailing prose after the keyword (`VERDICT: APPROVED — gates green`) is fine;
the parser reads the first word.

---

## Failure modes and what to do

| Symptom                                              | Do this                                                                                |
|------------------------------------------------------|----------------------------------------------------------------------------------------|
| `radiant_possess` not in your tool list              | The reload in [§ Reload](#reload-yourself) didn't run. Restart, re-check.             |
| `radiant_possess` exits with `phase X FAILED: …`     | Re-call `radiant_possess(task=…, workdir=…)`. The harness resumes from the failed phase — you do **not** start over. |
| `sampling/createMessage` returns timeout             | Raise `RADIANT_SAMPLING_TIMEOUT` to `180s` in the harness config, retry.               |
| Host returns `JSON-RPC -32601` for sampling          | The host does not implement sampling. **v3.6.0:** the harness now routes to *self-driven scaffold mode* automatically — it still fills `specs/0001-<slug>/spec.md`, `tasks.md`, `scripts/run.sh`, `docs/README.md`, `.radiant-harness/CONTEXT.md`, `handoff.md`, `verify.md` with deterministic templates. Each template carries a `[host-agent: fill in …]` marker pointing at the section you should replace. Persist this fact via the `~/.radiant-harness/agent-capabilities.json` probe cache so subsequent runs of the same host short-circuit to self-driven without paying the cost of another -32601. |
| `radiant loop` shell command exits 1 with "internal helper" gate | Expected. Use `radiant_possess` instead. If you really need the CLI for scripting, the user must set `RADIANT_INTERNAL=1` themselves; do **not** try to bypass. |
| Hallucinated `<function=…>` XML in `sampling/createMessage` response | The phase prompts are text-only by design (v3.5.1). If your host still emits XML, answer the prompt with Markdown and fenced code blocks, not tool-call syntax. The host agent (you) is meant to apply those with your own Read/Write/Bash tools. |

---

## Why this is the only path

The previous design wrapped the entire loop in a single MCP tool call
(`radiant_run(goal=…)`). It worked against synthetic sampling responders
where every call is instant, but failed with real hosts:

- **Hermes (mimo / xiaomi):** 20–40 s cold-start per sampling call. Four
  phases × ~30 s blow past every host's outer 300 s timeout.
- **Codex (GPT-5):** often didn't even see the tool surface — it would
  fall back to invoking `radiant loop` / `radiant model` from a shell,
  which `radiant` rejects for any caller that is not CI / itself.
- **OpenCode / MiniMax:** similar fallback behaviour.

The v3.3.0 redesign (refined in v3.5.1) decomposes the loop into bounded
primitives (`skill_list` / `skill_load` / `possess` / `phase_status`) so
**no single MCP round-trip can time out** and the host agent stays in
control of executing the work. This is the contract. Following it is the
difference between "the harness drove my task" and "I spent an hour
debugging why `radiant_run` hangs".

---

## Staying current

This file is **not** auto-regenerated by `radiant update` (that command
rewrites only `AGENTS.md`, the universal dev-team onboarding). When the
harness releases a new major version, re-read this file and the
`radiant_possess` tool description — they may have grown new parameters.

To refresh the binary in place:

```bash
curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh | bash
```

The one-liner is idempotent and verifies the SHA256 before installing.
