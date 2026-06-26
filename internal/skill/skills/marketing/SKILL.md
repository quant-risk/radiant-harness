# Skill: marketing

> MMM, MTA, incrementality, LTV, churn, segmentation. Marketing
> without causal measurement is spend, not investment.

## Decision tree

```
Marketing question
        │
        ▼
[Step 1] Identify metric (incremental revenue / conversions / brand)
        │
        ▼
[Step 2] Choose methodology (MMM / MTA / incrementality / LTV)
        │
        ▼
[Step 3] Build model (adstock, saturation, cross-effects)
        │
        ▼
[Step 4] Validate with incrementality test
        │
        ▼
[Step 5] Optimise allocation
        │
        ▼
[Step 6] Measure LTV impact on payback
```

## Workflow

### Marketing Mix Modeling (MMM)

**Model**: weekly revenue (or KPI) regressed on media spend by
channel + control variables.

```
Y_t = intercept + trend + seasonality + Σ β_c × adstock(Spend_c,t) + γ × controls + ε
```

**Adstock** (carry-over):
```
adstock(S_t) = S_t + λ × adstock(S_{t-1}),  0 < λ < 1
```

λ = retention rate. TV typically 0.3-0.7; paid search 0.1-0.3.

**Saturation** (diminishing returns):
```
effect = β × adstock^c,  0 < c < 1
```
or Hill function. Without saturation, you think more spend always
helps more — false.

**Controls**:
- Price, distribution, product launches
- Macro (GDP, weather, competitor activity)
- Holidays, events

**Tools**: LightweightMMM (Google), Robyn (Facebook), PyMC-MMM.

### Multi-Touch Attribution (MTA)

Assigns credit for a conversion across touchpoints.

| Model | Logic |
|-------|-------|
| **Last-click** | 100% to last touch |
| **First-click** | 100% to first touch |
| **Linear** | Equal across touches |
| **Time-decay** | More credit to recent touches |
| **Position-based** | 40% first, 40% last, 20% middle |
| **Data-driven (Shapley)** | Game-theoretic; values each touch by marginal contribution |
| **Markov** | Probabilistic removal effect |

**MTA limitation**: requires user-level data; loses accuracy in
cross-device, with iOS privacy changes.

### Incrementality testing

**Geo test** (most common):
- Treatment: select markets with paid campaign
- Control: matched markets without
- Measure: lift in outcome (revenue, app installs, etc.)
- Use synthetic control / diff-in-diff for matched control

**Audience test**:
- Treatment: exposed audience
- Control: holdout audience (PSA / unexposed)
- Measure: lift in conversion

**Ghost ads / public place test**:
- Ads not actually delivered to a control group
- Measure: difference in outcome

Design: power analysis; pre-registration; pre-period matching.

### LTV (Customer Lifetime Value)

**Formula**:
```
LTV = ARPU × Gross margin × (1 / Churn rate)
    = ARPU × Gross margin × Avg customer lifetime
```

**Cohort-based**: LTV by acquisition cohort; differs by channel /
campaign.

**Probabilistic (BG/NBD + Gamma-Gamma)**: Buy-Till-You-Die model.

**Discounted LTV**: future cash flows discounted to today.

### Churn prediction

**Define churn**: depends on business (subscription = cancel;
SaaS = inactivity; ecommerce = dormancy).

**Features**:
- Usage frequency, recency, depth
- Support tickets
- NPS / CSAT
- Demographic
- Tenure
- Product engagement

**Models**: logistic regression, gradient boosting, survival models.

**Actions**: re-engagement campaigns; offers; product improvements.

### Segmentation

| Method | Approach |
|--------|----------|
| **RFM** | Recency, Frequency, Monetary |
| **Demographic** | Age, gender, income |
| **Behavioural** | Usage patterns |
| **Psychographic** | Attitudes, lifestyle |
| **Clustering** | K-means, hierarchical |
| **Latent class** | Model-based (finite mixture) |

Each segment gets targeted strategy (offer, channel, message).

### Media planning

**Objective**: maximise (incremental revenue - cost) subject to
budget and reach constraints.

**Inputs**:
- MMM channel coefficients
- Saturation curves
- Reach × frequency
- CPM / CPC / CPA by channel

**Output**: optimal spend allocation by channel + week.

## Examples

### Example 1: MMM (B2C ecommerce)

```
Channels: paid search, paid social, display, TV
Data: 104 weeks; 2y weekly
Adstock: search=0.2, social=0.3, display=0.1, TV=0.6
Saturation: Hill function per channel
Controls: price, distribution, holiday dummies
Fit: R² = 0.78; MAPE = 8%
Output: channel contribution table
  Paid search: 22%
  Paid social: 18%
  Display: 12%
  TV: 28%
  Organic / brand: 20%
Recommendation: increase TV; reduce display (saturated)
```

### Example 2: incrementality (geo test)

```
Hypothesis: paid social drives +5% incremental revenue
Design:
  Treatment: 5 markets (matched) with $500k social spend
  Control: 5 matched markets (no social)
  Duration: 8 weeks
  Pre-period: 4 weeks matching
Analysis: diff-in-diff
Result: +3.2% incremental revenue (CI [+1.1%, +5.3%])
ROAS: 3.2x
```

### Example 3: LTV (subscription)

```
ARPU: $20/month
Gross margin: 80%
Monthly churn: 5%
Avg lifetime: 1/0.05 = 20 months
LTV = $20 × 0.8 × 20 = $320
CAC payback: $320 / ($20 × 0.8) = 20 months
Target: payback < 12 months → need churn < 8.3%
```

## Anti-patterns

### ❌ Correlation without incrementality test

Spend correlates with sales; no causal evidence. Always test.

### ❌ MMM without adstock

Ignores carry-over; underestimates long-tail channels (TV, brand).

### ❌ MMM without saturation

Assumes linear returns; misallocates budget.

### ❌ Last-click attribution

Ignores upper-funnel; undervalues brand and awareness.

### ❌ LTV without churn modelling

Optimistic if churn rising. Model churn; condition LTV on it.

### ❌ Marketing as cost centre

Marketing as investment requires ROI measurement; otherwise
indistinguishable from waste.

## Failure modes

| Failure | Recovery |
|---------|----------|
| MMM overfits | Simpler model; cross-validation; out-of-sample |
| MMM drift | Refit quarterly; monitor residuals |
| Incrementality test inconclusive | Larger sample; longer duration; better matching |
| Churn model stale | Refit; new features; behavioural refresh |
| LTV underestimates | Include cross-sell; longer horizon |
| Attribution broken (privacy) | Shift to MMM + incrementality |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/causal` | Causal inference framework; geo test design |
| `/causal-ml` | Uplift modeling; heterogeneous effects |
| `/bayesian` | Bayesian MMM; uncertainty in estimates |
| `/stats` | Significance testing; model validation |
| `/controlling` | Marketing budget tied to plan |