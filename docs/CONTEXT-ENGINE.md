# Context Engine

The Context Engine (`internal/context/`) solves the token efficiency problem:
instead of loading all 60 skills at session start (~55K tokens), it detects
your project's domain and loads only the 3–10 most relevant skills (~300 tokens).

## Architecture

```
radiant context detect
  └─→ Detector: filesystem signals + import scan
        ↓
  DetectionResult { Domain, Tier, RecommendedSkills, ... }
        ↓
radiant context assemble
  └─→ Registry: domain → skill subset (max 10)
        ↓
  Assembler: loads frontmatter.yaml only (not full SKILL.md)
        ↓
  Compressor: 4-pass token-aware compression
        ↓
  .radiant-harness/CONTEXT.md  (~300 tokens)
```

## Domain Detection

The detector assigns weights to filesystem signals:

| Signal | Weight | Domain |
|--------|--------|--------|
| `.sol` files | 15 | blockchain |
| `hardhat.config.js` | 20 | blockchain |
| `Cargo.toml` | 15 | systems |
| `go.mod` | 8 | backend |
| `package.json` | 8 | frontend |
| `requirements.txt` + `model.py` pattern | 12 | ml |
| `docker-compose.yml` | 8 | ops |
| `*.sol` imports in source | 5 | blockchain |

Up to 20 source files × 50 lines are scanned for import patterns.
The domain with the highest cumulative weight wins.

### Domains

| Domain | Triggered by |
|--------|-------------|
| `finance` | IFRS9, Basel, trading, risk keywords |
| `ml` | PyTorch, TensorFlow, sklearn, model.py |
| `frontend` | React, Vue, Angular, Next.js |
| `backend` | REST APIs, Go, Java Spring, databases |
| `ops` | Docker, Kubernetes, Terraform, CI/CD |
| `blockchain` | Solidity, Hardhat, Foundry, web3 |
| `systems` | Rust/Cargo, C, assembly, kernel |
| `science` | NumPy, SciPy, Jupyter, R |
| `general` | fallback |

### Tiers

| Tier | Signals |
|------|---------|
| `trivial` | <200 lines of code, single file |
| `feature` | standard feature work |
| `architecture` | cross-cutting changes, multiple packages |
| `product` | new product or major milestone |

## Skill Selection

```
Core skills (always loaded): nova-feature, validar, adr
+ Tier skills (1–3 based on tier): handoff, guard, depth-*
+ Domain skills (0–4 based on domain): refactor, api-design, finance-risk, ...
= 3–10 total (hard cap: 10)
```

## Compression (4-pass)

1. **Always**: strip `<!-- phase:done -->...<!-- /phase:done -->` blocks (free)
2. Trim skill descriptions to first sentence
3. Drop `## Loop Instructions` footer
4. Hard-trim at last line fitting budget

## Token Budget Profiles

| Profile | Total | Per-phase allocation |
|---------|-------|---------------------|
| `lean` | 10K | discover:1K, plan:2K, execute:5K, verify:1.5K, persist:0.5K |
| `standard` | 50K | discover:3K, plan:8K, execute:25K, verify:10K, persist:4K |
| `thorough` | 200K | discover:10K, plan:30K, execute:100K, verify:40K, persist:20K |

## CLI Reference

```bash
# Detect project domain
radiant context detect [--json]

# Assemble minimal CONTEXT.md
radiant context assemble [--budget=N] [--with-spec] [--skills=a,b,c]

# Compress existing CONTEXT.md to fit budget
radiant context compress --budget=2000

# Summarize a completed phase (reduces to ≤20% of original)
radiant context summarize --phase=execute

# Estimate token cost before running
radiant budget estimate [spec-file] [--profile=standard]
```

## Examples by Domain

### Go Backend
```
Detected: backend (score 28: go.mod=8, REST handler patterns=12, Dockerfile=8)
Tier: feature
Skills: nova-feature, validar, adr, handoff, guard, refactor, api-design
Tokens: ~280
```

### Solidity / Blockchain
```
Detected: blockchain (score 47: contracts/=12, .sol files=15, hardhat.config.js=20)
Tier: architecture
Skills: nova-feature, validar, adr, depth-vertical, guard, blockchain-audit
Tokens: ~310
```

### ML / Python
```
Detected: ml (score 24: requirements.txt=4, torch imports=12, model.py=8)
Tier: feature
Skills: nova-feature, validar, adr, handoff, ml-experiment, data-pipeline
Tokens: ~290
```

### React Frontend
```
Detected: frontend (score 16: package.json=8, React imports=8)
Tier: trivial
Skills: nova-feature, validar, adr
Tokens: ~180
```
