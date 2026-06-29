# Sprint 78 — v2.48.0 — Light vs Full: physical binary separation via build tags

## User ask (paraphrased)

> "Light e Full são coisas separadas. Light **não tem estrutura de API key**.
> Full sim. Eu vou subir a versão Light num repo e a versão Full em outro."

Concretely this means: when the Light binary is built, the API-key
HTTP layer (Anthropic / OpenAI / OpenRouter / HTTP LLM backend) **must
not be linked in**. The Light binary must be physically incapable of
talking to LLM providers via HTTP — its only LLM source is the host
agent via MCP sampling.

## Goal

One source tree, two artifacts:

```
# Full binary (current behaviour, has everything)
go build -o /tmp/radiant-full ./cmd/radiant
# → uses OpenAI / Anthropic / OpenRouter via API key, plus MCP sampling

# Light binary (NEW: no HTTP LLM at all)
go build -tags light_only -o /tmp/radiant-light ./cmd/radiant
# → only MCP sampling; HTTP LLM backend files are excluded at compile time
```

The user can then publish each binary to its own repo. CI in
`radiant-harness-light` runs `go build -tags light_only`; CI in
`radiant-harness-full` runs `go build` (default).

## Mechanism: `//go:build !light_only` tags

Each file that contains code dependent on API keys / LLM HTTP gets
a build constraint at the top:

```go
//go:build !light_only
```

When `go build -tags light_only` is invoked, files with this tag are
**excluded from the build**. The result:
- `internal/llm/anthropic.go` → not compiled → no Anthropic HTTP client.
- `internal/llm/client.go` → not compiled → no HTTP client logic.
- Subcommands that call into LLM HTTP → not compiled → don't even
  exist in the binary.

## Files to tag `//go:build !light_only`

### LLM HTTP layer
- `internal/llm/anthropic.go`
- `internal/llm/client.go`
- New: `internal/llm/backend_http.go` (the `HTTPBackend` struct + its
  methods, **moved out of `backend.go`** so the interface + sampling
  can stay untagged)

### Subcommands that depend on API-key LLM
- `cmd/radiant/cmd_fleet.go` (uses `internal/llm`)
- `cmd/radiant/cmd_loop.go`
- `cmd/radiant/cmd_run.go`
- `cmd/radiant/cmd_scaffolds.go` (8 LLM-driven scaffolds; tag the file)
- `cmd/radiant/cmd_audit.go` (registers `camada`, `evals`, `release`,
  `audit` subcommands — all LLM-driven)
- `cmd/radiant/cmd_doctor.go` (the Full doctor reads API keys; can
  refactor to make this branch later, but for now tag as Full)

### Helper code
- `cmd/radiant/helpers.go` (lots of LLM glue: resolveModel, renderIncidentDoc, etc.)
- `cmd/radiant/mcp_types.go` (references `llm.Backend` — works since
  SamplingBackend satisfies it, but `HTTPBackend` lives in tagged file)

## Files that stay UNTAGGED (Light + Full)

These compile in both builds:
- `cmd/radiant/main.go` (version var + entrypoint)
- `cmd/radiant/cmd_setup_mcp.go` + `cmd_setup_mcp_per_agent.go`
- `cmd/radiant/cmd_pr_review.go` (just parsing, no LLM HTTP)
- `cmd/radiant/cmd_security.go` (file scanning only)
- `cmd/radiant/cmd_init.go`, `cmd_session.go`, `cmd_ops.go`,
  `cmd_telemetry.go`, `cmd_context.go`, `cmd_skills.go`,
  `cmd_pricing.go`, `cmd_semantic.go`, `cmd_tools.go`

## The `Backend` interface split

Currently `internal/llm/backend.go` defines both the `Backend`
interface and the `HTTPBackend` struct in one file. To make
HTTPBackend tag-excludable, we split:

```
internal/llm/backend.go         (untagged)
  - Backend interface
  - SamplingBackend (always available via sampling.go)
  - Model, Message, ChatResponse, StreamCallback types (shared)

internal/llm/backend_http.go    (//go:build !light_only)
  - HTTPBackend struct
  - NewHTTPBackend constructor
  - HTTPBackend.Chat, ChatStream, ModelID methods
```

Light binary: `Backend` interface lives, `HTTPBackend` doesn't exist
→ only `SamplingBackend` satisfies it.

