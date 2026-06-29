# Validation Report — Sprint 68: v2.37.0 Light/Full + Semantic + Lazy-Executor

> **Date:** 2026-06-29
> **Project version:** v2.37.0
> **Branch:** `feature/light-full-release`
> **Base:** `9b28e77` (v2.36.0)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 68 ships the **"make it closed"** release. The harness now has a
clean two-mode abstraction (Light / Full), a curated semantic model
layer for credit-risk (CMN 4.966 / IFRS 9 / Basileia), a lazy-executor
skill with intensity filter, a single-source pricing catalog, and a
symlink-aware security check on the executor sandbox.

| Metric | Value |
|--------|-------|
| Commits | **9** (8 features + 1 plan) |
| Packages | **26 green**, 0 failures |
| Tests | **921 PASS, 0 FAIL** (`go test -count=1 -v ./...`) |
| New packages | 5 (`mode`, `pricing`, `semantic`, `tools`, plus extensions) |
| New CLI subcommands | 4 (`mode`, `pricing`, `semantic`, `--intensity`) |
| New skill | 1 (`lazy-executor`) |
| New metrics | 7 (PD, LGD, EAD, RWA, ExpectedLoss, provision_min_ifrs9, capital_required) |
| Files changed | 37 — **+4,747 / −1,050 LOC** |
| Cross-compile | linux/amd64, darwin/arm64, windows/amd64 — all OK |
| `go vet ./...` | clean |

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant ./cmd/radiant
-rwxr-xr-x  14M  /tmp/radiant    # darwin/arm64 host

$ go test -count=1 -v ./... | grep -cE "^--- PASS:"
921
$ go test -count=1 -v ./... | grep -cE "^--- FAIL:"
0
```

### Per-package breakdown

| Package | Time | Notes |
|---------|------|-------|
| `cmd/radiant` | 2.35s | 26 commands, `--help` smoke OK |
| `internal/benchmark` | 0.40s | |
| `internal/boot` | 0.51s | |
| `internal/config` | 0.61s | |
| `internal/context` | 0.77s | |
| `internal/engine` | 1.42s | **+ `pathIsSafe` symlink fix (3 new tests)** |
| `internal/fleet` | 11.40s | slowest — multi-agent dispatch |
| `internal/harness` | 6.96s | |
| `internal/improve` | 2.56s | |
| `internal/llm` | 7.25s | |
| `internal/loop` | 3.33s | **+ `assembleSemanticBlock` integration** |
| `internal/mode` | 2.65s | **NEW — 11 tests** |
| `internal/ontology` | 2.56s | |
| `internal/policy` | 2.52s | |
| `internal/pricing` | 2.05s | **NEW — 9 tests** |
| `internal/quality` | 2.04s | |
| `internal/routing` | 1.81s | |
| `internal/scaffold` | 2.66s | |
| `internal/schedule` | 1.69s | |
| `internal/semantic` | 1.81s | **NEW — 11 tests** |
| `internal/skill` | 1.78s | **+ `FilterForIntensity` (10 tests)** + 61 skill validation |
| `internal/slog` | 1.94s | |
| `internal/spec` | 1.81s | |
| `internal/tools` | 1.73s | **NEW scaffold — 6 tests** |
| `internal/webhook` | 16.86s | slowest absolute (HTTP fixtures) |
| `internal/worktree` | 2.62s | |

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64   go build -o /tmp/radiant-dist/radiant-linux-amd64   ./cmd/radiant  # 15M OK
$ GOOS=darwin  GOARCH=arm64   go build -o /tmp/radiant-dist/radiant-darwin-arm64 ./cmd/radiant  # 14M OK
$ GOOS=windows GOARCH=amd64   go build -o /tmp/radiant-dist/radiant-windows-amd64.exe ./cmd/radiant  # 15M OK
```

3/3 platforms clean. CGO_ENABLED default is fine (no cgo deps in tree).

---

## Smoke Tests — CLI surface

Validated against `/tmp/radiant` (darwin/arm64 host binary).

### `radiant mode` — operating-mode abstraction

```text
$ radiant mode show
Mode:    light
Source:  detected
Reason:  no API key found, defaulting to Light

  light — harness possesses the agent via MCP sampling (no API key)
  full  — harness is autonomous via direct HTTP to LLM providers (API key required)

$ radiant mode set light
✓ Mode set to light in .radiant.yaml
  (RADIANT_MODE env var or --mode flag still override this)

$ radiant mode show
Mode:    light
Source:  config
Reason:  .radiant.yaml mode: light

$ radiant mode set auto
Error: cannot persist 'auto' — pick light or full. Use 'unset' to remove the field.
EXIT=1
```

