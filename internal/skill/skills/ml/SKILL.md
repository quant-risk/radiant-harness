# Skill: ml

> Machine-learning project guidance: problem framing, data discipline,
> eval methodology, deployment, monitoring. A model without a baseline
> is a complicated random number generator.

## Decision tree

```
Project starts (or pivots to ML)
        │
        ▼
[Step 1] Problem framing        ── docs/ml/problem-framing.md
        │                          (metric, cost matrix, baseline)
        ▼
[Step 2] Data card              ── docs/ml/data-card.md
        │                          (sources, splits, leakage audit)
        ▼
[Step 3] Trivial baseline       ── predict majority / mean / last
        │                          (must be beaten before any model)
        ▼
[Step 4] Model + features       ── shortlist, then a chosen model
        │
        ▼
[Step 5] Eval protocol          ── docs/ml/eval-protocol.md
        │                          (held-out test, statistical sig)
        ▼
[Step 6] Reproducibility       ── seeds, lockfile, training script
        │
        ▼
[Step 7] Deployment             ── batch / online / edge
        │
        ▼
[Step 8] Monitoring plan        ── BEFORE first prediction served
        │                          (drift, distribution, rollback)
        ▼
[Step 9] Model card             ── docs/ml/model-card.md
                                   (intended use, out-of-scope,
                                    known failures)
```

## Workflow

### Step 1: Problem framing

**This step is non-negotiable. Do it before touching data.**

The first deliverable is `docs/ml/problem-framing.md` answering:

1. **What are we predicting?** One sentence. If you can't write it,
   you don't have a project — you have a vibe.
