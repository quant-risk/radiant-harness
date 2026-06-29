//go:build !light_only

package main

// ── ML scaffold commands ────────────────────────────────────────────────────
//
// `radiant model <type>`, `radiant predict <model>`, `radiant train <model>`,
// `radiant evaluate <model>`, `radiant drift <model>`, `radiant profile
// <dataset>`, `radiant stats <test>`, and `radiant causal-estimate <design>`
// are scaffold commands — they produce markdown planning docs that the
// operator fills in. The CLI emits the structured template; the LLM
// fills the actual content (via the corresponding skill).
//
// This file was extracted from helpers.go in Sprint 74 (v2.44.0) as
// part of the helpers.go debt-reduction effort. The cmd_spec.go /
// cmd_run.go / cmd_loop.go command registrations live alongside.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ── runStatsScaffold ────────────────────────────────────────────────────────

// runStatsScaffold produces a hypothesis-test plan scaffold
// following the radiant-stats skill. The user fills the
// concrete values and runs the actual test. The CLI scaffolds
// the input contract (H0/H1, alpha, power, effect size,
// multiple-testing correction, assumption checks).
func runStatsScaffold(test string, alpha, power float64, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/stats/%s-plan.md", test)
	}
	body := fmt.Sprintf(`# Stats test plan: %s

> Scaffolded by `+"`radiant stats %s`"+` on %s.
> Follows the radiant-stats skill workflow.

## 1. Hypothesis

- **H0**: <null hypothesis>
- **H1**: <alternative hypothesis>
- **Direction**: one-sided / two-sided
- **Test family**: %s

## 2. Design parameters

| Parameter | Value | Notes |
|-----------|-------|-------|
| Significance (α) | %.2f | Adjust for multiple comparisons |
| Power (1-β) | %.2f | Target ≥0.80 |
| Effect size | <small / medium / large / numeric> | Cohen's d, η², Cramér V, etc. |
| Sample size (planned) | <n> | From power analysis |

## 3. Sample + data

- Source: <dataset; row count; date range>
- Inclusion / exclusion criteria: <list>
- Out-of-sample / out-of-time plan: <test>

## 4. Assumption checks

- Normality: <Shapiro-Wilk / Q-Q plot> — required for parametric
- Homoscedasticity: <Levene / residual plot> — required for ANOVA / t-test
- Independence: <design review / Durbin-Watson>
- If any violated → fallback: <non-parametric alternative>

## 5. Multiple-testing correction

- Number of hypotheses (k): <n>
- Correction: <Bonferroni / Holm / BH-FDR / BY>
- Report: raw p-values AND corrected p-values

## 6. Reporting

- Effect size + 95%% CI (always, with p-value)
- Assumption-check results
- Robustness: alternative specs / subsamples

## 7. Anti-patterns checklist

- [ ] H0/H1 stated BEFORE looking at data (no HARKing)
- [ ] Sample size pre-committed (no optional stopping)
- [ ] Effect size reported alongside p-value
- [ ] Multiple-testing correction applied
- [ ] Assumptions checked; fallback documented
`, test, test, time.Now().UTC().Format("2006-01-02"), test, alpha, power)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in H0/H1, sample, effect size, then run the test.\n")
	fmt.Printf("    See radiant-stats skill for guidance.\n")
	return nil
}

