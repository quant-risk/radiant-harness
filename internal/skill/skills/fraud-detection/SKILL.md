# Skill: fraud-detection

> Identity, application, payment, ATO, first-party, friendly,
> insurance claims, money laundering. Fraud is adversarial;
> the model is a moving target.

## Decision tree

```
Transaction / event
        │
        ▼
[Step 1] Identity check (KYC, device, behaviour)
        │
        ▼
[Step 2] Risk score (rules + ML)
        │
        ├── Low         -> approve
        ├── Medium      -> step-up auth (3DS, OTP, biometrics)
        ├── High        -> hold for manual review
        └── Critical    -> block
        │
        ▼
[Step 3] Investigation (if reviewed)
        │
        ├── Legitimate -> clear; label for model training
        └── Fraud      -> decline; chargeback; recovery
        │
        ▼
[Step 4] Feedback loop (chargeback → label)
```

## Workflow

### Fraud types

| Type | Description | Detection |
|------|-------------|-----------|
| **Identity** | Stolen / synthetic identity used to open account | Document verification; SSN trace; biometric |
| **Application** | Fraudulent application with bad intent | Behavioural signals; device fingerprint; velocity |
| **Payment (CNP)** | Card-not-present; stolen card | 3DS; device; behavioural; BIN risk |
| **Account takeover** | Stolen credentials; unauthorised access | MFA; behavioural; impossible travel; device change |
| **First-party** | Real customer commits fraud (chargeback abuse) | Behaviour + claim history; consortium data |
| **Friendly** | Customer claims non-receipt; actually received | Delivery proof; signature; address confirmation |
| **Insurance** | Exaggerated / fabricated / staged claims | SIU investigation; forensics; pattern |
| **Money laundering** | Layering / integration of illicit funds | See `/aml-kyc` |

### Detection methods

| Method | Strengths | Weaknesses |
|--------|-----------|------------|
| **Rules** | Interpretable; fast | Bypassable; high false positives if narrow |
| **ML supervised** | High accuracy; handles non-linearity | Needs labelled data; concept drift |
| **ML anomaly** | Detects novel patterns | High false positives; needs tuning |
| **Graph** | Identifies fraud rings; collusion | Expensive to build; storage |
| **Hybrid** | Best of all | Complex; integration overhead |

### Rules configuration

Common rule types:
- **Velocity**: N transactions in time T
- **Threshold**: amount above X
- **Geographic mismatch**: shipping ≠ billing country
- **Device**: new device + high amount
- **Behaviour**: deviation from baseline
- **Consortium**: IP / device / email on fraud list

Layered:
- Layer 1: hard blocks (known bad BIN, sanctions)
- Layer 2: step-up auth (3DS, OTP)
- Layer 3: review queue
- Layer 4: post-hoc (chargeback monitoring)

### ML fraud model

**Target**: chargeback (1 = fraud, 0 = legitimate).

**Features**:
- Transaction: amount, currency, channel, time
- Customer: tenure, prior chargebacks, avg spend
- Device: fingerprint, IP geo, new vs returning
- Behavioural: time-of-day, merchant category, frequency
- Network: shared device, shared address, velocity across users

**Validation**:
- Hold-out test (recent time period)
- Precision >= 95% (false positives hurt UX)
- Recall on chargebacks
- Population stability (concept drift detection)

### Alert triage

SLA per severity:
- **Critical** (high value, high confidence): <1 hour
- **High**: <4 hours
- **Medium**: <24 hours
- **Low**: batched daily

Investigator capacity: ~50 alerts/day/investigator; tool accordingly.

### Chargeback + recovery

| Action | Trigger | Timeline |
|--------|---------|----------|
| Chargeback | Confirmed fraud | Within scheme window (e.g. 120 days) |
| Recovery | Confirmed fraud + funds retrievable | <30 days |
| Account closure | Confirmed fraud + repeat | Immediate |
| Law enforcement | Major fraud | Per jurisdiction |

### Feedback loop

**Critical**: chargeback → labelled data → model retraining.

Lag: chargebacks arrive 30-90 days after transaction. Models trained
on last month's chargebacks. Concept drift is real (new fraud
patterns).

**Solutions**:
- Faster feedback (issuer-reported fraud at transaction time)
- Synthetic labels from rule blocks
- Adversarial validation: fraud rate distribution shift detection

### Synthetic identity fraud

Hardest type: combines real + fake data to create new identity.
Detection:
- SSN trace inconsistencies
- Address history mismatch
- Credit file thinness (file too new)
- Behavioural signals (limited online history)

## Examples

### Example 1: payment fraud (CNP)

```
Transaction: $500 electronics, new device, geo mismatch
Risk score: 0.78 (high)
Action: step-up auth (3DS challenge)
Result: customer completes 3DS; transaction approved
Chargeback: none (legitimate)
Model: reinforces behavioural signal
```

### Example 2: account takeover

```
Login: new IP (Nigeria), new device, 3 failed attempts, then success
Risk score: 0.92 (critical)
Action: block; require password reset + MFA
Investigation: confirms ATO; user notified
Recovery: password reset; no transactions completed (good)
```

### Example 3: first-party fraud (chargeback abuse)

```
Customer: 5 chargebacks in 6 months; pattern of high-value then claim
Behaviour: address change before claim; delivery confirmed
Investigation: friendly fraud indicator
Action: chargeback denied (with delivery proof); account flagged
Outcome: 4th chargeback at 90 days; full investigation
```

## Anti-patterns

### ❌ High false-positive rate

Alert fatigue + customer friction. Calibrate precision.

### ❌ ML model without chargeback feedback

Concept drift; target leakage from delayed labels.

### ❌ Single rule layer

Sophisticated fraud bypasses; multi-layer defence.

### ❌ No investigator capacity

Alerts queue; never reviewed. Size team to alert volume.

### ❌ No feedback loop

Model stale; performance decays.

## Failure modes

| Failure | Recovery |
|---------|----------|
| False positive spike | Calibrate rules; tune thresholds |
| Concept drift | Retrain; refresh features |
| Investigator overwhelmed | Triage rules; auto-clear low risk |
| Chargeback surge | Root-cause analysis; tighten rules |
| Synthetic identity ring | Graph analysis; report consortium |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/aml-kyc` | Identity fraud, money laundering |
| `/ml` | Fraud model training |
| `/data` | Consortium data, chargeback data |
| `/security` | Credential stuffing, ATO |
| `/stats` | Fraud rate estimation |

## Citations

- ACFE Report to the Nations (annual)
- Federal Reserve SR 11-7 / model risk for fraud models
- PCI DSS (payment card industry)
- Visa / Mastercard fraud monitoring guides
- LexisNexis / Ekata identity verification