# Skill: actuarial-solvency

> Solvency II: SCR (standard formula + internal model), MCR,
> technical provisions, own funds, matching adjustment, ORSA,
> SFCR. The regulator's view of "enough capital" — and why.

## Decision tree

```
Solvency II calculation
        │
        ▼
[Step 1] Technical provisions (best estimate + risk margin)
        │
        ▼
[Step 2] SCR by module (market / counterparty / life / health / non-life / op)
        │
        ▼
[Step 3] Aggregation with correlation matrix
        │
        ▼
[Step 4] Own funds tiering + eligibility
        │
        ▼
[Step 5] Solvency ratio (eligible OF / SCR)
        │
        ▼
[Step 6] MCR
        │
        ▼
[Step 7] Reporting (QRTs, SFCR, RSR, ORSA)
```

## Workflow

### Technical provisions

**Best estimate (BE)**: PV of expected future cash flows, discounted
with EIOPA-prescribed rate (risk-free + matching/volatility adj).

```
BE = Σ E[CF_t / (1 + r_t)^t]
```

Cash flow projection:
- Best-estimate assumptions (no prudence margin)
- Mortality / lapse / expense / claims experience
- Time horizon: full runoff (contracts usually long-tailed)

**Risk margin (RM)**: cost of providing capital to support the
runoff.

```
RM = CoC × Σ SCR(t) / (1 + r_t+1)^(t+1)
```

CoC = 6% (EIOPA-prescribed).

### SCR — Standard formula

| Module | Sub-risks |
|--------|-----------|
| **Market** | IR, equity, property, spread, FX, concentration |
| **Counterparty default** | Type 1 (reinsurer) + Type 2 (intermediary) |
| **Life underwriting** | Mortality, longevity, lapse, catastrophe, expense, revision, health |
| **Health underwriting** | Similar to life (NSLT / SLT) |
| **Non-life underwriting** | Premium, reserve, catastrophe, lapse |
| **Operational** | Based on premiums or reserves |
| **Intangible asset** | (Limited; typically zero) |

**Aggregation**: SCR_aggregate = sqrt(Σ Σ Corr(i,j) × SCR_i × SCR_j)

EIOPA-prescribed correlation matrix; no diversification between
life and non-life.

### SCR — Internal model

Approval requires:
- Use test (probability distribution vs standard formula)
- Statistical quality tests (calibration, backtest)
- Documentation (policies, methodology, governance)
- Use test ongoing (annual)
- Supervisory review (EIOPA + home supervisor)

Approval takes 1-3 years.

### Own funds

| Tier | Examples | Limits |
|------|----------|--------|
| **Tier 1 unrestricted** | Share capital, retained earnings | ≥ 50% of SCR |
| **Tier 1 restricted** | Subordinated debt (≥10y) | Tier 1 ≤ 100% |
| **Tier 2** | Subordinated debt (5-10y) | Tier 2 ≤ 50% of SCR |
| **Tier 3** | Deferred tax, subordinated debt (<5y) | Tier 3 ≤ 15% of SCR |

### Matching adjustment (MA)

For portfolios with predictable cash flows (annuities, with-
profits):
- Spread on matching assets added to discount rate
- Strict eligibility: matching assets vs liabilities
- Reduces BE; increases SCR (interest rate risk reduced)
- EIOPA approval required

### Volatility adjustment (VA)

For all insurers:
- Adjustment to discount rate based on spreads observed
- Reduces sensitivity to interest rate shocks
- Country-specific reference portfolios

### ORSA (Own Risk and Solvency Assessment)

Forward-looking assessment (3-5 year horizon):
- Current solvency position
- Projected solvency under base + stress
- Reverse stress: scenario that breaks capital
- Management actions in stress

**Annual report**: Board sign-off; reviewed by supervisor.

### SFCR (Solvency and Financial Condition Report)

Public report (per EIOPA template):
- Business and performance
- System of governance
- Risk profile
- Valuation (technical provisions, assets)
- Capital management
- Any material changes during the year

**Audited**: Yes; signed by auditor and Board.

### QRTs (Quantitative Reporting Templates)

EIOPA quarterly + annual templates:
- Balance sheet
- Premium / claims / expenses by line
- Technical provisions by line
- SCR / MCR by module
- Own funds by tier
- Stress test results

XBRL submission to supervisor.

## Examples

### Example 1: SCR — non-life insurer (standard formula)

```
Market SCR: 80M
  IR: 30M; Equity: 20M; Property: 5M; Spread: 25M
Counterparty default SCR: 15M
Non-life SCR: 120M
  Premium: 60M; Reserve: 50M; Cat: 30M (after diversification)
Operational SCR: 18M
Aggregate SCR (with correlation):
  = sqrt(80² + 15² + 120² + 2×corr×products + 18²)
  = ~ 165M (post-diversification)
MCR: 50M (linear + floor)
Solvency ratio: 220 / 165 = 133%
```

### Example 2: SFCR summary

```
A. Business and performance:
  - GWP: 1.2B (motor 40%, property 30%, liability 20%, other 10%)
  - Combined ratio: 95% (attritional 88%, large loss 7%)
B. System of governance:
  - Board, Audit Committee, Risk Committee, Remuneration
  - Three lines of defence documented
C. Risk profile:
  - Top risks: market (40%), non-life underwriting (35%)
  - Stress test: solvency ratio drops to 105% in severe scenario
D. Valuation:
  - Technical provisions: BE 1.5B + RM 80M = 1.58B
E. Capital management:
  - Eligible OF: 220M; SCR: 165M; Ratio: 133%
```

## Anti-patterns

### ❌ Internal model without supervisor approval

Cannot use for regulatory capital; reverted to standard formula.

### ❌ Matching adjustment without eligibility

EIOPA reverses the capital relief; capital shortfall.

### ❌ Tier 2 / 3 over limits

Excess ineligible; SCR coverage dropped.

### ❌ Best estimate without risk margin

Technical provisions understated.

### ❌ ORSA as checkbox exercise

Regulator expects forward-looking, integrated risk assessment.

## Failure modes

| Failure | Recovery |
|---------|----------|
| SCR breach | Recovery plan; capital raise; risk reduction |
| Approval delayed | Continue with standard formula; resubmit |
| Matching adjustment reversed | Recompute SCR; capital action |
| Own funds ineligible | Tier change; equity issuance |
| ORSA rejected | Engage supervisor; revise |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/actuarial` | Underlying actuarial models |
| `/regulatory` | Other Solvency II reports (QRTs, RSR) |
| `/market-risk` | Market SCR components |
| `/credit-risk` | Counterparty default SCR |
| `/liquidity-risk` | Liquidity under Solvency II |

## Citations

- Directive 2009/138/EC (Solvency II)
- EIOPA Guidelines on SFCR / RSR
- EIOPA QRTs templates
- EIOPA Technical Standards