// runCausalEstimateScaffold produces a causal analysis plan
// scaffold following the radiant-causal and radiant-causal-ml
// skills. Includes a DAG template (mermaid), identification
// assumption, sensitivity checklist, and CATE exploration.
func runCausalEstimateScaffold(design string, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/causal/%s-plan.md", design)
	}
	body := fmt.Sprintf(`# Causal analysis plan: %s

> Scaffolded by `+"`radiant causal-estimate %s`"+` on %s.
> Follows the radiant-causal + radiant-causal-ml skills.

## 1. Design

Design family: **%s**

Choose one:
- RCT (randomised controlled trial)
- Observational with treatment (selection on observables)
- Quasi-experimental (DiD / RDD / synthetic control)
- Instrumental variable (endogenous treatment)
- Natural experiment

## 2. DAG (mermaid)

`+"```mermaid"+`
graph LR
  T[Treatment] --> Y[Outcome]
  X1[Confounder 1] --> T
  X1 --> Y
  X2[Confounder 2] --> T
  X2 --> Y
  M[Mediator] --> Y
  T --> M
  C[Collider]
  T --> C
  Y --> C
`+"```"+`

Verify:
- All confounders affecting T AND Y included
- Mediators on causal path T → M → Y (don't condition)
- Colliders T → C ← Y (DO NOT condition; induces bias)

## 3. Identification assumption

State the assumption explicitly:

> "Conditional on X, treatment assignment is independent of
> potential outcomes: Y(t) ⫫ T | X"

Test (where applicable):
- RCT: randomisation check + covariate balance
- PS: overlap (common support); standardised mean differences < 0.1
- DiD: parallel trends in pre-period (event-study plot)
- RDD: continuity at cutoff; McCrary density test; placebo cutoffs

## 4. Estimand

| Estimand | Definition | When |
|----------|-----------|------|
| ATE | E[Y(1) - Y(0)] | Population average |
| ATT | E[Y(1) - Y(0) \| T=1] | Average on treated |
| CATE | E[Y(1) - Y(0) \| X=x] | Heterogeneous effects |
| LATE | E[Y(1) - Y(0) \| complier] | IV: local to compliers |

## 5. Estimation strategy

- Method: <OLS / IPW / matching / DiD / RDD / IV / double ML / causal forest>
- Software: <Python (dowhy / causalml / econml) / R (MatchIt / did / rdrobust)>
- Standard errors: <robust / clustered / bootstrap>
- Sample size: <n>; power: <≥0.80 for MDE of X>

## 6. Heterogeneous effects (CATE)

Explore CATE by:
- Pre-specified subgroups (demographic, behavioural)
- Data-driven: causal forest (honest splitting)
- ML-based uplift: T/S/X/R-learner
- Plot: CATE on x-axis; outcome on y-axis

## 7. Sensitivity analysis

How strong would an unobserved confounder need to be to change
the conclusion?
- **E-value** (VanderWeele): minimum strength to nullify
- **Rosenbaum bounds**: γ sensitivity analysis
- **Placebo tests**: fake treatment; should find zero

## 8. Reporting

- Point estimate + 95%% CI (ATE; ATT; CATE by subgroup)
- Assumption tests + sensitivity
- Robustness checks: alternative specs; out-of-sample
- Limitations + scope of inference

## 9. Anti-patterns checklist

- [ ] DAG drawn BEFORE estimation
- [ ] Identification assumption stated explicitly
- [ ] Pre-trends tested (if DiD)
- [ ] Common support / overlap checked (if PS)
- [ ] Placebo tests included
- [ ] Sensitivity analysis reported (E-value or similar)
- [ ] CATE explored when meaningful (not just ATE)
`, design, design, time.Now().UTC().Format("2006-01-02"), design)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in DAG, identification assumption, and estimation strategy.\n")
	fmt.Printf("    See radiant-causal + radiant-causal-ml skills for guidance.\n")
	return nil
}

