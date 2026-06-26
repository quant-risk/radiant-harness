# HARNESS-QUANT — Quantitative & Scientific Extension Plan

**Status:** Active (Sprint 28)
**Owner:** radiant-harness contributors
**Last update:** Sprint 28 — quant + financial-risk + science skills batch

## Why

The base `radiant-harness` (v0.6.3) ships 21 CLI commands, 28 domain-
agnostic and software-focused skills, and one open skill schema
(MIT, `docs/SKILL-SCHEMA.md`). It's excellent for shipping software
products end-to-end, but does not yet cover:

- **Quantitative disciplines** — statistical modeling, econometrics,
  Bayesian inference, causal inference, deep learning, reinforcement
  learning.
- **Financial-risk and corporate-finance domains** — credit risk,
  market risk, liquidity risk, operational risk, accounting,
  controlling, valuation, marketing mix modeling, capital markets.
- **Scientific disciplines** — physics, quantum physics, chemistry,
  biology, quantum ML.

These are the day-job domains of Fortvna Risk Solutions
(quant risk consulting, CMN 4.966 / IFRS 9 / Basileia) and adjacent
disciplines where structured workflows (problem framing, eval
discipline, monitoring) pay back just as well as in pure software.

This document plans the **Harness-Quant extension** — a content
expansion of the same CLI tool, not a fork. The skill schema is the
open extension point: any domain that can express its workflow as
`frontmatter.yaml` + `SKILL.md` slots in cleanly.

## Architectural decision: skills, not new CLIs

We considered forking the binary (e.g. `radiant-quant`) or adding
new top-level commands (e.g. `radiant stats`, `radiant causal`).
Both add complexity (Go code, tests, cross-compile, command
discovery). **Skills** are the right primitive because:

- They use the existing schema; CI guard `TestAllBundledSkillsValidateCleanly`
  catches drift automatically.
- They ship inside the existing binary (already 6/6 cross-compile).
- They are open — anyone can author more in the same format.
- They integrate with the existing `radiant skills list`,
  `radiant skills validate`, `radiant update`, and the AGENTS.md
  generator.

If domain commands later justify the cost (e.g. `radiant causal-estimate`
that calls `statsmodels` or `dowhy` under the hood), they can be
added on top of the skill foundation.

## Skill catalog expansion

### Sprint 28 (this sprint): 22 new skills, batched

| # | Skill | Domain | Lines target |
|---|-------|--------|--------------|
| 1 | `radiant-stats` | Statistical modeling | ~350 |
| 2 | `radiant-econometrics` | Time series, panel, IV, structural | ~400 |
| 3 | `radiant-bayesian` | Bayesian inference, MCMC | ~350 |
| 4 | `radiant-causal` | DAGs, A/B, propensity, DiD, RDD | ~400 |
| 5 | `radiant-deep-learning` | Architectures, training, distributed | ~400 |
| 6 | `radiant-reinforcement-learning` | MDPs, policy gradient, off-policy | ~400 |
| 7 | `radiant-credit-risk` | PD/LGD/EAD, IFRS 9, Basileia | ~450 |
| 8 | `radiant-market-risk` | VaR, ES, stress testing | ~400 |
| 9 | `radiant-liquidity-risk` | LCR, NSFR, ALM | ~350 |
| 10 | `radiant-operational-risk` | ORM, KRIs, scenario analysis | ~350 |
| 11 | `radiant-accounting` | IFRS, financial reporting | ~400 |
| 12 | `radiant-controlling` | FP&A, controlling | ~350 |
| 13 | `radiant-valuation` | DCF, comps, M&A | ~400 |
| 14 | `radiant-marketing` | Marketing mix models, attribution | ~400 |
| 15 | `radiant-causal-ml` | Uplift modeling, heterogeneous effects | ~400 |
| 16 | `radiant-capital-markets` | Derivatives, fixed income, equity | ~400 |
| 17 | `radiant-finance` | Corporate finance general | ~350 |
| 18 | `radiant-quantum-ml` | Variational quantum eigensolvers, QAOA | ~350 |
| 19 | `radiant-physics` | Classical mechanics, EM, thermo | ~350 |
| 20 | `radiant-quantum-physics` | QM formalism, entanglement, gates | ~400 |
| 21 | `radiant-chemistry` | Molecular modeling, reactions | ~350 |
| 22 | `radiant-biology` | Genomics, proteomics, systems bio | ~350 |

