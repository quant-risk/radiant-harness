# Skill: credit-risk

> PD / LGD / EAD modelling, IFRS 9 staging, Basileia capital,
> scorecards, vintage analysis. Credit risk is the largest
> risk in most banks — modelling must be rigorous AND
> regulator-defensible.

## Decision tree

```
Credit risk question
        │
        ▼
[Step 1] Define target (PD / LGD / EAD / CCF / staging)
        │
        ▼
[Step 2] Definition of default (90+ DPD, UTP)
        │
        ▼
[Step 3] Sample (development vs validation, vintage)
        │
        ▼
[Step 4] Feature engineering
        │
        ▼
[Step 5] Method (logistic / survival / ML / structural)
        │
        ▼
[Step 6] Validation (discrimination + calibration + stability)
        │
        ▼
[Step 7] Regulatory mapping (IFRS 9 / Basileia)
        │
        ▼
[Step 8] Monitoring (PSI, drift, override tracking)
```

## Workflow

### Definition of Default (DD)

Two pillars (BCBS / IFRS 9 aligned):
1. **Past Due > 90 days** (counted from oldest unpaid instalment)
2. **Unlikely to Pay (UTP)** indicators:
   - Distressed restructuring
   - Bankruptcy / insolvency
   - Sale of credit obligation at material economic loss
   - Triggered acceleration / cross-default
   - Distressed exchange

Document the DD **before modelling**. Every model uses the same
DD within a portfolio.

### Sample design

| Validation type | Purpose |
|----------------|---------|
| **Out-of-sample (OOS)** | Hold-out random; tests generalisation |
| **Out-of-time (OOT)** | Train on N years, test on next; tests time stability |
| **Out-of-segment** | Train retail, test SME; tests transferability |

For credit risk, OOT is the gold standard (defaults have temporal
dependence).

**Vintage analysis**: track default rate by origination cohort.
Cohorts with worse performance than expected → capital add-on.

### PD models

**Scorecard approach** (traditional):
- WOE (Weight of Evidence) binning of features
- Logistic regression (interpretable, regulator-friendly)
- Score = a + b·log(odds)

**ML approaches**:
- Gradient boosting (XGBoost, LightGBM) — common for non-regulatory
- Neural networks — competitive but less interpretable
- Survival models (Cox PH, RSF) — handle time-to-default

**Validation metrics**:
- **Discrimination**: AUC, Gini (= 2·AUC - 1), KS statistic
- **Calibration**: Brier score, Hosmer-Lemeshow, calibration plot
- **Stability**: PSI (Population Stability Index), CSI (Characteristic
  Stability Index)

Thresholds (typical for retail PD):
- Gini ≥ 30
- PSI < 0.10 stable; 0.10-0.25 moderate; >0.25 unstable

### LGD models

| Type | Approach |
|------|----------|
| **Workout LGD** | Sum of (recovery cash flows × discount factor) / EAD |
| **Market LGD** | (EAD - recovery from sale) / EAD |
| **Default LGD** | Realised LGD on defaulted accounts |

Drivers: collateral type, LTV, seniority, industry, time-in-default.

### EAD / CCF (Credit Conversion Factor)

CCF = (EAD_at_default - drawn_at_observation) / undrawn_at_observation.

Models: regression on the relationship between undrawn commitment
and utilisation at default.

### IFRS 9 staging

| Stage | Criterion | ECL horizon |
|-------|-----------|-------------|
| **Stage 1** | Performing; no significant increase in credit risk | 12-month ECL |
| **Stage 2** | Significant increase in credit risk (SICR); not credit-impaired | Lifetime ECL |
| **Stage 3** | Credit-impaired (default) | Lifetime ECL on net carrying amount |

**SICR triggers** (must be backtested):
- PD increase (e.g. lifetime PD > X% absolute or Yx relative)
- 30 DPD rebuttable presumption
- Watch-list status
- Forbearance / distressed restructuring

**Backtest SICR**: does the Stage 2 bucket predict default? If
not, triggers are wrong.

