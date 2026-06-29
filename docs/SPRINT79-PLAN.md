# Sprint 79 — v2.49.0 — `hostdetect` + `radiant host-info`

## User ask (paraphrased from the prior turn)

> "Eu quero que qualquer agente execute e seja possuído e use o
> próprio agente que está o chamando para executar todo o workflow
> ponta a ponta sem API key, na versão full eu quero tudo, inclusive
> a possibilidade de usar API key, e uma forma de autodetectar qual
> plataforma ou agente está executando o radiant-harness."

Concretely this means:

1. **Detection:** at runtime, figure out which agent (if any) is
   invoking `radiant`.
2. **Behavior:** possession + auto-detect, so the harness can use
   the host agent's LLM without an API key.

Sprint 79 covers the **detection** half. Sprint 80 will cover the
**behavior** half (`internal/llm/pick.go` + apply to every Full
subcommand).

## Goal

A new `internal/hostdetect/` package detects in runtime which agent
host (if any) is currently driving radiant-harness. Ship with a
`radiant host-info` subcommand so the user (or an LLM) can see what
was detected.

## Detection strategy

Two layers, in order:

### Layer 1 — Env-var fingerprint (high confidence)

Each agent exports one or more distinguishing env vars when it's
running. We read `os.Environ()` once and match against a registry.

| Agent             | Env signature                                                  | Confidence |
|-------------------|----------------------------------------------------------------|------------|
| Claude Code       | `CLAUDE_CODE_ENTRY` or `CLAUDE_CODE_SSE_PORT`                 | High       |
| Cursor            | `CURSOR_TRACE_ID` or `CURSOR_HOME`                             | High       |
| Hermes            | `HERMES_VERSION` or `HERMES_HOME`                              | High       |
| Kimi CLI          | `KIMI_SHARE_DIR` or `KIMI_VERSION`                             | High       |
| OpenClaw          | `OPENCLAW_GATEWAY_URL` or `OPENCLAW_VERSION`                   | High       |
| Codex             | `CODEX_HOME` or `CODEX_THREAD_ID` or `CODEX_RUNTIME`           | High       |
| Cline             | `CLINE_USER` or `CLINE_VERSION`                                | High       |
| OpenCode          | `OPENCODE_HOME` or `OPENCODE_VERSION`                          | High       |
| VS Code Copilot   | `VSCODE_PID` or `VSCODE_IPC_HOOK_CLI`                          | High       |

If multiple env vars from the SAME agent match, confidence bumps
"High → Certain". If only ONE matches, it's "High" (still strong).

### Layer 2 — Parent process walk (fallback)

For agents that don't export unique env vars, or for unknown agent
hosts, walk the parent process tree:

```
PID 100 (radiant) → PPID 50 → PPID 1 → parent binary path

Match each parent's `comm`/executable name against a known list:
  - `claude-code`, `cursor`, `hermes`, `kimi`, `openclaw`, `codex`,
    `cline`, `opencode`, `vscode` (binary name variants)
```

Confidence is "Medium" if the direct PPID matches, "Low" if a
further ancestor matches.

### Result struct

```go
type HostInfo struct {
    Agent             AgentID  // "claude-code" | "cursor" | ... | AgentUnknown
    Confidence        int      // 0-100
    SupportsSampling  bool     // does this agent support sampling/createMessage?
    SampleEnvVars     []string // which env vars triggered the match
    PID               int      // radiant's PID
    PPID              int      // parent's PID
    ParentCmd         string   // parent's process name (best effort)
    DetectionSource   string   // "env" | "process-tree" | "none"
}
```

`AgentID` is a typed string alias so the harness can switch on it.

### Output

```
$ radiant host-info
Detected host agent:  claude-code (High confidence)
Sampling supported:  yes
Trigger env vars:     CLAUDE_CODE_ENTRY, CLAUDE_CODE_SSE_PORT
PID:                  12345  PPID: 12340
Parent cmd:           claude-code

$ radiant host-info --json
{
  "agent": "claude-code",
  "confidence": 95,
  "supports_sampling": true,
  "sample_env_vars": ["CLAUDE_CODE_ENTRY", "CLAUDE_CODE_SSE_PORT"],
  "pid": 12345,
  "ppid": 12340,
  "parent_cmd": "claude-code",
  "detection_source": "env"
}
```

## Files

```
internal/hostdetect/
  hostdetect.go          (NEW) — registry, Detector, Detect, HostInfo
  hostdetect_test.go     (NEW) — per-agent signatures + fallback
cmd/radiant/cmd_host_info.go         (NEW, untagged) — radiant host-info
cmd/radiant/main.go                  (modify) — register host-info (Light)
cmd/radiant/main_full.go             (modify) — register host-info (Full)
docs/HOST-AGENTS.md                  (NEW) — matrix of agents × detection × sampling
CHANGELOG.md, RELEASE-NOTES.md
```

## Behaviour: what Sprint 79 does NOT do yet

- **`PickBackend` not added.** Picking sampling-vs-HTTP based on
  `HostInfo` is **Sprint 80**. Sprint 79 only exposes the detection
  + command.
- **No default preference fallback**. The user (or Sprint 80's
  `PickBackend`) decides what to do with the result.
- **No full process-tree walk on macOS/Windows.** Sprint 79 does a
  one-step PPID read via `os.Getppid()`. Deep walks are Sprint 80
  if we need them.

## Tests

- 9 per-agent env-fingerprint tests (one per agent) — set env, call
  `Detect`, assert Agent + Confidence ≥ 50.
- 1 "no agent" test — empty env, simulate unknown parent cmd,
  expect `AgentUnknown` with `Confidence = 0`.
- 1 multi-agent-env test — set multiple agents' envs, expect the
  highest-confidence match wins.
- `cmd_host_info_test.go`: renderTable produces expected output,
  renderJSON produces valid JSON, --json flag honored.

## Stats (target)

- New files: 4 (hostdetect, test, cmd_host_info, HOST-AGENTS doc).
- Modified files: 4 (main.go, main_full.go, CHANGELOG, RELEASE-NOTES).
- LOC: ~600 added, ~50 modified.
- Tests: ~14 new.

## Risk

- **PID walk on macOS vs Linux vs Windows** — `os.Getppid()` works
  everywhere but parent cmd name lookup differs. Sprint 79 macOS-only
  via `/proc/<pid>/comm`; Windows deferred to Sprint 80+.
- **Env var names may shift** — agents update their env schemas over
  time. v2.49.0 captures the snapshot; Sprint 80+ adds the
  "looking at process tree" depth as a safety net.

## What's NOT in this sprint

- Sprint 80 will add `internal/llm/pick.go` (`PickBackend` with
  precedence: host-detected > API key > error) and apply it to
  every Full subcommand.
- Sprint 81 will wire Sprint 78's `radiant-full` to honour
  `HostInfo.SupportsSampling` automatically.
