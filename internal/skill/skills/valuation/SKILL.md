# Skill: valuation

> DCF, comparables, precedents, SOTP, LBO, real options. A
> valuation is a range, not a point — sensitivity is the
> deliverable, not the number.

## Decision tree

```
Valuation question
        │
        ▼
[Step 1] Choose methodology (DCF / comps / precedents / SOTP / LBO)
        │
        ▼
[Step 2] Build base case (assumptions, drivers)
        │
        ▼
[Step 3] Compute WACC (DCF) or select multiples (comps/precedents)
        │
        ▼
[Step 4] Calculate base-case value
        │
        ▼
[Step 5] Sensitivity (WACC × growth × margin)
        │
        ▼
[Step 6] Cross-check with alternative methodology
        │
        ▼
[Step 7] Report range + recommendation
```

## Workflow

### DCF (Discounted Cash Flow)

```
Enterprise Value = Σ FCFF_t / (1 + WACC)^t + TV / (1 + WACC)^n

Where:
  FCFF = EBIT × (1 - tax) + D&A - CapEx - ΔWC
  TV   = Terminal value (Gordon growth or exit multiple)
  WACC = Weighted Average Cost of Capital
```

**WACC**:
```
WACC = (E/V) × Re + (D/V) × Rd × (1 - t)
Re   = Rf + β × (Rm - Rf) + size premium (small-cap)
Rd   = cost of debt (effective yield)
t    = marginal tax rate
```

**Terminal value**:
- **Gordon growth**: TV = FCF × (1+g) / (WACC - g); g ≤ long-term GDP growth
- **Exit multiple**: TV = EBITDA × multiple; multiple from comparable companies

**Sensitivity**:
- 2D table: WACC (rows) × Growth (columns); or WACC × Margin
- Tornado chart: which input moves value most

### Comparable companies

| Multiple | When |
|----------|------|
| **EV / Revenue** | Early stage; pre-profit |
| **EV / EBITDA** | Mature; positive EBITDA |
| **P / E** | Profitable; equity value focus |
| **P / B** | Banks, financials |
| **EV / EBIT** | Cross-border; pre-D&A |

Select comparables:
- Same industry (GICS / SIC code)
- Similar size (revenue or market cap within 0.5x-2x)
- Similar growth / margin profile
- Geographic similarity

Calculate median, mean, 25th-75th percentile.

### Precedent transactions

Same as comparables but for M&A:
- EV / Revenue, EV / EBITDA, P / E
- Often higher than trading comps (control premium)
- Premiums: 25-40% typical for public targets

### LBO (Leveraged Buyout)

```
Returns = (Exit EBITDA × Exit multiple - Net debt) / Equity invested
```

Key inputs:
- Entry multiple (5-8x EBITDA typical)
- Debt / EBITDA leverage (5-7x for typical LBO)
- Cost of debt + interest coverage
- Hold period (3-5 years)
- Exit multiple (assumed flat or +1x)

Returns to PE fund: IRR + MOIC.

### SOTP (Sum-of-the-parts)

Value each segment separately, sum, less overhead.

Useful when segments have different multiples (e.g. conglomerate
with high-growth tech + mature manufacturing).

### Real options

For projects with optionality:
- **Defer**: option to wait for more info
- **Expand**: option to scale up if successful
- **Abandon**: option to exit if unsuccessful

Valued via Black-Scholes for options on non-traded assets; or
decision tree analysis.

### Impairment testing (IAS 36)

Required annually for goodwill + intangibles with indefinite life.

```
Recoverable amount = max(Fair value less costs to sell, Value in use)

If carrying amount > recoverable amount → impairment loss
```

**Value in use**:
- Cash flow projections (5y typical; longer if justified)
- Pre-tax discount rate (entity-specific WACC, grossed up for tax)
- Terminal growth (long-term GDP rate)
- Conservative assumptions (auditable)

If recoverable > carrying: no impairment; document conclusion.

## Examples

### Example 1: DCF (SaaS company)

```
Forecast (5y):
  Revenue: $10M → $25M (CAGR 25%)
  EBITDA margin: 10% → 30%
  FCFF: $1M, $3M, $5M, $8M, $11M
WACC: 12% (Rf 4% + β 1.2 × ERP 6% + size premium 1%)
Terminal: g=3%, TV = $11M × 1.03 / (0.12 - 0.03) = $126M
PV of explicit: $19M
PV of TV: $71M
Enterprise Value: $90M
Net debt: $5M
Equity value: $85M
```

### Example 2: comparables (e-commerce)

```
Comparables: 8 listed e-commerce companies
EV/Revenue: 2.1x median (range 1.4x - 4.5x)
EV/EBITDA: 18x median (range 12x - 28x)

Target metrics:
  Revenue: $200M
  EBITDA: $20M
Implied EV:
  Revenue: $200M × 2.1 = $420M
  EBITDA: $20M × 18 = $360M
Range: $360M - $420M
```

### Example 3: impairment test (cash-generating unit)

```
CGU: Retail banking division
Carrying amount: $500M
Forecast cash flows (5y): $60M, $65M, $70M, $75M, $80M
Pre-tax discount rate: 14%
Terminal growth: 2%
Value in use: $480M
Impairment loss: $20M ($500M - $480M)
```

## Anti-patterns

### ❌ Single point estimate

No sensitivity range; not robust. Always report range.

### ❌ Hockey-stick forecast

Unsustainable growth (e.g. terminal g=10%); discount to reality.

### ❌ WACC from unadjusted CAPM

Industry premium / size premium ignored; cost of capital wrong.

### ❌ Comps without adjustment

Apples to oranges; multiple not comparable.

### ❌ DCF post-tax WACC + pre-tax cash flows

Mismatch. Pre-tax discount rate + pre-tax cash flows (or vice
versa, consistently).

### ❌ Confusing equity value vs enterprise value

EV = Equity + Debt - Cash. Net debt at valuation date.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Impairment missed | Catch up; document; restate |
| Forecast hockey-stick | Recalibrate to historical growth; sustainable |
| Comparable multiple off | Select better comps; adjust |
| WACC wrong | Recompute; document components |
| Acquisition overpays | Bid discipline; walk away |
| Sensitivity range too tight | Wider inputs; downside scenarios |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/finance` | WACC components; capital structure |
| `/capital-markets` | Trading multiples; transaction precedents |
| `/accounting` | Fair value; impairment accounting |
| `/controlling` | Forecast cash flows tied to plan |