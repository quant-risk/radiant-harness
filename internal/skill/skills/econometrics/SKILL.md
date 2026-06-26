# Skill: econometrics

> Time series, panel data, instrumental variables, structural
> models. OLS on endogenous regressors is the original sin.

## Decision tree

```
Data + research question
        │
        ▼
[Step 1] Data structure (cross-section / time-series / panel)
        │
        ▼
[Step 2] Endogeneity check
        │
        ├── none     -> OLS with HAC SEs
        ├── suspected -> Durbin-Wu-Hausman; IV candidates
        └── definite -> IV / 2SLS / GMM / control function
        │
        ▼
[Step 3] Time-series diagnostics (if applicable)
        │   - stationarity (ADF/KPSS)
        │   - autocorrelation (ACF/PACF, Ljung-Box)
        │   - structural breaks (Chow, CUSUM)
        │
        ▼
[Step 4] Estimate + diagnostics
        │
        ▼
[Step 5] Robustness: alternative specs; placebo; out-of-sample
```

## Workflow

### Time series

**ARIMA(p,d,q)**:
- d: differencing order (from ADF test)
- p: AR order (from PACF)
- q: MA order (from ACF)

**Selection criteria**: AIC, BIC; out-of-sample RMSE.

**Diagnostics**:
- Residual autocorrelation: Ljung-Box
- Normality: Jarque-Bera
- Heteroscedasticity: ARCH-LM
- Stability: CUSUM, CUSUMSQ

**Vector autoregression (VAR)** for multivariate time series:
- Lag selection: AIC, HQ, SC
- Granger causality between variables
- Impulse response functions (IRFs)
- Variance decomposition

**VECM** for cointegrated series:
- Johansen test for rank of cointegration
- Long-run + short-run coefficients
- Speed of adjustment (α)

**GARCH(p,q)** for volatility modelling:
- GARCH(1,1) as default
- Asymmetric: GJR-GARCH, EGARCH for leverage effect
- t-distribution residuals for fat tails

### Panel data

**Fixed effects (FE)** vs **Random effects (RE)**:
- Hausman test: if reject RE → use FE
- FE controls for time-invariant unobservables
- RE more efficient if no correlation with regressors

**Clustered standard errors** for within-group correlation.

**Dynamic panel** (lagged dependent variable):
- Arellano-Bond (GMM)
- System GMM (Blundell-Bond)

### Instrumental variables

When regressor X is endogenous (correlated with error):

1. Find instrument Z: correlated with X, uncorrelated with error
2. First stage: X on Z (F-stat > 10, Staiger-Stock rule)
3. Second stage: Y on predicted X
4. Diagnostics:
   - First-stage F (weak instrument test)
   - Sargan / Hansen J-test (overidentification)
   - Durbin-Wu-Hausman (endogeneity test)

Common IV candidates:
- Distance to college (Angrist-Krueger)
- Rainfall (agricultural econ)
- Bartik instruments (shift-share)
- Policy timing (Card)

### Structural breaks

- **Chow test**: known break point
- **Bai-Perron**: multiple unknown breaks
- **CUSUM / CUSUMSQ**: parameter stability over time
- **Markov-switching**: regime changes

### Common pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Spurious regression | High R², low Durbin-Watson | Difference / cointegration test |
| Endogeneity | Biased OLS | IV / GMM / control function |
| Autocorrelation | Invalid SEs | HAC SEs; GLS |
| Heteroscedasticity | Invalid SEs | Robust SEs / WLS / FGLS |
| Multicollinearity | Inflated SEs | Drop / combine variables |
| Structural break | Coefficient instability | Break test; subsample; regime model |

## Examples

### Example 1: macro forecasting (ARIMA)

```
Series: monthly CPI inflation, 2010-2024
Test:   ADF -> unit root (d=1)
Model:  ARIMA(1,1,1) by AIC
Diagnostics:
  - Ljung-Box p=0.42 (no autocorrelation)
  - Jarque-Bera p=0.18 (normality OK)
  - ARCH-LM p=0.31 (no ARCH)
Forecast: 12 months ahead; RMSE=0.34
Robustness: ARIMA(0,1,1) gives similar RMSE
```

### Example 2: demand estimation (IV)

```
Question: price elasticity of cigarette demand
Data:     state-level panel, 2000-2020
Endogeneity: price correlated with demand shocks
IV:        state-level sales tax (correlated with price,
           uncorrelated with demand shocks)
First stage F=24.7 (strong IV)
2SLS: elasticity=-0.45 (SE=0.06)
Hausman: p<0.01 (endogeneity confirmed)
Sargan:  p=0.42 (instrument valid)
```

### Example 3: policy evaluation (panel FE)

```
Question: effect of minimum wage on employment
Data:     state-year panel, 1980-2020
Model:    FE(state) + FE(year) + clustered SE(state)
Estimate: -0.05 (5% employment drop per 10% MW increase)
Robustness:
  - Subsample pre/post 2000
  - Placebo: effect on non-tradable sector (smaller)
  - Out-of-sample: predicts 2018-2020 within SE
```

## Anti-patterns

### ❌ OLS on endogenous regressor

Bias can be huge. Use IV / 2SLS / GMM with explicit identification.

### ❌ Spurious regression

Regressing two random walks gives high R². Test stationarity;
difference or model cointegration.

### ❌ Standard errors that ignore autocorrelation

OLS SEs are wrong under serial correlation. Use HAC (Newey-West)
or panel-corrected SEs.

### ❌ Comparing nested models by R² alone

R² always increases with more variables. Use AIC/BIC or
out-of-sample RMSE.

### ❌ Ignoring structural breaks

A coefficient estimated over a sample with a break is a weighted
average of two regimes. Test for breaks.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Weak instrument (F < 10) | Find stronger IV; consider LIML |
| Over-rejection of IV validity | Look for heterogeneous effects; LATE interpretation |
| Unit root | Difference; model cointegration; ARIMA in differences |
| Structural break | Subsample analysis; Markov-switching model |
| Endogeneity not addressed | Bias persists; all inference invalid |
| Out-of-sample failure | Re-specify; simpler model; reg; CV |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/stats` | Underlying hypothesis testing discipline |
| `/causal` | Beyond OLS: identification strategies |
| `/bayesian` | Bayesian time series (state-space, BSTS) |
| `/ml` | ML for prediction vs econ for causal inference |
| `/credit-risk` | Panel survival models; credit scoring |