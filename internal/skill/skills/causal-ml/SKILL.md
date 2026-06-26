# Skill: causal-ml

> Uplift modeling, double ML, causal forests, heterogeneous
> treatment effects. ML gives flexibility; causal gives
> identification — combine them.

## Decision tree

```
Treatment effect question + heterogeneous effects wanted
        │
        ▼
[Step 1] Choose method
        │
        ├── Binary treatment, subgroups    -> uplift (T/S/X/R-learner)
        ├── High-dimensional confounders    -> double ML (DML)
        ├── Subgroup discovery             -> causal forest
        ├── Endogenous treatment           -> ML-based IV
        └── Many treatments / optimal policy -> causal forest + policy learning
        │
        ▼
[Step 2] Identify assumption (conditional ignorability, etc.)
        │
        ▼
[Step 3] Estimate
        │
        ▼
[Step 4] Validate (placebo, honest split, sensitivity)
        │
        ▼
[Step 5] Deploy (uplift scoring, policy)
```

## Workflow

### Uplift modeling

Goal: estimate τ(x) = E[Y(1) - Y(0) | X=x] for each individual.

| Method | Description | Strengths | Weaknesses |
|--------|-------------|-----------|------------|
| **T-learner** | Two models: μ1(x), μ0(x); τ(x) = μ1 - μ0 | Simple; flexible | High variance with imbalanced treatment |
| **S-learner** | One model: μ(T, x); τ(x) = μ(1, x) - μ(0, x) | Single model; efficient | Treatment often ignored by ML |
| **X-learner** | Two models + propensity weighting | Robust to imbalance | More complex |
| **R-learner** | Residual-on-residual; Robinson decomposition | Sample efficient | Sensitive to nuisance errors |
| **DR-learner** | Doubly robust; combine IPW + direct | Asymptotically efficient | Complex |

**Evaluation**:
- **Qini curve**: cumulative incremental outcomes vs population fraction targeted
- **AUUC**: area under uplift curve
- **Qini coefficient**: AUUC / random baseline

### Double ML (DML)

For high-dimensional confounders:

```
1. Estimate nuisance functions:
   ê(x) = E[T | X=x]  (propensity / first stage)
   m̂(x) = E[Y | X=x]  (outcome / partial out)

2. Residualise:
   Ỹ = Y - m̂(x)
   T̃ = T - ê(x)

3. Regress Ỹ on T̃ with regularisation:
   θ = argmin Σ (Ỹ - θ × T̃)² + λ ||θ||²
```

Sample splitting (cross-fitting): train nuisance on one fold,
predict on other. Reduces overfitting leakage.

**Tools**: `DoubleMachineLearning` (EconML), `dml` (R), `doubleml`
(Python).

### Causal forests

Generalized Random Forest with causal objective:

```
1. Honest splitting: split sample into train (build tree) + estimate (compute CATE per leaf)
2. Local moment conditions:
   τ(x) = argmin Σ α_i × (Y_i - m(X_i) - θ × (T_i - e(X_i)))
3. Subgroup identification: leaves with different τ → heterogeneous effects
```

**Tools**: `grf` (R), `econml.dml.CausalForestDML` (Python),
`causalml`.

### Causal trees

Single tree (vs forest):

- Recursive partitioning based on treatment effect heterogeneity
- Identify subgroups with high / low τ
- Less stable than forest; more interpretable

### ML-based IV

When T is endogenous, IV with ML:

```
1. First stage: T = g(X, Z) + ν (ML model)
2. Compute instrument strength (partial F)
3. Second stage: Y = α × T̂ + h(X) + ε
```

Two-sample IV (2SIV), deep IV, kernel IV — use ML for nuisance.

### Optimal policy learning

Given τ(x), who to treat?

- **Policy**: π(x) = 1 if τ(x) > c, else 0
- **c** = cost of treatment / value of outcome
- **Doubly robust learning**: combine outcome + treatment propensities
- **Validation**: counterfactual evaluation

### Sensitivity analysis

How robust is τ to unobserved confounding?

- **E-value** (VanderWeele): minimum strength to nullify
- **Rosenbaum bounds**: how much hidden bias
- **Partial identification**: range of plausible τ under various
  assumptions
- **Sensitivity to model**: alternative models, alternative
  hyperparameters

## Examples

### Example 1: uplift modeling (marketing campaign)

```
Data: 100k customers; 50% treated (campaign)
Method: X-learner with XGBoost base models
Validation:
  - Qini coefficient: 0.42 (vs 0.20 random; 0.65 oracle)
  - Top 10% by uplift: 3.2x conversion vs control
  - Bottom 10%: 0.7x (negative — don't target)
Deployment: target top 30% by uplift score
Outcome: +18% incremental conversions vs random targeting
```

### Example 2: Double ML (high-dim confounders)

```
Treatment: T (job training program)
Outcome: Y (annual income)
Confounders: X (300 demographic + behavioural features)
Method: DML with Lasso for nuisance functions
Sample splitting: 5-fold cross-fitting
Result: τ = $2,400 (SE = $600), p < 0.001
Sensitivity: E-value = 3.2 (robust to moderate unobserved confounding)
```

### Example 3: causal forest (subgroup discovery)

```
Treatment: T (new drug)
Outcome: Y (recovery)
Forest: 2000 trees, honest splitting
Top subgroups (high τ):
  - Young patients with high baseline severity: τ = +25%
  - With comorbidity X: τ = +18%
Lowest subgroups (τ ≤ 0):
  - Older with comorbidity Y: τ = -5% (do not treat)
Decision: precision medicine; target high-τ groups
```

## Anti-patterns

### ❌ T-learner with extreme propensity

High variance in treated / control if imbalanced. Use X-learner.

### ❌ Causal forest without honest splitting

Overfits subgroups. Honest split is non-negotiable.

### ❌ Uplift model without qini curve

Hard to assess business value. Always plot qini.

### ❌ DML without sample splitting

First-stage overfitting leaks to second stage. Use cross-fitting.

### ❌ No placebo test

Real effect vs spurious correlation. Run placebo.

### ❌ No sensitivity analysis

Hidden confounder could change conclusion. Report E-value.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Qini negative | Model worse than random; check propensity |
| Subgroup too small | Combine leaves; larger forest |
| Treatment imbalance | X-learner or DR-learner; reweight |
| CATE unstable | More trees; honest splitting; more data |
| DML first-stage weak | Stronger features; alternative estimator |
| Sensitivity shows fragility | Need more data or stronger identification |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/causal` | Causal inference fundamentals |
| `/ml` | ML prediction; base models |
| `/bayesian` | Bayesian causal inference; uncertainty |
| `/marketing` | Marketing use cases |
| `/stats` | Significance testing |