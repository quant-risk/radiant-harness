# Skill: market-risk

> VaR, Expected Shortfall, stress testing, FRTB capital. VaR
> without backtest is just a number.

## Decision tree

```
Trading book + risk factors
        │
        ▼
[Step 1] Choose VaR method (historical / parametric / Monte Carlo)
        │
        ▼
[Step 2] Define risk factors (IR / FX / EQ / CO / CS)
        │
        ▼
[Step 3] Estimate volatilities + correlations
        │
        ▼
[Step 4] Compute VaR (1-day, 99%) + ES (97.5%)
        │
        ▼
[Step 5] Backtest (Kupiec POF, Christoffersen, traffic light)
        │
        ▼
[Step 6] Stress test (historical + hypothetical + reverse)
        │
        ▼
[Step 7] Capital (FRTB IMA or SA)
```

## Workflow

### VaR methods

| Method | Formula | Strengths | Weaknesses |
|--------|---------|-----------|------------|
| **Historical** | Quantile of P&L distribution | No distribution assumption; captures fat tails | Window choice; no extrapolation; heavy in computation |
| **Parametric (variance-covariance)** | z·σ·√t · portfolio | Fast; analytical | Assumes normality (underestimates tails) |
| **Monte Carlo** | Simulate many paths; take quantile | Handles non-linearity; flexible | Slow; model risk |

**EWMA (Exponentially Weighted Moving Average)** for adaptive
volatility: σ²_t = (1-λ)·r²_{t-1} + λ·σ²_{t-1}, λ ≈ 0.94 daily.

### Expected Shortfall

ES = E[L | L > VaR]. Coherent risk measure (unlike VaR).

FRTB default: ES at 97.5% confidence.

### Backtesting

**Kupiec POF (Proportion of Failures)**:
- H0: exception rate = 1 - confidence level (e.g. 1% for 99% VaR)
- LR_POF = -2·ln(p^x·(1-p)^(T-x) / (π^x·(1-π)^(T-x)))
- χ²(1) test; reject if exception rate too far from expected

**Christoffersen independence**:
- H0: exceptions independent (no clustering)
- LR_ind = -2·ln(π₀^x₀·(1-π₀)^(x₁)·π₁^x₁·(1-π₁)^(x₀)) + ...
- χ²(1) test

**Traffic light (Basel)**:
- Green: 0-4 exceptions (99% VaR, 250 days)
- Yellow: 5-9 exceptions → capital multiplier x1.4 to x1.5
- Red: 10+ exceptions → x2.0 or revert to SA

### Stress testing

| Type | When |
|------|------|
| **Historical** | Replay 2008, 2020, 2022 scenarios |
| **Hypothetical** | Parallel shifts, curve twists, FX shocks |
| **Reverse** | Find scenario that breaches capital |

Regulator-defined scenarios: BCBS prescribed shocks (rates, FX,
equity, commodity).

### FRTB capital

**Internal Models Approach (IMA)**:
- VaR (99%, 1-day) → capital = max(VaR_t-1, m·σ_VaR)
- SVaR (stressed) — same model, stressed period
- IRC (incremental risk charge) — default + migration
- DRC (default risk charge) — Jump-to-default

**Standardised Approach (SA)**:
- Risk weights per asset class + bucket
- Correlation formulas (Basel-prescribed)
- Delta + Vega + Curvature risks

### Risk factors

| Factor | Typical model |
|--------|--------------|
| Interest rate (IR) | PCA of yield curve; parallel shift + twist + butterfly |
| FX | Random walk + stochastic vol |
| Equity | Geometric Brownian motion; sector factors |
| Commodity | Mean-reverting (Ornstein-Uhlenbeck) |
| Credit spread (CS) | Reduced-form (intensity model) |

### Common pitfalls

| Pitfall | Fix |
|---------|-----|
| Parametric VaR underestimates tails | Use t-distribution or filtered historical |
| Too-short historical window | Use 3-5 years or stress period |
| Correlation breakdown in stress | Use stressed correlations |
| Procyclicality | Through-the-cycle adjustments |
| Missing risk factors | Add to model; capital add-on if not material |

## Examples

### Example 1: parametric VaR (rates + FX)

```
Portfolio: 100M USD; 50M EUR-denominated bonds
Risk factors: USD/EUR FX; EUR yield curve
Volatilities: σ_FX = 8% / yr; σ_rate = 1% / yr
Correlation: ρ = -0.3
VaR_1d,99% = z · σ_p · V = 2.33 · sqrt(...) · 100M = ~1.2M USD
```

### Example 2: backtest (250 days, 99% VaR)

```
Exceptions: 4 (expected: 2.5)
Kupiec POF: LR = 0.82; p = 0.36 (fail to reject H0)
Christoffersen: LR = 0.41; p = 0.52 (independent)
Traffic light: GREEN
```

### Example 3: stressed VaR (2008)

```
Stress period: 2008-01-01 to 2009-06-30
Same model, calibrated on stress period
SVaR_99%,1d = 4.2x current VaR
Capital: max(VaR, k · SVaR); k = 0.5 in some jurisdictions
```

## Anti-patterns

### ❌ VaR without backtest

VaR is a forecast; without backtest you don't know if it's accurate.

### ❌ Same-day VaR only

Overnight + intraday gaps matter. Use full revaluation.

### ❌ Parametric VaR with fat tails

Underestimates tail risk. Use t-distribution or filtered
historical.

### ❌ Historical VaR with too-short window

Misses regime changes. Use 3-5 years minimum.

### ❌ Stress scenarios without rationale

Arbitrary shocks not anchored to economic scenarios. Document
methodology.

### ❌ Same correlation in stress as in normal

Correlations go to 1 in stress. Use stressed correlations.

## Failure modes

| Failure | Recovery |
|---------|----------|
| VaR exception rate too high | Capital multiplier; refine model |
| Stress loss > capital | Reduce positions; capital add-on |
| Model risk identified | Conservative overlay; multiple models |
| Procyclical capital | Through-the-cycle adjustments |
| Liquidity horizon wrong | Longer horizon for illiquid positions |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/econometrics` | Volatility models (GARCH); time series |
| `/stats` | Backtest statistics |
| `/bayesian` | Bayesian VaR; posterior over losses |
| `/credit-risk` | Counterparty credit risk; CVA |
| `/liquidity-risk` | Funding liquidity; cross-risk effects |