// runModelScaffold produces a structured model spec scaffold
// following the radiant-ml skill workflow. Captures: target,
// features, baseline, primary metric, training recipe,
// monitoring, and ethics review.
func runModelScaffold(typ, successMetric, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/model/%s-spec.md", typ)
	}
	metricLine := "<fill in business-aligned success metric>"
	if successMetric != "" {
		metricLine = fmt.Sprintf("\"%s\"", successMetric)
	}
	body := fmt.Sprintf(`# Model spec: %s

> Scaffolded by `+"`radiant model %s`"+` on %s.
> Follows the radiant-ml skill workflow.

## 1. Problem framing

- **Task**: <classification / regression / ranking / generation / embedding>
- **Target variable**: <what we're predicting>
- **Population**: <who / what>
- **Decision-maker**: <who uses the prediction>
- **Success metric**: %s
- **Cost matrix**: Type I vs Type II errors; explicit quantification

## 2. Data

- **Source**: <dataset; row count; date range>
- **Label provenance**: <who labelled, with what rubric, when>
- **Train / val / test split**: <criteria; lock test IDs BEFORE any feature work>
- **Known biases**: <where the data is NOT representative of production>
- **Leakage audit**: <fields not available at inference time>

## 3. Baseline

Trivial baseline (must be beaten meaningfully):
- Predict majority class / mean / last value
- Compute metric on baseline
- Target: ML model beats baseline by >=10-30%% relative

## 4. Features

- Top features by importance
- Engineered features (volume × price × mix, etc.)
- Forbidden features (PII, leakage, regulatory)
- Time-alignment with target

## 5. Method

- Algorithm: <logistic / GBDT / neural net / etc.>
- Justification: <why this class>
- Training recipe: <optimizer, LR schedule, batch size, regularization>
- Compute budget: <training time, GPU/CPU hours>

## 6. Evaluation

- **Primary metric**: %s
- **Diagnostic metrics**: <AUC, calibration, fairness slices>
- **Held-out test**: <once; not touched during development>
- **Statistical significance**: <>=5 seeds; report CI>

## 7. Monitoring (post-deployment)

- **Data drift**: PSI per top feature; alert >0.2
- **Prediction distribution**: weekly mean + CI
- **Outcome**: <feedback loop if available; business metric>
- **Rollback path**: <fallback model / heuristic / no-prediction>

## 8. Ethics + fairness

- Demographic slices: <break down by protected attributes>
- Calibration parity: <do predictions reflect true probability across groups>
- Out-of-scope uses: <documented in model card>

## 9. Anti-patterns checklist

- [ ] Target + success metric stated BEFORE data exploration
- [ ] Test IDs locked BEFORE feature engineering
- [ ] Trivial baseline computed + beaten meaningfully
- [ ] Primary metric is business-aligned (not just accuracy)
- [ ] Held-out test set untouched during development
- [ ] >=5 random seeds; report CI
- [ ] Monitoring + rollback path defined BEFORE deployment
- [ ] Fairness audit completed
`, typ, typ, time.Now().UTC().Format("2006-01-02"), metricLine, metricLine)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in target, baseline, metric, training recipe, monitoring plan.\n")
	fmt.Printf("    See radiant-ml skill for guidance.\n")
	return nil
}

// runPredictScaffold produces a structured prediction request
// scaffold for serving a model. Captures: input contract,
// latency SLO, error semantics, monitoring hook, fall-back
// policy. Complements the model spec with the serving layer.
func runPredictScaffold(modelID string, latencyMs int, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/predict/%s-request.md", modelID)
	}
	body := fmt.Sprintf(`# Prediction request: %s

> Scaffolded by `+"`radiant predict %s`"+` on %s.

## 1. Model

- Model ID: %s
- Version: <semver>
- Owner: <team / on-call>
- Model card: <link to docs/model/%s-spec.md>

## 2. Inputs (request contract)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| <feature_1> | <type> | yes / no | <description> |
| <feature_2> | <type> | yes / no | <description> |
| ... | ... | ... | ... |

Validation: JSON Schema at API gateway; reject malformed inputs.

## 3. Latency budget

- p99 target: %d ms
- Timeout: %d * 1.5 = %d ms (cut losses)
- Cold-start: <warm-up plan>

## 4. Outputs (response contract)

`+"```json"+`
{
  "prediction": "<type>",
  "probability": "<float, 0-1>",
  "model_version": "%s",
  "request_id": "<uuid>",
  "served_at": "<ISO 8601>"
}
`+"```"+`

## 5. Error semantics

| Error | HTTP code | Meaning |
|-------|-----------|---------|
| Malformed input | 400 | Client error; do not retry |
| Auth failure | 401 / 403 | Re-authenticate |
| Model unavailable | 503 | Retry with backoff |
| Timeout | 504 | Retry; alert if persistent |

## 6. Fall-back policy

When model fails or is unavailable:
- Fallback to: <simpler model / heuristic / no-prediction>
- Log fallback rate; alert if >X%%

## 7. Monitoring

- Request rate, error rate, p50/p95/p99 latency
- Prediction distribution (drift alert)
- Fall-back rate
- User-reported anomalies

## 8. Testing

- Contract test: <golden requests + expected outputs>
- Load test: <target RPS at p99 latency>
- Chaos test: <kill instance mid-traffic>

## 9. Anti-patterns checklist

- [ ] Input contract validated at edge (not deep in code)
- [ ] Timeout < caller timeout; cut losses
- [ ] Error semantics documented; clients can branch
- [ ] Fall-back policy defined; tested
- [ ] Monitoring + alerting in place BEFORE first production request
`, modelID, modelID, time.Now().UTC().Format("2006-01-02"), modelID, modelID, latencyMs, latencyMs, latencyMs*3/2, modelID)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in input contract, latency tuning, error semantics, monitoring.\n")
	return nil
}

