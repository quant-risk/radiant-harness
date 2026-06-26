# Skill: deep-learning

> DL architectures, training recipes, distributed training,
> fine-tuning, preference learning. Deep learning is engineering
> as much as modelling — most gains come from training recipe,
> not architecture choice.

## Decision tree

```
Task + data + compute
        │
        ▼
[Step 1] Architecture (CNN / RNN / Transformer / ViT / diffusion / GAN)
        │   - default to pretrained
        │   - smaller model + more data often beats bigger + less
        ▼
[Step 2] Training recipe
        │   - optimizer (AdamW default)
        │   - LR schedule (warmup + cosine)
        │   - batch size + gradient accumulation
        │   - mixed precision (bf16/fp16)
        │   - gradient clipping
        ▼
[Step 3] Distributed (if needed)
        │   - DDP (data parallel) → multi-GPU
        │   - FSDP (fully sharded) → multi-node
        │   - DeepSpeed (ZeRO) → memory-efficient
        ▼
[Step 4] Eval + monitoring (TensorBoard / W&B)
        │
        ▼
[Step 5] Fine-tune or preference learning (if applicable)
```

## Workflow

### Architecture choice

| Task | Default architecture | Why |
|------|---------------------|-----|
| Image classification | ViT (pretrained) or ConvNeXt | State-of-the-art accuracy |
| Object detection | YOLO / DETR / DINO | Real-time vs accuracy trade-off |
| Segmentation | SAM / Mask2Former | Foundation models work well |
| Text classification | Fine-tuned BERT-family | Strong baseline |
| Text generation | Decoder-only LLM (Llama, Qwen) | Pretrained + instruction tune |
| Speech recognition | Whisper | Multilingual; strong |
| Translation | NLLB / mBART | Multilingual |
| Tabular | Still GBDT (XGBoost) wins | DL rarely beats GBDT |
| Time series | Informer / PatchTST | Recent SOTA |
| Multimodal | CLIP / Flamingo / LLaVA | Pretrained vision-language |

**Default rule**: use a pretrained model when one exists. Only
train from scratch with sufficient data + compute + reason.

### Optimizers

| Optimizer | When |
|-----------|------|
| **SGD + momentum** | Vision CNNs (ResNet tradition); well-tuned |
| **AdamW** | Default for transformers; LR 1e-4 to 5e-5 |
| **Lion** | Memory-efficient; competitive with AdamW |
| **Adafactor** | Very large models; memory-constrained |
| **LAMB / LARS** | Large batch training |

### LR schedule

| Schedule | When |
|----------|------|
| **Constant** | Rarely optimal; only as baseline |
| **Linear warmup + linear decay** | Simple; works for fine-tuning |
| **Warmup + cosine decay** | Default for training |
| **Warmup + cosine + cooldown** | Modern (MiniCPM style) |
| **Cyclic LR** | For ensembling or when stuck |

Typical: 5-10% warmup, then cosine to 10% of peak LR.

### Batch size + accumulation

| Scenario | Recipe |
|----------|--------|
| Fits in memory | Use largest batch that fits |
| Doesn't fit | Gradient accumulation; effective batch = batch × accum |
| Very large batch (>8k) | LAMB/LARS; warmup; LR scaling |

Effective LR scales with batch size (linear scaling rule, with
caveats).

### Mixed precision

- **bf16**: better numerical range; default on Ampere+ GPUs
- **fp16**: faster on older GPUs; needs loss scaling
- **fp32**: master weights; only for stability-critical parts

Throughput gain: 2-3x. Almost always worth it.

### Regularization

| Technique | When |
|-----------|------|
| **Weight decay** | Always (decoupled, AdamW) |
| **Dropout** | Before classifier head; less in middle layers |
| **Stochastic depth** | Very deep networks |
| **Data augmentation** | Always (task-specific) |
| **Label smoothing** | Classification; small benefit |
| **Early stopping** | Always (track val loss) |

### Distributed training

