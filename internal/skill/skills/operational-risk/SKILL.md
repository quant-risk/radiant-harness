# Skill: operational-risk

> ORM framework, KRIs, scenario analysis, RCSA, loss data, BCP.
> Operational risk is everything that isn't market or credit —
> and it's where most losses happen.

## Decision tree

```
Operational risk question
        │
        ▼
[Step 1] Choose framework (BIA / SMA / IMA / internal)
        │
        ▼
[Step 2] Loss data collection (gross + threshold)
        │
        ▼
[Step 3] RCSA cycle (per unit, per category)
        │
        ▼
[Step 4] Scenario analysis (severity × frequency)
        │
        ▼
[Step 5] KRI definition + monitoring
        │
        ▼
[Step 6] BCP / DR (RTO / RPO)
        │
        ▼
[Step 7] Capital calculation (per framework)
```

## Workflow

### Basel III OR frameworks

| Framework | Approach | When |
|-----------|----------|------|
| **BIA** (Basic Indicator) | 15% of avg positive annual gross income | Simple; small banks |
| **SMA** (Standardised) | 8 business lines × 3 risk types (β coefficients) | Standardised; common |
| **IMA** (Internal Models) | LDA: VaR 99.9%, 1y, on internal data + scenarios | Large banks; approval required |

### Risk categories (Basel event types)

| Level 1 | Level 2 examples |
|---------|------------------|
| **Internal fraud** | Misappropriation, deliberate tax evasion |
| **External fraud** | Theft, hacking, forgery |
| **Employment** | Discrimination, wrongful termination, harrassment |
| **Clients** | Suitability, disclosure, fiduciary breaches |
| **Business disruption** | System failures, utility outages |
| **Technology** | Hardware/software failures, capacity issues |
| **Physical assets** | Natural disasters, terrorism |
| **Execution** | Process failures, vendor issues |

### Loss data collection

| Field | Example |
|-------|---------|
| Event date | 2024-06-15 |
| Discovery date | 2024-06-17 |
| Gross loss | $250,000 |
| Recovery | $50,000 |
| Net loss | $200,000 |
| Category | External fraud / hacking |
| Business line | Retail banking |
| Description | Phishing attack on 12 customers |

**Threshold**: typically $5K-$20K. Below threshold = near-miss
(also captured for analysis).

### RCSA (Risk and Control Self-Assessment)

Per business unit, per process:

| Step | Output |
|------|--------|
| **Identify risks** | Risk register; inherent risk score |
| **Identify controls** | Control inventory; control type (preventive / detective / corrective) |
| **Assess residual risk** | After controls |
| **Action plans** | For high residual risk |

Scoring: typically Likelihood (1-5) × Impact (1-5) = Risk score.

### Scenario analysis

For each material scenario:

| Component | Description |
|-----------|-------------|
| **Narrative** | Plausible scenario (e.g. "major cyber attack on payment system") |
| **Severity** | Estimated loss (point estimate + range) |
| **Frequency** | Estimated occurrences per year |
| **Capital impact** | Severity × frequency, used in IMA |
| **Mitigations** | Existing controls; further mitigations |

**Examples**:
- Major cyber attack: $5M-$50M severity; 0.05/yr frequency
- Failed system migration: $2M-$10M severity; 0.3/yr frequency
- Rogue trader: $10M-$100M severity; 0.1/yr frequency

### KRIs (Key Risk Indicators)

| KRI | Threshold | Action |
|-----|-----------|--------|
| System uptime | < 99.9% | Escalate to ops |
| Failed transactions | > 0.5% | Investigate |
| Phishing reports | > 50/week | Awareness campaign |
| Employee turnover (key roles) | > 20%/yr | Retention plan |
| Vendor SLA breaches | > 3/quarter | Vendor review |

KRIs are leading indicators (predict risk); LAER (Loss Actual Event
Reporting) is lagging.

### Business Continuity (BCP) / Disaster Recovery (DR)

| Term | Definition |
|------|-----------|
| **RTO** (Recovery Time Objective) | Max downtime tolerated |
| **RPO** (Recovery Point Objective) | Max data loss tolerated |
| **MBCP** (Minimum Business Continuity) | Critical functions to maintain |
| **DR site** | Secondary site for failover |

Test annually (tabletop + actual failover for critical systems).

### Operational Resilience

Beyond BCP — ability to prevent + recover + adapt:

- **Important Business Services** (IBS): critical services to clients
- **Impact tolerances**: max tolerable disruption
- **Severe but plausible scenarios**

Regulatory focus (BCBS, FCA, Fed): prevent disruption, not just
recover from it.

## Examples

### Example 1: SMA capital (commercial bank)

```
8 business lines × 3 risk types (β in %):
  Corporate finance: 18 / 18 / 18
  Trading & sales: 18 / 18 / 45
  Retail banking: 12 / 12 / 12
  Commercial banking: 15 / 15 / 15
  ...

Capital = Σ (β_i × gross income_i)
       ≈ 12% of gross income (avg)
```

### Example 2: RCSA (retail lending process)

```
Process: customer onboarding
Risks:
  - Identity theft (likelihood 4, impact 5, inherent 20)
  - Document forgery (likelihood 3, impact 5, inherent 15)
  - KYC failure (likelihood 3, impact 4, inherent 12)

Controls:
  - Document verification (detective, partial)
  - Biometric check (preventive, strong)
  - PEP/sanctions screening (preventive, strong)
  - Manual review (detective, partial)

Residual risk: 6 (low)
```

### Example 3: scenario (cyber attack)

```
Scenario: ransomware on payment system
Severity: $5M-$20M (estimate from incident DB + expert input)
Frequency: 0.05/yr (1 in 20 years)
Mitigations: backups, segmentation, incident response plan
Capital impact: $12.5M × 0.05 = $625k expected annual loss
Action: monitor ransomware trends; quarterly drill
```

## Anti-patterns

### ❌ OR capital without scenario analysis

SMA is rough; BIA is rougher. Internal models need scenarios.

### ❌ RCSA as checkbox exercise

No real assessment = no value. Tie RCSA to risk appetite.

### ❌ Loss data only above threshold

Tail under-reported. Capture near-misses too.

### ❌ KRIs without action

Alerts no one reads. Tie KRIs to action thresholds.

### ❌ BCP never tested

Fails on the day. Test annually for critical systems.

### ❌ Cyber treated separately from OR

Cyber is OR. Integrate risk taxonomies; share scenarios.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Major loss event | Crisis management; insurance recovery; lessons learned |
| Capital shortfall | Reallocate; reduce risk; raise capital |
| BCP tested, fails | Fix gaps; re-test; update RTO/RPO |
| KRI breach ignored | Investigate; tighten thresholds; escalate |
| Loss data quality poor | Re-train reporters; validate; audit |
| RCSA stale | Annual cycle; mid-cycle updates for material changes |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/security` | Cyber risk; access control; threat modelling |
| `/credit-risk` | Fraud detection; credit process failures |
| `/market-risk` | Trading system failures; rogue traders |
| `/incident` | Incident response framework |
| `/data` | Data quality; data lineage; data risk |