## Behaviour after Sprint 78

### Light binary (`radiant-light`)
- `radiant --version` → e.g. `2.48.0-light`
- `radiant mcp serve` → MCP sampling only (no API key infrastructure)
- `radiant setup-mcp` → 11 agents
- `radiant doctor` → subset (basic checks; no API key probe)
- `radiant init`, `radiant context`, `radiant skills`, `radiant telemetry`,
  `radiant pricing`, `radiant semantic`, `radiant security`,
  `radiant review-pr` (light subset), `radiant session`, `radiant ops`,
  `radiant tools`
- `radiant loop`, `radiant run`, `radiant fleet`, `radiant audit`,
  `radiant camada`, `radiant evals`, `radiant release`, `radiant eval`,
  `radiant scaffold-*` → NOT REGISTERED (binary doesn't even contain them)

### Full binary (`radiant-full`)
- All commands (current behaviour)

### Sharing between binaries
- Both export Light commands with the same names as today
  (`mcp serve`, `setup-mcp`, etc.).
- Full-only commands are simply absent in Light — no error needed, just
  unknown subcommand.

## Verification

1. **Full build**: `go build ./cmd/radiant` → success, all tests pass.
2. **Light build**: `go build -tags light_only ./cmd/radiant` → success.
3. **Light binary inspection**:
   ```bash
   nm /tmp/radiant-light | grep -i 'anthropic\|openai\|openrouter\|HTTPBackend'
   # → should NOT find HTTPBackend symbols
   strings /tmp/radiant-light | grep -i 'RADIANT_OPENROUTER_API_KEY\|api.openai\|api.anthropic'
   # → should NOT find API URL strings
   ```
4. **Both binaries**: --version prints `2.48.0` (Full) or `2.48.0-light`
   (Light), all 5 platforms cross-compile, all tests pass on each.

## Files

```
internal/llm/backend.go           (no tag, light+full)
internal/llm/backend_http.go      (//go:build !light_only)  [NEW]
internal/llm/anthropic.go         (//go:build !light_only)
internal/llm/client.go            (//go:build !light_only)
internal/llm/sampling.go          (no tag, light+full)

cmd/radiant/cmd_fleet.go          (//go:build !light_only)
cmd/radiant/cmd_loop.go           (//go:build !light_only)
cmd/radiant/cmd_run.go            (//go:build !light_only)
cmd/radiant/cmd_scaffolds.go      (//go:build !light_only)
cmd/radiant/cmd_audit.go          (//go:build !light_only)
cmd/radiant/cmd_doctor.go         (//go:build !light_only)
cmd/radiant/helpers.go            (//go:build !light_only)
cmd/radiant/mcp_types.go          (//go:build !light_only)

cmd/radiant/version.go            [NEW]         (no tag)
cmd/radiant/version_full.go       [NEW, //go:build !light_only]
cmd/radiant/main.go               (modified; the Light version)
cmd/radiant/main_full.go          [NEW, //go:build !light_only]  (the Full version)

cmd/radiant/main.go               [now tag-gated]
CHANGELOG.md, RELEASE-NOTES.md, docs/LIGHT-VS-FULL.md
```

## What's NOT in this sprint (Sprint 79+)

- **Runtime host-agent detection** (`internal/hostdetect/`).
- **`radiant host-info` command** — list detected host agents.
- **PickBackend in Full binary** — choose host-detect-sampling vs HTTP
  based on context. (Following sprint.)
- **Light binary subcommand parity** — currently Light lacks
  doctor/audit/scaffold etc. We can re-export a slimmer subset later
  if needed.

## Risk / notes

- **Tests break** — many tests live in the Full-only files. Light
  build skips them. This is correct behaviour but means light binary
  doesn't run the Full test suite. Documented.
- **Some shared types are in tagged files** — if Light references
  any type from `internal/llm/client.go`, the Light build will
  fail. We track these by trying to compile `-tags light_only` and
  fixing any reference that breaks.
- **`mcp_types.go` references `llm.Backend`** — works because
  `Backend` interface is in untagged `backend.go`. The actual
  `HTTPBackend` it might construct is tagged but only constructed in
  Full subcommands (which are themselves tagged). Light only
  constructs `SamplingBackend` via sampling.go (untagged).
