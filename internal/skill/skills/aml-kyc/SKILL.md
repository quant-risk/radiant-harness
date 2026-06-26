# Skill: aml-kyc

> AML / KYC / sanctions / PEP / SAR. AML is the largest
> compliance programme at most banks; failing at it is a
> consent-order event.

## Decision tree

```
New customer / transaction
        │
        ▼
[Step 1] Customer Identification Program (CIP)
        │
        ▼
[Step 2] Customer Due Diligence (CDD)
        │
        ├── Low risk          -> SDD
        ├── Medium           -> CDD
        └── High / PEP / high-risk jurisdiction -> EDD
        │
        ▼
[Step 3] Sanctions screening (real-time, every list update)
        │
        ▼
[Step 4] PEP screening (ongoing)
        │
        ▼
[Step 5] Beneficial ownership (>= 25%; lower if high risk)
        │
        ▼
[Step 6] Ongoing monitoring (transactions + list updates + risk refresh)
        │
        ▼
[Step 7] SAR / STR if suspicious activity
```

## Workflow

### Customer Identification Program (CIP)

Minimum identifiers (FATF Rec 10):
- Legal name
- Address (residential / registered)
- Date of birth (individuals) / incorporation (entities)
- Government-issued ID number
- For entities: registration number, jurisdiction

Verify identity via reliable, independent sources (ID + database
check).

### Customer Due Diligence (CDD)

| Risk | Action |
|------|--------|
| **Low** | SDD: name + ID + address verified |
| **Medium** | CDD: source of wealth, expected activity, ID verification |
| **High** | EDD: source of funds, senior management approval, enhanced monitoring |

Risk factors: PEP, high-risk jurisdiction (FATF list), complex
ownership, cash-intensive, unusual activity.

### Sanctions screening

Lists (minimum):
- **OFAC SDN** (US)
- **EU Consolidated** (EU)
- **UN Sanctions**
- **UK HMT** (UK)
- Local jurisdiction list

Fuzzy matching: name + DOB + nationality + ID; threshold calibrated
to minimise false negatives (e.g. 85% match → manual review).

**Real-time** at:
- Customer onboarding
- Before each payment
- On every list update (typically daily)

### PEP screening

PEP = politically exposed person. Categories:
- Domestic PEP (head of state, senior officials)
- Foreign PEP (equivalent in other country)
- International organisation PEP (UN, IMF, etc.)
- Family / close associate of PEP

**EDD required** for PEPs: source of wealth, senior management
approval, enhanced ongoing monitoring.

Refresh: annually for PEPs (promotion, change of role).

### Beneficial ownership (FATF Rec 24)

Identify natural persons who own or control:
- 25% ownership (or lower if higher risk)
- Control through other means (e.g. voting rights, board)

Verify identity of beneficial owners.

### Transaction monitoring

Rules:
- **Threshold**: amount above X (e.g. $10k for CTR)
- **Velocity**: N transactions in time T
- **Pattern**: structuring (multiple sub-threshold), unusual
  jurisdictions, unusual counterparties
- **Behavioural**: deviation from customer profile

**False-positive management**: monitor ratio; calibrate quarterly.

### SAR / STR

When suspicious activity detected:
1. Internal investigation (don't tip off customer)
2. Compliance / MLRO decision (file or close)
3. SAR/STR filing to FIU (Financial Intelligence Unit)
4. Documentation (decision, rationale, supporting evidence)

**No tipping off**: communicating SAR to subject is a criminal
offence in most jurisdictions.

### Correspondent banking (FATF Rec 13)

EDD for correspondent banks:
- Respondent's AML controls
- Respondent's jurisdiction
- Use of "payable-through accounts"

Wolfsberg Questionnaire + AML questionnaire standard.

### Country risk

Use FATF mutual evaluations + EU high-risk third country list +
OFAC sanctions programmes + Transparency International CPI.

Higher risk → EDD; restrictions on correspondent banking.

## Examples

### Example 1: high-risk customer onboarding (PEP)

```
Customer: foreign senior government official
CDD level: EDD
Source of wealth: documented (3+ years of evidence)
Source of funds: documented (transaction-level)
Approval: senior management sign-off (Compliance + Business)
PEP screening: confirmed; refresh annually
Sanctions: clear (no list match)
Ongoing monitoring: enhanced; alerts on any change
```

### Example 2: transaction monitoring alert

```
Alert: customer X sends $9,500 to 5 different accounts in 24h
Rule: structuring (sub-threshold)
Investigation: customer is small business; pattern unusual
SAR decision: file SAR (pattern + amount + velocity)
Documentation: alert ID, customer profile, transactions,
                investigation notes, decision rationale
Filing: to FinCEN within 30 days
```

### Example 3: sanctions hit

```
Customer Y matches OFAC SDN at 92% (name similarity)
Real-time screening: triggered at payment
Action: payment blocked; alert to Compliance
Investigation: reviewed ID + DOB; confirmed false positive (different person)
Documentation: alert ID, similarity score, decision, evidence
Process: clear from alert queue; log for audit
```

## Anti-patterns

### ❌ Sanctions screening batch-only

Payments processed before screening. Real-time required.

### ❌ Beneficial ownership < 25%

Misses shell companies. Lower threshold for high risk.

### ❌ PEP screening at onboarding only

Doesn't catch later-promoted PEPs. Ongoing + annual refresh.

### ❌ TM rules with high false-positive rate

Alert fatigue; missed true positives. Calibrate.

### ❌ Tip-off on SAR

Criminal offence. Internal investigation only.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Sanctions hit missed | Re-screen all pending payments; report to OFAC |
| False positive flood | Recalibrate rules; tune thresholds |
| PEP not screened on promotion | Backfill; add to monitoring |
| Beneficial ownership gap | Re-collect; document; remediation |
| SAR late | File; document reason; report to compliance |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/security` | Cyber risk, access control |
| `/data` | Customer data, screening lists |
| `/regulatory` | SAR/STR filings, audit |
| `/operational-risk` | AML programme risk |

## Citations

- FATF Recommendations (40 Recommendations)
- FATF Mutual Evaluation Reports
- US Bank Secrecy Act (BSA)
- OFAC Sanctions Compliance Guidance
- EU AML Directives (5AMLD, 6AMLD)
- Wolfsberg AML Principles