**Verified:** resolution chain (flag > env > config > auto-detect) works.
`auto` correctly refused as persistent value (only a resolved mode).

### `radiant pricing` — single-source catalog

```text
$ radiant pricing list | head -5
PRESET               PROVIDER    MODEL                        INPUT/1K  OUTPUT/1K  MAX_TOKENS  VERIFIED
abab-7               openrouter  minimax/abab-7               $0.00014  $0.00056   16000       2026-06-29
claude-haiku-4-5     openrouter  anthropic/claude-haiku-4-5   $0.00080  $0.00400   16000       2026-06-29
claude-opus-4-8      openrouter  anthropic/claude-opus-4-8    $0.01500  $0.07500   32000       2026-06-29
claude-sonnet-4-6    openrouter  anthropic/claude-sonnet-4-6  $0.00300  $0.01500   32000       2026-06-29
…

$ radiant pricing list | wc -l
27     # 25 presets + 2 header lines (PASS — all 12 vendors present)

$ radiant pricing stale
Source:        builtin
Loaded at:     2026-06-29T06:39:26-03:00
Stale (>2160h0m0s old): false

$ radiant pricing refresh
To refresh the pricing table:
  1. Edit internal/pricing/data/pricing.yaml
  2. Update the verified_at date for changed rows
  3. Run: go test ./internal/pricing/  (round-trip parse check)
  4. Commit and rebuild
The data file is embedded at build time via //go:embed.
There is no runtime override path by design — pricing is
a build-time concern, not a config-time one.
```

**Verified:** 25 curated presets × 12 vendors (OpenRouter, Anthropic,
OpenAI, Mistral, Groq, Google, Deepseek, ZhipuAI, Moonshot, Xiaomi,
xAI, Minimax). All three previously-duplicated rate tables
(`PresetModels`, `PricePerMTokensUSD`, `providerPricing`) now read from
this single YAML.

### `radiant semantic` — credit-risk domain

```text
$ radiant semantic list
Available domains:
  credit-risk            Credit Risk Metrics (Basileia + IFRS 9 + CMN 4.966) (7 metrics, v1.0.0)
  market-risk            (no model embedded)
  liquidity-risk         (no model embedded)
  operational-risk       (no model embedded)

$ radiant semantic resolve credit-risk PD
# PD (percent)
Probability of Default — likelihood that a counterparty fails to
meet its obligations over a 12-month horizon. …
**Regulation:** CMN 4.966 §4.2.1 (Basileia PD definition)
**Tags:** [pd probability_of_default default_risk basileia]
**Scopes:** customer.segment ∈ {Retail, SME, Corporate, Sovereign}
**Formula:**
lookup(rating_pd_table, {policy.rating})
where rating_pd_table maps AA→0.0003, A→0.0010, B→0.0050,
C→0.0150, D→0.0500, E→0.1500

$ radiant semantic resolve credit-risk EAD
**Regulation:** CMN 4.966 §4.2.2 (EAD definition)
**Scopes:** product.type ∈ {loan, guarantee, undrawn_credit_line, letter_of_credit}
**Formula:**
{exposure.drawn} + {exposure.off_balance} * ccf({product.type})
where ccf(guarantee)=0.50, ccf(undrawn_credit_line)=0.75,
ccf(committed_credit_line)=0.50, ccf(letter_of_credit)=0.20

$ radiant semantic search credit-risk capital
Matches for "capital" in credit-risk:
  - RWA (currency) — Risk-Weighted Assets — EAD multiplied by a risk weight K that
  - capital_required (currency) — Minimum regulatory capital required for this exposure — Basileia
```

**Verified:** `normaliseWithCase` correctly splits `ExpectedLoss` →
`expected_loss`; `{scope.field}` cross-references resolve; regulation
strings render inline.

### `radiant skills list`

```text
$ radiant skills list | wc -l
65   # 61 skills + 3 header lines + bundle separator
$ radiant skills list | grep -E "lazy-executor"
    lazy-executor          1.0.0      trivial,fea… Força o executor autonomous loop a produzir o mínimo…
```

**Verified:** `lazy-executor` bundled and passes the
`TestAllBundledSkillsValidateCleanly/lazy-executor` subtest (along
with all 60 other skills).

### `radiant doctor`

```text
$ radiant doctor
  ✗  API key                 none found — set OPENROUTER_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY
  ✓  git installed           git version 2.50.1 (Apple Git-155)
  ✓  git repo                ok
  ✓  worktrees               no stale worktrees
  ✓  model                   claude-sonnet-4-6 (default)
  ✓  radiant binary          /tmp/radiant
  ✓  mode                    light (detected: no API key found, defaulting to Light)
```

