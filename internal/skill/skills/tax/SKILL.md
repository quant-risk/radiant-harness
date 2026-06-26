# Skill: tax

> Corporate income tax, transfer pricing, VAT/ICMS, treaties,
> BEPS Pillar 2. Tax is the largest single P&L line in many
> companies; get the doctrine right.

## Decision tree

```
Tax question
        │
        ▼
[Step 1] Identify tax type + jurisdiction
        │
        ▼
[Step 2] Cite statute + regulation
        │
        ▼
[Step 3] Substance test + anti-avoidance
        │
        ▼
[Step 4] Treaty / domestic interaction
        │
        ▼
[Step 5] Computation + memo
        │
        ▼
[Step 6] Documentation on file
```

## Workflow

### Corporate income tax (BR)

**Bases**:
- Lucro Real (>= R$78M revenue or optional): taxable = accounting
  profit + adjustments (additions, exclusions, compensations)
- Lucro Presumido (R$4.8-78M): presumed profit (8%, 16%, 32% by
  activity) × revenue
- Simples Nacional (≤ R$4.8M): unified tax on revenue

**Adjustments (Lucro Real)**:
- Additions: non-deductible expenses, provisions not deductible
- Exclusions: dividends received, equity-method income
- Compensations: negative basis from prior periods (limit 30%)

**Tax rate**: 15% + 10% surtax over R$240k/quarter (or R$20k/month).

**State tax (ICMS)**: state VAT on goods + services + interstate
transport. 7-18% depending on state + product.

**Service tax (ISS)**: municipal tax on services; 2-5%.

**Payroll**: INSS (employer social contribution) + FGTS +
ratings.

### Deferred tax (IFRS / CPC 32)

Temporary differences:
- **Taxable**: future tax → deductible (e.g. accelerated
  depreciation for tax)
- **Deductible**: future tax → payable (e.g. warranty provision)

DTA = deductible × tax rate; recognised if "probable" future
taxable profit.

DTL = taxable × tax rate; always recognised.

### Transfer pricing (BR / OECD)

**BR (old rules, Lei 14.596/2023 transition)**:
- Import: 60% margin (PRL), or comparable uncontrolled price
- Export: 28% margin (commodities), or CUP / resale price
- Services: cost + margin

**OECD (Pillar 2 / BEPS 2.0 era)**:
- Arm's length principle: comparable uncontrolled price (CUP),
  resale price, cost plus, transactional net margin, profit split

**Documentation**:
- Master file (group-wide)
- Local file (Brazilian entity)
- CbCR (country-by-country report)

**Functional analysis**: functions, assets, risks (FAR).

### Indirect taxes

| Tax | Jurisdiction | Base |
|-----|-------------|------|
| **VAT** (EU) | EU | Value added at each stage |
| **GST** (AU/CA/IN) | Multi | Goods + services |
| **ICMS** (BR) | State | Goods + services + transport |
| **IPI** (BR) | Federal | Industrialised products |
| **PIS/COFINS** (BR) | Federal | Revenue (cumulative/non-cumulative) |
| **Sales tax** (US state) | State | Retail sales |

VAT mechanics:
- Output VAT (collected on sales)
- Input VAT (paid on purchases)
- Net payable = output - input

### Treaties

Double tax treaties (DTTs) typically:
- Reduced WHT rates
- Tie-breaker for residency
- Mutual agreement procedure (MAP)
- Exchange of information

Brazil has DTTs with ~35 countries; UK DTT (1968), Netherlands
DTT, etc. Rates vary.

**Limitation on Benefits (LOB)**: anti-treaty-shopping; substantial
activity test, ownership test.

**Principal Purpose Test (PPT)**: anti-abuse; deny if main purpose
is treaty benefit.

### BEPS Pillar 2

**GloBE rules** (Global Anti-Base Erosion):
- 15% global minimum effective tax rate (ETR)
- IIR (Income Inclusion Rule): top-up tax at parent
- UTPR (Undertaxed Profits Rule): backstop
- QDMTT (Qualified Domestic Minimum Top-up Tax): country can
  pre-empt with own top-up tax

**Brazil**: Lei 14.596/2023 introduced CBC (Contribuição sobre
Bens e Serviços) to replace PIS/COFINS/ICMS/ISS — major reform.

### Withholding tax

| Type | Typical rate |
|------|--------------|
| **Dividends** (BR domestic) | 0% |
| **Dividends** (to non-resident) | 0% (post-1995 legislation) |
| **Interest** (to non-resident) | 15% or treaty rate |
| **Royalties** (to non-resident) | 15% or treaty rate |
| **Services** (to non-resident) | 15% or 25% |

### Tax controversy

**Audit defence**:
- Documentation contemporaneous
- Substance test passed
- Anti-avoidance arguments addressed
- Procedural compliance

**Levels**: administrative (CARF in BR), judicial.

## Examples

### Example 1: Lucro Real computation (BR)

```
Accounting profit: 1,000
Adjustments:
  + Non-deductible fines: 50
  + Provision for doubtful accounts (limit): 30
  - Reversal of contingent provision: 20
  - Equity-method income (already taxed): 100
  - Tax loss compensation: -200
Taxable profit: 760

Tax: 15% × 760 + 10% × (760-240) = 114 + 52 = 166
Effective rate: 16.6%
```

### Example 2: transfer pricing (import of services, BR)

```
Intercompany: BR parent pays USD 1M to US subsidiary for mgmt services
Cost basis (US sub): USD 700k
Margin required (BR old rules, services): cost + 5%
TP adjustment: 1,000 - 700 × 1.05 = 1,000 - 735 = 265 (income)
Add to Lucro Real: 265k × R$/USD rate → BRL impact
```

### Example 3: BEPS Pillar 2 GloBE

```
Multinational: 5 jurisdictions
Jurisdiction A ETR: 8% (low-tax)
Pillar 2 top-up rate: 15% - 8% = 7%
Top-up tax: 7% × GloBE income in A = 7% × 100M = 7M
Collected at parent (IIR) or via UTPR if no IIR
```

## Anti-patterns

### ❌ Treaty benefit without beneficial owner test

WHT exposure; re-characterisation by tax authority.

### ❌ TP documentation only at audit time

Weak defence; contemporaneous documentation required.

### ❌ Tax structure without substance

Anti-avoidance rules (GAAR in BR, BEPS in OECD) apply.

### ❌ Deferred tax on temporary differences without recoverability test

DTA may need valuation allowance; check future taxable profit.

### ❌ Tax expense = book tax without reconciliation

Deferred tax + current tax = total; reconciliation required.

### ❌ Single jurisdiction assumption

Multinationals have transfer pricing, treaty, withholding issues
across borders.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Tax assessment | Administrative appeal; judicial; documentation |
| Treaty re-characterisation | MAP (Mutual Agreement Procedure) |
| Pillar 2 top-up tax | Restructure; consider QDMTT |
| Withholding exposure | Documentation; treaty claim; refund |
| DTA not recoverable | Valuation allowance; recalibrate forecasts |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/accounting` | Tax provisioning (IFRS / CPC 32) |
| `/finance` | M&A structuring; cross-border |
| `/valuation` | Tax effect on valuation |
| `/capital-markets` | Cross-border securities |

## Citations

- Brazilian Federal Revenue (Receita Federal) — Instruções Normativas
- OECD Model Tax Convention
- OECD BEPS 2.0 Pillar 2 Model Rules
- Deloitte / EY / PwC / KPMG International Tax Guides