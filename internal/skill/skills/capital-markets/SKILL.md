# Skill: capital-markets

> Derivatives pricing, fixed income, credit markets, structured
> products, equity factor models. A price is a model + market
> data + calibration; document all three.

## Decision tree

```
Instrument to price / analyse
        │
        ▼
[Step 1] Identify asset class (equity / rates / FX / credit)
        │
        ▼
[Step 2] Choose model (BS / binomial / MC / Heston / SABR / ...)
        │
        ▼
[Step 3] Source market data (spot, vol surface, yield curve)
        │
        ▼
[Step 4] Calibrate model parameters
        │
        ▼
[Step 5] Price + Greeks
        │
        ▼
[Step 6] Validate vs liquid benchmark
        │
        ▼
[Step 7] Model risk disclosure (alternative models; range)
```

## Workflow

### Derivatives pricing — Options

**Black-Scholes** (vanilla European):
```
C = S·N(d1) - K·e^(-rT)·N(d2)
P = K·e^(-rT)·N(-d2) - S·N(-d1)

d1 = [ln(S/K) + (r + σ²/2)T] / (σ√T)
d2 = d1 - σ√T
```

Assumptions: log-normal underlying; constant vol; European exercise.

**Greeks**:
- Delta (Δ): ∂V/∂S — sensitivity to underlying
- Gamma (Γ): ∂²V/∂S² — convexity
- Vega (ν): ∂V/∂σ — sensitivity to vol
- Theta (Θ): ∂V/∂t — time decay
- Rho (ρ): ∂V/∂r — sensitivity to rates

**Extensions**:
- **Heston**: stochastic volatility; mean-reverting vol
- **SABR**: stochastic vol with explicit smile
- **Local volatility**: Dupire; fits full smile
- **Jump-diffusion**: Merton; for crashes / events

### Pricing — American / Bermudan

- **Binomial tree**: discrete-time; American exercise
- **Trinomial tree**: smoother convergence
- **Longstaff-Schwartz**: Monte Carlo + regression for American
- **Crank-Nicolson**: PDE method

### Fixed income analytics

**Yield to maturity (YTM)**: solves PV(cash flows) = price.

**Duration**:
- Macaulay: weighted avg time to cash flow
- Modified: % price change for 1bp yield change
- Effective: includes convexity
- Key rate: sensitivity at specific maturities

**Convexity**: 2nd-order price-yield sensitivity; positive for
option-free bonds.

**OAS (Option-Adjusted Spread)**:
- Strip option value from yield
- Compare to benchmark curve
- Works for callable, MBS, structured products

**Spread measures**:
- G-spread: yield - govt
- Z-spread: constant spread over treasury curve
- I-spread: over swap curve
- Asset swap spread: over swap rate

### Credit markets

**CDS (Credit Default Swap)**:
- Premium leg: quarterly fixed payments
- Default leg: (1 - recovery) at default
- Par spread: solves PV(premium) = PV(default)

**CDO (Collateralised Debt Obligation)**:
- Tranches: equity / mezz / senior
- Waterfall: losses absorbed by equity first
- Correlation drives tranche pricing

**CLO**: CDO backed by leveraged loans.

### Structured products

| Product | Components |
|---------|-----------|
| **Mortgage-backed** | Mortgage pool + prepayment model + cash flow waterfall |
| **Auto ABS** | Auto loan pool + loss curve + waterfall |
| **CLO** | Loan portfolio + waterfall + triggers |
| **Synthetic CDO** | CDS portfolio + waterfall |
| **Reverse convertible** | Bond + short put |
| **Autocallable** | Bond + call option |

### Equity factor models

**CAPM**: E[R] = Rf + β × (E[Rm] - Rf)

**Multi-factor**:
- Fama-French 3: market, size (SMB), value (HML)
- Carhart 4: + momentum (UMD)
- Fama-French 5: + profitability (RMW), investment (CMA)
- q-factor (Hou-Xue-Zhang): investment, ROE
- AQR: value, momentum, quality, low-risk

**Risk decomposition**:
- Factor exposures (betas)
- Idiosyncratic (specific) risk
- Total = systematic + specific

### Portfolio construction

| Method | Approach |
|--------|----------|
| **Mean-variance (Markowitz)** | Maximise return for variance; efficient frontier |
| **Risk parity** | Equal risk contribution per asset |
| **Black-Litterman** | Combine market views with equilibrium prior |
| **Min variance** | Minimise variance (no expected returns) |
| **Max Sharpe** | Maximise Sharpe ratio |
| **Equal weight** | 1/N; surprisingly hard to beat |

## Examples

### Example 1: option pricing (BS)

```
S = 100, K = 105, r = 5%, σ = 20%, T = 0.5y
d1 = [ln(100/105) + (0.05 + 0.04/2) × 0.5] / (0.2 × √0.5)
   = [-0.0488 + 0.0225] / 0.1414 = -0.186
d2 = -0.186 - 0.141 = -0.327
N(d1) = 0.426; N(d2) = 0.372
C = 100 × 0.426 - 105 × e^(-0.025) × 0.372
  = 42.6 - 37.7 = 4.9
```

### Example 2: bond analytics

```
Bond: 5y, 5% coupon, semi-annual, $100 face
YTM: 4.5%
Macaulay duration: 4.49y
Modified duration: 4.49 / (1 + 0.0225) = 4.39y
Price: $102.4
1bp yield rise → price drops 0.0439% = $0.045
Convexity: 22.4
Approximation: ΔP/P = -D × Δy + ½ × C × (Δy)²
```

### Example 3: CDS pricing

```
5y CDS, recovery 40%, hazard rate 200 bps
Premium leg PV: S × (1 - e^(-hT))/h × annuity factor
Default leg PV: (1 - R) × (1 - e^(-hT))/h
Par spread: (1 - R) × h = 0.6 × 0.02 = 120 bps
```

## Anti-patterns

### ❌ Pricing without model validation

No benchmark check; trust the number blindly. Always validate.

### ❌ Black-Scholes for everything

Wrong for many underlyings (skew, jumps, vol-of-vol).

### ❌ Stale market data

Pricing on outdated vol surface; freshness matters.

### ❌ Ignoring model risk

Single-model output; no range. Disclose alternative models.

### ❌ Vol from single point

Vol surface matters (smile, skew); not flat σ.

### ❌ Wrong rate in DCF

Risk-free vs swap vs Treasury — pick right one for currency.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Model price off market | Recalibrate; check data; alternative model |
| Greeks unstable | Numerical noise; bump size matters |
| Yield curve mismatch | Use consistent curve (overnight, OIS, swap) |
| Spread widens | Credit deterioration or liquidity; investigate |
| Structured product losses | Model risk; correlation breakdown; mark model |
| Factor model off | Missing factor; time-varying betas |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/finance` | Capital structure; WACC components |
| `/market-risk` | VaR; stress testing |
| `/credit-risk` | Counterparty risk; credit exposure |
| `/valuation` | Multiples; DCF discount rate |
| `/bayesian` | Bayesian estimation of vol / correlation |