**Verified:** mode check now integrated. Single failure (`API key`) is
the expected default in this Light-mode demo; same doctor would clear
all checks with `OPENROUTER_API_KEY=…` exported.

---

## What's Committed

Branch `feature/light-full-release` (9 commits ahead of `9b28e77`):

| # | SHA | Type | Summary |
|---|-----|------|---------|
| 1 | `ca4966a` | docs | v2.37.0 implementation plan |
| 2 | `fabf5cf` | feat(mode) | `internal/mode/` (214 LOC) + 11 tests + `--mode` flag |
| 3 | `b665a2e` | feat(pricing) | `internal/pricing/` (213 LOC) + 9 tests + `radiant pricing` |
| 4 | `15ede2f` | feat(skill) | `lazy-executor` skill + `intensity.go` (10 tests) + `--intensity` |
| 5 | `59a666d` | fix(security) | `pathIsSafe` resolves symlinks before boundary check (3 tests) |
| 6 | `e4388e1` | feat(semantic) | `internal/semantic/` (430 LOC) + `credit-risk.yaml` (7 metrics) + 11 tests |
| 7 | `ce06c57` | docs | README + CHANGELOG + RELEASE-NOTES for v2.37.0 |
| 8 | `8efcfac` | refactor(cmd) | `helpers.go` 4931→3894 lines (−21%), split into `mcp_types.go`, `ci.go`, `diagram.go`, `release.go` |
| 9 | `81abbc8` | feat(tools) | `internal/tools/` registry scaffold (168 LOC, 6 tests) — Fase 6 prep |

### File-level diffstat

```text
$ git diff 9b28e77..HEAD --shortstat
 37 files changed, 4747 insertions(+), 1050 deletions(-)
```

Highlights:
- `internal/mode/mode.go` (+214) — `Mode` enum, `Resolve`, `Detect`
- `internal/pricing/catalog.go` (+213) — embed loader + accessors
- `internal/pricing/data/pricing.yaml` (+73) — single-source pricing
- `internal/semantic/semantic.go` (+430) — types, Resolve, Search, Render
- `internal/semantic/metrics/credit-risk.yaml` (+180) — 7 metrics, regex-tested
- `internal/skill/intensity.go` (+95) — intensity filter
- `internal/skill/skills/lazy-executor/SKILL.md` (+219) — ladder SKILL
- `internal/tools/tools.go` (+168) — registry scaffold (Fase 6 prep)
- `cmd/radiant/helpers.go` (−1037) — split into themed files
- `cmd/radiant/cmd_mode.go` (+74) — new
- `cmd/radiant/cmd_pricing.go` (+62) — new
- `cmd/radiant/cmd_semantic.go` (+88) — new
- `cmd/radiant/cmd_loop.go` (modified) — `--intensity` flag
- `docs/MODES.md` (+182) — operator guide
- `docs/IMPLEMENTATION-PLAN.md` (+158) — the plan this release executed
- `README.md`, `CHANGELOG.md`, `RELEASE-NOTES.md` — refreshed

---

## Architecture Snapshot

```
                        ┌──────────────────────────────────┐
                        │       radiant CLI (Go)           │
                        │       v2.37.0                    │
                        └──────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            ▼                       ▼                       ▼
   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
   │  Light mode     │    │  Full mode      │    │  --mode auto    │
   │  (default)      │    │  (opt-in)       │    │  (resolved)     │
   └─────────────────┘    └─────────────────┘    └─────────────────┘
            │                       │
            ▼                       ▼
   ┌─────────────────┐    ┌─────────────────┐
   │ MCP sampling/   │    │ HTTP backend    │
   │ createMessage   │    │ OpenRouter/     │
   │ → host agent    │    │ OpenAI/         │
   │   credentials   │    │ Anthropic/      │
   │ Zero API key    │    │ Groq/Mistral/   │
   └─────────────────┘    │ xAI             │
                          │ API key required│
                          └─────────────────┘
                                    │
                                    ▼
                          ┌─────────────────┐
                          │ Loop engine     │
                          │ (Discover→Plan  │
                          │  →Execute→      │
                          │  Verify→Persist)│
                          └─────────────────┘
                                    │
                ┌───────────────────┼───────────────────┐
                ▼                   ▼                   ▼
        ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
        │ Lazy exec   │     │ Semantic    │     │ Pricing     │
        │ (intensity  │     │ model       │     │ catalog     │
        │  filter)    │     │ (credit-    │     │ (single     │
        │             │     │  risk)      │     │  source)    │
        └─────────────┘     └─────────────┘     └─────────────┘
```

