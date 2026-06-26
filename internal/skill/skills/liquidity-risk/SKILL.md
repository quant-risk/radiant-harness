# Skill: liquidity-risk

> LCR, NSFR, ALM, cash flow forecasting, intraday liquidity,
> contingency funding. Liquidity crises happen when solvency
> is fine — funding runs out.

## Decision tree

```
Funding liquidity question
        │
        ▼
[Step 1] Regulatory ratios (LCR, NSFR)
        │
        ▼
[Step 2] HQLA classification + haircuts
        │
        ▼
[Step 3] Outflow assumptions (deposit behaviour, drawdowns)
        │
        ▼
[Step 4] Cash flow forecast (behavioural, multi-horizon)
        │
        ▼
[Step 5] Stress scenarios (idiosyncratic, market, combined)
        │
        ▼
[Step 6] Intraday monitoring
        │
        ▼
[Step 7] Contingency funding plan (CFP)
```

## Workflow

### LCR (Liquidity Coverage Ratio)

```
LCR = HQLA / Net cash outflows over 30 days
```

**Must hold ≥ 100%**.

**HQLA tiers**:
- **Level 1**: cash, central bank reserves, government bonds (0% RW)
- **Level 2A**: government-guaranteed bonds, qualifying covered bonds (15% haircut)
- **Level 2B**: some corporate bonds, equities (25-50% haircut)
- Level 2 capped at 40% of HQLA

**Outflow assumptions** (30-day stress):
- Retail deposits: 3-10% (stable), 10-15% (less stable)
- Unsecured wholesale: 40-100% (operational) or 100%
- Secured funding: 0-25% (depending on collateral)
- Off-balance: 5-40% (committed credit / liquidity facilities)
- Derivatives: net derivative outflows

### NSFR (Net Stable Funding Ratio)

```
NSFR = Available Stable Funding (ASF) / Required Stable Funding (RSF)
```

**Must hold ≥ 100%**.

**ASF factors** (liabilities):
- Equity + liabilities > 1y: 100%
- Stable retail deposits: 95%
- Less stable retail: 90%
- Wholesale funding > 6m: 50%
- Wholesale funding < 6m: 0%

**RSF factors** (assets):
- HQLA Level 1: 0%
- HQLA Level 2A: 15%
- Loans to retail / SMEs > 1y: 85%
- Loans to retail < 1y: 50%
- Unencumbered non-HQLA: 50-100%

### Cash flow forecasting

| Horizon | Purpose |
|---------|---------|
| Intraday | Settlement, CLS, real-time |
| 1-7 days | Operational funding |
| 1-3 months | Tactical |
| 1 year | Strategic, FTP |

**Behavioural modelling**:
- Core deposits (stable fraction of demand deposits)
- Prepayment speeds (mortgages, consumer loans)
- Drawdown rates (undrawn commitments)
- Rollover rates (wholesale funding)

Backtest: forecast vs actual; calibrate.

### Stress scenarios

| Scenario | Outflow shock |
|----------|---------------|
| **Idiosyncratic** | Bank-specific (rating downgrade, scandal); 5% retail, 20% wholesale |
| **Market-wide** | Sector crisis; 5-10% retail, 30-40% wholesale |
| **Combined** | Both; severe |
| **Reverse** | Find scenario that breaches 30-day survival |

### Intraday liquidity

Track real-time:
- Opening balances at central bank
- Expected vs actual settlement flows
- CLS / FX settlement
- Collateral availability
- Repo availability

Tools: intraday dashboards; alerts on threshold breaches.

### Contingency Funding Plan (CFP)

Triggers + actions + escalation:

| Trigger | Action |
|---------|--------|
| LCR drops below 110% | Internal alert; review funding plan |
| LCR drops below 105% | Treasury activates; reduce outflows |
| LCR drops below 100% | Crisis mode; activate CFP |
| Wholesale market closes | Liquidity buffer drawdown; central bank facility |
| Rating downgrade | Pre-arranged funding; communication |