| Strategy | Memory | Compute | Use |
|----------|--------|---------|-----|
| **DDP** (DataParallel) | Full per GPU | Linear speedup | Multi-GPU single-node |
| **FSDP** | Sharded | Linear speedup | Multi-GPU / multi-node |
| **DeepSpeed ZeRO-1** | Optimizer sharded | Linear | Memory-constrained |
| **DeepSpeed ZeRO-2** | + gradients | Linear | Larger models |
| **DeepSpeed ZeRO-3** | + params | Linear | Very large models |
| **Pipeline (GPipe)** | Activations sharded | Near-linear | Very deep models |
| **Tensor parallel** | Params sharded | Linear | Inference / very large |

**Profile first**: DDP overhead can exceed gain for small models.

### Fine-tuning

| Method | Memory | Quality | When |
|--------|--------|---------|------|
| **Full fine-tune** | High | Best (with enough data) | Lots of data |
| **LoRA** | Low | Good | Default for LLMs |
| **QLoRA** | Very low | Good | Memory-constrained |
| **Prefix tuning** | Low | OK | Limited compute |
| **Adapter** | Low | OK | Multi-task |

LoRA rank: 8-64 typically; higher for complex tasks.

### Preference learning

| Method | Approach | Use |
|--------|----------|-----|
| **RLHF** | SFT → reward model → PPO | Traditional alignment |
| **DPO** | SFT → direct preference loss | Simpler than RLHF; competitive |
| **IPO** | DPO variant | Less overfitting |
| **KTO** | Binary feedback | When preferences noisy |
| **ORPO** | SFT + preference jointly | Single-stage |

## Examples

### Example 1: fine-tune Llama-3-8B with QLoRA

```
Base:       Llama-3-8B (16GB)
Quantise:   4-bit (QLoRA) → ~5GB
Adapters:   LoRA r=16 → ~50MB trainable
Hardware:   1× A100 40GB
Recipe:
  - AdamW, lr=2e-4
  - Cosine schedule, 3% warmup
  - Batch=4, grad accum=4 (effective 16)
  - bf16 mixed precision
  - 3 epochs
Result:    matches full fine-tune on classification task
```

### Example 2: ViT from scratch (image classification)

```
Data:    100k labeled images
Model:   ViT-S/16 (22M params)
Recipe:
  - AdamW, lr=1e-3, weight decay 0.05
  - Warmup 10 epochs + cosine
  - Batch 256 (4 GPUs × 64)
  - bf16
  - Aug: RandAugment + Mixup + CutMix
  - Epochs: 300
  - EMA of weights
Result: 78% top-1 (ImageNet-style)
```

### Example 3: DPO alignment

```
Base: Llama-3-8B-Instruct (already SFT'd)
Data: 50k preference pairs (chosen vs rejected)
Method: DPO
  - β = 0.1
  - lr = 5e-6
  - 1 epoch
  - bf16
Result: improved helpfulness on internal eval; minimal regression on safety
```

## Anti-patterns

### ❌ Training from scratch when pretrained exists

Use pretrained. Save weeks of compute.

### ❌ No LR schedule

Constant LR is rarely optimal. Use warmup + decay.

### ❌ fp32 everywhere

Wastes 2-3x throughput. Use bf16/fp16 with care.

### ❌ Distributed without profiling

DDP overhead can exceed gain for small models. Profile first.

### ❌ No checkpointing

Crash at hour 23 of 24 = data loss. Checkpoint every N steps.

### ❌ No eval during training

Train blind = miss overfitting, divergence, hardware failures.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Loss NaN | Lower LR; gradient clip; bf16; check data |
| Loss plateau | LR schedule; architecture; data quality |
| GPU OOM | Reduce batch; gradient checkpointing; FSDP/DeepSpeed |
| Diverged | Roll back to checkpoint; lower LR; warmup |
| Overfit | More data; augmentation; regularization; early stop |
| Slow convergence | Better init; LR schedule; architecture |
| Reproducibility broken | Seed; deterministic algorithms; version control |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/ml` | Project-level ML workflow |
| `/reinforcement-learning` | RL fine-tuning; RLHF |
| `/bayesian` | Bayesian neural networks |
| `/quantum-ml` | Hybrid quantum-classical |