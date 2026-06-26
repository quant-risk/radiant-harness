# Skill: stats

> Statistical modeling: hypothesis testing, CIs, regression,
> ANOVA, power, multiple testing. p-values without effect sizes
> are the original sin of quantitative work.

## Decision tree

```
Project reports statistical inference
        │
        ▼
[Step 1] State H0 / H1 BEFORE data (no peeking)
        │
        ▼
[Step 2] Power analysis -> required sample size (n)
        │
        ▼
[Step 3] Choose test (parametric / non-parametric)
        │
        ▼
[Step 4] Check assumptions (normality, homoscedasticity, independence)
        │
        ▼
[Step 5] Run test -> p-value, CI, effect size
        │
        ▼
[Step 6] Multiple-testing correction (if k > 1)
        │
        ▼
[Step 7] Write analysis-report.md
```

## Workflow

### Hypothesis framing

The hypothesis must be **specific** and **falsifiable**:

- H0: "There is no difference in conversion rate between variant A and B"
- H1: "Variant B has higher conversion rate than A"
- NOT: "Variant B is better" (too vague; not falsifiable)

| Component | Why |
|-----------|-----|
| **Direction** (one-sided / two-sided) | Pre-commits you to not flipping after seeing the data |
| **Effect size threshold** | "Meaningfully different" needs a number |
| **Population** | What does the sample represent? |
| **Decision rule** | What p-value / effect size triggers action? |

### Power analysis

Given α=0.05, power=0.80, effect size d=0.5, compute required n.
Tools: `statsmodels.stats.power`, R `pwr` package, G*Power.

| Effect size (Cohen d) | n per group (α=0.05, power=0.80) |
|----------------------|----------------------------------|
| 0.2 (small) | 394 |
| 0.5 (medium) | 64 |
| 0.8 (large) | 26 |

**Common mistake**: collecting data until p<0.05 ("optional stopping").
Each peek inflates Type I error. Pre-commit to n.

### Test selection

| Data shape | Parametric | Non-parametric |
|-----------|-----------|----------------|
| 1 sample vs known mean | one-sample t-test | Wilcoxon signed-rank |
| 2 independent samples | two-sample t-test | Mann-Whitney U |
| Paired | paired t-test | Wilcoxon signed-rank |
| >=3 groups | one-way ANOVA | Kruskal-Wallis |
| Categorical (2x2) | chi-square | Fisher exact |
| Categorical (rx c) | chi-square | — |

### Assumption checks

- **Normality**: Shapiro-Wilk (small n); Q-Q plot; histogram
- **Homoscedasticity**: Levene's test; residuals vs fitted plot
- **Independence**: Durbin-Watson (autocorrelation); study design

If violated → use non-parametric test, transform data, or use
robust methods (bootstrapped CIs).

### Multiple testing

k independent hypotheses × α=0.05 → expected false positives = k × 0.05.

| Method | Controls | When |
|--------|----------|------|
| **Bonferroni** | FWER (strong) | Few tests; conservative |
| **Holm** | FWER (less conservative) | Step-down alternative to Bonferroni |
| **BH (Benjamini-Hochberg)** | FDR | Many tests; exploratory |
| **BY (Benjamini-Yekutieli)** | FDR under arbitrary dependence | Dependence between tests |

Report uncorrected + corrected p-values; let the reader see.

### Effect size + CIs

A p-value tells you "is there an effect?". An effect size tells you
"how big?". A CI tells you "how uncertain?".

| Statistic | Effect size | CI |
|-----------|-------------|-----|
| t-test | Cohen d | d ± 1.96·SE |
| ANOVA | eta-squared, partial eta-squared | Bootstrap |
| chi-square | Cramér V | Bootstrap |
| regression | standardised β, R² | Bootstrap |

**Always report effect size + CI.** A statistically significant
effect of d=0.01 is not meaningful; a non-significant effect with
a wide CI is not "no effect" — it's "underpowered".

## Examples

### Example 1: A/B test (web product)

```
H0:  conversion_A == conversion_B
H1:  conversion_B > conversion_A (one-sided)
α:   0.05
Power: 0.80
Effect: MDE 1% absolute lift (baseline 5%)
Sample size: ~3000 per variant (using Evan Miller's calc)

Run 14 days; collect 6024 visitors / variant
Observed: A=5.1%, B=5.9%
Test:    two-proportion z-test, one-sided
Result:  z=1.83, p=0.034 (one-sided), CI [+0.1%, +1.6%]
Decision: SHIP variant B
```

### Example 2: regression diagnostics

```
Model: y ~ x1 + x2 + x1:x2
n:     1000
Assumptions:
  - linearity: residual plot OK
  - homoscedasticity: Breusch-Pagan p=0.12 OK
  - normality: Shapiro-Wilk p=0.04 -> suspect at tails
  - independence: Durbin-Watson 1.97 OK
Multicollinearity: VIF max=2.3 OK
Outliers: Cook's D max=0.04 OK
Decision: report with robust SEs
```

### Example 3: multiple comparisons (omics)

```
Study: 200 gene-expression contrasts
Family: 200 hypotheses, want FDR < 0.05
Method: Benjamini-Hochberg
Result: 47 significant at FDR<0.05; 8 at FDR<0.01
Report:  raw p-values + BH-adjusted p-values + q-values
```

## Anti-patterns

### ❌ p-value without effect size or CI

p<0.05 with d=0.01 is a "significant" effect that's not meaningful.
Always report effect size + CI.

### ❌ Multiple tests without correction

20 t-tests at α=0.05 → ~1 false positive expected. Always correct.

### ❌ HARKing

Stating H1 after seeing the data. Pre-commit to H0/H1 BEFORE
looking at the data.

### ❌ Optional stopping

Collecting data until p<0.05. Inflates Type I error dramatically.
Pre-commit to sample size.

### ❌ Parametric test on non-normal data

t-test requires approximately normal residuals. If violated, use
non-parametric test or transform.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Underpowered study | Replicate; meta-analyse; report wide CI |
| Assumptions violated | Switch to non-parametric; transform; use bootstrap |
| Multiple testing inflation | Apply correction; report raw + adjusted |
| Effect size trivial | Don't chase p<0.05 on trivial effects |
| Outliers dominate | Robust methods; sensitivity analysis |
| Stopping rule violated | Acknowledge; report all looks; adjust alpha |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/bayesian` | Bayesian alternative; better for small samples, sequential |
| `/causal` | Beyond correlation; treatment effects |
| `/evals` | Stat-rigour for ML eval methodology |
| `/ml` | When prediction is the goal, not inference |