# Skill: synthetic-data

> Synthetic data generation: agentic (Autodata), self-instruct,
> statistical methods, privacy preservation, quality evaluation.
> Synthetic data is a tool; downstream utility is the proof.

## Decision tree

```
Need synthetic data
        │
        ▼
[Step 1] Define purpose (training / eval / augmentation / privacy)
        │
        ▼
[Step 2] Choose generation method
        │   - LLM self-instruct (text)
        │   - Agentic data scientist (Autodata; tasks)
        │   - Statistical / noise (tabular)
        │   - GAN / VAE / diffusion (image, audio)
        │   - Differential privacy (privacy-preserving)
        ▼
[Step 3] Generate
        │
        ▼
[Step 4] Evaluate (fidelity, diversity, utility, privacy)
        │
        ▼
[Step 5] Iterate (meta-optimise if Autodata-style)
        │
        ▼
[Step 6] Document (data card)
```

## Workflow

### When to use synthetic data

| Use case | Why synthetic |
|----------|---------------|
| **Augmentation** | Expand small datasets (especially minority classes) |
| **Eval data** | Real data sensitive / unavailable; synthetic proxies |
| **Privacy** | Share data without exposing real records |
| **Testing pipelines** | Deterministic fixtures; demo data |
| **Cold-start** | No real data yet; bootstrap with synthetic |
| **Training (LLM)** | Self-instruct / agentic for instruction following |

**Don't use synthetic to predict real** when you have real data
— utility gap is usually large.

### Methods

#### Self-Instruct (LLM)

```
1. Seed with N (e.g. 100) hand-written examples
2. Prompt LLM: "Generate K more diverse examples"
3. Filter: dedup, quality checks, format validation
4. Repeat with filtered outputs as new seeds
5. Stop at desired size
```

Diversity controls:
- Temperature (0.7-0.9)
- Top-p (0.9)
- Add explicit "be diverse" prompts
- Embedding-based dedup (cosine similarity > 0.8 → drop)

#### Agentic Data Scientist (Autodata, Kulikov 2026)

```
1. Define task + scoring rubric
2. Data scientist agent proposes dataset
3. Score proposed dataset against rubric
4. Meta-optimise the agent (RL on rubric scores)
5. Final agent generates high-quality dataset
```

Result: agentic outperforms classical self-instruct on:
- CS research tasks
- Legal reasoning
- Reasoning with mathematical objects

Key: the agent LEARNS what makes good data via meta-optimisation.

#### Statistical (Tabular)

For tabular data (numerical + categorical):
- Fit marginal distributions per column
- Sample independently (loses correlation)
- Better: fit joint via copulas or Bayesian networks
- Best: CTGAN / TVAE (deep generative models for tabular)

Validation:
- KS test per numerical column
- Chi-square per categorical
- Correlation matrix comparison
- Downstream ML utility

#### GAN / VAE / Diffusion (Image, Audio)

- DCGAN, StyleGAN (images)
- VAE (compact latent)
- Diffusion models (high-quality, slow)
- Audio: WaveNet, Jukebox

#### Differential Privacy (Privacy-preserving)

Laplace or Gaussian mechanism adds calibrated noise:
- Privacy budget ε (smaller = more private)
- DP-GAN, PATE, Opacus (PyTorch)
- Membership inference resistance

### Quality evaluation

| Dimension | Metric |
|-----------|--------|
| **Fidelity** | Marginal / joint / correlation match; KS test; chi-square |
| **Diversity** | Coverage of input space; embedding-based dedup rate |
| **Utility** | TSTR (train synthetic / test real); TRTR baseline |
| **Privacy** | Membership inference attack AUC; nearest-neighbour distance |
| **Bias** | Demographic parity, equalised odds; compared to source |

### Privacy leakage tests

**Membership inference**: attacker tries to determine if a real
record was in the training set. AUC > 0.55 indicates leakage.

**Nearest-neighbour**: for each synthetic record, find nearest
real record. If distance < threshold → leakage.

**Attribute inference**: predict held-out attribute from others;
high accuracy → correlation preserved (could be leakage).

### Bias amplification

| Source | Synthetic |
|--------|-----------|
| 50% male, 50% female | Could become 80% male if agent over-samples |
| Rare group at 1% | Could be 0% (mode collapse) or 5% (over-correction) |

Always measure: P(synthetic | group) vs P(real | group).

