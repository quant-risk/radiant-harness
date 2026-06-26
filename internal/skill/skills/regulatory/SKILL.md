# Skill: regulatory

> Regulatory reporting (BCBS, BACEN, CVM, Fed, EBA, etc.),
> compliance, capital + liquidity, disclosure. Regulatory
> submissions are an exam artifact; reconcile to source.

## Decision tree

```
Regulatory submission
        │
        ▼
[Step 1] Identify regulator + report + deadline
        │
        ▼
[Step 2] Source data + lineage
        │
        ▼
[Step 3] Apply regulatory rules (different from accounting)
        │
        ▼
[Step 4] Reconciliation to accounting + variance investigation
        │
        ▼
[Step 5] Independent review
        │
        ▼
[Step 6] Submission + sign-off
```

## Workflow

### Brazilian regulators

| Regulator | Institution | Reports |
|-----------|-------------|---------|
| **BACEN** | Central Bank | DLO (prudential), DLI, IOC, DPM |
| **CVM** | Securities Commission | ITR (quarterly), DFP (annual), FIDCs |
| **SUSEP** | Private insurance | FIP/SUSEP quarterly |
| **ANS** | Health | SIB, ANS-EF |
| **PREVIC** | Pension | DCAA, DPREV |

**BACEN DLO (Demonstrações Financeiras Padronizadas)**:
- Monthly/quarterly
- Capital adequacy (Basileia)
- Liquidity (LCR/NSFR)
- Credit risk exposures
- Market risk (Rban)

**BACEN IOC (Informações de Operações com Derivativos)**:
- OTC derivatives positions
- Counterparty exposures

**CVM ITR / DFP**:
- Quarterly (ITR) + annual (DFP) financial statements
- Public companies + investment funds

### US regulators

| Regulator | Institution | Reports |
|-----------|-------------|---------|
| **Fed** | Federal Reserve | FR Y-9C (consolidated), Call Reports |
| **OCC** | Office of the Comptroller | Call Reports |
| **FDIC** | Federal Deposit Insurance | Call Reports, DIF |
| **SEC** | Securities and Exchange | 10-K, 10-Q, 8-K; investment adviser; fund |

**FFIEC Call Report**: quarterly, all US banks.

**FR Y-9C**: bank holding company consolidated.

**CCAR / DFAST**: stress test (Fed; large BHCs).

### EU regulators

| Regulator | Institution | Reports |
|-----------|-------------|---------|
| **EBA** | Banking | COREP, FINREP |
| **ESMA** | Securities | AIFMD, MiFID II |
| **EIOPA** | Insurance | Solvency II SFCR / RSR |
| **ECB** | Central bank | SSM (single supervisory mechanism) |

### Basel III / IV capital reporting

**Pillar 1**: minimum capital requirements (credit, market, operational).

**Pillar 2**: supervisory review; ICAAP, SREP.

**Pillar 3**: market discipline; disclosures (capital, risk, RWA).

**Capital ratios**:
- CET1 >= 4.5% (+ 2.5% capital conservation buffer)
- Tier 1 >= 6%
- Total capital >= 8%
- Leverage ratio >= 3-4%

**Output floor** (Basel IV): RWA from internal models >= 72.5% of
standardised RWA.

### Reconciliation: regulatory vs accounting

Differences:
- **Scope of consolidation**: prudential vs accounting
- **Asset valuation**: amortised cost vs fair value
- **Provisions**: incurred (IFRS 9 Stage 1/2/3) vs expected loss
  (regulatory)
- **Capital deductions**: goodwill, intangibles, deferred tax

Variance investigation:
- Document each adjustment
- Sign-off by both finance and risk
- Auditor review if material

### Data lineage

End-to-end:
```
Source system (loan tape)
  → Data warehouse (validated)
  → Risk engine (PD/LGD/EAD)
  → RWA calculator
  → Regulatory submission template
  → Submitted
```

Every step logged; auditable; version-controlled.

### Sign-off

Typical chain:
1. **1st line**: data owner / business unit
2. **2nd line**: risk / compliance review
3. **Internal audit**: periodic review (annual)
4. **External audit**: regulatory submission attestation
5. **Board / risk committee**: high-level sign-off

### Common pitfalls

| Pitfall | Recovery |
|---------|----------|
| Late submission | Calendar tracking; escalation; fines |
| Reconciliation gap | Investigate; document; sign-off |
| Methodology change undocumented | Version control; regulator notification |
| Data quality issue | Re-run; remediation; explanation |
| New product not in scope | Map to existing; get regulatory clarity |
| Cross-border inconsistency | Coordinate; align definitions |

## Examples

### Example 1: BACEN DLO capital adequacy

```
CET1: 60B
Tier 1: 65B
Total capital: 75B
RWA: 700B

CET1 ratio: 8.6% (vs requirement 4.5% + 2.5% conservation = 7%)
Tier 1 ratio: 9.3%
Total ratio: 10.7%
Leverage ratio: 8% (Tier 1 / exposure)

Buffer: 1.6% above conservation requirement (8.6% - 7%)
```

### Example 2: Solvency II SFCR

```
Eligible own funds: 500M
SCR: 350M (1-in-200, 1y)
MCR: 175M (linear + cap)

Solvency ratio: 500 / 350 = 143% (>100%, healthy)
SCR coverage: comfortable
```

### Example 3: CCAR stress test (US BHC)

```
Baseline: CET1 ratio 12.0%
Severely adverse scenario: projected CET1 ratio 8.5%
Stress capital buffer: 5.5%
Action: capital plan + buyback approval

Submission: April; results: June; capital actions: July-December
```

## Anti-patterns

### ❌ Late submission

Penalty + reputational. Calendar adherence.

### ❌ Reconciliation gap unexplained

Restatement risk; examiner finding.

### ❌ Methodology change undocumented

"Discovered" at exam; remediation; consent order.

### ❌ Single source of truth not maintained

Different numbers to different stakeholders; audit finding.

### ❌ Data lineage gap

Can't trace from submission back to source; examiner red flag.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Missed deadline | Filing extension; remediation plan |
| Restatement | Investigate; restate; re-submit |
| Examiner finding | MRIA (Matter Requiring Attention); remediation |
| Methodology rejected | Re-design; regulator dialogue |
| Cross-border inconsistency | Engage home + host supervisor |
| Pillar 2 add-on | Capital plan; risk reduction |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/credit-risk` | RWA calculation; Basel IRB |
| `/market-risk` | Market risk capital; FRTB |
| `/liquidity-risk` | LCR / NSFR reporting |
| `/operational-risk` | Op risk capital; loss data |
| `/accounting` | Reconciliation to accounting |
| `/tax` | Tax effect on capital |

## Tools

| Tool | Purpose |
|------|---------|
| **SAS Regulatory Reporting** | Multi-jurisdiction reporting |
| **Wolters Kluwer OneSumX** | Regulatory reporting |
| **Moody's RiskOrigin** | Regulatory capital |
| **MSCI / Axiom** | Basel reporting |
| **In-house** | Often; spreadsheets + validation |

## Citations

- BCBS standards (Basel III, Basel IV final)
- Resolução CMN 4.557 (BR — Basileia III implementation)
- Circular SUSEP 648/2022 (insurance)
- Solvency II Directive 2009/138/EC
- IFRS Standards (IASB)