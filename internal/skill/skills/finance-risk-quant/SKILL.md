# Skill: finance-risk-quant

## Overview

Finance & Risk covers the full spectrum of quantitative finance: credit risk (PD/LGD/EAD), market risk (VaR/ES/Greeks), portfolio optimization, backtesting, stress testing, AML/KYC, fraud detection, actuarial science, accounting, tax, valuation, and capital markets. This skill merges domain knowledge from 16 specialized areas with implementation patterns from QuantLib, backtrader, vectorbt, riskfolio-lib, zipline, pyfolio, and empyrical.

**When to use**: Building risk models, pricing engines, trading systems, regulatory reporting, or any quantitative finance workflow.


> Comprehensive finance, risk management, and quantitative analysis. Covers
> credit risk (PD/LGD/EAD, IFRS 9, Basel), market risk (VaR, ES, Greeks),
> portfolio optimization, backtesting frameworks, risk metrics, trading
> strategies, AML/KYC, fraud detection, stress testing, actuarial, accounting,
> tax, valuation, and capital markets. Integrates code from QuantLib, backtrader,
> vectorbt, riskfolio-lib, pyfolio, empyrical, yfinance.

## Decision Tree

```
Finance/Risk/Quant question
        │
        ▼
[Step 1] Identify domain
        ├── Credit Risk → PD/LGD/EAD/ECL models
        ├── Market Risk → VaR/ES/Greeks/stress
        ├── Portfolio → Optimization/backtesting
        ├── Trading → Strategy development/simulation
        ├── Compliance → AML/KYC/fraud
        ├── Actuarial → Insurance/reserving
        ├── Accounting → IFRS/GAAP/consolidation
        ├── Tax → Corporate/transfer pricing/BEPS
        ├── Valuation → DCF/comps/LBO
        └── Capital Markets → Derivatives/fixed income
        │
        ▼
[Step 2] Select framework + toolchain
        │
        ▼
[Step 3] Data acquisition (yfinance, market data, internal)
        │
        ▼
[Step 4] Model/analysis construction
        │
        ▼
[Step 5] Validation (backtesting, stress testing, model risk)
        │
        ▼
[Step 6] Reporting (metrics, tear sheets, governance)
```

---

## Repo Architecture Reference

### 1. QuantLib (C++ — Pricing Engines, Instruments, Term Structures)

**Architecture**: Engine-based pricing pattern. `Instrument` owns a pointer to a
`PricingEngine`. When `instrument.NPV()` is called, it delegates to the engine's
`calculate()`. Term structures (`YieldTermStructure`, `BlackVolTermStructure`)
are observable — changing the evaluation date triggers recalculation chain.

**Key Abstractions**:
- `Instrument` → base class for Bond, Option, Swap, CDS
- `PricingEngine` → AnalyticEuropeanEngine, MCEuropeanEngine, FDHestonVanillaEngine,
  MCEuropeanHestonEngine, JumpDiffusionEngine, QDPlusAmericanEngine
- `YieldTermStructure` → discount(), zeroRate(), forwardRate(); pure virtual `discountImpl(Time)`
- `OneFactorModel` → ShortRateDynamics + ShortRateTree (trinomial); affine: A(t,T)·exp(-B(t,T)·r)
- `BlackScholesProcess` → combines underlying spot, vol term structure, risk-free rate
- `DayCounter` → Actual360, Actual365Fixed, Thirty360, Business252, ActualActual

**Pipeline Flow**:
```
Market Data → Quote/Handle → TermStructure → StochasticProcess → PricingEngine → Instrument.NPV()
```

**Hidden Gems**:
- `Handle<Quote>` pattern: automatic recalculation when market data changes
- Trinomial tree fitting to term structure (Hull-White, BDT, CIR)
- Jump-diffusion (Merton), Heston stochastic vol, SABR engines
- Bond: cleanPrice/dirtyPrice/yield/Z-spread/OAS all cross-validated internally
- `OneFactorAffineModel`: P(t,T,r) = A(t,T)·exp(-B(t,T)·r) analytical bond pricing

**Anti-Patterns**:
- ❌ Using Black-Scholes for everything (ignores skew, jumps, vol-of-vol)
- ❌ Stale term structure — always update evaluation date
- ❌ Mixing day counters in yield curve construction
- ❌ Not setting the pricing engine before calling NPV()

### 2. backtrader (Python — Strategy Backtesting)

**Architecture**: Event-driven line-based system. `Cerebro` is the orchestrator; feeds data →
strategy → broker → observers/analyzers. Everything is a `LineIterator` that processes bars
sequentially. Metaclass-driven registration of indicators and strategies.

**Key Abstractions**:
- `Strategy` → `__init__()` (declare indicators), `next()` (per bar logic),
  `notify_order()`, `notify_trade()`
- `BackBroker` → order matching with slippage (slip_perc, slip_fixed), commission models
- `Cerebro` → `addstrategy()`, `adddata()`, `run()`, `plot()`
- `DataFeed` → CSV, Yahoo Finance, Pandas DataFrame, IB, Oanda, VC
- Analyzers: Sharpe, DrawDown, TradeAnalyzer, SQN, VWR, Calmar, PyFolio bridge
- `LineSeries` → all data, indicators, strategies share the same line interface

