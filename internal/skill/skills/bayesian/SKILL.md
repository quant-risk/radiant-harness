# Skill: bayesian

> Bayesian inference: priors, posteriors, MCMC diagnostics,
> posterior predictive checks, model comparison. Bayesian
> inference is full uncertainty quantification, not point
> estimates with confidence intervals.

## Decision tree

```
Model + data + priors
        │
        ▼
[Step 1] Prior elicitation + justification
        │
        ▼
[Step 2] Specify model (likelihood + priors + structure)
        │
        ▼
[Step 3] Sample (HMC/NUTS, 4 chains minimum)
        │
        ▼
[Step 4] Diagnostics (R-hat, ESS, divergences, trace plots)
        │
        ▼
[Step 5] Posterior predictive check (PPC)
        │
        ▼
[Step 6] Sensitivity analysis (alternative priors)
        │
        ▼
[Step 7] Report (posterior summaries + PPC + sensitivity)
```

## Workflow

### Prior elicitation

| Prior type | When | Example |
|-----------|------|---------|
| **Weakly informative** | Default choice; regularises without strong opinion | N(0, 10) for regression coefficient |
| **Informative** | Strong domain knowledge; small sample | Expert estimate ± SE |
| **Strong** | Physical constraint or regulatory requirement | Variance > 0; probability ∈ [0,1] |
| **Reference (Jeffreys)** | Want "objective" Bayesian; no info | p(θ) ∝ sqrt(|I(θ)|) |
| **Hierarchical** | Group structure (patients, schools) | Group-level hyperprior |

Document priors in `docs/bayesian/prior-elicitation.md`:
- Why this prior?
- Source: literature / expert / physical / default
- Sensitivity: what if I use this alternative?

### MCMC diagnostics

| Metric | Threshold | What it catches |
|--------|-----------|-----------------|
| **R-hat** | < 1.01 | Chains haven't mixed |
| **ESS (bulk)** | > 400 | Enough effective samples |
| **ESS (tail)** | > 400 | Enough in tails (for credible intervals) |
| **Divergences** | 0 | HMC explored forbidden regions |
| **Tree depth** | Mostly max | HMC taking long jumps |
| **Trace plots** | Visually mixed | Visual confirmation |

**If R-hat > 1.01**: chains haven't converged. Run longer,
reparameterise, or simplify model.

**If divergences > 0**: geometry is bad. Increase `target_accept`,
reparameterise, or fix model.

**If ESS < 100**: too few effective samples. Run more iterations.

### Posterior predictive checks (PPC)

Generate replicated data from the posterior, compare to observed:

```python
# In PyMC / Stan
posterior_predictive = pm.sample_posterior_predictive(trace)
ppc = az.plot_ppc(trace, data=y)
```

The model should be able to **reproduce the data**. If observed
data is in the extreme tails of the posterior predictive, the
model is mis-specified.

### Model comparison

| Method | Use |
|--------|-----|
| **WAIC** | Widely Applicable IC; estimates out-of-sample deviance |
| **LOO-CV** | Leave-one-out cross-validation; uses PSIS |
| **Bayes factor** | Ratio of marginal likelihoods; sensitive to priors |
| **Posterior model probability** | With equal prior, posterior over models |

Tools: `arviz.compare`, `loo`, `bayestestR` (R), `pymc.stats`.

### Sensitivity analysis

Conclusions should be **robust to reasonable prior choices**:

1. Fit with weakly informative prior.
2. Fit with informative prior.
3. Compare posteriors; if drastically different, conclusions
   depend on priors (red flag).
4. Fit with alternative priors (e.g., heavier tails).
5. Report all.

## Examples

### Example 1: A/B test (Bayesian)

```
Data: 5000 visitors / variant; 250 vs 290 conversions
Model: Beta(1,1) prior on each conversion rate
Result: posterior p_B > p_A has probability 0.94
Decision rule: ship B if P(B>A) > 0.95
Decision: ship B (or run longer)
```

### Example 2: hierarchical model (multi-region)

```
Data: 100 regions, sales ~ region_effect + price + season
Hierarchical: region_effect ~ Normal(mu, sigma)
              mu ~ Normal(0, 10)
              sigma ~ HalfNormal(5)
Posterior shrinkage: small-sample regions shrink toward mu
Estimate: partial pooling improves out-of-sample RMSE by 18%
```

### Example 3: time series (state-space)

```
Model: local linear trend + seasonal
  y_t = mu_t + gamma_t + eps_t
  mu_t = mu_{t-1} + delta_{t-1} + eta_t
  gamma_t = -sum(gamma_{t-s+1..t}) + omega_t
Inference: NUTS in Stan / PyMC
Diagnostics: R-hat=1.00, ESS>1000, no divergences
PPC: model reproduces ACF, mean, variance
Forecast: 12 months ahead with 80% + 95% credible intervals
```

## Anti-patterns

### ❌ Flat priors without justification

"Let the data speak" sounds good but flat priors are improper for
many models. Use weakly informative priors with justification.

### ❌ Ignoring divergences

Divergences in NUTS mean the sampler explored a region of bad
geometry. Don't trust the posterior until they're resolved.

### ❌ Reporting point estimates without CIs

Bayesian inference gives full posteriors. Reporting only the mean
wastes the information.

### ❌ No posterior predictive check

Model that fits the data by accident is mis-specified. Always
check that posterior predictive reproduces observed data.

### ❌ Bayes factor from improper priors

Bayes factors are sensitive to priors. Use proper priors, or use
WAIC/LOO for model comparison.

## Failure modes

| Failure | Recovery |
|---------|----------|
| R-hat > 1.01 | Run more; reparameterise; simpler model |
| Divergences | target_accept → 0.99; reparameterise; non-centered parametrisation |
| ESS too low | More iterations; thinning; better mixing |
| PPC fails | Model mis-specified; revisit |
| Sensitive to priors | Need more data; or report sensitivity |
| Improper posterior | Check prior support; use proper prior |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/stats` | Frequentist alternative for comparison |
| `/causal` | Bayesian causal inference (BART, BART-CA) |
| `/ml` | Bayesian deep learning; BNN |
| `/econometrics` | Bayesian time series (BSTS); hierarchical panel |