**Total target: ~7700 lines** of new SKILL.md content + ~1700 lines of
new frontmatter.yaml across 44 files.

### What each skill covers (high-level)

#### Quantitative core
- **`radiant-stats`**: hypothesis testing (parametric + non-parametric),
  CIs, regression diagnostics, ANOVA, sampling, power analysis,
  multiple testing correction, bootstrap, permutation tests.
- **`radiant-econometrics`**: time series (ARIMA, VAR, VECM, GARCH,
  state-space), panel data (FE/RE, clustered SE), instrumental
  variables (2SLS, GMM), structural models, cointegração.
- **`radiant-bayesian`**: Bayesian inference, MCMC (HMC/NUTS),
  PyMC/Stan, prior/posterior predictive checks, Bayes factors,
  model averaging, calibration.
- **`radiant-causal`**: DAGs (Pearl), potential outcomes (Rubin),
  A/B testing (frequentist + Bayesian), propensity scores,
  difference-in-differences, regression discontinuity, synthetic
  control, mediation, heterogeneous treatment effects.
- **`radiant-deep-learning`**: architectures (CNN/RNN/Transformer/ViT
  /diffusion), training (optimizers, LR schedules, regularization,
  mixed precision), distributed (DDP/FSDP/DeepSpeed), fine-tuning
  (LoRA/QLoRA/full), preference learning (RLHF/DPO/PPO).
- **`radiant-reinforcement-learning`**: MDPs, value/policy iteration,
  model-free (DQN, REINFORCE), policy gradient (PPO/SAC/A3C),
  exploration, off-policy (importance sampling, CQL, BCQ),
  multi-agent, sim-to-real.

#### Financial risk + corporate finance (Fortvna core)
- **`radiant-credit-risk`**: PD/LGD/EAD modeling, IFRS 9 staging
  (Stage 1/2/3), Basileia capital, scorecards, behavioral models,
  stress testing, vintage analysis, low-default portfolios.
- **`radiant-market-risk`**: VaR (historical, parametric, Monte Carlo),
  Expected Shortfall, stress testing, backtesting (Kupiec, Christoffersen),
  FRTB IMA/SA, market risk factors (IR, FX, equity, commodity).
- **`radiant-liquidity-risk`**: LCR, NSFR, ALM, cash flow forecasting,
  intraday liquidity, contingency funding plan, deposit behaviour.
- **`radiant-operational-risk`**: ORM framework, KRIs, scenario
  analysis, loss data collection, RCSA, business continuity,
  Basel III OR capital.
- **`radiant-accounting`**: IFRS (Brazil CPC), financial statements,
  consolidation, fair value, hedge accounting, lease, revenue
  recognition (IFRS 15), impairment.
- **`radiant-controlling`**: FP&A, budgeting, variance analysis,
  cost allocation, KPI trees, driver-based planning, rolling
  forecast.
- **`radiant-valuation`**: DCF, comparable companies, precedent
  transactions, sum-of-the-parts, leveraged buyout, real options,
  impairment testing (IAS 36).
- **`radiant-marketing`**: marketing mix modeling (MMM), attribution
  (MTA), incrementality testing, customer LTV, churn modeling,
  brand tracking, media planning.
- **`radiant-causal-ml`**: uplift modeling (T-learner, S-learner,
  X-learner, R-learner), heterogeneous treatment effects, double
  ML, causal forests, instrumental variables with ML, sensitivity
  analysis.
- **`radiant-capital-markets`**: derivatives pricing (Black-Scholes,
  binomial, Monte Carlo, Heston, SABR), fixed income (duration,
  convexity, key rate, OAS), credit markets (CDS, CDO, CLO),
  structured products, equity (factor models, CAPM).
- **`radiant-finance`**: corporate finance general — capital
  structure (Modigliani-Miller, trade-off, pecking order), WACC,
  CAPM, dividend policy, working capital management, M&A process.

#### Science (extension for completeness)
- **`radiant-quantum-ml`**: variational quantum eigensolvers (VQE),
  QAOA, quantum kernels, variational circuits, hybrid quantum-
  classical models, near-term intermediate-scale quantum (NISQ)
  era constraints, error mitigation.
- **`radiant-physics`**: classical mechanics, electromagnetism,
  thermodynamics, statistical mechanics, optics, waves, fluid
  dynamics — problem-solving methodology, dimensional analysis,
  limiting cases, computational physics.