**Pipeline Flow**:
```
DataFeed → Cerebro.run() → Strategy.next() [per bar] → Order → Broker → Position/Trade → Analyzers
```

**Hidden Gems**:
- `qbuffer(savemem=1)` for memory-efficient backtesting of large datasets
- Observer pattern for real-time monitoring (buysell, drawdown, benchmark overlay)
- Store abstraction for live trading (Interactive Brokers, Oanda, VisualChart)
- `Sizer` for position sizing strategies (fixed, percent, Kelly)
- Signal-based strategy alternative to `next()` for simpler logic

**Anti-Patterns**:
- ❌ Using future data in `__init__()` (look-ahead bias)
- ❌ Not setting commission scheme (unrealistic results)
- ❌ Ignoring slippage in high-frequency or illiquid strategies
- ❌ Single data feed when cross-asset strategies need multiple

### 3. vectorbt (Python — Vectorized Portfolio Simulation)

**Architecture**: Fully vectorized using Numba JIT compilation. `Portfolio` is the central class.
Three simulation modes: `from_orders` (fastest, predefined), `from_signals` (adds entry/exit
automation + stop loss/take profit), `from_order_func` (most flexible, custom Numba callbacks).

**Key Abstractions**:
- `Portfolio` → from_orders(), from_signals(), from_order_func()
- `Records` → orders, trades, positions, drawdowns stored as structured record arrays
- `ReturnsAccessor` → `.vbt.returns` on any pd.Series for instant risk metrics
- `SignalFactory` → generate entry/exit signals from indicator crossovers
- Enums: SizeType, Direction, OrderSide, OrderStatus

**Pipeline Flow**:
```
Signals/Orders → broadcast(shape) → Numba simulate_nb(row-by-row) → Order records → Portfolio(metrics)
```

**Hidden Gems**:
- `from_order_func` with `@njit` callbacks for event-driven simulation in Numba
- Broadcasting: pass 1D/2D/scalar for any parameter; vectorbt auto-broadcasts
- `flex_simulate_nb` for multiple orders per symbol per bar
- Built-in TA-Lib integration via `vbt.talib`
- Records architecture: mapped arrays with column mapping for multi-asset analysis

**Anti-Patterns**:
- ❌ Using `from_order_func` when `from_signals` suffices (unnecessary complexity)
- ❌ Not using Numba decorators (100x slower)
- ❌ Ignoring the broadcasting system and looping manually

### 4. riskfolio-lib (Python — Portfolio Optimization)

**Architecture**: CVXPY-based convex optimization. `Portfolio` class for mean-variance and
advanced optimization. `HCPortfolio` for hierarchical clustering-based methods (HRP, HERC, NCO).
Supports 22+ risk measures, network/cluster constraints via SDP and integer programming.

**Key Abstractions**:
- `Portfolio` → `optimization()`, `efficient_frontier()`, `risk_parity()`,
  `black_litterman_optimization()`, `constraints_builder()`
- `HCPortfolio` → hierarchical risk parity, hierarchical ERC, nested clustered optimization
- RiskFunctions → standalone: MAD(), CVaR_Hist(), VaR_Hist(), Sharpe_Risk(), Risk_Contribution()
- Constraints: linear, integer, network (SDP/IP), cluster, centrality, turnover, tracking error
- OWA (Ordered Weighted Average) weights for tail risk measures

**Pipeline Flow**:
```
Returns → estimate(mu, cov, kurt) → set constraints → optimize(risk_measure, objective) → weights
```

**Hidden Gems**:
- 22+ risk measures in a single framework (most libs offer 3-5)
- Network/cluster constraints via SDP and integer programming
- Gerber statistic for correlation (noise-robust alternative to Pearson)
- Brinson attribution (factor-based performance attribution)
- DBHT (Directed Bubble Hierarchical Tree) clustering for asset allocation

**Anti-Patterns**:
- ❌ Using MV when returns have fat tails (use CVaR or EVaR instead)
- ❌ Ignoring estimation error in expected returns (use shrinkage or Black-Litterman)
- ❌ Not constraining cardinality for large universes

### 5. zipline (Python — Algorithmic Trading Engine)

**Architecture**: Pipeline-based with DataPortal abstraction. Algorithm runs in a simulation
loop with daily/minute bars. `initialize()` → `handle_data()` per bar. Pipeline API for
cross-sectional factor models.

**Key Abstractions**:
- `TradingAlgorithm` → initialize(), handle_data(), before_trading_start()
- `Pipeline` → compute factors across universe; CustomFilter, CustomFactor
- `DataPortal` → unified data access (bcolz, CSV, bundle)
- `Blotter` → order management
- `RiskMetricsCumulative` → real-time risk tracking
- `PerformanceTracker` → portfolio + benchmark tracking

**Hidden Gems**:
- Pipeline API for cross-sectional factor computation (Fama-French style)
- Bundle system for data ingestion (custom data sources)
- Built-in risk metrics cumulative tracking
- Slippage and commission models (VolumeShareSlippage)

**Anti-Patterns**: ❌ Not using Pipeline; ❌ Survivorship bias; ❌ Future data in factors.

### 6. pyfolio (Python — Performance Tear Sheets)

**Architecture**: Stateless analysis library. Takes returns/positions/transactions as
pd.Series/DataFrames. Produces matplotlib-based tear sheets with statistical overlays.

