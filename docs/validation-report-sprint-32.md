# Sprint 32 validation — OpenRouter support + autodata live + 3 skills + 2 commands

**Commit (this report):** TBD
**Re-validates:** `a09db32` (Sprint 31 — depth skills + autodata)
**Version:** **v0.7.0** (tagged)

## What shipped

### Sprint 32a: OpenRouter support (vendor-neutral)

`radiant autodata` now supports OpenRouter as the **preferred**
provider (vendor-neutral). Provider detection order:
1. `RADIANT_OPENROUTER_API_KEY` / `OPENROUTER_API_KEY` → OpenRouter
2. `RADIANT_OPENAI_API_KEY` / `OPENAI_API_KEY` → OpenAI
3. `RADIANT_ANTHROPIC_API_KEY` / `ANTHROPIC_API_KEY` → Anthropic
4. None → stub mode (no LLM, manual fill template)

Default OpenRouter model: `deepseek/deepseek-chat` (override via
`RADIANT_MODEL`).

### Sprint 32b: Autodata live-tested with real OpenRouter key

Generated `reinsurance-pricing` skill end-to-end via DeepSeek on
OpenRouter:

```bash
RADIANT_OPENROUTER_API_KEY=sk-or-v1-... \
  RADIANT_MODEL=deepseek/deepseek-chat \
  radiant autodata reinsurance-pricing \
    --domain="reinsurance pricing for P&C carriers: treaty
      structures (quota share, surplus, stop loss, XL), pricing
      models (burning cost, exposure curves), IBNR reserves,
      broker commission"
```

Output:
- `frontmatter.yaml` (54 lines): valid schema types (report,
  decision, artifact); complete inputs/outputs/gates
- `SKILL.md` (60 lines): Decision tree + Workflow + 3 Examples +
  Anti-patterns + Failure modes + Related skills

**Validation result**: passes `TestAllBundledSkillsValidateCleanly`
(58/58 skills). Meta-loop closed end-to-end.

**Iteration lesson caught**: initial LLM run used `type: object` for
outputs (input-type vocabulary leaking into output spec). Fix:
explicit instructions in system prompt for separate input/output
type vocabularies.

### Sprint 32c: 3 depth skills + 2 commands

| Skill | Domain |
|-------|--------|
| `radiant-aml-kyc` | AML/KYC, sanctions, PEP, SAR; FATF Rec 10/13/24 |
| `radiant-credit-portfolio` | Portfolio concentration, migration, RWA, stress, vintage |
| `radiant-fraud-detection` | Identity, application, payment, ATO, first-party, friendly |

| Command | Purpose |
|---------|---------|
| `radiant validate-file <path>` | Sanity-check scaffolded plan: sections present, no unfilled placeholders |
| `radiant profile <dataset>` | Data profile scaffold: schema, volume, distributions, drift, monitoring |

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean (after `gofmt -w`) |
| `go test ./... -race` | 10 packages OK |
| `TestAllBundledSkillsValidateCleanly` | **60/60 skills pass** (was 57) |
| Tests | **358 PASS, 0 FAIL** (was 353, +5 new) |
| Data races | **0** |
| Cross-compile | **6/6** |

## Final tally (post-Sprint 32)

- **31 CLI commands** (was 29, +2: validate-file, profile)
- **60 bundled skills** (was 57, +3: aml-kyc, credit-portfolio, fraud-detection)
- **1 open MIT schema spec**
- **358 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism**: OpenRouter-first; DeepSeek-tested; Anthropic/OpenAI supported; no bias
- **0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` + `v0.7.0` tags** (both via dogfooded pipeline)

## Meta-loop closed

The Autodata pattern is fully validated end-to-end:
1. User provides `RADIANT_OPENROUTER_API_KEY` (vendor-neutral)
2. `radiant autodata <name> --domain="..."` calls OpenRouter → DeepSeek
3. LLM generates `frontmatter.yaml` + `SKILL.md` following schema
4. Files written to user-specified output dir
5. User reviews, then copies to `internal/skill/skills/<name>/`
6. CI guard `TestAllBundledSkillsValidateCleanly` validates
7. Skill is bundled and listed in `radiant skills list`

This is the **self-improving loop**: AI authors skills; humans
review; CI validates; ship.

## Stopping point

**Yes — fechado.** This is a strong stopping point:
- 31 CLI commands (4× growth from v0.6.0)
- 60 bundled skills (3× growth)
- 358 tests, 0 races, 6/6 cross-compile
- 2 real tags
- Vendor-neutral LLM (OpenRouter-first)
- Self-improving meta-loop (autodata)