// runTrainScaffold produces a training plan scaffold following
// the radiant-ml + radiant-deep-learning skill workflow. Captures
// data split, training recipe, compute budget, checkpointing,
// reproducibility.
func runTrainScaffold(modelID, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/train/%s-plan.md", modelID)
	}
	body := fmt.Sprintf(`# Training plan: %s

> Scaffolded by `+"`radiant train %s`"+` on %s.
> Follows the radiant-ml + radiant-deep-learning skill workflow.

## 1. Inputs

- Model spec: <docs/model/%s-spec.md>
- Data: <path; row count; date range>
- Features: <list>
- Target: <what>
- Train / val / test split: <criteria; lock test IDs>

## 2. Training recipe

| Component | Choice | Notes |
|-----------|--------|-------|
| Algorithm | <algorithm> | <why> |
| Optimizer | <AdamW / SGD / etc.> | LR / momentum / weight decay |
| LR schedule | <warmup + cosine / linear> | initial, peak, decay |
| Batch size | <n> | effective = batch × grad_accum |
| Epochs / steps | <n> | + early-stopping criteria |
| Regularization | <dropout / weight decay / data aug> | |
| Mixed precision | <bf16 / fp16> | speedup vs stability |
| Gradient clipping | <max_norm 1.0> | RNNs + unstable loss |

## 3. Compute budget

| Resource | Quantity | Cost |
|----------|----------|------|
| GPU/CPU | <type × count> | <USD or hours> |
| Wall-clock | <target hours> | <max budget> |
| Storage | <data + checkpoints> | <GB> |
| Memory | <RAM/VRAM per node> | |

## 4. Checkpointing + reproducibility

- Save checkpoint every N steps; keep last 3 + best (by val metric)
- Save: model weights, optimizer state, RNG state, step, metric
- Random seeds: numpy, python, torch/cuda; version-controlled
- Lockfile: requirements.txt / pyproject.lock / conda env
- Docker image: <digest> for full reproducibility
- WandB / TensorBoard run ID: <link>

## 5. Monitoring during training

- Loss curve (per step)
- Validation metric (per epoch)
- Gradient norm (alert if explodes)
- GPU utilisation (efficiency)
- Throughput (samples/sec)

## 6. Anti-patterns checklist

- [ ] Test IDs locked before training
- [ ] Random seeds set + version-controlled
- [ ] Mixed precision enabled (where supported)
- [ ] Gradient clipping on (RNN / unstable)
- [ ] Eval during training (don't train blind)
- [ ] Checkpoint every N steps; rotate last 3 + best
- [ ] Lockfile + Docker image captured
`, modelID, modelID, time.Now().UTC().Format("2006-01-02"), modelID)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in algorithm, recipe, compute, reproducibility.\n")
	return nil
}

