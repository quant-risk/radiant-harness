# Skill: stress-test

> Scenario design, sensitivity, regulatory stress (CCAR, EBA,
> BCBS), satellite models, recovery planning. Stress tests are
> stories about the future; the narrative must be plausible.

## Decision tree

```
Stress question
        │
        ▼
[Step 1] Identify stress type (sensitivity / scenario / reverse)
        │
        ▼
[Step 2] Choose scenario (historical / hypothetical / reverse)
        │
        ▼
[Step 3] Define macro path + risk-factor shocks
        │
        ▼
[Step 4] Build satellite models (macro → risk parameter)
        │
        ▼
[Step 5] Apply shocks to portfolio
        │
        ▼
[Step 6] Compute stressed P&L + capital + RWA
        │
        ▼
[Step 7] Define management actions
        │
        ▼
[Step 8] Report + governance sign-off
```

## Workflow

### Scenario types

| Type | Description | When |
|------|-------------|------|
| **Sensitivity** | Single-factor shock (e.g. rates +200bp) | Tactical; quick screen |
| **Scenario** | Coherent multi-factor (recession: rates + GDP + unemployment) | Strategic; capital planning |
| **Reverse** | Find scenario that breaches capital | Identify tail; recovery planning |
| **Historical** | Replay 2008, 2020, 2022 | Sanity check; intuition |
| **Hypothetical** | Plausible but never-occurred | Forward-looking |

### Severity ladder

Typical 3-scenario design:
1. **Baseline**: central forecast
2. **Adverse**: mild-to-moderate recession
3. **Severely adverse**: deep recession + market dislocation

### Macro variables

| Variable | Typical range | Source |
|----------|---------------|--------|
| GDP growth | -10% to +5% | IMF / OECD |
| Unemployment | +200 to +600 bps | BLS / Eurostat |
| Interest rate | -200 to +400 bps | Central bank paths |
| Equity index | -10% to -55% | Historical episodes |
| House prices | -10% to -35% | BIS / national |
| Credit spreads | +50 to +300 bps | Historical |

### Satellite models

Link macro → risk parameter:
- **GDP → PD**: PD = f(GDP, unemployment, sector)
- **Interest rates → prepayment**: f(rate, age, season)
- **Equity → LGD**: f(equity, collateral)
- **Macro → drawdowns**: f(GDP, sector)

Avoid direct 1:1 scaling: "GDP drops 5%, PD increases 5%" ignores
non-linearity.

### CCAR / DFAST (US)

| Feature | Detail |
|---------|--------|
| Frequency | Annual |
| Horizon | 9 quarters |
| Scenarios | Baseline + Adverse + Severely Adverse (Fed-defined) |
| Output | Stressed capital ratios + capital plan |
| Bank size | BHCs > $100B |
| Public | Yes (CCAR); DFAST semi-public |

**Stress capital buffer (SCB)**: max(0, 2.5% + (4.5% - stressed
CET1 ratio)).

### EBA stress test (EU)

| Feature | Detail |
|---------|--------|
| Frequency | Biennial (even years) |
| Horizon | 3 years |
| Scenarios | Baseline + Adverse |
| Output | Stressed CET1 + risk-weighted metrics |
| Bank size | SSM-supervised (significant institutions) |
| Public | Yes (aggregate); individual bank data later |

### BCBS stress test

Bottom-up vs top-down. Used for global SIBs; informs G-SIB
buckets.

### EIOPA stress test (insurance)

For insurers; tests market, underwriting, combined shocks.
Biennial.

### Reverse stress test

1. Define solvency breach (e.g. SCR ratio < 100%)
2. Back-solve: which scenario causes this?
3. Identify vulnerability drivers
4. Document management actions to prevent

### Recovery and resolution

**Recovery plan**: actions insurer can take to restore solvency
without regulatory intervention:
- Asset sales
- Capital raise
- Reinsurance restructuring
- New business restrictions
- Lapse assumption changes (life)

**Resolution plan**: actions regulator can take:
- Portfolio transfer
- Bridge institution
- Bail-in
- Statutory administration

### Management actions

For each scenario, define:
- Capital actions (raise, dividend cut)
- Asset actions (sell, hedge)
- Liability actions (lapse incentive, reinsurance)
- Operational actions (cost reduction)

**Limitation**: only actions under firm control within scenario
timeframe. No "we'll just raise $10B in Q2" if market is closed.

## Examples

### Example 1: CCAR-like stress (US bank)

```
Scenario: severely adverse
Horizon: 9 quarters
Macro path:
  GDP: -3.5% (cumulative)
  Unemployment: 10% peak
  Equity: -55% (cumulative)
  House prices: -25%
  Credit spreads: +200 bps
Satellite:
  PD multiplier: 1.8x in year 1, peak 2.5x in year 2
  LGD: +500 bps
Stressed results:
  CET1 ratio: 12.0% baseline → 8.5% severely adverse
  SCB: max(0, 2.5 + (4.5 - 8.5))... actually max(0, 2.5%) + SCB from stress
  Final SCB: 5.5% (per Fed methodology)
Capital plan: suspend buybacks; raise $2B if needed
```

### Example 2: reverse stress (insurer)

```
Define breach: SCR ratio < 100%
Back-solve: what combination of mortality shock + market drop?
Required: mortality +30% AND equity -40% simultaneously
Narrative: pandemic + market crash
Mitigation: pandemic excess-of-loss reinsurance; equity hedge program
Document: scenario in ORSA
```

### Example 3: sensitivity (single-factor)

```
Interest rate +200bp shock:
  - Bond portfolio: -5% mark-to-market
  - Annuity liabilities: -8% (convexity)
  - Net economic impact: +3% (positive for insurer)
Equity -30% shock:
  - Equity portfolio: -30% (no hedge)
  - Fund unit-linked liabilities: -30% (matched)
  - Net economic impact: 0 (matched)
Credit spread +200bp:
  - Corporate bond portfolio: -8%
  - Net economic impact: -8%
```

## Anti-patterns

### ❌ Sensitivity without scenario

Single-factor shock lacks narrative. Always pair with scenarios.

### ❌ Direct macro → risk parameter

Non-linearity matters. Use satellite models.

### ❌ No management actions

Adverse outcome with no recovery plan = just a number.

### ❌ Single scenario

Missing tail risk. Always multiple scenarios.

### ❌ Different methodology in stress vs baseline

Comparability lost. Same model framework throughout.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Severity too lenient | Add severely adverse scenario |
| Recovery plan infeasible | Re-design with realistic constraints |
| Result not comparable to baseline | Same methodology |
| Supervisor challenge | Engagement; explanation; revise |
| Macro shock unrealistic | Satellite model calibration |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/credit-risk` | Stress on credit portfolio |
| `/market-risk` | Market shocks; VaR/ES in stress |
| `/liquidity-risk` | Liquidity stress (LCR in stress) |
| `/operational-risk` | Op risk stress |
| `/actuarial-solvency` | Insurance stress under Solvency II |

## Citations

- Fed SR 12-3 / CCAR instructions (annual update)
- EBA Stress Test Methodological Note (biennial)
- BCBS Stress Testing Principles (2017)
- EIOPA Insurance Stress Test (biennial)