**Key Functions**:
- `create_full_tear_sheet()` → returns + positions + transactions comprehensive analysis
- `create_returns_tear_sheet()` → returns-only (drawdown, rolling, monthly heatmap)
- `create_position_tear_sheet()` → position concentration, turnover
- `create_txn_tear_sheet()` → transaction costs, turnover analysis
- `create_round_trip_tear_sheet()` → round-trip trade analysis (win/loss, duration)
- `perf_attrib` → factor-based performance attribution (style + sector)

**Hidden Gems**:
- Bootstrap analysis for performance metrics (confidence intervals)
- Bayesian cone for forward-looking return projections
- Interesting periods overlay (2008 crisis, COVID, etc.)
- Factor exposure decomposition (style vs sector)
- Capacity analysis (impact of scaling AUM)

### 7. empyrical (Python — Risk Metrics Library)

**Architecture**: Pure numpy/pandas computation. Each metric is a standalone function.
Rolling variants auto-generated via `_create_unary_vectorized_roll_function`.

**Key Functions**: sharpe_ratio, sortino_ratio, calmar_ratio, omega_ratio, max_drawdown,
annual_return (CAGR), annual_volatility, value_at_risk, conditional_value_at_risk,
alpha_beta, information_ratio, tail_ratio, downside_risk. All have `roll_*` variants.

**Hidden Gems**:
- `_adjust_returns()` for risk-free rate subtraction (avoids copy when factor=0)
- Vectorized roll functions with output buffer pre-allocation
- `aggregate_returns()` for period conversion (daily→monthly→yearly)
- Proper handling of NaN in cum_returns (fills with 0)
- Levy stability exponent alpha parameter in annual_volatility

### 8. yfinance (Python — Market Data Retrieval)

**Architecture**: Scraper-based with lazy loading. `Ticker` inherits `TickerBase`.
Data from Yahoo Finance API via `_http` session. Scraper classes: PriceHistory, Quote,
Fundamentals, Analysis, Holders, FundsData.

**Key**: Ticker.history(), .financials, .balance_sheet, .options, .info;
Tickers (batch download); Screener API (day_gainers, etc.); WebSocket live streaming;
MIC code support (Ticker(('OR','XPAR'))); ISIN resolution; caching (tz, ISIN).

**Anti-Patterns**: ❌ Fetching in loops (use Tickers batch); ❌ Ignoring rate limits
(YFRateLimitError); ❌ Not caching; ❌ Ignoring timezone handling (UTC default).

---

## Credit Risk

### PD / LGD / EAD / ECL

**Definition of Default** (BCBS / IFRS 9 aligned):
1. Past Due > 90 days (counted from oldest unpaid instalment)
2. Unlikely to Pay (UTP): distressed restructuring, bankruptcy, sale at material loss,
   triggered acceleration/cross-default, distressed exchange

**PD Models**:
- Scorecard: WOE (Weight of Evidence) binning + logistic regression (interpretable, regulator-friendly)
- ML: XGBoost, LightGBM (non-regulatory); neural nets (competitive but less interpretable)
- Survival: Cox Proportional Hazards, Random Survival Forest (time-to-default)
- Structural: Merton KMV (for corporates): PD = N((-log(V/D) - (μ-σ²/2)T) / (σ√T))

**Validation Metrics**:
- Discrimination: AUC ≥ 0.70, Gini (= 2·AUC - 1) ≥ 30, KS statistic
- Calibration: Brier score, Hosmer-Lemeshow test, calibration plot
- Stability: PSI < 0.10 stable, 0.10-0.25 moderate, > 0.25 unstable
- CSI (Characteristic Stability Index) per feature

**LGD Models**: Workout (Σ recovery CF × discount / EAD); Market (EAD - sale recovery) / EAD;
Default (realised LGD). Drivers: collateral type, LTV, seniority, time-in-default.

**EAD / CCF**: CCF = (EAD_at_default - drawn) / undrawn. Models: regression on utilisation.

**IFRS 9 Staging**:

| Stage | Criterion | ECL Horizon |
|-------|-----------|-------------|
| 1 | Performing; no SICR | 12-month ECL |
| 2 | Significant increase in credit risk (SICR) | Lifetime ECL |
| 3 | Credit-impaired (default) | Lifetime ECL on net carrying |

SICR triggers (must be backtested): PD increase (absolute/relative), 30 DPD rebuttable
presumption, watch-list, forbearance/distressed restructuring.

**Basel IRB Capital Formula**:
```
K = LGD × [N((1-R)^(-0.5) × G(PD) + (R/(1-R))^0.5 × G(0.999)) - PD] × MA
```
Where R = asset correlation, MA = maturity adjustment. FIRB: bank estimates PD;
AIRB: bank estimates PD, LGD, EAD.

**Low-Default Portfolios** (sovereign, bank, large corporate): Bayesian hierarchical,
shadow rating, through-the-cycle (TTC) PD, long-term averages from external data.

