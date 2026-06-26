# Skill: finance

> Capital structure, dividend policy, working capital, M&A, cost
> of capital. Corporate finance is the language of value
> creation — every decision moves WACC or FCF.

## Decision tree

```
Corporate finance question
        │
        ▼
[Step 1] Identify decision (capital / dividend / WC / M&A)
        │
        ▼
[Step 2] Compute cost of capital (WACC)
        │
        ▼
[Step 3] Identify cash flow impact
        │
        ▼
[Step 4] Compare to alternatives (NPV / IRR / sensitivity)
        │
        ▼
[Step 5] Risk impact (rating, leverage, flexibility)
        │
        ▼
[Step 6] Decision + governance
```

## Workflow

### Cost of capital (WACC)

```
WACC = (E/V) × Re + (D/V) × Rd × (1 - t)

Re (cost of equity):
  - CAPM: Rf + β × (Rm - Rf)
  - Multi-factor: + SMB, HML, momentum
  - Size premium (small-cap)
  - Industry premium (specific)

Rd (cost of debt):
  - Effective yield on outstanding debt
  - Or expected yield on new debt
  - Tax shield: × (1 - t)
```

**Source each component**:
- Rf: 10y govt bond (matching currency + duration)
- ERP (Rm - Rf): historical or survey (Damodaran)
- Beta: regression vs market (5y weekly; unlever + relever for target capital structure)
- Size premium: Damodaran; larger for smaller cap

### Capital structure

**Theories**:
- **Modigliani-Miller (no tax)**: capital structure irrelevant; value = FCF / r
- **MM with tax**: debt tax shield; optimal = 100% debt (extreme)
- **Trade-off**: tax shield vs bankruptcy cost; optimal interior
- **Pecking order**: internal > debt > equity (Myers-Majluf)
- **Market timing**: issue equity when overvalued

**Practical decision**:
- Maintain investment-grade rating (or specific leverage target)
- Preserve financial flexibility (revolver headroom, M&A capacity)
- Match debt to asset duration (real assets = long-term debt)
- Currency-match cash flows

**Metrics**:
- Net debt / EBITDA
- Debt / Equity
- EBITDA / Interest coverage
- FFO / Debt (rating agency)

### Dividend policy

**Theories**:
- **Modigliani-Miller**: dividend irrelevant (in perfect markets)
- **Bird-in-hand**: dividends preferred (less uncertainty)
- **Tax preference**: capital gains taxed lower; prefer low dividend
- **Signalling**: dividend change signals management view

**Forms**:
- Cash dividend (regular + special)
- Share buyback (more tax-efficient in many jurisdictions)
- Stock dividend / split (cosmetic)

**Decision factors**:
- Stable FCF generation
- Investment opportunities (reinvest if ROIC > WACC)
- Capital structure targets
- Shareholder preferences (institutional vs retail)
- Tax jurisdiction

### Working capital management

**Components**:
- Inventory: days of supply; turnover
- Receivables: DSO (Days Sales Outstanding); collection
- Payables: DPO (Days Payable Outstanding); stretch

**Cash conversion cycle** = DSO + DIO - DPO

**Optimisation**:
- Just-in-time inventory (cost of stockouts)
- Receivables: credit policy; factoring; supply chain finance
- Payables: longer payment terms; reverse factoring
- Net working capital as % of revenue; trending

**Trade-offs**:
- Lower WC = more cash, but risk of stockouts / customer dissatisfaction
- Higher WC = smoother operations, but tied-up cash
- Industry-specific norms (retail vs SaaS very different)

### M&A process

| Phase | Output |
|-------|--------|
| **Strategy** | Why M&A; what we buy |
| **Targeting** | Long list → short list |
| **Due diligence** | Financial, legal, tax, commercial, tech, HR |
| **Valuation** | Range of value; offer price |
| **Deal structuring** | Cash / stock / mix; CVR; earn-out |
| **SPA / APA** | Representations, warranties, indemnities |
| **Closing** | Conditions precedent; regulatory approval |
| **Integration** | Day 1 plan; 100-day plan; synergy capture |

**Synergy types**:
- Revenue (cross-sell, geographic)
- Cost (duplication removal, scale)
- Financial (lower WACC, tax)
- Strategic (market position, IP)

**Risks**:
- Cultural integration (often the biggest)
- Customer / employee attrition
- Overpayment (winner's curse)
- Synergy underdelivery
- Regulatory / antitrust

### Capital budgeting

| Method | Use |
|--------|-----|
| **NPV** | Gold standard; discounts FCF |
| **IRR** | When NPV hard to communicate; project IRR vs hurdle |
| **Payback** | Quick screen; ignores time value |
| **Profitability index** | When capital constraint binds |
| **Real options** | When project has optionality |

**Hurdle rate** = WACC + project-specific risk premium (if any).

## Examples

### Example 1: capital structure decision

```
Current: 30% debt / 70% equity; net debt $1B; EBITDA $400M
Target: 40% debt / 60% equity
New debt: $400M (added)
Use: share buyback ($400M)
Trade-offs:
  + Tax shield (5%)
  + EPS accretion
  - Rating downgrade risk
  - Reduced flexibility
Decision: maintain 35% / 65%; gradual buyback
```

### Example 2: dividend policy

```
FCF: $200M
Capex: $100M
Free cash to return: $100M
Options:
  - Dividend: $1.00/share × 50M shares = $50M (50% payout)
  - Buyback: $50M @ $50 = 1M shares (2% reduction)
  - Mix: $30M dividend + $70M buyback
Rationale: stable dividend + opportunistic buyback
```

### Example 3: working capital optimisation

```
Current:
  DSO: 60 days
  DIO: 90 days
  DPO: 45 days
  CCC: 105 days
  Revenue: $500M; NWC = $144M

Target:
  DSO: 50 (-10 days; factoring top customers)
  DIO: 75 (-15 days; JIT)
  DPO: 50 (+5 days; renegotiate)
  CCC: 75 days
  Cash freed: $41M
```

## Anti-patterns

### ❌ WACC from unadjusted CAPM

Missing size / industry premium. Cost of capital understated.

### ❌ Optimal capital structure without trade-offs

"Just go to 60% debt" without considering bankruptcy cost /
flexibility / rating.

### ❌ Dividend policy without cash flow

Promising dividend you can't fund = credit event.

### ❌ Working capital optimisation without relationships

Stretching payables damages supplier relationships.

### ❌ M&A synergy without integration plan

Synergies on slide; never realised.

## Failure modes

| Failure | Recovery |
|---------|----------|
| WACC too low → projects accepted that destroy value | Recompute; tighten hurdle |
| Overlevered → downgrade; covenant breach | De-lever; asset sales; equity issuance |
| Dividend cut | Communicate; align with FCF; bridge |
| WC optimisation → stockouts | Inventory policy; safety stock |
| M&A integration fails | Bring in integration team; milestone plan |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/valuation` | DCF for project valuation |
| `/capital-markets` | Debt issuance; equity offerings |
| `/accounting` | Goodwill; acquisition accounting |
| `/controlling` | Forecast tied to plan |
| `/credit-risk` | Rating impact; covenant analysis |