2. **Who is the user of the prediction?** (e.g. "ops team gets a
   daily list"; "the model decides in real-time whether to show
   the loan offer"). The user determines latency, cost, and
   acceptable error rates.
3. **What is the success metric?** Single number. Business-aligned,
   not just ML-friendly. Examples:
   - "Reduce churn-prediction false negatives by 30% at fixed
     precision"
   - "5% improvement in click-through rate, measured on the
     held-out week"
   - "Match or exceed human radiologist accuracy at <1s latency"
4. **What is the cost matrix?** Type I vs Type II errors are NOT
   equally costly. Fraud: missing one is 100x worse than a false
   alarm. Spam: false positives destroy user trust. Make this
   explicit.
5. **What is the trivial baseline?** Predict majority class.
   Predict last value. Predict mean. This is the floor; the ML
   model must beat it meaningfully.

| Symptom | Diagnosis |
|---------|-----------|
| "We need 99% accuracy" | Define the actual metric first. Accuracy is rarely the right one for asymmetric problems. |
| "More data will help" | Sometimes. Define the eval first — many "more data" projects are "wrong feature" projects. |
| "Just use a neural net" | Until you've tried the trivial baseline, you don't know if you need ANY model. |
| "We'll figure out the metric after training" | You won't. You'll rationalize whatever the model produced. |

### Step 2: Data card

`docs/ml/data-card.md` is the source-of-truth for data:

- **Sources**: where each row came from; how it was collected
- **Label provenance**: who labelled, when, with what rubric
- **Known biases**: where the data is NOT representative of
  production
- **Train/val/test split policy**: by what criterion (time?
  user? random?); what fraction
- **Leakage audit**: any field in the dataset that wouldn't be
  available at inference time, or that encodes the label
- **Versioning**: dataset version + commit hash; each model
  release pins to a data version

Update the data card every time the dataset changes. If the data
card is stale, your model card is fiction.

### Step 3: Trivial baseline

Before training any model, compute the trivial baseline:

| Task type | Trivial baseline |
|-----------|------------------|
| Classification (balanced) | Predict majority class |
| Classification (imbalanced) | Predict majority + class weights analysis |
| Regression | Predict mean |
| Ranking | Random ordering |
| Generation | Predict the most common token |
| Time-series | Predict last value (or seasonal naive) |

The ML model must beat this by a **meaningful margin** —
typically 10-30% relative improvement on the success metric,
not 0.5%. If it doesn't, you don't have an ML project. You
have an engineering project that needs the right data
infrastructure first.

### Step 4: Model + features

Shortlist 2-4 model classes appropriate to the data modality
and scale. For most projects:

| Modality | Shortlist (start here) |
|----------|------------------------|
| Tabular | GBDT (XGBoost/LightGBM/CatBoost); linear model as baseline |
| Text (small data) | Fine-tuned encoder (BERT-family) or logistic regression on TF-IDF |
| Text (large data) | Fine-tuned LLM (Llama, Qwen) or pretrained encoder + head |
| Image | Pretrained vision transformer (ViT, CLIP); CNN if data is plentiful |
| Audio | Whisper for ASR; pretrained encoders for classification |
| Graph | GraphSAGE / GAT; node2vec as baseline |
| Time-series | Temporal fusion transformer; GBDT on engineered features |

Don't start with the biggest model. Start with the simplest one
that could plausibly work. Beat the trivial baseline. Then add
complexity only as needed.

**Features**: spend 80% of feature engineering time on data
quality and feature/label alignment. The "smart feature" that
moves the metric by 0.3% is rarely worth the complexity.

### Step 5: Eval protocol

`docs/ml/eval-protocol.md` defines:

- **Held-out test set**: a slice of data the model NEVER sees
  during training, validation, hyperparameter tuning, or feature
  selection. Touched ONCE.
- **Metrics**: which metrics (always the success metric; plus
  diagnostic metrics). If the business metric is "30% improvement
  in CTR", that's the metric you optimize and report.
- **Statistical significance**: number of runs, confidence
  intervals, or paired bootstrap. A 0.5% improvement on one run
  is noise.
- **Regression threshold**: how much the metric must drop before
  we block a release.

Cross-validation is fine for model selection, but does NOT count
for release decisions. Only the held-out test set counts.

| Eval anti-pattern | What's wrong | Fix |
|-------------------|--------------|-----|
| Report val score as "the metric" | Overfit to val via tuning | Hold out a true test set, touch once |
| "We improved by 0.5%" on one run | Statistical noise | Run 5+ seeds, report mean ± std |
| "Looks better on the spot-check" | Vibes-driven eval | Always evaluate on the held-out test, with statistical comparison |
| Compare to last release's test score from a different test set | Apples to oranges | Lock the test set; never replace it for "fresher" data |
| Eval on data that leaked into training | Inflated numbers | Audit the split policy; verify labels were not in train |

### Step 6: Reproducibility

Before declaring a result "real":

- Random seeds set in code (PyTorch, NumPy, Python, CUDA)
- Dependency versions pinned (lockfile or `requirements.txt`
  with hashes; or `pyproject.toml` with `uv lock`)
- Training script committed
- Hardware documented (GPU model, count)
- Data version pinned (commit hash of the dataset)

Re-running the script should reproduce the metrics within ±0.5%
on the val set. If not, the result is a fluke.

### Step 7: Deployment

| Pattern | When | Operational shape |
|---------|------|-------------------|
| **Batch** | Offline scoring (recommendations, risk scores, daily prioritisation) | Cron or scheduler; predictions land in a DB; downstream consumes |
| **Online** | Real-time decisions (fraud, routing, dynamic pricing) | Inference service (KServe, Triton, custom); low-latency; horizontal scaling |
| **Edge** | On-device (mobile, embedded, browser) | Quantised model, ONNX Runtime / TFLite; offline-capable |
| **Embedded in product** | LLM features inside an existing app | API call from app to model service; prompt + context assembly |

Each has different latency, cost, and reliability profiles.
Pick the deployment target BEFORE you start training — model
size, quantisation, and architecture depend on it.

### Step 8: Monitoring plan

**Define BEFORE the first prediction is served.** Not after.

Monitor in three layers:

1. **Data drift**: input distribution shifts vs training.
   - Per-feature KS test or PSI (Population Stability Index).
   - Alert threshold: PSI > 0.2 on top features.
2. **Prediction distribution**: output distribution shifts.
   - Alert if mean predicted probability moves >X% week-over-week.
3. **Outcome / business metric**: the success metric itself,
   measured on a feedback loop if possible.
   - "Conversion rate on predicted-positive cohort dropped Y%".

Define a **rollback path**: if a metric breaches threshold,
can you fall back to the previous model? To a heuristic?
To "no prediction"?

### Step 9: Model card

`docs/ml/model-card.md` per release:

- Model architecture + version
- Training data slice (date range, source, version)
- Eval results on the held-out test set
- Intended use cases
- **Out-of-scope use cases** (where the model should NOT be
  deployed)
- Known failure modes (e.g. "performance degrades on
  out-of-distribution text")
- Ethical considerations (bias audit, fairness analysis)

The model card is for downstream consumers — including future
you — to know when NOT to use this model.

## ML-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Train/test leakage | Inflated metrics; ship a broken model | Lock test IDs; audit feature pipeline |
| Distribution shift | Model accuracy drops silently in production | Drift monitoring; periodic retraining |
| Data quality issues | Garbage in, garbage out | Data validation (Great Expectations, pandera); reject-on-ingest |
| Label noise | Model learns noise, not signal | Inter-annotator agreement; label audit on disagreements |
| Concept drift | The relationship between features and label changes | Windowed retraining; alert when target metric shifts |
| Cost-sensitive error | Optimize for the wrong metric | Cost matrix in problem framing; pick precision/recall/F-beta |
| Reproducibility | "Works on my machine"; can't debug | Seeds + lockfiles + Docker image |

## Examples

### Example 1: churn prediction (tabular, classification)

```
Task:      binary classification — will a customer churn in 30 days?
Modality:  tabular (10 years of customer history)
Scale:     medium (2M customers, 200 features)
Framework: scikit-learn + XGBoost
Target:    batch (daily scoring → CRM)

Problem framing:
  - User: retention team
  - Metric: 30% reduction in false negatives at fixed precision
  - Cost matrix: missed churn (FN) is 10x cost of false alarm (FP)
  - Trivial baseline: predict "no churn" → recall 0%

Pipeline:
  - Data card: source=CRM + billing + product usage; splits by user_id
  - Baseline: predict majority → 0% recall, accuracy 92%
  - Model: XGBoost, 5-fold CV for hyperparameter selection
  - Test set: held-out 20% of users, never touched during tuning
  - Reproducibility: seeds=42, lockfile, training script committed
  - Deployment: batch job → CSV → CRM import
  - Monitoring: PSI per feature, weekly; alert if PSI > 0.2

Result: recall 0.62 at precision 0.35 (vs baseline recall 0).
```

### Example 2: LLM-powered support router (text, generation)

```
Task:      multi-class classification — route support tickets
Modality:  text (ticket body)
Scale:     small (50k labeled tickets; 8 categories)
Framework: fine-tuned encoder (DeBERTa-v3-base)
Target:    online (<200ms p99)

Problem framing:
  - User: support team and the bot itself
  - Metric: 5% improvement in routing accuracy vs current keyword
    heuristic; latency budget 200ms p99
  - Cost matrix: misroute (any) is 5x cost of "ask clarifying
    question" (low-confidence fallback)
  - Trivial baseline: keyword match → 71% accuracy

Pipeline:
  - Data card: tickets from past 2 years; labels from human
    routing + 10% re-label audit
  - Baseline: keyword match → 71% accuracy
  - Model: DeBERTa-v3-base, 8-class head
  - Eval: held-out 10% by ticket_id (no overlap with train)
  - Reproducibility: seeds=0, transformers==4.41, torch==2.3
  - Deployment: online via Triton; quantised for <200ms
  - Monitoring: per-class precision drift; weekly

Result: 89% accuracy, p99 latency 145ms; beats baseline by 18pp.
```

### Example 3: fine-tuning an LLM for code review (text, generation)

```
Task:      text generation — review comments on a PR diff
Modality:  text (diff + surrounding context)
Scale:     large (500k historical PRs with review comments)
Framework: Llama-3-8B fine-tuned (LoRA)
Target:    embedded in product (IDE plugin)

Problem framing:
  - User: developers in the IDE
  - Metric: 25% of comments are accepted by the dev without edit
  - Cost matrix: bad suggestion is worse than no suggestion
    (developer ignores the tool)
  - Trivial baseline: no comments → 0% acceptance (vacuous win)

Pipeline:
  - Data card: PRs from last 3 years; comments from human reviewers
  - Baseline: zero-shot Llama-3-8B → 8% acceptance
  - Fine-tuning: LoRA on 500k examples; 3 epochs
  - Eval: held-out 5% of PRs; acceptance measured by devs in shadow mode
  - Reproducibility: seeds=42, peft==0.10, vLLM==0.4
  - Deployment: bundled with IDE plugin; ONNX-quantised
  - Monitoring: comment-acceptance rate by team; rollback to v0
    if acceptance <15%

Result: 27% acceptance (above 25% target); beat baseline by 19pp.
```

## Anti-patterns

### ❌ Optimizing accuracy when costs are asymmetric

Accuracy treats false positives and false negatives equally. For
fraud, medical diagnosis, or safety-critical routing, the cost
matrix matters. Use precision/recall, F-beta, or a custom loss.

### ❌ Touching the test set during development

The test set is your one shot at an honest number. If you tune
hyperparameters, select features, or compare models using the
test set, your "test accuracy" is overfit to that set. Lock the
test set; use val for tuning.

### ❌ "We have a model" without a baseline

A model that loses to the trivial baseline is not a model.
Always report the baseline. Always. The first number in any
ML results table should be the baseline.

### ❌ Vibes-driven evaluation

"It looks good" is not an eval methodology. Run on the
held-out test set. Compute statistical confidence. Compare to
baseline. LLM outputs need the same discipline.

### ❌ "We'll add monitoring later"

Drift is silent. By the time a user notices accuracy dropped,
the damage is done. Define drift metrics, alert thresholds,
and a rollback path BEFORE the first prediction is served.

### ❌ Training on data from the future

Data leakage of time-travel kind is the #1 silent failure mode.
Audit the pipeline: every label must have a timestamp BEFORE
the prediction would have been made.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Held-out test set accidentally used for tuning | Lock the test set IDs in a separate file; remove from training code; re-evaluate |
| Concept drift detected | Schedule retraining on recent data; A/B test old vs new; ship new if better on a fresh holdout |
| Label noise discovered | Audit labelers; re-label disagreements; retrain |
| Model card drift (out-of-date) | Make model card update a release gate |
| Drift monitoring shows PSI > threshold but no rollback plan | Stop serving predictions; serve heuristic fallback; freeze model version |
| Eval set was overwritten ("just this once, for fresher data") | Re-evaluate on the original test set; if gone, the release decision is invalidated |
| Reproducibility broken (seed drift, dependency upgrade) | Re-run from lockfile; if can't reproduce, treat the result as suspect |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial project framing (uses ml inputs) |
| `/roadmap` | Track data refresh, model retraining cadence |
| `/evals` | Set up AC-driven eval framework for model quality |
| `/data` | Data pipeline + data card authoring |
| `/incident` | Model accuracy drop; drift alert; rollback |
| `/audit` | Review model card, data card, eval protocol for completeness |