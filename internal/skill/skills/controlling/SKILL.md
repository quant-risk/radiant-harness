# Skill: controlling

> FP&A: budgeting, forecasting, variance, cost allocation, KPIs.
> A budget is a hypothesis; the actuals are data. The job of
> controlling is to update the hypothesis.

## Decision tree

```
Planning + performance question
        │
        ▼
[Step 1] Planning cycle (annual / rolling / scenario)
        │
        ▼
[Step 2] Methodology (top-down / bottom-up / driver-based / ZBB)
        │
        ▼
[Step 3] Drivers + assumptions
        │
        ▼
[Step 4] Build model
        │
        ▼
[Step 5] Variance analysis (actual vs plan)
        │
        ▼
[Step 6] Update forecast (rolling)
        │
        ▼
[Step 7] Management reporting
```

## Workflow

### Planning cycle

| Cycle | Cadence | Use |
|-------|---------|-----|
| **Annual budget** | Yearly | Strategic plan; target setting |
| **Rolling forecast** | Monthly/quarterly (12-18 months forward) | Updated view of year |
| **Scenario analysis** | Ad-hoc | What-if; strategic decisions |
| **Mid-year update** | Half-year | Re-baseline if materially off |

### Methodology

| Method | Approach | Strengths | Weaknesses |
|--------|----------|-----------|------------|
| **Top-down** | Senior sets targets; cascades | Fast; aligned | Often unrealistic |
| **Bottom-up** | Each unit builds; aggregates | Realistic | Slow; sum > target |
| **Driver-based** | Volume × price × mix × cost-driver | Flexible; scenario-ready | Model complexity |
| **Zero-based (ZBB)** | Every line justified from zero | Cuts waste | Time-consuming |
| **Beyond-budgeting** | No fixed targets; rolling only | Adaptive | Less accountability |

### Driver-based planning

Identify the **5-10 key drivers** that move the business:

| Driver | Example |
|--------|---------|
| Volume | Units sold, transactions, customers |
| Price | Average selling price |
| Mix | Product mix, channel mix |
| Headcount | FTE by role |
| Compensation | Salary + benefits |
| Marketing | Spend by channel |
| Churn | Customer churn rate |

Each P&L line expressed as driver × formula. Allows scenario
analysis: change drivers, see P&L impact.

### Variance analysis

**Variance = Actual - Plan**

Decompose into drivers:
- Revenue variance = volume × price × mix × FX
- Cost variance = headcount × comp × efficiency
- Margin variance = revenue - cost (with attribution)

**Significance**: variances > X% of plan investigated; Y% of
prior period.

**Explanation**: not just the number; the WHY. "Volume missed
because customer X delayed order" beats "volume -5%".

### Cost allocation

| Method | When |
|--------|------|
| **Direct costing** | Costs assigned directly to product / service |
| **Activity-based (ABC)** | Costs assigned by activity driver |
| **Step-down** | Service department costs stepped to operating |
| **Reciprocal** | Mutual services between departments |
| **Proportional** | % of revenue or headcount |

Avoid: arbitrary allocation that distorts decisions. Each
allocation should reflect actual resource consumption.

### KPIs and OKRs

| Layer | Definition | Example |
|-------|-----------|---------|
| **Strategic KPI** | Long-term outcome | Revenue growth, market share |
| **Operational KPI** | Day-to-day performance | NPS, conversion, churn |
| **Driver KPI** | Leading indicator | Pipeline, capacity, headcount |

**KPI tree**: roll up driver → operational → strategic.

### Rolling forecast

**12-quarter rolling forecast**: each month, drop the past month,
add a new month at the end. Never re-baseline; always rolling.

This avoids the "annual budget is stale" problem.

### Management reporting

| Layer | Cadence | Audience | Content |
|-------|---------|----------|---------|
| **Daily / weekly** | Daily | Operational | KPIs, exceptions |
| **Monthly business review (MBR)** | Monthly | Management | Performance vs plan / forecast |
| **Quarterly business review (QBR)** | Quarterly | Senior management | Strategy + execution |
| **Board pack** | Quarterly | Board | Strategic, financial, risk |

Standard pack structure: executive summary, financial performance,
operational KPIs, risks + mitigations, outlook.

## Examples

### Example 1: driver-based revenue model

```
Revenue = Customers × ARPU × (1 - churn_rate)
       = 1M × $50 × (1 - 0.05)
       = $47.5M annual

Scenarios:
  Base:       $47.5M
  Bull:       +20% customers, +10% ARPU → $66M
  Bear:       -10% customers, -5% ARPU → $40.5M
```

### Example 2: variance analysis (monthly)

```
Revenue: actual $4.2M vs plan $4.5M (variance -$300k)
Drivers:
  Volume:  plan 1,000; actual 950 (-50, -$250k)
  Price:   plan $4,500; actual $4,421 (-$79 per unit, -$75k)
  Mix:     plan 60% premium; actual 55% (-$25k)
Total: -$350k (matches revenue variance within rounding)
Driver attribution: 71% volume, 21% price, 7% mix
```

### Example 3: cost allocation (shared services)

```
Shared services (IT, Finance, HR): $5M total
Allocation methods:
  IT:        by user count (50% by department FTE; 50% by usage)
  Finance:   by transaction count (AP, AR, GL postings)
  HR:        by headcount

Rationale: each method reflects actual resource consumption
Allocation table: department → cost; tied to GL
```

## Anti-patterns

### ❌ Budget as fixed target

Not adjusted when assumptions change. Drives gaming.

### ❌ Variance without driver analysis

"We missed by 5%" is not analysis. Always decompose into drivers.

### ❌ Cost allocation as % of revenue

Arbitrary; distorts decisions. Use activity-based where possible.

### ❌ No rolling forecast

Stuck on stale plan for 12 months. Adopt rolling.

### ❌ Driver-based without driver validation

Model is fiction. Calibrate drivers to historicals; backtest.

### ❌ KPI overload

100 KPIs = no focus. 5-10 strategic; 10-20 operational.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Significant miss | Investigate; update forecast; explain to leadership |
| Driver model breaks | Recalibrate; simplify; document assumption change |
| Cost allocation contested | Rationale documented; periodic review |
| Forecast accuracy poor | Calibration; tighter input controls |
| Budget gaming | Beyond-budgeting; relative targets |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/accounting` | Statutory vs management accounts |
| `/finance` | Capital allocation; WACC |
| `/valuation` | DCF assumptions tied to plan |
| `/marketing` | Marketing mix modelling ties to revenue forecast |