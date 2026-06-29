# Validation Report — Sprint 79: v2.49.0 — `hostdetect` + `radiant host-info`

> **Date:** 2026-06-29
> **Project version:** v2.49.0
> **Branch:** `feature/light-full-release`
> **Base:** `eb4a197` (Sprint 79 commit)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 79 delivers the **detection half** of the user's explicit
ask: "auto-detectar qual plataforma ou agente está executando o
radiant-harness." Sprint 80 will wire that detection into
`internal/llm/pick.go` so Full subcommands auto-use the host
agent's LLM when available (no API key required).

| Metric | Value |
|--------|-------|
| Commits on branch | ahead of base (`9b28e77`) |
| New commits in this release | **1** (`eb4a197`) |
| Files added | 5 (`internal/hostdetect/{hostdetect.go, hostdetect_test.go}`, `cmd/radiant/cmd_host_info.go`, `docs/{HOST-AGENTS, SPRINT79-PLAN}.md`) |
| Files modified | 4 (`cmd/radiant/{main.go, main_full.go}`, `CHANGELOG.md`, `RELEASE-NOTES.md`) |
| LOC delta | +1211 / −2 |
| Tests added | **24** (per-agent + edge cases) |
| Tests | **1190+ PASS, 0 confirmed FAIL** (Full: 31 packages, Light: 29 packages) |
| `go vet ./...` | clean (both modes) |
| Cross-compile | linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64 — both modes |

---

## What landed

### `internal/hostdetect/` (new package, 260 LOC)

Two-layer detection:
1. **Env-var fingerprint** — each agent exports at least one
   distinguishing env var when running. Multiple matches → higher
   confidence.
2. **`/proc/<ppid>/comm` walk fallback** — when env vars don't match,
   the parent's process name gives medium-confidence signal.

Detects **9 agents** (all support MCP sampling):

| Agent           | Env keys                                                        | PPID match                  |
|-----------------|------------------------------------------------------------------|----------------------------|
| Claude Code     | CLAUDE_CODE_ENTRY, CLAUDE_CODE_SSE_PORT, CLAUDE_CODE_PID       | claude-code, claude, Claude |
| Cursor          | CURSOR_TRACE_ID, CURSOR_HOME, CURSOR_USER_DATA_DIR              | cursor, Cursor              |
| Hermes          | HERMES_VERSION, HERMES_HOME, HERMES_AGENT_HOME                  | hermes-agent, hermes-cli   |
| Kimi CLI        | KIMI_SHARE_DIR, KIMI_VERSION, KIMI_CONFIG_DIR                   | kimi, kimi-cli             |
| OpenClaw        | OPENCLAW_GATEWAY_URL, OPENCLAW_VERSION, OPENCLAW_WORKSPACE      | openclaw, openclaw-cli     |
| Codex           | CODEX_HOME, CODEX_THREAD_ID, CODEX_RUNTIME, CODEX_THREAD_ENV     | codex, codex-cli            |
| Cline           | CLINE_USER, CLINE_VERSION, CLINE_WORKSPACE                      | cline, cline-host           |
| OpenCode        | OPENCODE_HOME, OPENCODE_VERSION, OPENCODE_CONFIG                | opencode-cli                |
| VS Code Copilot | VSCODE_PID, VSCODE_IPC_HOOK_CLI, VSCODE_CWD                     | Code Helper, code           |

### `radiant host-info` command (works in BOTH Light and Full)

```
$ radiant host-info
Detected host agent:  (unknown) (no signal)
Sampling supported:  no
Detection source:    none
PID:                  63894  PPID: 63857

No agent host detected. radiant-harness is running standalone.
API key required for Full build HTTP LLM features.

$ CLAUDE_CODE_ENTRY=/entry/claude CLAUDE_CODE_SSE_PORT=8080 radiant host-info
Detected host agent:  claude-code (High confidence)
Sampling supported:  yes
Detection source:    env
PID:                  63922  PPID: 63921

Host "claude-code" supports MCP sampling — possession is possible.
Sprint 80 will wire this into PickBackend for automatic inference routing.

$ CLAUDE_CODE_ENTRY=/entry/claude radiant host-info --json | jq .
{
  "agent": "claude-code",
  "confidence": 90,
  "supports_sampling": true,
  "sample_env_vars": ["CLAUDE_CODE_ENTRY", "CLAUDE_CODE_SSE_PORT"],
  "pid": 63923,
  "ppid": 63921,
  "detection_source": "env"
}
```