### Credit Portfolio
Concentration (HHI, single-name ≤ 10% capital, sector/geo limits); migration matrices
(annual transition from S&P/Moody's + internal overlay); EL = Σ EAD × PD × LGD;
UL via single-factor Gaussian copula (ρ ≈ 12-24%); vintage analysis (cohort PD over time);
Credit VaR: Monte Carlo correlated defaults → loss distribution → 99.9% quantile.

### Code: Credit Risk

```python
import numpy as np
import pandas as pd
from sklearn.linear_model import LogisticRegression

# WOE binning for PD scorecard
def woe_binning(df, feature, target, bins=10):
    df = df.copy()
    df['bin'] = pd.qcut(df[feature], bins, duplicates='drop')
    g = df.groupby('bin')[target].agg(['count', 'sum'])
    g.columns = ['total', 'bad']
    g['good'] = g['total'] - g['bad']
    g['dist_bad'] = g['bad'] / g['bad'].sum()
    g['dist_good'] = g['good'] / g['good'].sum()
    g['woe'] = np.log(g['dist_good'] / g['dist_bad'])
    g['iv'] = (g['dist_good'] - g['dist_bad']) * g['woe']
    return g

# IFRS 9 staging
def stage_loan(row, pd_current, pd_lifetime, sicr_threshold=2.0):
    if row['dpd'] >= 90 or row['utp_flag']:
        return 3  # credit-impaired
    elif pd_lifetime / pd_current > sicr_threshold or row['dpd'] >= 30:
        return 2  # SICR
    return 1  # performing

# ECL calculation
def ecl(ead, pd, lgd, discount_rate=0.0, horizon=1):
    return ead * pd * lgd / (1 + discount_rate) ** horizon

# Basel IRB capital
from scipy.stats import norm
def irb_capital(pd, lgd, r, maturity_adj=1.0):
    rho = 0.12 * (1 - np.exp(-50*pd)) / (1 - np.exp(-50)) + 0.24 * (1 - (1-np.exp(-50*pd))/(1-np.exp(-50)))
    k = lgd * (norm.cdf((1-rho)**(-0.5) * norm.ppf(pd) + (rho/(1-rho))**0.5 * norm.ppf(0.999)) - pd)
    return k * maturity_adj
```

---

## Market Risk

### VaR Methods

| Method | Formula | Strengths | Weaknesses |
|--------|---------|-----------|------------|
| Historical | Quantile of P&L distribution | No distribution assumption; captures fat tails | Window choice; no extrapolation |
| Parametric | z·σ·√t · portfolio value | Fast; analytical | Assumes normality (underestimates tails) |
| Monte Carlo | Simulate many paths; take quantile | Handles non-linearity; flexible | Slow; model risk |

**EWMA volatility**: σ²_t = (1-λ)·r²_{t-1} + λ·σ²_{t-1}, λ ≈ 0.94 daily.

**Expected Shortfall**: ES = E[L | L > VaR]. Coherent risk measure (unlike VaR).
FRTB default: ES at 97.5% confidence level.

### Backtesting
**Kupiec POF**: H0: exception rate = 1 - confidence level.
LR_POF = -2·ln(p^x·(1-p)^(T-x) / (π^x·(1-π)^(T-x))); χ²(1) test.

**Christoffersen independence**: H0: exceptions independent (no clustering).

**Traffic Light (Basel)**: Green (0-4 exceptions), Yellow (5-9, ×1.4-1.5 multiplier),
Red (10+, ×2.0 or revert to standardised approach). 250 days, 99% VaR.

### Greeks (Options)

| Greek | Formula | Meaning |
|-------|---------|---------|
| Delta (Δ) | ∂V/∂S | Sensitivity to underlying |
| Gamma (Γ) | ∂²V/∂S² | Convexity (rate of delta change) |
| Vega (ν) | ∂V/∂σ | Sensitivity to volatility |
| Theta (Θ) | ∂V/∂t | Time decay |
| Rho (ρ) | ∂V/∂r | Sensitivity to interest rates |

### Code: Market Risk

```python
import numpy as np
from scipy import stats

def parametric_var(returns, confidence=0.99, portfolio_value=1e6, horizon=1):
    """Parametric (variance-covariance) VaR"""
    z = stats.norm.ppf(1 - confidence)
    return -z * returns.std() * np.sqrt(horizon) * portfolio_value

def historical_var(returns, confidence=0.99, portfolio_value=1e6, horizon=1):
    """Historical simulation VaR"""
    return -np.percentile(returns, (1-confidence)*100) * portfolio_value * np.sqrt(horizon)

def ewma_volatility(returns, lam=0.94):
    """EWMA (RiskMetrics) volatility"""
    vol = np.zeros(len(returns))
    vol[0] = returns.std()
    for i in range(1, len(returns)):
        vol[i] = np.sqrt((1-lam) * returns.iloc[i-1]**2 + lam * vol[i-1]**2)
    return vol

def kupiec_test(violations, n_obs, confidence=0.99):
    """Kupiec Proportion of Failures test"""
    p = 1 - confidence
    pi_hat = violations / n_obs
    lr = -2 * (np.log((p**violations * (1-p)**(n_obs-violations)) /
                       (pi_hat**violations * (1-pi_hat)**(n_obs-violations))))
    p_value = 1 - stats.chi2.cdf(lr, df=1)
    return lr, p_value

# empyrical for comprehensive metrics
import empyrical as ep
metrics = {
    'sharpe': ep.sharpe_ratio(returns),
    'sortino': ep.sortino_ratio(returns),
    'max_dd': ep.max_drawdown(returns),
    'calmar': ep.calmar_ratio(returns),
    'omega': ep.omega_ratio(returns),
    'var': ep.value_at_risk(returns),
    'cvar': ep.conditional_value_at_risk(returns),
    'tail': ep.tail_ratio(returns),
}
alpha, beta = ep.alpha_beta(returns, benchmark)
```

---

## Portfolio Optimization

### Methods

| Method | Approach | Strengths | Weaknesses |
|--------|----------|-----------|------------|
| Mean-Variance | Maximize return for risk; efficient frontier | Analytical; foundational | Sensitive to estimates |
| Risk Parity | Equal risk contribution per asset | No expected return needed | May concentrate in low-vol |
| Black-Litterman | Market equilibrium + investor views | Stable; incorporates views | View specification |
| Min Variance | Minimize variance only | No expected return needed | Low return |
| Max Sharpe | Maximize Sharpe ratio | Risk-adjusted optimal | Sensitive to estimates |
| HRP/HERC | Hierarchical clustering + risk budget | Robust to estimation error | Clustering choice |

### Code: riskfolio-lib

```python
import riskfolio as rp
import pandas as pd

# Basic mean-variance optimization
port = rp.Portfolio(returns=returns)
port.assets_stats(method_mu='hist', method_cov='hist')
w_mv = port.optimization(model='Classic', rm='MV', obj='Sharpe', hist=True)

# CVaR optimization (better for fat tails)
w_cvar = port.optimization(model='Classic', rm='CVaR', obj='Sharpe')

# Hierarchical Risk Parity (no expected returns needed)
hport = rp.HCPortfolio(returns=returns)
w_hrp = hport.optimization(model='Classic', rm='MV', linkage='ward')

# Efficient frontier
frontier = port.efficient_frontier(model='Classic', rm='MV', points=50)

# Risk parity
w_rp = port.rp_optimization(model='Classic', rm='MV', rf=0)

# Black-Litterman
viewdict = {'AAPL': 0.10, 'MSFT': 0.08}
w_bl = port.black_litterman_optimization(
    views=viewdict, sigma=sigma, delta=2.5,
    model='Classic', rm='MV', obj='Sharpe'
)
```

### Code: vectorbt

```python
import vectorbt as vbt
import pandas as pd

# SMA crossover
fast = vbt.MA.run(price, window=10)
slow = vbt.MA.run(price, window=30)
entries = fast.ma_crossed_above(slow)
exits = fast.ma_crossed_below(slow)
pf = vbt.Portfolio.from_signals(price, entries, exits, init_cash=10000, fees=0.001)

print(pf.stats())  # total return, sharpe, max drawdown, win rate
print(pf.trades.records_readable)
print(pf.drawdowns.records_readable)
```

---

## Backtesting Frameworks

### backtrader Pipeline

```python
import backtrader as bt

class SmaCross(bt.Strategy):
    params = (('fast', 10), ('slow', 30),)
    def __init__(self):
        self.crossover = bt.indicators.CrossOver(
            bt.indicators.SMA(self.data.close, period=self.p.fast),
            bt.indicators.SMA(self.data.close, period=self.p.slow))
    def next(self):
        if self.crossover > 0: self.buy()
        elif self.crossover < 0: self.sell()

cerebro = bt.Cerebro()
cerebro.addstrategy(SmaCross)
cerebro.adddata(bt.feeds.PandasData(dataname=df))
cerebro.broker.setcash(100000)
cerebro.broker.setcommission(commission=0.001)
cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name='sharpe')
cerebro.addanalyzer(bt.analyzers.DrawDown, _name='drawdown')
cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name='trades')
results = cerebro.run()
```

### pyfolio Tear Sheet

```python
import pyfolio as pf
pf.create_full_tear_sheet(returns, positions=positions,
    transactions=transactions, benchmark_rets=benchmark,
    live_start_date='2024-01-01', round_trips=True)
```

---

## Risk Metrics Reference

### Return-Based (empyrical)

| Metric | Function | Formula |
|--------|----------|---------|
| Annual Return (CAGR) | `annual_return()` | (Π(1+r))^(252/n) - 1 |
| Annual Volatility | `annual_volatility()` | std(r) × √252 |
| Sharpe Ratio | `sharpe_ratio()` | (mean(r)-rf)/std(r) × √252 |
| Sortino Ratio | `sortino_ratio()` | (mean(r)-MAR)/downside_risk |
| Max Drawdown | `max_drawdown()` | min((cum-peak)/peak) |
| Calmar Ratio | `calmar_ratio()` | annual_return / |max_drawdown| |
| Omega Ratio | `omega_ratio()` | Σ(r > τ) / Σ(r < τ) |
| VaR (Historical) | `value_at_risk()` | percentile(r, α) |
| CVaR | `conditional_value_at_risk()` | E[r | r < VaR_α] |
| Alpha/Beta | `alpha_beta()` | OLS regression vs benchmark |
| Information Ratio | `information_ratio()` | (r_p - r_b) / tracking_error |
| Tail Ratio | `tail_ratio()` | percentile(r,95) / |percentile(r,5)| |

### Portfolio (riskfolio-lib)
22 risk measures: MV, MAD, GMD, MSV, FLPM, SLPM, CVaR, EVaR, RLVaR, TG, RG,
MDD, ADD, CDaR, EDaR, RLDaR, UCI, WR, CVRG, TGRG, EVRG, RVRG.

---

## AML / KYC / Fraud Detection

### AML/KYC Workflow
CIP (Customer Identification) → CDD/EDD (Due Diligence based on risk) →
Sanctions screening (OFAC SDN, EU, UN, UK HMT — real-time) → PEP screening →
Beneficial ownership (≥25%) → Ongoing monitoring → SAR/STR if suspicious.

Transaction monitoring rules: threshold ($10K CTR), velocity (N txns in T),
structuring (sub-threshold), behavioural deviation from customer profile.
No tipping off — criminal offence in most jurisdictions.

### Fraud Detection
Types: identity, application, payment (CNP), account takeover, first-party,
friendly, insurance claims, synthetic identity.

Methods: rules (fast, interpretable, bypassable); ML supervised (high accuracy,
needs labels, concept drift); ML anomaly (novel patterns, high FP);
graph (fraud rings, expensive); hybrid (best of all, complex).

Key metrics: precision ≥ 95% (false positives hurt UX), recall on chargebacks.
Feedback loop: chargeback arrives 30-90 days after txn → label → retrain.
Solutions: faster feedback (issuer-reported), synthetic labels from rule blocks.

---

## Stress Testing

| Type | Description | When |
|------|-------------|------|
| Sensitivity | Single-factor shock (rates +200bp) | Tactical screening |
| Scenario | Multi-factor (recession: GDP + unemployment + rates) | Capital planning |
| Reverse | Find scenario that breaches capital | Vulnerability analysis |
| Historical | Replay 2008, 2020, 2022 | Sanity check |

Severity ladder: Baseline → Adverse → Severely Adverse.

Satellite models (macro → risk parameter): GDP→PD, rates→prepayment,
equity→LGD, macro→drawdowns. Non-linearity matters — avoid direct 1:1 scaling.

Regulatory frameworks: CCAR/DFAST (US, annual, 9-quarter, public); EBA (EU, biennial,
3-year, SSM-supervised); BCBS (global SIBs); EIOPA (insurance, biennial).

Management actions: capital (raise, dividend cut), asset (sell, hedge), liability
(reinsurance, lapse), operational (cost reduction). Only actions under firm control.

---

## Actuarial

**Life Insurance**: Mortality tables (qx); Lee-Carter (log(qx,t) = ax + bx·kt),
CBD, APC models; longevity risk (increasing life expectancy → annuity exposure);
lapse modelling (voluntary termination, persistency).

**P&C (Non-life)**: Frequency (Poisson, Negative Binomial, zero-inflated);
severity (Lognormal, Pareto, Gamma — body + tail); combined ratio = loss + expense;
pricing = pure premium × loading.

**Reserving**: Chain-ladder (stable patterns), Bornhuetter-Ferguson (limited history),
Cape Cod (expected loss ratio), Mack (uncertainty), Bootstrap (full posterior).

**Solvency II**: SCR modules (market, counterparty, life/NL underwriting, operational);
1-in-200, 1-year VaR; standard formula or internal model. MCR = floor at 25% SCR.

**IFRS 17**: GMM (FCF + risk adjustment + CSM, default), PAA (short-duration ≤1y),
VFA (direct participation). CSM = profit recognised over coverage period.

---

## Accounting

**IFRS Key Standards**: IFRS 9 (financial instruments, ECL staging, hedge accounting);
IFRS 15 (5-step revenue recognition); IFRS 16 (leases: ROU asset + lease liability);
IFRS 13 (fair value: Level 1/2/3 hierarchy); IAS 36 (impairment: recoverable amount);
IAS 37 (provisions, contingent liabilities).

**Hedge Accounting (IFRS 9)**: Fair value hedge (derivative ↔ asset at FV);
cash flow hedge (forecast transaction → OCI); net investment hedge (foreign op).

**Fair Value**: Level 1 (quoted prices), Level 2 (observable inputs), Level 3
(unobservable — significant judgment, sensitivity disclosed).

---

## Tax

**Transfer Pricing**: Arm's length (CUP, resale price, cost plus, TNMM, profit split);
documentation (master file, local file, CbCR); functional analysis (FAR).

**BEPS Pillar 2**: 15% global minimum ETR; IIR (income inclusion at parent),
UTPR (undertaxed profits backstop), QDMTT (qualified domestic top-up).

**Deferred Tax**: DTA = deductible temporary differences × tax rate (recognised if
probable future profit); DTL = taxable × tax rate (always recognised).

---

## Valuation

**DCF**: EV = Σ FCFF/(1+WACC)^t + TV/(1+WACC)^n.
FCFF = EBIT×(1-t) + D&A - CapEx - ΔWC. TV = FCF×(1+g)/(WACC-g).
WACC = (E/V)×Re + (D/V)×Rd×(1-t). Re = Rf + β×ERP + size premium.

**Multiples**: EV/Revenue (pre-profit), EV/EBITDA (mature), P/E (equity focus),
P/B (banks/financials). Select comps: same industry, similar size/growth.

**LBO**: Returns = (Exit EBITDA × Exit multiple - Net debt) / Equity.
Typical: 5-8x entry, 5-7x leverage, 3-5y hold.

**Real Options**: Defer, expand, abandon. Valued via Black-Scholes or decision trees.

---

## Capital Markets

**Options Pricing**: Black-Scholes (vanilla European); Heston (stochastic vol);
SABR (smile); local vol (Dupire); jump-diffusion (Merton); binomial/trinomial
(American/Bermudan); Longstaff-Schwartz (MC + regression for American).

**Fixed Income**: YTM; duration (Macaulay, Modified, Effective, Key Rate);
convexity; spreads (G-spread, Z-spread, I-spread, OAS).

**Credit Markets**: CDS (par spread = (1-R)×hazard rate); CDO (tranches, waterfall,
correlation); CLO (leveraged loan backed).

**Factor Models**: CAPM; Fama-French 3 (market, SMB, HML); Carhart 4 (+momentum);
FF5 (+profitability, investment); q-factor (Hou-Xue-Zhang).

---

## Data Acquisition with yfinance

```python
import yfinance as yf

# Single ticker
ticker = yf.Ticker('AAPL')
hist = ticker.history(period='1y', interval='1d')
financials = ticker.financials
options = ticker.option_chain('2025-12-19')

# Batch (faster)
tickers = yf.Tickers('AAPL MSFT GOOGL')
data = tickers.download(period='1y')

# MIC code support
ticker = yf.Ticker(('OR', 'XPAR'))  # French stock
```

---

## Anti-Patterns (Cross-Domain)

**Credit**: ❌ Non-representative sample; high Gini but bad calibration;
SICR without backtest; LDP zero-PD; macro overlays without rationale.

**Market**: ❌ VaR without backtest; parametric VaR with fat tails;
same correlation in stress as normal; too-short historical window.

**Portfolio**: ❌ MV with fat tails (use CVaR); ignoring estimation error;
no cardinality constraints; single point estimate without sensitivity.

**Backtesting**: ❌ Look-ahead bias; survivorship bias; ignoring costs;
overfitting to in-sample; no OOS/OOT validation.

**Data**: ❌ Fetching in loops (use batch); ignoring rate limits;
timezone mismatches; not caching repeated queries.

---

## Failure Modes & Recovery

| Domain | Failure | Recovery |
|--------|---------|----------|
| Credit | Model drift (PSI > 0.25) | Re-fit; investigate; conservative add-on |
| Credit | Calibration decay | Refit PD; overlay; document |
| Market | VaR exception rate too high | Capital multiplier; refine model |
| Market | Stress loss > capital | Reduce positions; capital add-on |
| Portfolio | Optimization infeasible | Relax constraints; regularized solver |
| Portfolio | Concentration breach | Sell-down; committee review |
| Trading | Strategy overfit | Walk-forward; regularization; simpler model |
| AML | Sanctions hit missed | Re-screen all pending; report to OFAC |
| Fraud | Concept drift | Retrain; refresh features; adversarial validation |
| Actuarial | Reserve deficient | Top-up; investigate; pricing review |
| Accounting | Restatement required | Investigate; quantify; restate comparatives |
| Tax | Tax assessment | Administrative appeal; judicial; documentation |

---

## Citations

**Regulatory**: BCBS Basel III/IV framework, IRB approach, FRTB, stress testing principles;
IASB IFRS 9/13/15/16/17; US Fed SR 11-7 (Model Risk), CCAR/DFAST instructions;
EBA stress test methodology, Model Risk Management guidelines (2024);
FATF 40 Recommendations; OECD BEPS 2.0 Pillar 2 model rules;
Solvency II Directive 2009/138/EC; UK PRA SS1/23.

**Academic**: Markowitz "Portfolio Selection" (1952); Black-Scholes "Pricing of Options" (1973);
Merton "Pricing of Corporate Debt" (1974); Fama-French "Common Risk Factors" (1993);
Rockafellar-Uryasev "Optimization of CVaR" (2000); Lee-Carter "US Mortality" (1992);
De Prado "Advances in Financial Machine Learning" (2018).

**Libraries**: QuantLib (quantlib.org); backtrader (backtrader.com); vectorbt (vectorbt.dev);
riskfolio-lib (riskfolio-lib.readthedocs.io); pyfolio (quantopian/pyfolio);
empyrical (quantopian/empyrical); yfinance (ranaroussi/yfinance).


## Verification Checklist

- [ ] Data quality checks on market/credit data
- [ ] Model assumptions validated (normality, stationarity)
- [ ] Backtesting performed with walk-forward analysis
- [ ] VaR backtests (Kupiec, Christoffersen) passed
- [ ] Stress scenarios cover historical + hypothetical
- [ ] Risk limits defined and monitored
- [ ] Model risk assessment completed
- [ ] Regulatory requirements met (Basel, IFRS 9)
- [ ] Independent model validation performed
- [ ] Audit trail maintained

## Implementation Classes

```python
"""
Portfolio Risk Analytics — VaR, ES, Greeks, and stress testing.
"""
import numpy as np
import pandas as pd
from scipy import stats
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple


@dataclass
class VaRResult:
    var: float
    es: float  # Expected Shortfall (CVaR)
    confidence: float
    method: str
    horizon_days: int


class PortfolioRiskEngine:
    """Compute VaR, ES, Greeks, and stress scenarios for portfolios."""

    def __init__(self, returns: pd.DataFrame, weights: np.ndarray):
        self.returns = returns
        self.weights = weights
        self.portfolio_returns = returns @ weights
        self.cov_matrix = returns.cov()

    def parametric_var(self, confidence: float = 0.99,
                       horizon: int = 1) -> VaRResult:
        """Variance-covariance VaR assuming normal distribution."""
        mu = self.portfolio_returns.mean()
        sigma = self.portfolio_returns.std()
        z = stats.norm.ppf(1 - confidence)
        var_1d = -(mu + z * sigma)
        var_h = var_1d * np.sqrt(horizon)
        es_h = sigma * np.sqrt(horizon) * stats.norm.pdf(z) / (1 - confidence)
        return VaRResult(var=var_h, es=es_h + mu * horizon,
                         confidence=confidence, method="parametric",
                         horizon_days=horizon)

    def historical_var(self, confidence: float = 0.99,
                       horizon: int = 1) -> VaRResult:
        """Historical simulation VaR."""
        sorted_ret = np.sort(self.portfolio_returns)
        idx = int((1 - confidence) * len(sorted_ret))
        var_1d = -sorted_ret[idx]
        es_1d = -sorted_ret[:idx].mean()
        return VaRResult(var=var_1d * np.sqrt(horizon),
                         es=es_1d * np.sqrt(horizon),
                         confidence=confidence, method="historical",
                         horizon_days=horizon)

    def monte_carlo_var(self, confidence: float = 0.99,
                        horizon: int = 1,
                        n_sims: int = 10000) -> VaRResult:
        """Monte Carlo VaR with correlated returns."""
        mu = self.returns.mean().values
        chol = np.linalg.cholesky(self.cov_matrix.values)
        sims = np.random.randn(n_sims, len(self.weights))
        correlated = sims @ chol.T + mu
        portfolio_sims = correlated @ self.weights
        sorted_sims = np.sort(portfolio_sims)
        idx = int((1 - confidence) * n_sims)
        var_1d = -sorted_sims[idx]
        es_1d = -sorted_sims[:idx].mean()
        return VaRResult(var=var_1d * np.sqrt(horizon),
                         es=es_1d * np.sqrt(horizon),
                         confidence=confidence, method="monte_carlo",
                         horizon_days=horizon)

    def kupiec_test(self, var_series: pd.Series,
                    returns: pd.Series,
                    confidence: float = 0.99) -> Dict:
        """Kupiec POF test for VaR backtesting."""
        exceptions = (returns < -var_series).sum()
        n = len(returns)
        p_hat = exceptions / n
        p_0 = 1 - confidence
        lr = -2 * (np.log((1 - p_0) ** (n - exceptions) * p_0 ** exceptions)
                   - np.log((1 - p_hat) ** (n - exceptions) * p_hat ** exceptions))
        p_value = 1 - stats.chi2.cdf(lr, df=1)
        return {"exceptions": int(exceptions), "expected": n * p_0,
                "lr_statistic": lr, "p_value": p_value,
                "reject_h0": p_value < 0.05}


class CreditRiskModel:
    """PD/LGD/EAD modeling with Basel IRB capital calculation."""

    def __init__(self, pd_model=None, lgd_model=None):
        self.pd_model = pd_model
        self.lgd_model = lgd_model

    def basel_irb_capital(self, pd: float, lgd: float, ead: float,
                          maturity: float = 2.5,
                          asset_corr: float = None) -> Dict:
        """Basel IRB one-factor model capital requirement."""
        if asset_corr is None:
            asset_corr = 0.12 * (1 - np.exp(-50 * pd)) / (1 - np.exp(-50)) + \
                         0.24 * (1 - (1 - np.exp(-50 * pd)) / (1 - np.exp(-50)))
        g_pd = stats.norm.ppf(pd)
        b = (0.11852 - 0.05478 * g_pd) ** 2
        ma = (1 + (maturity - 2.5) * b) / (1 - 1.5 * b)
        k = lgd * (stats.norm.cdf(
            (1 - asset_corr) ** -0.5 * g_pd +
            (asset_corr / (1 - asset_corr)) ** 0.5 * stats.norm.ppf(0.999)
        ) - pd) * ma
        return {"capital_requirement": k * ead,
                "risk_weighted_assets": k * ead * 12.5,
                "correlation": asset_corr, "maturity_adj": ma}

    def ifrs9_staging(self, pd_12m: float, pd_lifetime: float,
                      significant_increase: bool) -> int:
        """IFRS 9 staging: Stage 1 (12m ECL), Stage 2 (lifetime ECL), Stage 3 (credit-impaired)."""
        if significant_increase or pd_lifetime > 3 * pd_12m:
            return 3 if pd_lifetime > 0.05 else 2
        return 1


---

## Decision tree

See **When to use** above.

## Workflow

1. Read the skill body above.
2. Identify the relevant section for your task.
3. Apply the patterns and examples provided.
4. Verify against the listed anti-patterns.

## Examples

Examples are interleaved throughout the skill body above.

## Anti-patterns

See the **When to use** criteria above. The skill is *not* applied when:
- The task is outside the skill's declared scope.
- Simpler alternatives exist.

## Failure modes

- **Misapplied scope**: invoking the skill for tasks it doesn't cover.
- **Outdated reference**: real-world library APIs may have shifted since synthesis.
- **Pattern drift**: the skill's patterns describe idealized APIs, not exact production code.

## Related skills

- `econometrics`
- `machine-learning-pipelines`
- `causal-ml-inference`