### Low-default portfolios

LDPs (sovereign, bank, large corporate) have few defaults → can't
fit standard models.

Approaches:
- **Long-term averages**: external data, regulatory benchmarks
- **Bayesian hierarchical**: pooling across segments
- **Shadow rating**: maps to rated entities
- **Through-the-cycle (TTC) PD**: average over business cycle

### Stress testing

| Stress type | Approach |
|-------------|----------|
| **Sensitivity** | Shock one variable (e.g. +200bps unemployment) |
| **Scenario** | Macro scenario (recession, rate shock) |
| **Reverse** | Find macro state that breaches capital threshold |

Tools: top-down vs bottom-up; satellite models for macro-to-PD
link.

### Basileia capital

**Standardised approach**: risk weights by asset class + external
rating.

**Internal Ratings-Based (IRB)**:
- Foundation: bank estimates PD; regulator provides LGD, EAD
- Advanced: bank estimates PD, LGD, EAD

Capital formula:
```
K = LGD × [N((1-R)^(-0.5) × G(PD) + (R/(1-R))^0.5 × G(0.999)) - PD] × MA
```
Where R = correlation, MA = maturity adjustment.

## Examples

### Example 1: retail PD scorecard (IFRS 9)

```
Target: 12-month default (90+ DPD or UTP)
Sample: 100k accounts, origination 2018-2022
Features: bureau data, internal behaviour, application data
Method: WOE binning + logistic regression
Validation:
  - AUC 0.78; Gini 56
  - Brier 0.045; Hosmer-Lemeshow p=0.32
  - PSI 0.07 (stable)
SICR triggers:
  - Lifetime PD increase > 2x absolute or > 5% relative
  - 30 DPD
  - Watch-list
Backtest: Stage 2 accounts default at 4.2x Stage 1 rate
Approval: committee + governance
```

### Example 2: corporate PD (IRB Advanced)

```
Data: 5,000 corporate obligors, 10y history
Method: structural model (Merton KMV) + scorecard overlay
Calibration: PD = 1 - N((log(V/D) + (mu - sigma^2/2)T) / (sigma sqrt(T)))
Validation: traffic-light approach (green/amber/red)
Capital: foundation IRB; bank estimates PD
```

### Example 3: low-default portfolio (sovereign)

```
Data: 30 sovereigns, 30y history
Method: Bayesian hierarchical (partial pooling to global mean)
Prior: weakly informative from external agencies (S&P, Moody's)
Result: TTC PD = 0.03% with credible interval [0.01%, 0.08%]
Use: regulatory capital; stress test overlay
```

## Anti-patterns

### ❌ Training on non-representative sample

Cross-sectional only, no vintage analysis. OOT performance drops;
capital understated.

### ❌ High Gini but bad calibration

Model discriminates but probabilities are wrong. Both matter;
report both.

### ❌ Single train/test split for time series

Default rates are autocorrelated; test must be OOT.

### ❌ SICR without backtest

Staging triggers may not predict default. Always backtest.

### ❌ LDP with naive zero default

Zero-default segments still have PD > 0 (sovereigns default,
banks default). Use Bayesian or long-term averages.

### ❌ Macroeconomic overlays without rationale

Post-hoc overlay to match capital targets is gaming. Use proper
macro models.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Model drift (PSI > 0.25) | Re-fit; investigate drivers; conservative add-on |
| Calibration decay | Refit PD; overlay; document |
| SICR breach | Re-tune triggers; backtest; recalibrate |
| Model underestimates in downturn | Stress test; macro overlay |
| LDP zero-PD assumption | Bayesian; long-term average |
| Override rate rising | Investigate; tighten governance |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/stats` | Underlying hypothesis testing + regression |
| `/bayesian` | LDPs; hierarchical PD models |
| `/causal` | Causal impact of forbearance, modifications |
| `/ml` | ML-based PD models; uplift modeling |
| `/market-risk` | Counterparty credit risk; CVA |