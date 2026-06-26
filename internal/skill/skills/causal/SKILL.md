# Skill: causal

> Causal inference: DAGs, identification, A/B, propensity, DiD,
> RDD, synthetic control, mediation, heterogeneous effects.
> Correlation without identification is just a number.

## Decision tree

```
Research question (effect of X on Y)
        │
        ▼
[Step 1] Draw DAG (treatment, outcome, confounders)
        │
        ▼
[Step 2] Identify design (RCT / obs / quasi / IV)
        │
        ├── RCT              -> ATE from randomisation
        ├── Observational    -> need conditional ignorability
        │   -> Propensity scores / IPW / matching
        ├── Quasi-experimental -> DiD / RDD / synthetic control
        └── Endogenous X      -> IV / control function
        │
        ▼
[Step 3] State identification assumption (write it down)
        │
        ▼
[Step 4] Estimate ATE
        │
        ▼
[Step 5] Estimate CATE (subgroups)
        │
        ▼
[Step 6] Sensitivity analysis
        │
        ▼
[Step 7] Report
```

## Workflow

### DAG (Directed Acyclic Graph)

Draw the causal graph FIRST. Identify:

- **Treatment** (variable you're manipulating)
- **Outcome** (variable you measure)
- **Confounders** (variables affecting both treatment AND outcome)
- **Mediators** (variables on the causal path)
- **Colliders** (variables caused by both treatment and outcome;
  conditioning on these induces bias)

Tools: `dagitty`, `causalgraphicalmodels` (Python), ggdag (R).

### Identification

| Design | Assumption | Test |
|--------|-----------|------|
| **RCT** | Randomisation | Randomisation check; covariate balance |
| **Propensity score** | Conditional ignorability (unconfoundedness) | PS overlap; balance after weighting |
| **DiD** | Parallel trends | Pre-trend test |
| **RDD** | Continuity at cutoff | McCrary density test; placebo cutoffs |
| **Synthetic control** | Convex hull contains treated | Pre-period fit |
| **IV** | Exclusion restriction; relevance | First-stage F; overid test |

### ATE / ATT / CATE

| Estimand | Definition | When |
|----------|-----------|------|
| **ATE** | E[Y(1) - Y(0)] | Average effect over population |
| **ATT** | E[Y(1) - Y(0) | T=1] | Effect on treated |
| **CATE** | E[Y(1) - Y(0) | X=x] | Heterogeneous by X |
| **LATE** | E[Y(1) - Y(0) | complier] | Local to IV compliers |

### Specific methods

**Propensity score methods**:
- Logistic regression → PS
- **Matching**: nearest neighbour, caliper, optimal
- **Stratification**: subclassify on PS quintiles
- **IPW (Inverse Probability Weighting)**: reweight by 1/PS
- Diagnostics: PS overlap (common support); standardised
  mean differences after weighting (target <0.1)

**Difference-in-differences**:
- Two-way FE: Y_it = α + β TREAT_i + γ POST_t + δ(TREAT × POST) + ε
- δ = ATT
- **Parallel trends test**: pre-treatment trends similar
- Robustness: alternative control groups; placebo periods;
  event-study design

**Regression discontinuity**:
- Sharp: P(T=1) jumps from 0 to 1 at cutoff
- Fuzzy: probability jumps, not deterministic
- Estimate: local linear regression near cutoff
- Bandwidth selection: Imbens-Kalyanaraman, MSE-optimal

**Synthetic control**:
- Construct weighted combination of untreated units
- Weights chosen to match pre-treatment outcome
- Inference: permutation test (placebo units)

### Sensitivity analysis

How strong would an unobserved confounder need to be to change
the conclusion?

| Method | Implementation |
|--------|---------------|
| **E-value** | VanderWeele; minimum strength to nullify |
| **Rosenbaum bounds** | Gamma sensitivity; how much hidden bias |
| **Oster bounds** | Selection on observables proxy |
| **Placebo tests** | Fake treatment; should find zero effect |

### Mediation analysis

Total effect = direct + indirect (through mediator).

- **Baron-Kenny**: simple; bias-prone
- **Imai et al.**: sensitivity-aware; modern
- Tools: `mediation` (R), `statsmodels.stats.mediation`

## Examples

### Example 1: A/B test (RCT)

```
Design:  randomised controlled trial
Sample:  10,000 users, 50/50 split
Outcome: 30-day retention (binary)
Analysis:
  - ATE = 0.024 (2.4 pp lift)
  - 95% CI [0.011, 0.037]
  - p-value: 0.0003
CATE:    bigger lift on mobile (3.1 pp) than web (1.7 pp)
Decision: ship
```

### Example 2: DiD (policy change)

```
Question: effect of minimum wage increase on teen employment
Design:  DiD (treated = states with MW increase; control = others)
Data:    state-year panel 2010-2020
Estimate: ATT = -0.04 (4% drop)
Pre-trends: parallel in 2010-2015; small divergence 2016-2018
Robustness: 2-way FE; placebo 2010-2014 (zero effect); alternative control states
```

### Example 3: synthetic control (state policy)

```
Question: effect of California's tobacco control program
Design:  synthetic control (CA = treated; other states = donor pool)
Method:  weighted combination of 30 states
Weights: TX=0.18, NY=0.15, FL=0.12, ...
Pre-period fit: RMSPE = 0.04 (good)
Effect: 17% reduction in tobacco sales vs synthetic CA
Inference: permutation test p < 0.05
```

## Anti-patterns

### ❌ Correlation as causation

"No DAG, no identification, just correlation" — opinion piece, not
analysis. Always draw the DAG.

### ❌ ATE without CATE

Average effect hides heterogeneity. Always explore CATE when
meaningful.

### ❌ DiD without pre-trend test

Parallel trends is an assumption. Test it; if it fails, the model
is invalid.

### ❌ PSM without overlap check

If treated and control don't overlap in PS, matched pairs are bad.
Check common support.

### ❌ RDD with bandwidth too wide

Wide bandwidth averages over heterogeneous effects near cutoff.
Use MSE-optimal or Imbens-Kalyanaraman.

### ❌ No sensitivity analysis

Point estimates without "how robust is this to hidden bias?" are
incomplete.

## Failure modes

| Failure | Recovery |
|---------|----------|
| DAG changes mid-analysis | Re-specify; document |
| Pre-trends violated | Event-study; alternative control; synthetic control |
| PS overlap poor | Trim; alternative matching; IPW with overlap |
| RCT underpowered | Replicate; meta-analyse; report wide CI |
| Sensitivity shows fragility | Need more data; or weaker claim |
| Heterogeneous effects unexplained | Investigate moderators; report all |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/stats` | Underlying hypothesis testing |
| `/bayesian` | Bayesian causal inference (BART) |
| `/causal-ml` | ML-augmented causal inference (uplift, double ML) |
| `/econometrics` | IV / GMM / panel within causal framework |
| `/evals` | A/B test rigour |