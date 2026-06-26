# Skill: accounting

> IFRS / BR GAAP / US GAAP, financial statements, consolidation,
> fair value, hedge accounting, lease, impairment. Every
> accounting policy is a defensible judgment under audit.

## Decision tree

```
Accounting question
        │
        ▼
[Step 1] Identify framework (IFRS / US GAAP / BR CPC)
        │
        ▼
[Step 2] Identify standard + paragraph
        │
        ▼
[Step 3] Document accounting policy memo
        │
        ▼
[Step 4] Document judgment + estimates
        │
        ▼
[Step 5] Implement + journal entries
        │
        ▼
[Step 6] Audit trail + disclosure
```

## Workflow

### IFRS framework (Brazil CPC)

| IFRS | CPC | Topic |
|------|-----|-------|
| IFRS 9 | CPC 48 | Financial instruments |
| IFRS 10 | CPC 36 | Consolidated financial statements |
| IFRS 11 | CPC 37 | Joint arrangements |
| IFRS 12 | CPC 45 | Disclosure of interests in other entities |
| IFRS 13 | CPC 46 | Fair value measurement |
| IFRS 15 | CPC 47 | Revenue from contracts with customers |
| IFRS 16 | CPC 06 | Leases |
| IFRS 17 | CPC 50 | Insurance contracts |
| IAS 1 | CPC 26 | Presentation of financial statements |
| IAS 36 | CPC 01 | Impairment of assets |
| IAS 37 | CPC 25 | Provisions, contingent liabilities |
| IAS 38 | CPC 04 | Intangible assets |

### Consolidation (IFRS 10)

Control = power + exposure to variable returns + ability to use
power to affect returns.

| Scenario | Treatment |
|----------|-----------|
| >50% voting rights | Subsidiary; full consolidation |
| 20-50% + significant influence | Associate; equity method |
| Joint control | Joint venture (equity) or joint operation (line-by-line) |
| Structured entity | Control if substance over form |

Process: trial balance → elimination entries → consolidated FS.

### Revenue recognition (IFRS 15)

5-step model:

1. **Identify the contract** (parties, rights, payment terms, approved)
2. **Identify performance obligations** (distinct goods/services)
3. **Determine transaction price** (fixed + variable consideration, time value of money)
4. **Allocate transaction price** to performance obligations (stand-alone selling price)
5. **Recognise revenue** when (or as) obligation satisfied (point in time vs over time)

Common judgments:
- Variable consideration (rebates, refunds): constrained estimate
- Stand-alone selling price: observable or estimated
- Over-time vs point-in-time: criteria in IFRS 15.35

### Lease (IFRS 16)

Lessee: recognise right-of-use asset + lease liability (almost
all leases).

| Component | Treatment |
|----------|-----------|
| Lease liability | PV of lease payments + options + guarantees - incentives |
| ROU asset | Liability + initial direct costs - lease incentives |
| Depreciation | Straight-line over lease term (or useful life if ownership transfers) |
| Interest | Effective interest method |

Lessor: classify as finance lease (5 criteria) or operating lease.

### Hedge accounting (IFRS 9)

| Hedge type | Hedging instrument | Hedged item |
|-----------|--------------------|--------------|
| **Fair value hedge** | Derivative | Asset/liability at fair value |
| **Cash flow hedge** | Derivative | Forecast transaction / variable cash flow |
| **Net investment hedge** | Derivative | Net investment in foreign operation |

Documentation required:
- Risk management objective + strategy
- Hedging instrument
- Hedged item
- How effectiveness will be assessed

Effectiveness: 80-125% (IAS 39); IFRS 9 has economic relationship
test.

### Fair value (IFRS 13)

| Level | Inputs | Examples |
|-------|--------|----------|
| **Level 1** | Quoted prices in active markets | Listed equity, government bonds |
| **Level 2** | Observable inputs other than L1 | Interest rate swaps, corporate bonds |
| **Level 3** | Unobservable inputs | Private equity, complex derivatives |

Level 3 = significant judgment; valuation process documented;
sensitivity disclosed.

### Impairment (IAS 36 + IFRS 9)

**IAS 36**: recoverable amount = max(fair value less costs to
sell, value in use). If carrying > recoverable → impairment loss.

Indicators of impairment (annual test):
- Significant decline in market value
- Adverse changes in technology, market, economic environment
- Interest rate increases affecting discount rate

Goodwill + intangibles with indefinite life: annual impairment test
regardless of indicators.

**IFRS 9 (financial assets)**: ECL = EAD × PD × LGD.
- Stage 1: 12-month ECL
- Stage 2: lifetime ECL (SICR)
- Stage 3: lifetime ECL (credit-impaired)

### Tax

| Component | Treatment |
|-----------|-----------|
| **Current tax** | Tax payable / receivable for current period |
| **Deferred tax** | Temporary differences; tax loss carryforwards |
| **Uncertain tax positions** | IFRIC 23 / FIN 48; probability assessment |

Deferred tax: balance sheet liability method; applied to all
temporary differences (asset/liability approach).

## Examples

### Example 1: revenue (SaaS subscription)

```
Contract: $120k/year subscription; 1-year term
Performance obligation: access to platform
Transaction price: $120k (fixed)
Recognition: ratably over 12 months (over-time)
Journal entries (monthly):
  Dr. Accounts receivable  10k
  Cr. Deferred revenue    10k
  (upon billing)
  Dr. Deferred revenue    10k
  Cr. Revenue             10k
  (monthly recognition)
```

### Example 2: lease (office space, 5y)

```
Lease: 5 years, $100k/year, payable monthly
Implicit rate: 5%
PV of payments: $432k
Initial recognition:
  Dr. ROU asset       432k
  Cr. Lease liability 432k
Monthly:
  Dr. Interest expense (declining balance)
  Cr. Lease liability
  Dr. Depreciation    7.2k
  Cr. Accumulated depreciation
```

### Example 3: hedge (cash flow hedge of forecast sales)

```
Forecast: $10M USD sales in 6 months
Hedge: forward contract to sell $10M USD @ fixed rate
Documentation:
  - Hedged item: forecast USD sales (highly probable)
  - Hedging instrument: forward FX
  - Risk: USD/BRL FX volatility
  - Effectiveness: dollar-offset test
OCI: gains/losses on forward go to OCI
Reclassified to P&L when sales occur
```

## Anti-patterns

### ❌ Handwave "based on accounting principles"

No IFRS citation = indefensible at audit. Always cite.

### ❌ Judgment without documentation

Estimates undocumented; no audit trail. Document every judgment.

### ❌ Manual consolidation without controls

Spreadsheet-only = errors. Use consolidation software with
controls; reconciliations.

### ❌ Revenue on contract signature

Premature recognition. Performance obligation must be satisfied.

### ❌ Hedge without documentation

Ineffective by default. Document strategy, instrument, item,
effectiveness assessment.

### ❌ Level 3 fair value without process

Unobservable inputs need valuation process; sensitivity disclosed.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Restatement required | Investigate; quantify; restate comparatives |
| Audit qualification | Address finding; restate; remediate controls |
| Tax dispute | IFRIC 23 analysis; legal review; reserve |
| Impairment missed in year | Catch up next year; disclose |
| Hedge ineffective | Stop hedge accounting; reclassify to P&L |
| Consolidation error | Investigate; restate; tighten controls |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/valuation` | Fair value; impairment testing |
| `/controlling` | Management accounts; variance analysis |
| `/capital-markets` | Financial instruments; disclosures |
| `/finance` | Corporate finance context |