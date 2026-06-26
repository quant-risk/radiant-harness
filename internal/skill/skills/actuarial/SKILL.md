# Skill: actuarial

> Life insurance (mortality, longevity, lapse), P&C (frequency,
> severity, reserving), reinsurance, capital modelling (Solvency II,
> IFRS 17), pricing. Actuarial assumptions are defensible only
> when grounded in portfolio data + explicit uncertainty.

## Decision tree

```
Insurance product / liability
        │
        ▼
[Step 1] Line of business + modelling target
        │
        ▼
[Step 2] Data quality + experience analysis (A/E)
        │
        ▼
[Step 3] Choose assumption set + margin
        │
        ▼
[Step 4] Build model (mortality / frequency / reserving / pricing)
        │
        ▼
[Step 5] Validate (backtest, sensitivity, stress)
        │
        ▼
[Step 6] Regulatory mapping (Solvency II / IFRS 17)
```

## Workflow

### Life insurance

**Mortality table**:
- Base: qx (probability of death at age x)
- Adjustments: age rating, smoker status, socio-economic
- Improvement: secular trend (mortality decreasing over time)

**Models**:
- Lee-Carter: log(qx,t) = ax + bx·kt + ε
- CBD (Cairns-Blake-Dowd): MNI-specific
- APC (Age-Period-Cohort): disentangles age / period / cohort effects

**Longevity risk**: opposite of mortality; increasing life expectancy
threatens annuities/pensions.

**Lapse**: voluntary termination; impacts persistency assumptions.

**Modelling workflow**:
1. Data: policy-by-policy exposure + deaths
2. Fit mortality model to internal data
3. Compare to industry table (industry: SOA, CSO, TMO)
4. Apply improvement trend (M-enhancement or cohort-specific)
5. Set margin (typically 5-15% on rates)

### P&C (Non-life)

**Frequency modelling**:
- Poisson, Negative Binomial
- Zero-inflated models (for low-frequency lines)
- Trend + seasonality

**Severity modelling**:
- Lognormal, Pareto, Gamma, Weibull
- Body: ground-up vs excess
- Tail: heavy-tailed (Pareto) for cat/large loss
- Mixture for attritional + large

**Combined ratio** = loss ratio + expense ratio

**Pricing** = pure premium × loading factor

**Reserving**:

| Method | Use |
|--------|-----|
| **Chain-ladder** | Stable development pattern; needs history |
| **Bornhuetter-Ferguson** | Limited history; prior + development |
| **Cape Cod** | Expected loss ratio; weights by earned |
| **Mack** | Uncertainty around chain-ladder |
| **Bootstrap** | Full posterior on reserves |
| **Stochastic BF** | Parameter uncertainty |

### Solvency II

**SCR (Solvency Capital Requirement)**:

| Module | Description |
|--------|-------------|
| **Market risk** | IR, equity, property, spread, FX |
| **Counterparty default** | Reinsurer / counterparty failure |
| **Life underwriting** | Mortality, longevity, lapse, catastrophe, expense |
| **Health underwriting** | Similar to life + health-specific |
| **Non-life underwriting** | Premium + reserve + catastrophe |
| **Operational risk** | Based on premiums or reserves |
| **Intangible asset risk** | (Limited) |

SCR via standard formula or internal model.

**MCR (Minimum Capital Requirement)**: linear combination of
technical provisions + capital; floor at 25% of SCR.

### IFRS 17

Three measurement models:

| Model | When | Measurement |
|-------|------|-------------|
| **GMM** (General) | Default | Building blocks: FCF + risk adjustment + CSM |
| **PAA** (Premium Allocation) | Short-duration (≤1y) | Liability for remaining coverage = unearned premium |
| **VFA** (Variable Fee Approach) | Contracts with direct participation | CSM absorbs financial risk |

**Contractual Service Margin (CSM)**: profit recognised over coverage
period; locked at initial recognition, adjusted for changes in
estimates.

### Reinsurance

| Type | Description |
|------|-------------|
| **Quota share** | % of each risk ceded |
| **Surplus** | Up to a limit per risk |
| **Stop loss** | Aggregate loss above threshold |
| **Excess of loss (XL)** | Per-occurrence or per-policy |
| **Catastrophe (Cat XL)** | Aggregate from single event |

**Ceded commission**: paid by reinsurer to cedent (covers acquisition).