CFP tested quarterly (tabletop exercise).

### ALM (Asset-Liability Management)

| Risk | Mitigation |
|------|-----------|
| Repricing risk | Match duration of assets + liabilities; hedging |
| Yield curve risk | DV01 buckets; curve strategies |
| Basis risk | Hedge basis exposure |
| Optionality risk | Prepayment modelling; convexity hedge |
| FX risk (in banking book) | Match currency; hedge |

Tools: IRRBB (Interest Rate Risk in the Banking Book) — EVE
(Economic Value of Equity) and NII (Net Interest Income) sensitivity.

## Examples

### Example 1: LCR calculation (commercial bank)

```
HQLA:
  Level 1 (cash, gov bonds): 50B
  Level 2A (gov-guaranteed): 20B (haircut 15% -> 17B)
  Level 2B (corporate bonds): 10B (haircut 50% -> 5B)
  Total HQLA (after cap): 70B
Outflows (30d stress):
  Retail stable: 30B @ 5% = 1.5B
  Retail less stable: 40B @ 10% = 4.0B
  Unsecured wholesale: 50B @ 40% = 20.0B
  Secured: 30B @ 15% = 4.5B
  Off-balance: 40B @ 10% = 4.0B
  Total outflows: 34B
Inflows: 5B (limited by 75% cap)
Net outflows: 29B
LCR = 70/29 = 241% (well above 100%)
```

### Example 2: NSFR calculation

```
ASF:
  Equity: 30B @ 100% = 30B
  Long-term debt: 50B @ 100% = 50B
  Stable retail: 80B @ 95% = 76B
  Less stable retail: 50B @ 90% = 45B
  Wholesale > 6m: 30B @ 50% = 15B
  Total ASF: 216B

RSF:
  HQLA: 70B @ 0% = 0B
  Retail loans: 100B @ 85% = 85B
  Wholesale loans: 50B @ 50% = 25B
  Other assets: 30B @ 100% = 30B
  Total RSF: 140B

NSFR = 216/140 = 154% (well above 100%)
```

### Example 3: cash flow forecast (behavioural)

```
Forecast horizon: 1y
Customer behaviour:
  - core deposits: 60% stable (no outflow)
  - prepayment: 8% CPR (mortgage)
  - drawdown rate: 25% (undrawn credit lines)
Wholesale:
  - 40% rollover within 1y
  - 60% new issuance
Stress: idiosyncratic (rating downgrade)
Result: liquidity buffer covers 90 days; NSFR stays > 105%
```

## Anti-patterns

### ❌ LCR with no behavioural haircut

Contractual outflows underestimate runs. Always apply behaviour.

### ❌ NSFR ignoring encumbered assets

Encumbered assets can't count as available funding. Track them.

### ❌ Cash flow forecast assuming contractual maturity

Behavioural assumptions matter. Calibrate to history.

### ❌ Contingency plan never tested

Triggers not actionable in stress. Quarterly tabletop.

### ❌ Intraday liquidity not monitored

Settlement failure risk. Real-time monitoring essential.

### ❌ Same correlations in liquidity stress

Funding correlates in crisis (everything gapping together). Use
stressed assumptions.

## Failure modes

| Failure | Recovery |
|---------|----------|
| LCR breach | Activate CFP; reduce outflows; HQLA build |
| NSFR breach | Long-term funding issuance; reduce long-term assets |
| Intraday shortfall | Repo / central bank facility; delay payments |
| Funding market closure | Liquidity buffer; central bank window |
| Behavioural model wrong | Re-calibrate; conservative overlay |
| Stress loss > buffer | Re-plan; raise capital; asset sales |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/market-risk` | Cross-risk effects; funding spread risk |
| `/credit-risk` | Counterparty risk; downgrade scenarios |
| `/operational-risk` | Operational continuity; payment systems |
| `/econometrics` | Behavioural modelling; time series |