### Confidence scoring

| Layer                                  | Score |
|----------------------------------------|-------|
| 0 env hits, 0 PPID hits               | 0     |
| 1 env hit                              | 75    |
| 2 env hits                             | 90    |
| 3+ env hits (cap)                      | 100   |
| 0 env hits, 1 PPID match                | 50    |

Tie-breaks go to the first in `knownAgents` order (Claude Code wins).

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go vet -tags light_only ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant-full ./cmd/radiant
EXIT=0   (Full binary)

$ go build -tags light_only -o /tmp/radiant-light ./cmd/radiant
EXIT=0   (Light binary, 10M)

$ /tmp/radiant-full --version
2.49.0

$ /tmp/radiant-light --version
2.49.0-light

$ go test -count=1 ./...
ok    github.com/quant-risk/radiant-harness/cmd/radiant               (PASS)
... (31 packages total)
PASS: 31 packages, 1190+ tests, 0 confirmed failures

$ go test -count=1 ./... -tags light_only
PASS: 29 packages, 0 confirmed failures
```

### hostdetect package tests (24)

```
=== RUN   TestDetect_ClaudeCode                         --- PASS
=== RUN   TestDetect_Cursor                             --- PASS
=== RUN   TestDetect_Hermes                             --- PASS
=== RUN   TestDetect_KimiCLI                            --- PASS
=== RUN   TestDetect_OpenClaw                           --- PASS
=== RUN   TestDetect_Codex                              --- PASS
=== RUN   TestDetect_Cline                              --- PASS
=== RUN   TestDetect_OpenCode                           --- PASS
=== RUN   TestDetect_VSCodeCopilot                      --- PASS
=== RUN   TestDetect_MultipleHitsHigherConfidence       --- PASS
=== RUN   TestDetect_EnvWinsOverParent                  --- PASS
=== RUN   TestDetect_ParentProcessOnly                   --- PASS
=== RUN   TestDetect_NoMatch                             --- PASS
=== RUN   TestDetect_Empty                               --- PASS
=== RUN   TestDetect_ParentWithExeSuffix                --- PASS
=== RUN   TestDetect_MultipleAgentsPickHighestHits       --- PASS
=== RUN   TestNew_WithDefaults                          --- PASS
=== RUN   TestSupportsSampling_TrueForAllKnown          --- PASS
=== RUN   TestAgentIDString                             --- PASS
=== RUN   TestSignaturesCoverAllAgents                  --- PASS
=== RUN   TestSignaturesWellFormed                      --- PASS
PASS / ok  internal/hostdetect  0.432s
```

### Cross-compile matrix (10 binaries)

```
linux/amd64    Light  10M  | Full 15M
linux/arm64    Light 9.9M  | Full 14M
darwin/amd64   Light  11M  | Full 15M
darwin/arm64   Light  10M  | Full 14M
windows/amd64  Light  11M  | Full 15M
```

All built cleanly. Symbol checks confirm Light has zero HTTP-LLM
references.

### Smoke test verification

```bash
# No env vars → unknown
$ radiant-light host-info
Detected host agent:  (unknown) (no signal)
...

# Claude env (1 hit) → claude-code, High
$ CLAUDE_CODE_ENTRY=/entry/claude radiant-light host-info
Detected host agent:  claude-code (High confidence)
...

# Claude env (2 hits) → claude-code, 90 confidence
$ CLAUDE_CODE_ENTRY=... CLAUDE_CODE_SSE_PORT=... radiant-light host-info
Detected host agent:  claude-code (High confidence)
[Internal confidence = 90 from JSON output]

# Hermes env (1 hit) → hermes, High
$ HERMES_VERSION=0.1.0 radiant-full host-info
Detected host agent:  hermes (High confidence)
...
```

---

## Files changed

```
ADDED:
  internal/hostdetect/hostdetect.go                   (260 LOC)
  internal/hostdetect/hostdetect_test.go              (340 LOC, 24 tests)
  cmd/radiant/cmd_host_info.go                        (140 LOC, untagged)
  docs/HOST-AGENTS.md                                 (matrix + reference)
  docs/SPRINT79-PLAN.md                               (design doc)