### Meta-optimisation (Autodata key insight)

```
For each iteration:
  1. Data scientist agent generates dataset D_i
  2. Score S(D_i) on downstream task / rubric
  3. Update agent params θ to maximise E[S(D_θ)]
  4. Repeat
```

Result: agent improves at creating datasets that score well.
Even more important: meta-optimisation can outperform any fixed
agent.

## Examples

### Example 1: LLM self-instruct (instruction following)

```
Seed: 100 hand-written instructions (variety: open-ended, classification, code, math)
Method: GPT-4 generates 10 per seed = 1000 candidates
Filter:
  - Length 5-500 tokens (drop extremes)
  - Dedup (cosine < 0.85)
  - Quality check (passes classifier on "is this a real instruction?")
Final: 8,500 unique, high-quality instructions
Downstream utility: Llama-3-8B + self-instruct beats base by 12% on MT-Bench
```

### Example 2: tabular synthetic (CTGAN)

```
Source: 50k customer records (PII)
Method: CTGAN with DP-SGD (ε=1.0, δ=1e-5)
Validation:
  - KS per column: 7/10 columns not significantly different
  - Correlation matrix: cosine similarity 0.92
  - TRTR utility: model trained on real, tested on real = 0.85 AUC
  - TSTR utility: model trained on synthetic, tested on real = 0.78 AUC
  - Gap = 0.07 (acceptable)
  - Membership inference AUC: 0.52 (no leakage)
```

### Example 3: agentic data scientist (Autodata-style)

```
Task: CS research paper classification (intro / methods / results / discussion)
Agent: LLM fine-tuned iteratively
Iteration 0: random baseline = 60% accuracy
Iteration 1: agent trained on initial dataset = 65%
Iteration 5: agent meta-optimised = 78%
Iteration 10: agent meta-optimised + diversity penalty = 82%
Final dataset: 5k high-quality labelled examples
Model trained: 84% accuracy (vs 65% classical self-instruct)
```

## Anti-patterns

### ❌ Synthetic without downstream test

Looks great (good fidelity) but doesn't help the model. Always
TRTR / TSTR.

### ❌ Training on synthetic to predict real

Utility gap is usually large. Use real data when you have it.

### ❌ No privacy assessment

Leakage via nearest-neighbour or membership inference. Always
test.

### ❌ Bias amplification

Source has 1% rare group; synthetic amplifies to 5% (or collapses
to 0%). Measure and correct.

### ❌ Self-instruct without diversity control

Mode collapse: all outputs look the same. Diversity penalties;
embedding-based dedup.

### ❌ DP with overly large ε

"ε = 100" is meaningless. Typical ε ∈ [0.1, 10] for meaningful
privacy.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Synthetic not useful | Re-evaluate method; downstream task; data augmentation |
| Bias amplified | Re-balance; reject sampling; re-train agent with bias penalty |
| Privacy leakage | DP mechanism; larger noise; clipping |
| Mode collapse | Diversity penalty; temperature; explicit "be diverse" |
| Distribution drift | Re-generate periodically; monitor production |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/ml` | Downstream task; training pipeline |
| `/data` | Source data card; lineage |
| `/causal` | Synthetic data for causal inference |
| `/stats` | Distribution matching; KS tests |
| `/security` | DP; privacy leakage; re-identification risk |

## Tools

| Tool | Purpose |
|------|---------|
| **SDV (Synthetic Data Vault)** | Tabular synthetic data |
| **CTGAN / TVAE** | Deep generative tabular |
| **Faker** | Simple / rule-based synthetic |
| **Mostly AI** | Tabular + time-series with privacy |
| **Gretel** | Tabular + text + privacy |
| **Opacus / PyDP** | Differential privacy (PyTorch) |
| **Smartnoise** | DP SQL queries |

## Citations

- Kulikov et al. (2026). "Autodata: An agentic data scientist
  to create high quality synthetic data." arXiv:2606.25996
- Wang et al. (2022). "Self-Instruct: Aligning Language Models
  with Self-Generated Instructions." ACL.
- Dwork, Roth (2014). "The Algorithmic Foundations of Differential
  Privacy." FnT TCS.
- Xu et al. (2019). "Modeling Tabular Data using Conditional
  GAN." NeurIPS (CTGAN).
- Jordon et al. (2022). "Synthetic Data — what, why and how?"
  Royal Society.