// runEvaluateScaffold produces an evaluation plan scaffold
// following the radiant-ml + radiant-stats skill workflow.
// Captures metrics, held-out test, statistical significance,
// robustness, fairness slices.
func runEvaluateScaffold(modelID, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/eval/%s-eval.md", modelID)
	}
	body := fmt.Sprintf(`# Evaluation plan: %s

> Scaffolded by `+"`radiant evaluate %s`"+` on %s.
> Follows the radiant-ml + radiant-stats skill workflow.

## 1. Inputs

- Model: %s (trained per docs/train/%s-plan.md)
- Held-out test: <IDs locked BEFORE training; not touched since>
- Baselines: trivial (majority / mean / last) + simple (LR / GBDT)

## 2. Metrics

| Metric | Definition | Why |
|--------|-----------|-----|
| **Primary** | <business-aligned> | <decision-driven> |
| Discrimination | <AUC / Gini / KS> | <separates classes> |
| Calibration | <Brier / H-L / calibration plot> | <probability correctness> |
| Stability | <PSI / CSI> | <input drift> |
| Latency | <p50 / p95 / p99> | <serving SLO> |

## 3. Statistical significance

- Number of runs: <N >= 5>
- Different random seeds; report mean ± std
- Confidence interval: <95%% CI on primary metric>
- Statistical test: <paired bootstrap; p-value with effect size>

## 4. Robustness checks

| Variation | Expected impact |
|-----------|-----------------|
| Subsample (e.g. last 20%% of test) | <report> |
| Out-of-distribution inputs | <report> |
| Adversarial inputs (if applicable) | <report> |
| Different random seeds | <std around mean> |

## 5. Fairness slices

| Protected attribute | Group | Performance gap |
|--------------------|-------|------------------|
| <attribute> | <group A vs B> | <delta> |
| Calibration parity | <across groups> | <delta> |
| Demographic parity | <across groups> | <delta> |

## 6. Failure modes

| Failure | Detection | Recovery |
|---------|-----------|----------|
| Worst-case subset fails | per-slice metrics | Refine; investigate bias |
| Calibration drift | H-L / Brier over time | Re-fit |
| Distribution shift | PSI/CSI | Retraining trigger |

## 7. Anti-patterns checklist

- [ ] Held-out test untouched during development
- [ ] Primary metric is business-aligned
- [ ] >=5 random seeds; report mean ± std
- [ ] 95%% CI on primary metric
- [ ] Robustness checks (subsample, OOD, seeds)
- [ ] Fairness slices reported
- [ ] Failure modes documented
`, modelID, modelID, time.Now().UTC().Format("2006-01-02"), modelID, modelID)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in metrics, statistical tests, fairness, robustness.\n")
	return nil
}