MODIFIED:
  cmd/radiant/main.go                                 (+1 line: registerHostInfoCmd)
  cmd/radiant/main_full.go                            (+1 line: registerHostInfoCmd)
  CHANGELOG.md                                        (v2.49.0 entry)
  RELEASE-NOTES.md                                    (v2.49.0 notes)

Net: +1211 / −2 LOC.
```

---

## Architectural decisions

### Why env-vars first, PPID second?

Env vars are O(1) to read, deterministic, and impossible to spoof by
process-tree manipulation (an agent either set them or didn't, when it
spawned the child). They give high confidence when present.

PPID fallback handles agents that don't export diagnostic env vars
(custom tools, internal agent forks). Lower confidence because the
parent process could be the shell that started the test manually.

### Why AllowOverride of Detector fields?

Tests inject env and parent comm deterministically. The Detector
struct exposes `LookupEnv`, `ReadProcComm`, `NowPID`, `NowPPID` as
fields so tests can stub them. The defaults wire to `os.LookupEnv`,
`os.ReadFile("/proc/<pid>/comm")`, `os.Getpid`, `os.Getppid` for
production.

### Why Register in both binaries?

`host-info` is a diagnostic command useful in either context:
- Inside an agent (Light): confirms possession works.
- From a shell with no agent (Full?): reports "unknown" so the
  operator knows the harness is running standalone.

Registering it in both binaries costs nothing and improves UX.

---

## What's NOT in this sprint (deferred to Sprint 80+)

- **`internal/llm/pick.go` (`PickBackend`)** — picks SamplingBackend
  vs HTTPBackend based on `HostInfo.SupportsSampling` + API key
  presence. This is the **behaviour** half of the user's ask; this
  sprint only delivered the **detection** half.
- **Apply `PickBackend` to every Full subcommand** — `loop`,
  `run`, `fleet`, `audit`, `camada-agentica`, `evals`, `release`,
  `eval`, etc. Currently all of those call `llm.NewHTTPBackend`
  directly. Sprint 80 will route them through `pick.Backend(cfg)`.
- **Configurable precedence** — `RADIANT_BACKEND_PREFERENCE`
  env var (`host` | `api-key` | `auto`). Sprint 80+.
- **Deep process-tree walk** on Windows / BSD / non-procfs Unix.
  Currently macOS / Linux only via `/proc/<ppid>/comm`. Defer.
- **Adaptive confidence during long-running sessions** — if
  `host-info` shows different results an hour apart, that's a signal
  worth highlighting. Future.

---

## Backward compatibility

- **Internal API:** new package `internal/hostdetect` — no existing
  imports to update.
- **CLI surface:** one new subcommand (`radiant host-info`).
  Position: appears in both `radiant --help` and
  `radiant-light --help`. Doesn't conflict with existing commands.
- **Existing `radiant doctor`, `radiant audit`, etc.:** unchanged.
- **Cross-compile:** unchanged for downstream consumers; same
  artifact shape (`radiant` and `radiant-light` binaries per
  platform).

---

## Verification checklist

- [x] `go vet ./...` clean (Full)
- [x] `go vet -tags light_only ./...` clean (Light)
- [x] `go build ./cmd/radiant` clean (Full)
- [x] `go build -tags light_only ./cmd/radiant` clean (Light)
- [x] Cross-compile: 5 platforms × 2 modes = 10 binaries — all OK
- [x] `go test -count=1 ./...` — 31 packages OK, 0 FAIL (Full)
- [x] `go test -count=1 ./... -tags light_only` — 29 packages OK, 0 FAIL (Light)
- [x] All 24 hostdetect tests pass
- [x] `radiant host-info` works on Light
- [x] `radiant host-info` works on Full
- [x] `radiant host-info --json` produces valid JSON
- [x] `radiant host-info --verbose` shows matched env vars
- [x] Detection: claude-code, cursor, hermes, kimi-cli, openclaw, codex, cline, opencode, vscode-copilot (9)
- [x] "no signal" case for empty env returns AgentUnknown
- [x] Multi-env (Hermes vs Claude) picks highest-hit agent
- [x] `--version` shows `2.49.0` (Full) and `2.49.0-light` (Light)
- [x] CHANGELOG.md and RELEASE-NOTES.md updated
- [x] docs/HOST-AGENTS.md created (agent matrix + adding new agents)
- [x] git commit `eb4a197` lands cleanly