### Pricing

**Pure premium** = expected loss + LAE + risk margin

**Loading** = acquisition + admin + profit + contingencies

**Rate adequacy**: priced loss ratio + expense ratio < 100% profitable.

### Capital modelling

**Economic capital** = VaR at high confidence (e.g. 99.5% over 1y)

Sources of volatility:
- Insurance risk (mortality / claims)
- Market risk (asset returns)
- Credit risk
- Operational

Internal model vs standard formula trade-off.

## Examples

### Example 1: chain-ladder reserving (motor)

```
Triangle (cumulative paid claims, by origin year × dev year):
           12    24    36    48    60 (months)
2020:    1,000 2,500 4,200 5,500 6,400
2021:    1,100 2,800 4,700 6,100 ?
2022:    1,200 3,100 5,100 ?     ?
2023:    1,300 3,300 ?     ?     ?
2024:    1,400 ?     ?     ?     ?

Development factors (avg):
  12-24: 2.45; 24-36: 1.65; 36-48: 1.30; 48-60: 1.16

Ultimate claims:
2020: 6,400 × 1.00 = 6,400 (developed)
2021: 6,100 × 1.16 = 7,076
2022: 5,100 × 1.30 × 1.16 = 7,691
2023: 3,300 × 1.65 × 1.30 × 1.16 = 8,219
2024: 1,400 × 2.45 × 1.65 × 1.30 × 1.16 = 8,512

Total IBNR = sum of ultimates - paid to date
```

### Example 2: Solvency II SCR (life)

```
Modules (1-in-200, 1y):
  Mortality: 12M
  Longevity: 8M
  Lapse: 5M
  Catastrophe: 3M
  Market: 25M
  Counterparty: 4M
  Operational: 6M
Aggregate (with diversification): 42M (correlation matrix applied)
SCR: 42M
MCR: 18M (linear formula + cap)
```

### Example 3: IFRS 17 GMM

```
Premium received: 1,000
Expected claims: 700
Risk adjustment (cost-of-capital): 50
Expected profit (CSM): 250
Coverage period: 5y

Release pattern:
  Year 1: 50 CSM released
  Year 2: 50 CSM released
  ...
  Year 5: 50 CSM released

P&L each year: claims incurred (140) - risk adjustment release (10)
              - CSM release (50) = -200
```

## Anti-patterns

### ❌ Industry table without fit

Applies to no one. Always calibrate to portfolio.

### ❌ No backtest

Can't validate model accuracy. Run A/E and back-test against actual.

### ❌ Mortality without improvement

Misses generation risk; underestimates longevity exposure.

### ❌ Reserving on paid only

Misses IBNER (incurred but not enough reserved). Use incurred.

### ❌ Pricing without capital cost

Long-tail lines underpriced; portfolio-level loss.

### ❌ IFRS 17 GMM by default

Use PAA for short-duration contracts (simpler).

## Failure modes

| Failure | Recovery |
|---------|----------|
| Mortality table deviates | Re-fit; investigate data quality; update |
| Reserve deficient | Top-up; investigate; pricing review |
| Pricing loss | Re-rate; non-renew loss-making segments |
| Solvency breach | Capital injection; risk reduction |
| IFRS 17 measurement error | Restate; methodology review |
| Reinsurer default | Recovery; diversification; collateral |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/stats` | Underlying statistical inference |
| `/bayesian` | Stochastic reserving; parameter uncertainty |
| `/econometrics` | Time-series mortality trends |
| `/credit-risk` | Reinsurance counterparty risk |
| `/capital-markets` | ALM matching assets + liabilities |
| `/accounting` | IFRS 17 measurement models |

## Tools

| Tool | Purpose |
|------|---------|
| **R (actuar)** | Mortality, reserving, experience analysis |
| **Python (lifelines)** | Survival analysis |
| **Igloo / ResQ** | Stochastic reserving |
| **MG-ALFA / Prophet** | Industry mortality tables |
| **SAS / Emblem** | Insurance analytics |
| **RMS / AIR / CoreLogic** | Catastrophe modelling |

## Citations

- Dickson, Hardy, Waters, "Actuarial Mathematics for Life Contingent Risks"
- Wüthrich, Merz, "Stochastic Claims Reserving in Insurance"
- EU Solvency II Directive 2009/138/EC
- IFRS 17 Insurance Contracts (IASB)