// runDriftScaffold produces a drift monitoring plan scaffold
// following the radiant-ml skill workflow. Captures PSI / CSI
// thresholds, alert escalation, retraining trigger, rollback.
func runDriftScaffold(modelID, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/drift/%s-monitor.md", modelID)
	}
	body := fmt.Sprintf(`# Drift monitoring: %s

> Scaffolded by `+"`radiant drift %s`"+` on %s.
> Follows the radiant-ml skill workflow.

## 1. Inputs

- Model: %s
- Training distribution: <baseline period>
- Production distribution: <incoming data>

## 2. Population Stability Index (PSI)

- Computed weekly on production features
- Compare to training distribution
- Thresholds:
  - < 0.10: stable (green)
  - 0.10 - 0.25: moderate (amber) - investigate
  - > 0.25: significant (red) - retrain or fall back

## 3. Characteristic Stability Index (CSI)

- Per-feature drift (not just population)
- Same thresholds as PSI
- Alert if top-N features drift

## 4. Prediction distribution

- Mean, percentiles over time
- Alert if mean shifts >2 sigma
- Alert if distribution shape changes (KS test vs baseline)

## 5. Outcome monitoring

- If outcome feedback available: A/E over time
- Bias drift: should stay near 1.0
- Alert if persistent bias in any direction

## 6. Alert escalation

| Severity | Trigger | Action | SLA |
|----------|---------|--------|-----|
| Info | PSI 0.05-0.10 | Dashboard | — |
| Warning | PSI 0.10-0.25 | Investigation | 5 business days |
| Critical | PSI > 0.25 | Retraining or fallback | 24 hours |

## 7. Retraining trigger

- Auto-trigger: PSI > 0.25 for 2 consecutive weeks
- Manual trigger: any warning escalated
- Champion / challenger: challenger overtakes on metric

## 8. Rollback plan

- Fall-back to previous model version
- Fall-back to simpler model (LR / GBDT)
- Fall-back to "no prediction" (label as low-confidence)
- Rollback tested quarterly

## 9. Anti-patterns checklist

- [ ] PSI computed on production features (weekly)
- [ ] Thresholds set per feature importance
- [ ] Outcome monitoring (if feedback available)
- [ ] Alert escalation documented; tested
- [ ] Retraining trigger defined
- [ ] Rollback plan tested
`, modelID, modelID, time.Now().UTC().Format("2006-01-02"), modelID)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in thresholds, alerts, retraining, rollback.\n")
	return nil
}

// runValidate performs basic sanity checks on a scaffolded
// plan or spec: file exists, has expected sections (heuristic:
// markdown headings or scaffold-specific anchors), and no
// obvious TODOs or placeholders left. Returns non-zero on
// failure so callers (CI / pre-commit) can gate on it.
func runProfileScaffold(dataset, out string) error {
	if out == "" {
		out = fmt.Sprintf("docs/profile/%s-profile.md", dataset)
	}
	body := fmt.Sprintf(`# Data profile: %s

> Scaffolded by `+"`radiant profile %s`"+` on %s.
> Follows the radiant-data + radiant-drift skills.

## 1. Source

- Path: <path or URL>
- Format: <CSV / Parquet / Delta / SQL / API>
- Owner: <team>
- Refresh: <frequency>

## 2. Schema

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| <col_1> | <type> | yes / no | <description> |
| <col_2> | <type> | yes / no | <description> |
| ... | ... | ... | ... |

## 3. Volume

| Metric | Value | Trend |
|--------|-------|-------|
| Row count | <n> | <growing / stable> |
| Daily insert rate | <n> | |
| Storage size | <GB> | |
| Cardinality (key columns) | <n> | |

## 4. Quality

| Metric | Threshold | Current |
|--------|-----------|---------|
| Null rate (per column) | <5%% | <report> |
| Duplicate rate (key) | 0%% | <report> |
| Referential integrity | 100%% | <report> |
| Schema drift events / month | <1 | <report> |

## 5. Distributions

For key columns:
- Numeric: mean, std, min, max, quantiles
- Categorical: top-K categories, distribution
- Date: range, gaps
- Free text: length distribution, top tokens

## 6. Drift monitoring

| Column | PSI threshold | KS p-value | Alert |
|--------|---------------|-----------|-------|
| <col> | <0.2> | <0.05> | <action> |

## 7. Monitoring plan

- Daily: row count, null rate, schema drift
- Weekly: PSI per top-10 columns
- Monthly: distribution summary + outlier report

## 8. Anti-patterns checklist

- [ ] Schema documented; lineage tracked
- [ ] Quality thresholds set; alerts configured
- [ ] Drift monitoring on top-K columns
- [ ] PII identified; access controls enforced
- [ ] Retention policy defined
`, dataset, dataset, time.Now().UTC().Format("2006-01-02"))

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("  ✓ Scaffolded %s\n", out)
	fmt.Printf("    Fill in source, schema, distributions, drift metrics.\n")
	return nil
}