Resolution chain (mode): `--mode` flag > `RADIANT_MODE` env >
`.radiant.yaml` > auto-detect (MCP config → Light; API key → Full;
default → Light, safe).

---

## Gaps (carried into Sprint 69+)

These are **explicit, documented** follow-ups — not regressions.

1. **Tool Use formal wire-up in the executor** (multi-sprint).
   `internal/tools/` registry exists (168 LOC, 6 tests, scaffold
   commit `81abbc8`) but the executor still extracts ` ```go `
   blocks via regex. Switching requires coordinated edits in:
   - `internal/loop/executor*.go` — replace code-block extraction with
     structured tool calls (JSON-schema-validated).
   - `internal/loop/verifier_prompt.go` — teach verifier to evaluate
     tool-call traces, not just diff hunks.
   - `internal/loop/tracer.go` — emit tool-call events alongside
     text diffs.
   - `internal/gaterun/` — gate runner must allow tool invocations as
     gate evidence.
   - Estimated effort: **2-3 sprints**.

2. **i18n of the 60 skills** (~40% still in PT-BR). Schema is
   locale-agnostic; this is a translation pass, not a schema change.

3. **More `cmd/radiant/helpers.go` extractions** (currently 3894 lines,
   still the largest file in the tree). Candidates (each 200-400 LOC):
   - `audit.go` (project layout + AC traceability)
   - `telemetry.go` (privacy-first stats)
   - `scaffolds.go` (model/eval/train/stats/profile/predict/etc.)
   - `pr_review.go` (`radiant review-pr`)
   - `autodata.go` (LLM-skill auto-authoring)

4. **More semantic-model domains.** Currently only `credit-risk` ships
   embedded. `market-risk`, `liquidity-risk`, `operational-risk` are
   listed by `radiant semantic list` but say "(no model embedded)" —
   they're placeholders waiting for the next domain author.

5. **Pricing refresh automation.** `radiant pricing refresh` is
   documentation-only (manually edit YAML + rebuild). A scheduled
   pull from a source-of-truth API (e.g. OpenRouter's `/models`
   endpoint) is designed but not implemented.

6. **Mode-specific executor prompts.** Currently `assembleSemanticBlock`
   fires for *all* modes when domain matches. For Light mode, the host
   agent already has its own context — duplication should be measured.

---

## Compatibility Notes

- **No breaking changes.** `--mode` defaults to `auto`; `--intensity`
  defaults to `full`. Existing `.radiant.yaml` files without `mode:`
  or `intensity:` fields keep working.
- **New optional YAML keys** (`.radiant.yaml`):
  ```yaml
  mode: light          # or 'full' (NOT 'auto' — auto is resolved-only)
  intensity: full      # or 'lite' | 'ultra' | 'off'
  ```
- **Embed-based read-only data** (pricing YAML, semantic YAML, skills)
  cannot be overridden at runtime. User-level overrides go in
  `<projectDir>/metrics/<domain>.yaml` and win over embedded.
- **`radiant mode set auto`** correctly errors with explicit message
  — `auto` is a resolution policy, not a persistent value.

---

## Merge Plan

```bash
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/projects/radiant-harness-main
git log 9b28e77..HEAD --oneline        # 9 commits
git diff 9b28e77..HEAD --stat          # 37 files / +4747 / -1050
git tag v2.37.0                        # then push tag
git merge --no-ff feature/light-full-release -m "Release v2.37.0 — Light/Full modes, semantic model, lazy-executor"
```

Or open PR from `feature/light-full-release` → main and let CI gate.

---

## What this release unlocks (for the next sprint)

1. **Fortvna can ship radiant as a vendor-neutral delivery surface.**
   The semantic model layer makes the harness useful for credit-risk
   work out-of-the-box — the LLM resolves "RWA for Corporate
   exposure" against the curated Basileia formula instead of
   hallucinating.

2. **Claude Code users get a zero-config entry point.** Light mode
   needs no API key — the host agent's credentials carry the
   inference. One `radiant setup-mcp` and you're running.

3. **The pricing consolidation pays off** when the loop's cost
   reporting starts driving real budget enforcement in Sprint 69.

4. **`internal/tools/` registry is ready for Tool Use wire-up** in
   Sprint 69-70. The scaffold keeps the executor change self-contained.

---

**Signed off:** Sprint 68 validation pass. Ready to merge and proceed
to Sprint 69 (Tool Use wire-up + remaining `helpers.go` extractions).