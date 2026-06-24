# Release Notes — 0.2.0 (Go rewrite)

> Vendor-neutral, multi-platform, multi-LLM. No agent is privileged.

## What's new

### Security hardening

- **Agent binary allowlist** — `internal/harness/agent.go`. Refuses to spawn
  anything outside `{claude, codex, copilot, cursor, gemini}`. Adding a
  new adapter is an explicit code edit, not a config knob.
- **Gate command allowlist** — tasks.md gates are tokenized and each
  binary must be in the closed set (`node`, `npm`, `pnpm`, `yarn`, `go`,
  `make`, `pytest`, `python`, `cargo`, `jest`, …). `rm`, `curl`, `wget`
  are rejected by name.
- **Path sandboxing** — code blocks emitted by the LLM are checked
  against the project directory before being written.

### Crash safety

- **Atomic state persistence** — temp-file + fsync + rename, so a crash
  mid-write never leaves a half-written `progress.json`.
- **Advisory flock** — concurrent `radiant run` invocations on the same
  project serialize instead of corrupting state.
- **Timeouts everywhere** — 10 min per agent, 5 min per gate, with
  context cancellation propagating.

### Vendor neutrality (this release)

The CLI no longer treats Claude as the default agent:

- `radiant init` with `--yes` (no `--agent=`) now scaffolds **all** agents
  instead of silently picking Claude.
- `radiant init` without flags refuses to guess — the operator must
  declare which agent(s) they want. The error message lists all six
  supported vendors in alphabetical order.
- `DetectAgent()` scans `$PATH` alphabetically; no agent is privileged.
- The `claude` example in the README is one of many, not the first.
- All `--agent=` examples in the README and Makefile smoke test now
  exercise `--all` to assert multi-vendor behavior.

### LLM client

- **Provider-agnostic** — OpenRouter, OpenAI, Anthropic (via OpenRouter
  proxy or custom BaseURL), or any OpenAI-compatible endpoint.
- **10 curated presets** spanning Anthropic, OpenAI, Google, DeepSeek,
  Xiaomi. Add new vendors by editing `PresetModels` in
  `internal/llm/client.go` — no spec/format change needed.
- **Retry with backoff** — exponential + full jitter on 5xx, fail-fast on
  4xx. Capped at 5 attempts (initial + 4 retries).
- **Streaming** — SSE-aware with backpressure-friendly scan buffer.
- **32k default `MaxTokens`** — matches the size of real SDD specs; per-
  preset override available.

### Engine consolidation

- Parallel tasks within a phase are capped by a semaphore (4 by default)
  so we don't burst provider rate limits.
- Engine now actually runs gates (was a no-op stub before this release).
- Engine validates emitted code-block paths against the project boundary.

### Spec parser

- AC IDs in any of `AC-1`, `AC1`, `AC_1`, `AC 1`, `ac-1` are normalized
  to `AC-1`. Tasks and spec can mix forms.
- "And" clauses in Given/When/Then are appended to the most recent
  non-empty clause instead of being silently dropped.
- Tasks parser handles 5- and 6-column rows; tolerates `·` as a column
  separator inside "Covers AC".
- `groupPhases` now correctly groups consecutive parallel tasks into a
  single parallel phase.

### Quality scripts

- `radiant validate --gates` actually executes the task gates found in
  tasks.md (was static-only before). Each gate is validated against the
  allowlist and run with a 5-minute timeout. Skipped gates (binary not
  in allowlist) are reported but don't block.

### Build & distribution

- Single binary via `go build` (8.9 MB on Linux amd64).
- Cross-platform via goreleaser: linux/darwin/windows × amd64/arm64.
- Docker multi-stage Alpine build (Go 1.22 runtime).
- **GitHub Actions CI** on Go 1.22 and 1.24: gofmt, go vet, build, test,
  smoke, cross-build, coverage.

### VS Code extension

- Specs / Tasks / Progress tree views now populated (Tasks and Progress
  were empty stubs before).
- Status bar shows live state, feature, current/total tasks, and % done.
- File watcher on `.radiant-harness/progress.json` keeps the UI live.
- "Run gate" command available from the tasks.md context menu.

## Compatibility

- Templates and skills are reused from 0.1.0 (TypeScript) — no spec/tasks
  changes needed.
- Manifest format unchanged.
- Existing `.radiant-harness/progress.json` files load transparently.

## Known limitations

- **No auto model routing.** Pick your model explicitly per run. Future
  feature.
- **`internal/plugin/`** is a stub — the plugin system is documented
  but not implemented. Either wire it up or remove the package.
- **`internal/benchmark/`** has 138 lines no caller uses. Audit and
  either expose or remove.
- **Engine path uses OpenAI-compatible API only.** Direct Anthropic
  Messages API (which has a different shape) requires an OpenRouter
  proxy or a custom BaseURL.

## Migration from 0.1.0 (TypeScript)

1. Replace `npx @igoruehara/spec-driven init` with `radiant init --all`.
2. Replace `npx spec-driven validate` with `radiant validate --gates`.
3. Replace `npx spec-driven run` with `radiant run specs/NNNN-…`.
4. Existing specs and tasks.md files are unchanged.
