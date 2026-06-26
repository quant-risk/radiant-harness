# Skill: model-risk

> Model lifecycle (development → validation → approval →
> ongoing monitoring → retirement). A model without independent
> validation is a number, not a tool.

## Decision tree

```
Model lifecycle
        │
        ▼
[Step 1] Tier the model (risk-based)
        │
        ▼
[Step 2] Development + documentation
        │
        ▼
[Step 3] Independent validation
        │   - Conceptual soundness
        │   - Statistical validation
        │   - Outcomes analysis
        │   - Sensitivity
        │
        ▼
[Step 4] Model risk committee approval
        │
        ▼
[Step 5] Use in production
        │
        ▼
[Step 6] Ongoing monitoring (quarterly)
        │
        ▼
[Step 7] Re-validation (annual; or after material change)
        │
        ▼
[Step 8] Retirement (when superseded or no longer used)
```

## Workflow

### Tiering

| Tier | Examples | Validation depth |
|------|----------|------------------|
| **Tier 1** | IRB PD; market risk VaR; regulatory capital | Full: theory, statistical, outcomes, sensitivity |
| **Tier 2** | Behavioural scoring; pricing models | Moderate: theory + statistical + ongoing |
| **Tier 3** | Operational models; supporting models | Limited: documented assumption review |

### Conceptual soundness

- Theory + assumptions documented
- Review by subject-matter expert
- Plausibility of model structure
- Alignment with industry practice
- Known weaknesses documented

### Statistical validation

| Metric | When |
|--------|------|
| **Discrimination** | AUC, Gini, KS (binary); R² (regression) |
| **Calibration** | Brier, Hosmer-Lemeshow, calibration plot |
| **Stability** | PSI, CSI, characteristic stability |
| **Sensitivity** | One-factor-at-a-time; tornado chart |
| **Benchmark** | Compare to challenger model |
| **Backtest** | Out-of-sample; out-of-time |

### Outcomes analysis

Track predictions vs actuals over time:

| Statistic | When |
|-----------|------|
| Mean error (bias) | Should be ~0 over time |
| MAE / RMSE | Magnitude of error |
| Drift | Are errors increasing? |
| Sliced analysis | Performance across segments |

**Trigger**: persistent degradation → investigate → remediate
or retire.

### Challenger model

Independent model that runs alongside production:
- Different methodology (e.g. gradient boosting vs logistic)
- Different features
- Different training data

Purpose: detect production model degradation; provide backup if
production fails.

### Regulatory framework

| Jurisdiction | Guidance |
|--------------|----------|
| **US** | SR 11-7 (Fed/OCC/FDIC); SS 1/2018; SS 1/2024 |
| **EU** | EBA Guidelines on Model Risk Management; TRV |
| **UK** | PRA SS1/23 (Model Risk Management) |
| **Basel** | BCBS Sound management of model risk |

Common themes:
- Independent validation
- Ongoing monitoring
- Tiered approach
- Outcomes analysis
- Model inventory
- Annual re-validation

### Ongoing monitoring

| Metric | Frequency | Action |
|--------|-----------|--------|
| Population Stability Index (PSI) | Monthly | >0.2 → investigate |
| Characteristic Stability (CSI) | Monthly | >0.2 → investigate |
| Discrimination (AUC) | Quarterly | >5% drop → re-validate |
| Calibration (Brier) | Quarterly | >10% increase → re-validate |
| Outcomes vs predictions | Quarterly | Bias trend → investigate |

### Governance

- **Model owner**: accountable for development + use
- **Model developer**: builds and tests
- **Independent validator**: tests + challenges
- **Model risk committee**: approves use + ongoing monitoring
- **Senior management**: oversees MRM framework

### Documentation

Per model:
- Methodology + assumptions
- Data sources
- Validation results (conceptual + statistical + outcomes)
- Limitations + known weaknesses
- Intended use + out-of-scope use
- Approval history (versions, approvals)
- Ongoing monitoring schedule

## Examples

### Example 1: Tier-1 PD model validation

```
Model: retail PD scorecard
Tier: 1 (regulatory capital)
Validation:
  Conceptual: theory documented; expert review approved
  Statistical:
    - AUC 0.78; Gini 56
    - Brier 0.045; H-L p=0.32
    - PSI 0.07 (stable)
  Outcomes:
    - Backtest 2024: A/E 1.02 (slight under-prediction)
    - Sliced: stable across segments
  Challenger:
    - Gradient boosting: AUC 0.81 (better)
    - Not adopted (interpretability for IRB)
Approval: Model Risk Committee, 2024-12-15
Ongoing: monthly PSI + quarterly outcomes
Re-validation: 2025-12 (annual)
```

### Example 2: Challenger model

```
Production: logistic regression on 50 features
Challenger: gradient boosting on same features

Production AUC: 0.78
Challenger AUC: 0.81 (3pp better)

Outcome (12-month):
  Production: A/E 1.05 (under-predicts)
  Challenger: A/E 0.99 (well-calibrated)

Decision: keep production (interpretability);
          but flag — production may need recalibration
          if A/E drift persists
```

### Example 3: model retirement

```
Model: legacy credit score v2 (2018)
Status: superseded by v3 (2023); still in use for legacy portfolios
Annual review 2024:
  - Production volume declining (<5% of originations)
  - Maintenance overhead high
  - v3 model handles 95% of new business
Decision: retire v2 by 2025-Q2
Communication: portfolio owners; risk committee
Migration: re-score legacy portfolio with v3
```

## Anti-patterns

### ❌ Developer-only validation

Independence compromised. Always independent team.

### ❌ One-off validation

Models degrade. Ongoing monitoring is required.

### ❌ No challenger

Status quo bias; production could degrade silently.

### ❌ Good metrics without outcomes

Calibration + discrimination don't predict real-world performance.
Always track outcomes.

### ❌ Long approval cycle with no interim use

Calibration runs out of date while waiting. Tier-based depth.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Validation finds issue | Block deployment; remediate |
| Outcomes degrade | Investigate; recalibrate; retire |
| Challenger outperforms | Investigate; consider replacement |
| Supervisor challenge | Engagement; documentation; remediation |
| Model deprecated | Sunset plan; archive |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/ml` | ML model lifecycle |
| `/stats` | Statistical validation tests |
| `/credit-risk` | IRB PD validation |
| `/market-risk` | VaR validation |
| `/actuarial` | Actuarial model validation |

## Citations

- US Fed SR 11-7 (Guidance on Model Risk Management, 2011)
- US OCC SS 1/2018; SS 1/2024
- EBA Guidelines on Model Risk Management (2024)
- BCBS Sound management of model risk (2013)
- UK PRA SS1/23