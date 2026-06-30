# HOSTS — Per-host opt-in matrix for async subprocess gates

This document is the offline-readable counterpart of
`radiant doctor --async-host`. The CLI tool reads the same
matrix at runtime (see `internal/fleet/...` no — the matrix
lives in `cmd_doctor.go::hostAsyncRecommendations`).

## How to read this table

For each of the 13 Light-mode hosts, the table answers:

- **Loop async subprocess recommended?** — should you set
  `RADIANT_ASYNC_SUBPROCESS=1` (or `--async-subprocess` on
  `radiant mcp serve`)?
- **Fleet async subprocess recommended?** — should you set
  `RADIANT_FLEET_ASYNC_SUBPROCESS=1` (or `--fleet-async-subprocess`)?

The "Reason" column explains WHY (or why not). When in doubt,
the conservative default is **NO** — inline is faster for
small workloads and adds no extra process tree.

## The matrix (v3.7.10)

| Host | Loop async subprocess | Fleet async subprocess | Reason |
|------|----------------------|------------------------|--------|
| **Hermes** | ✅ Recommended | ❌ Not recommended | Hermes TUI gates tool-call completion on subprocess exit (120s window); opting in keeps work out of the TUI process tree. Inline 4-phase loop completes in <500ms but the TUI's subprocess model still benefits from the explicit fork. |
| **Claude Code** | ❌ Not recommended | ❌ Not recommended | No documented tool-call timeout pressure for inline phases; opt-in available but not required. |
| **Cursor** | ❌ Not recommended | ❌ Not recommended | Same as Claude Code. |
| **Windsurf** | ❌ Not recommended | ❌ Not recommended | Same as Cursor. |
| **Zed** | ❌ Not recommended | ❌ Not recommended | Same as Cursor. |
| **VS Code Copilot** | ❌ Not recommended | ❌ Not recommended | Same as Cursor. |
| **MiniMax Code** | ❌ Not recommended | ❌ Not recommended | Same as Claude Code. |
| **Codex** | ❌ Not recommended | ❌ Not recommended | Open shell subprocess model does not benefit from a second-level fork. |
| **Cline** | ❌ Not recommended | ❌ Not recommended | VS Code extension subprocess model — inline fine. |
| **OpenCode** | ❌ Not recommended | ❌ Not recommended | Terminal-native; inline finishes before the host's read window. |
| **Google Gemini CLI** | ❌ Not recommended | ❌ Not recommended | Terminal-native; inline fine. |
| **Kimi CLI** | ❌ Not recommended | ❌ Not recommended | Terminal-native; inline fine. |
| **OpenClaw** | ❌ Not recommended | ❌ Not recommended | MCP-first; inline fine. |
| **(no agent detected)** | ❌ Not recommended | ❌ Not recommended | Cannot recommend — wire a host via `radiant setup-mcp --agent=<host>`. |

## How to opt in

**Per-shell (env var):**

```bash
export RADIANT_ASYNC_SUBPROCESS=1        # loop async subprocess
export RADIANT_FLEET_ASYNC_SUBPROCESS=1  # fleet async subprocess
radiant mcp serve
```

**Per-invocation (CLI flag):**

```bash
radiant mcp serve --async-subprocess
radiant mcp serve --fleet-async-subprocess
```

CLI flag wins over env var when both are set.

## How to verify your opt-in

```bash
radiant doctor --async-host
```

This emits:

```
radiant doctor --async-host
────────────────────────────────────────────────────────────
  agent                       hermes (confidence N)
  loop async subprocess (env)  false (RADIANT_ASYNC_SUBPROCESS)
  fleet async subprocess (env)  false (RADIANT_FLEET_ASYNC_SUBPROCESS)
────────────────────────────────────────────────────────────
  loop async subprocess:    RECOMMENDED
    Hermes TUI: 120s tool-call window; inline 4-phase loop ...
  fleet async subprocess:  NOT RECOMMENDED
    no documented CI host reproducing a fleet cross-process need yet.
────────────────────────────────────────────────────────────
  → opt in:  radiant mcp serve --async-subprocess
    (or:    export RADIANT_ASYNC_SUBPROCESS=1)
```

When a recommendation is unmade, `doctor --async-host` exits
non-zero so CI / lint can catch regressions.

## When to update this matrix

**Add a new host:** the matrix lives in
`cmd_doctor.go::hostAsyncRecommendations`. Adding an entry
requires a `Reason` string explaining the recommendation — without
one the entry stays "Not recommended" (conservative default).

**Flip a host from "Not recommended" to "Recommended":** this is
the kind of change that affects every user of that host, so it
needs:

1. A real reproduction (traceback from a CI run showing the
   failure mode the subprocess gate fixes).
2. A maintainer sign-off in the PR.
3. A CHANGELOG entry explaining the new opt-in rationale.

**Flip a host from "Recommended" to "Not recommended":** rare
(usually means the host fixed the underlying issue). Same
requirements as above.

## When NOT to opt in

- **You're running a small loop.** Inline mode finishes the full
  4-phase loop in <500ms. Subprocess mode adds fork/exec overhead
  for no latency win on small workloads.
- **You're debugging.** Inline mode keeps the work in the parent
  process where `dlv` / `pprof` can attach. Subprocess mode puts
  the work in a child that's harder to instrument.
- **You don't have a reproduction.** The matrix is conservative
  on purpose. Don't opt in "just because" — there's a real
  fork/exec overhead, and the inline path works for the vast
  majority of hosts.

## Related documentation

- `CHANGELOG.md` — version history for the async subprocess
  primitives (v3.7.7 loop, v3.7.9 fleet, v3.7.10 opt-in matrix).
- `docs/ROADMAP.md` — current sprint + backlog (look for the
  "Real CI host reproducing fleet cross-process need" item to
  see what reproduction would flip fleet async to "Recommended").
- `cmd/radiant/cmd_doctor.go` — source of truth for the matrix.
- `cmd/radiant/cmd_mcp_serve.go` — CLI flag + env var precedence
  chain (CLI > env > default off).