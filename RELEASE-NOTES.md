# Release Notes — v2.37.0 (Light/Full + Semantic + Lazy-Executor)

> "Make it closed" — the release that turns radiant-harness from a
> working prototype into a complete, vendable product.

## Headlines

### 1. Two operating modes (Light / Full)

The harness can now be deployed two ways. The choice is a runtime
decision, not a build-time one — same binary, same loop engine, same
state machine, same verifier. What differs is who pays for the tokens.

| Mode | Inference path | Setup |
|------|---------------|-------|
| **Light** | Harness calls MCP `sampling/createMessage` on the host agent | `radiant setup-mcp --agent=claude` |
| **Full**  | Harness calls LLM HTTP endpoints directly | `export OPENROUTER_API_KEY=…` |

`radiant mode show` reports the resolved mode and the source (flag,
env, config, auto-detect). Auto-detect: presence of MCP config →
Light; presence of API key → Full; default → Light (safe).

### 2. Semantic model layer (credit-risk domain)

The "what it means here" layer that fixes the failure mode described
in the post that inspired this release: "instructions scale poorly,
context drifts, answers go wrong".

`internal/semantic/metrics/credit-risk.yaml` ships 7 metrics with
formulas, scopes, and regulation references:

- **PD** (Probability of Default) — CMN 4.966 §4.2.1
- **LGD** (Loss Given Default) — CMN 4.966 §4.2.3
- **EAD** (Exposure at Default) — CMN 4.966 §4.2.2
- **RWA** (Risk-Weighted Assets) — CMN 4.966 §4.2.1.4
- **ExpectedLoss** — IFRS 9 §5.5
- **provision_min_ifrs9** — CMN 4.966 §4.4
- **capital_required** — CMN 4.966 §4.1.1

The loop runner auto-detects the project domain and injects the
matching model's full markdown into the executor system prompt.
`radiant semantic resolve credit-risk RWA` returns the formula and
regulation inline.

### 3. Lazy-executor skill

Port of the [ponytail ladder](https://github.com/DietrichGebert/ponytail)
in PT-BR, adapted to the radiant context where the verifier already
cuts code that doesn't satisfy ACs. Three intensities:

- `lite` — build what was asked, suggest lazy alt in one line
- `full` — ladder enforced (default)
- `ultra` — YAGNI extremist, challenge the request itself

`--intensity=lite|full|ultra` on `radiant loop start`. Default `full`
so the skill is always injected unless explicitly off.

## Other changes

- **Pricing catalog** — `internal/pricing/data/pricing.yaml` consolidates
  the three duplicated rate tables. `radiant pricing list|stale|refresh`.
- **pathIsSafe security fix** — resolves symlinks before the boundary
  check. A symlink inside the project pointing outside is now rejected
  (was a TOCTOU hole that a confused or hostile LLM could exploit).
- **Documentation** — `docs/MODES.md` (full operator guide),
  `docs/IMPLEMENTATION-PLAN.md` (the plan this release executed),
  README updated, CHANGELOG with full diff.

## Stats

- 8 commits on branch `feature/light-full-release`.
- 4 new packages: `internal/mode/`, `internal/pricing/`,
  `internal/semantic/`, plus extensions to `internal/skill/`,
  `internal/engine/`, `internal/loop/`.
- 4 new CLI subcommands: `mode`, `pricing`, `semantic`,
  plus `--intensity` flag.
- 1 new skill: `lazy-executor`.
- 7 new metrics in `credit-risk.yaml`.
- ~50 new tests. All 25 packages green. `go vet` clean.

## Compatibility

- No breaking changes. `--mode` and `--intensity` default to auto/safe
  values when not specified. `radiant mode show` and `radiant pricing
  list` are pure read commands.
- Existing `.radiant.yaml` files keep working — `mode:` and `intensity:`
  are optional fields with sensible defaults.
- Embed-based semantic YAML is read-only at runtime; user overrides
  go in `<projectDir>/metrics/<domain>.yaml` and win over embedded.

## Upgrade instructions

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v2.37.0
# or:
git pull
make build
./bin/radiant --version   # should report 2.37.0
./bin/radiant doctor      # new mode check, new pricing freshness check
./bin/radiant mode show   # see your active mode
./bin/radiant pricing list # see the new canonical rates table
```

---

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
