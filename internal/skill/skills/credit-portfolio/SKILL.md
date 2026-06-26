# Skill: credit-portfolio

> Portfolio credit risk: concentration, migration, RWA,
> stress, vintage, limits. Single-name risk is necessary;
> portfolio risk is the discipline.

## Decision tree

```
Portfolio position
        │
        ▼
[Step 1] Concentration analysis (single-name / sector / geo)
        │
        ▼
[Step 2] Migration + transition matrices
        │
        ▼
[Step 3] Aggregation: portfolio PD / LGD / EAD / EL / UL
        │
        ▼
[Step 4] RWA calculation (standardised or IRB)
        │
        ▼
[Step 5] Stress test (sector shocks + correlation)
        │
        ▼
[Step 6] Limit management (single-name + portfolio)
        │
        ▼
[Step 7] Vintage / cohort analysis (back-test PDs)
```

## Workflow

### Concentration analysis

| Metric | Formula | Threshold |
|--------|---------|-----------|
| **Single-name** | max exposure / capital | ≤ 10% |
| **Top-10** | sum of top 10 / total | varies by portfolio |
| **HHI** | Σ (share_i)² | lower = more diversified |
| **Sector** | sector exposure / total | per sector limit |
| **Geographic** | geo exposure / total | per country limit |

**Granularity**: rating, sector, geography, maturity, product.

### Credit migration matrix

```
        BBB    BB    B    CCC    Default
BBB   [0.95  0.04  0.01  0.00  0.00]
BB    [0.10  0.80  0.08  0.01  0.01]
B     [0.02  0.10  0.75  0.08  0.05]
CCC   [0.01  0.05  0.10  0.60  0.24]
Default [0    0     0    0    1.00]
```

Annual transition probabilities; multi-year = matrix multiplication.

**Calibration**: typically from external ratings (S&P, Moody's) +
internal data overlay.

### Portfolio aggregation

**Expected Loss (EL)** = Σ EAD × PD × LGD

**Unexpected Loss (UL)** = portfolio VaR at 99.9% over 1y
- Assumes single-factor Gaussian copula (industry standard)
- Asset correlation ρ ≈ 12-24% (regulatory formula)
- UL ≈ √(Σ Σ ρ_ij × UL_i × UL_j)

**Diversification benefit**: UL < Σ UL_i; ratio is diversification
ratio.

### RWA calculation

**Standardised approach**: risk weights by asset class + external
rating.

**IRB**: RWA = K × 12.5 × EAD, where K = capital formula.

For portfolio: aggregate using correlations.

### Stress testing

Scenarios:
- Sector downturn: PD +50% in specific sector
- Country downturn: corporate PDs +100% in country X
- Systematic: all PDS +30%, LGD +10%

Correlations matter:
- Single-factor model: all assets correlated via systematic factor
- Multi-factor: sector + geo + idiosyncratic

Without correlation: underestimates tail risk.

### Vintage analysis

Cohort PD: default rate by origination cohort. Test if:
- New vintages performing worse than older
- Through-cycle vs point-in-time alignment

Plot: cohort PD over time (cure-adjusted, ragged).

### Limit management

| Limit type | Typical level |
|-----------|---------------|
| Single-name | 10% of capital |
| Sector | 25% of capital |
| Country | 15% of capital |
| Product | 30% of capital |

Breaches escalated to credit committee.

### Securitisation / portfolio sale

When concentration risk is excessive:
- True sale of loans (removes from balance sheet)
- Synthetic securitisation (risk transfer only)
- CLO / ABS issuance
- Credit derivatives (CDS, tranched)

### Credit VaR

Monte Carlo:
1. Simulate correlated defaults (asset correlations)
2. For each default, draw LGD (loss given default)
3. Aggregate loss distribution
4. 99.9% VaR = tail quantile

## Examples

### Example 1: concentration report

```
Portfolio: corporate loans, $5B total
Top 10 names: 35% ($1.75B)
Top single name: 8% ($400M)  [limit: 10%, OK]
HHI: 0.04 (diversified)
Sector breakdown:
  - Manufacturing: 25%
  - Real estate: 20%
  - Retail: 15%
  - Tech: 15%
  - Energy: 10%
  - Other: 15%
Geographic:
  - US: 60%
  - EU: 25%
  - Asia: 10%
  - Other: 5%
Breaches: none
```

### Example 2: portfolio migration

```
Portfolio: BBB-rated corporate book
Annual transition: 4% downgrade to BB; 1% upgrade to A
PD implied: 4% (annual)
Cumulative 5y default prob: ~15%
Stress: 2x downgrade rate → 30% cumulative
RWA impact: ~30% increase
```

### Example 3: portfolio stress

```
Baseline: EL = 50M
Stress (sector downturn):
  - Manufacturing PD: 5% → 15% (3x)
  - LGD unchanged
EL stressed = 130M
UL stressed = 800M (vs 400M baseline)
Capital impact: ~25% increase
Management actions: reduce manufacturing exposure
```

## Anti-patterns

### ❌ Single-name concentration > 10% capital

No early warning. Set + monitor.

### ❌ Migration matrix without seasoning

Biased estimates. Use seasoned cohorts.

### ❌ Stress test without correlation

Assumes independence; underestimates tail.

### ❌ RWA from PD without LGD/EAD

Underestimates loss. Model all three.

### ❌ No vintage analysis

Can't detect deterioration in new vintages.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Concentration breach | Sell-down; committee review |
| Migration acceleration | Re-rate; recalibrate PD |
| Stress loss exceeds buffer | Reduce exposure; capital action |
| RWA variance vs accounting | Investigate; reconcile |
| Vintage deterioration | Tighten underwriting |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/credit-risk` | Single-name PD/LGD/EAD models |
| `/market-risk` | Credit spread risk |
| `/stress-test` | Portfolio + market stress |
| `/model-risk` | Migration matrix validation |
| `/actuarial` | Reserving for credit portfolios |

## Citations

- BCBS CRE (Credit Risk) Standards
- Moody's / S&P default studies
- BCBS Stress Testing Principles
- Internal Ratings-Based (IRB) formula documents