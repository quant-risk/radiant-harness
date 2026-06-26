# Sprint 28 validation — Harness-Quant extension (22 new skills)

**Commit (this report):** TBD
**Re-validates:** `b4a2aa5` (Sprint 27 — 4 skills)
**Version:** v0.6.3 (in source)
**Plan:** `docs/HARNESS-QUANT.md`

## What shipped

This is the **biggest single sprint in the project's history**.

### Plan document (new)

- **`docs/HARNESS-QUANT.md`** — strategic plan for the quant +
  financial-risk + science extension. Explains the architectural
  decision (skills, not new CLIs / forks), the skill catalog
  expansion (22 new), and the post-Sprint 28 catalog (50 total).

### 22 new domain skills

| # | Skill | Domain | Lines |
|---|-------|--------|-------|
| **Quantitative core (6)** | | | |
| 1 | `radiant-stats` | Hypothesis testing, CIs, ANOVA, power, multiple testing | 279 |
| 2 | `radiant-econometrics` | Time series, panel, IV, GMM, structural | 277 |
| 3 | `radiant-bayesian` | MCMC, HMC/NUTS, PyMC/Stan, model comparison | 260 |
| 4 | `radiant-causal` | DAGs, A/B, propensity, DiD, RDD, synthetic control | 289 |
| 5 | `radiant-deep-learning` | Architectures, training, distributed, fine-tuning, RLHF | 312 |
| 6 | `radiant-reinforcement-learning` | MDPs, DQN/PPO/SAC, off-policy, multi-agent | 327 |
| **Financial risk (4)** | | | |
| 7 | `radiant-credit-risk` | PD/LGD/EAD, IFRS 9 staging, Basileia, LDPs | 335 |
| 8 | `radiant-market-risk` | VaR, ES, backtest, FRTB IMA/SA, stress | 264 |
| 9 | `radiant-liquidity-risk` | LCR, NSFR, ALM, cash flow forecasting, CFP | 321 |
| 10 | `radiant-operational-risk` | ORM, RCSA, scenario, loss data, KRIs, BCP | 300 |
| **Corporate finance (4)** | | | |
| 11 | `radiant-accounting` | IFRS/CPC, consolidation, leases, hedge, impairment | 310 |
| 12 | `radiant-controlling` | FP&A, budgeting, variance, drivers, rolling forecast | 285 |
| 13 | `radiant-valuation` | DCF, comps, precedents, SOTP, LBO, IAS 36 | 306 |
| 14 | `radiant-capital-markets` | Derivatives, fixed income, credit, structured, factors | 294 |
| **Cross-domain (3)** | | | |
| 15 | `radiant-finance` | Capital structure, dividend, WC, M&A process, WACC | 307 |
| 16 | `radiant-marketing` | MMM, MTA, incrementality, LTV, churn, segmentation | 322 |
| 17 | `radiant-causal-ml` | Uplift, double ML, causal forests, ML-IV | 283 |
| **Science (5)** | | | |
| 18 | `radiant-physics` | Classical mechanics, EM, thermo, statistical, optics | 290 |
| 19 | `radiant-quantum-physics` | Schrödinger, entanglement, gates, measurement | 296 |
| 20 | `radiant-quantum-ml` | VQE, QAOA, VQC, quantum kernels, NISQ | 305 |
| 21 | `radiant-chemistry` | DFT, MD, force fields, kinetics, spectroscopy | 292 |
| 22 | `radiant-biology` | Genomics, proteomics, scRNA-seq, MD, CRISPR | 324 |

**Total new content: 6578 lines** across 44 files (22 frontmatter + 22 SKILL.md) + 1 plan doc.

## Domain coverage (post-Sprint 28)

| Category | Count | Skills |
|----------|-------|--------|
| Process / lifecycle | 6 | nova-feature, nova-product, kickoff, handoff, incident, roadmap |
| Discovery / design | 6 | clarificar, validar, mapear, diagramar, adr, metricas |
| Quality / correctness | 4 | auditar, evals, revisar-pr, camada-agentica |
| Infrastructure | 2 | setup-ci, integracoes |
| Software domain | 10 | mobile, data, frontend, ml, game, cli, api, security, blockchain, iot |
| **Quantitative core** | **6** | stats, econometrics, bayesian, causal, deep-learning, rl |
| **Financial risk** | **4** | credit-risk, market-risk, liquidity-risk, operational-risk |
| **Corporate finance** | **4** | accounting, controlling, valuation, capital-markets |
| **Cross-domain quant** | **3** | finance, marketing, causal-ml |
| **Science** | **5** | physics, quantum-physics, quantum-ml, chemistry, biology |

**50 skills total.** Covers nearly every major professional domain
relevant to risk consulting, financial modelling, scientific
computing, and software engineering.

## Iteration discipline

Multiple issues caught + fixed at dev time, all by the existing
CI guard (`TestAllBundledSkillsValidateCleanly`):

1. **`type: list` / `type: integer`**: Caught in earlier sprints;
   schema fix propagated to this sprint (no occurrences).
2. **Unquoted YAML colon-in-description**: ~10 occurrences
   across the 22 skills. Auto-fixed via Python script.
3. **YAML list indentation (`- "..."` vs `  - "..."`)**:
   ~10 occurrences across the 22 skills. Fixed manually after
   Python script missed edge cases.

**All issues caught at dev time, not at user time.** The CI guard
paid for itself in this sprint.

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| `TestAllBundledSkillsValidateCleanly` | **50/50 skills pass** (was 28) |
| Tests | **337 PASS, 0 FAIL** |
| Data races | **0** |
| Cross-compile | **6/6** |

## Final tally (post-everything)

- **21 CLI commands** + **50 bundled skills** (was 28, +22) + **1 open MIT schema spec**
- **337 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**
- **`HARNESS-QUANT.md` plan document**

## What this means

The harness now covers:
- **Quant risk consulting** (Fortvna's day job): PD/LGD/EAD, IFRS 9,
  Basileia, VaR/ES, LCR/NSFR, OR, credit/market/liquidity/operational
  risk modelling
- **Quantitative research**: stats, econometrics, Bayesian, causal,
  DL, RL
- **Corporate finance**: DCF, M&A, valuation, capital structure,
  controlling, accounting
- **Marketing analytics**: MMM, attribution, incrementality, LTV
- **Capital markets**: derivatives pricing, fixed income, structured
- **Science**: physics, quantum physics, chemistry, biology, quantum ML

This is **rare for an open-source CLI**: a single tool that can
support quant risk modelling, financial valuation, scientific
research, AND software engineering — with the same workflow
discipline applied across all.

## Stopping point

This is a **major milestone**. The harness is now comprehensive
across professional domains.

Remaining candidates are all niche additions (more science,
niche verticals) or capability investments (CLI commands on top
of skills, e.g. `radiant stats`, `radiant causal-estimate`).

Or **tag v0.7.0 for real** — this is a major version bump
warranted by the quant expansion.