- **`radiant-quantum-physics`**: Schrödinger equation, Hilbert
  space formalism, entanglement, quantum gates, density matrices,
  quantum measurement, decoherence, Bell inequalities.
- **`radiant-chemistry`**: molecular modeling (DFT, MD, force fields),
  reaction mechanisms, kinetics, thermodynamics, organic / inorganic
  / physical chemistry, spectroscopy, drug discovery workflows.
- **`radiant-biology`**: genomics (NGS, variant calling), proteomics,
  systems biology, molecular biology (CRISPR, sequencing),
  bioinformatics pipelines, drug-target interaction.

### Post-Sprint 28 catalog (50 skills total)

| Category | Count | Skills |
|----------|-------|--------|
| Process / lifecycle | 6 | nova-feature, nova-product, kickoff, handoff, incident, roadmap |
| Discovery / design | 6 | clarificar, validar, mapear, diagramar, adr, metricas |
| Quality / correctness | 4 | auditar, evals, revisar-pr, camada-agentica |
| Infrastructure | 2 | setup-ci, integracoes |
| Software domain | 10 | mobile, data, frontend, ml, game, cli, api, security, blockchain, iot |
| **Quantitative core** | **6** | stats, econometrics, bayesian, causal, deep-learning, rl |
| **Financial-risk** | **4** | credit-risk, market-risk, liquidity-risk, operational-risk |
| **Corporate finance** | **4** | accounting, controlling, valuation, capital-markets |
| **Cross-domain quant** | **3** | finance, marketing, causal-ml |
| **Science** | **5** | physics, quantum-physics, quantum-ml, chemistry, biology |

## Roadmap

| Sprint | Scope | Skills added |
|--------|-------|--------------|
| **Sprint 28 (this)** | Quant core + financial risk + science | +22 |
| Sprint 29 (TBD) | Operational tooling: `radiant stats` / `radiant causal` CLI commands on top of skill foundation | 0 new skills |
| Sprint 30 (TBD) | Author-generated skills from community | +TBD |

If new top-level commands are justified (e.g. `radiant causal-estimate`
that calls `dowhy` under the hood), they'd be added as Go code in
`cmd/radiant/main.go` with their own tests, behind the existing
skill foundation.

## Constraints (unchanged from base)

- **Open MIT schema** (`docs/SKILL-SCHEMA.md`) is the only authoring
  contract. Skills MUST validate against it.
- **CI guard** `TestAllBundledSkillsValidateCleanly` catches schema
  drift at test time.
- **Vendor-neutral**: no domain skill biases toward a single tool,
  framework, or vendor. Lists trade-offs; lets the user pick.
- **Local-first / zero telemetry**: no skill sends data anywhere.
- **Tier eligibility**: each skill declares `tier_eligible`
  (Trivial / Feature / Architecture) so `radiant update` and the
  AGENTS.md generator can route correctly.

## Open questions

1. Should `radiant-ml` be narrowed now that `radiant-deep-learning`
   and `radiant-bayesian` exist? Suggest: trim `radiant-ml` to
   the generic ML workflow (problem framing, baseline, monitoring)
   and let the specialized skills cover depth.
2. Should we ship a `radiant-quant` thin CLI wrapper for users
   who only want the quant subset? Pro: faster startup, smaller
   `radiant skills list` output. Con: command surface fragmentation.
3. Should the skill schema gain a `discipline` field (e.g.
   `discipline: finance`, `discipline: science`) so users can
   filter by domain in `radiant skills list --discipline=finance`?
   That's a schema change; deferred until needed.
4. Should each quant skill ship a worked Python notebook under
   `examples/<skill>/<example>.ipynb`? Could go in a follow-up
   sprint.

## Validation strategy (this sprint)

Each skill MUST validate against `TestAllBundledSkillsValidateCleanly`.
The CI guard's known schema rule:

- `inputs.*.type` ∈ `string | number | enum | object | path`
  (NOT `list`, NOT `integer` — use `string` for lists, `number`
  for integers)

Pre-validate before commit:
```bash
go test ./internal/skill/ -run TestAllBundledSkillsValidateCleanly -v
```

## Files touched (this sprint)

```
docs/HARNESS-QUANT.md                  (this file — NEW)
internal/skill/skills/<22 new skills>/
  frontmatter.yaml
  SKILL.md
docs/validation-report-sprint-28.md    (post-sprint report)
```

**Estimated diff**: +9000 lines across 46 new files + 1 plan doc.