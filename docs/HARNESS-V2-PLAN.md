# Radiant Harness v2.0 — Implementation Plan

> **Data**: 2026-06-26  
> **Status**: Aprovado — aguardando implementação  
> **Versão base**: v0.7.0 (60 skills, 31 comandos, 324+ testes)  
> **Versão alvo**: v1.0.0 (Context-First, Loop-Native, Self-Improving)

---

## 1. Diagnóstico da v0.7.0

### Forças (preservar)

| Componente | Estado |
|------------|--------|
| State machine crash-safe (`internal/harness/state.go`) | Excelente |
| 3-layer security (agent + gate + path allowlists) | Excelente |
| MCP server JSON-RPC 2.0 (`radiant mcp serve`) | Excelente |
| 6 IDE adapters (Claude/Cursor/Copilot/Codex/Gemini/Windsurf) | Bom |
| LLM clients (Anthropic/OpenAI/OpenRouter/BaseURL) | Excelente |
| Cross-compile 6 plataformas | Excelente |
| 60 skills com open schema MIT | Excelente |
| 324+ testes, `-race` flag | Excelente |

### Lacunas (resolver na v2.0)

| Lacuna | Impacto | Sprint |
|--------|---------|--------|
| Skills carregadas monoliticamente (todos os 60 no contexto) | Alto — desperdício de tokens | 33 |
| Sem ponto de entrada universal para agentes | Alto — agente precisa de knowledge prévio | 34 |
| Sem loop autônomo Discover → Verify → Persist | Alto — intervenção humana entre fases | 35 |
| Hooks básicos (só SessionStart) | Médio — sem reatividade a eventos | 36 |
| Sem controle de token budget | Médio — custos imprevisíveis | 37 |
| Sem self-improvement a partir de traces | Médio — harness não aprende | 38 |
| Sem coordenação multi-agente | Baixo — um agente por vez | 39 |

---

## 2. Arquitetura de Packages Novos

### `internal/context/`

```
internal/context/
├── detector.go      # Lê sinais do projeto, determina domain + tier
├── assembler.go     # Monta CONTEXT.md mínimo com skills relevantes
├── compressor.go    # Comprime contexto quando > 70% do budget
└── summarizer.go    # Sumariza fases completadas (Sprint 37)
```

**Sinais de detecção** (`detector.go`):

| Sinal | Domain | Tier |
|-------|--------|------|
| `go.mod` presente | backend/go | feature |
| `package.json` presente | frontend | feature |
| `pom.xml` / `build.gradle` | backend/java | feature |
| `specs/` com ≥1 spec | any | architecture |
| `docs/product/` existe | any | product |
| `go.mod` + imports `risk` ou `quant` | finance | architecture |
| `requirements.txt` + imports `torch` | ml | architecture |
| `Cargo.toml` | systems | feature |
| `*.sol` files | blockchain | feature |
| `k8s/` ou `helm/` dirs | ops | feature |

**Skill recommendation logic** (`assembler.go`):

```
domain=finance     → credit-risk, market-risk, regulatory, nova-feature, validar
domain=ml          → ml, deep-learning, stats, nova-feature, validar
domain=frontend    → frontend, api, nova-feature, validar, adr
domain=backend/go  → api, nova-feature, validar, adr, security
domain=ops         → nova-feature, adr, setup-ci, validar, security
domain=any+tier=product → nova-product, kickoff, mapear, adr, nova-feature
```

### `internal/boot/`

```
internal/boot/
├── boot.go       # Orquestra o manifest
└── manifest.go   # Estrutura e renderização do manifest
```

**Manifest schema** (JSON):

```json
{
  "version": "2.0",
  "project": {
    "domain": "finance",
    "tier": "feature",
    "name": "credit-scoring-api"
  },
  "recommended_skills": ["credit-risk", "api", "nova-feature", "validar"],
  "available_commands": ["radiant spec", "radiant loop start", "radiant validate"],
  "loop": {
    "pattern": "discover → plan → execute → verify → persist",
    "start": "radiant loop start \"<goal>\" --budget=50000"
  },
  "budget_estimate": {
    "min_tokens": 15000,
    "max_tokens": 60000,
    "profile": "standard"
  },
  "context_file": ".radiant-harness/CONTEXT.md"
}
```

**Manifest Markdown** (≤500 tokens):

```markdown
# Radiant Boot — credit-scoring-api

Domain: finance | Tier: feature

## Recommended Skills
- credit-risk: Credit risk modeling and scoring
- api: REST/gRPC API design and implementation
- nova-feature: Feature specification and implementation
- validar: Acceptance criteria validation

## Next Steps
1. Load context: `radiant context assemble`
2. Start loop: `radiant loop start "<goal>" --budget=50000`
3. Check status: `radiant loop status`

Context: .radiant-harness/CONTEXT.md (~2KB, 4 skills)
Budget estimate: 15K–60K tokens
```

### `internal/loop/`

```
internal/loop/
├── cycle.go     # State machine do ciclo
├── verifier.go  # Adversarial verifier
├── budget.go    # Token + iteration budget
└── trace.go     # JSONL trace recorder
```

**Cycle state machine** (`cycle.go`):

```
States:  idle → discover → plan → execute → verify → persist → done
                                    ↑              |
                                    └── (failed) ──┘ (max 3x)

Exit conditions:
  success:  verifier aprova resultado
  budget:   tokens usados ≥ budget limit
  max-iter: iterações ≥ max-iter limit
  critical: 3 falhas consecutivas no mesmo estado
```

**Trace JSONL** (`trace.go`):

```json
{"ts":"2026-06-26T10:00:00Z","run":"run-abc123","phase":"execute","agent":"claude-sonnet-4-6","action":"write_file","tokens_in":1200,"tokens_out":800,"result":"ok","evidence":"tests pass: 12/12"}
{"ts":"2026-06-26T10:01:00Z","run":"run-abc123","phase":"verify","agent":"claude-haiku-4-5","action":"review","tokens_in":2000,"tokens_out":400,"result":"approved","evidence":"all ACs satisfied"}
```

**Budget manager** (`budget.go`):

```go
type Budget struct {
    MaxTokens    int     // hard limit
    MaxIter      int     // iteration limit
    WarnAt       float64 // warn threshold (default 0.70)
    UsedTokens   int
    UsedIter     int
}

func (b *Budget) Check() BudgetStatus  // ok | warning | exceeded
func (b *Budget) Consume(tokens int)
func (b *Budget) Remaining() int
```

### `internal/improve/`

```
internal/improve/
├── analyzer.go   # Analisa traces, agrupa falhas por padrão
├── proposer.go   # Propõe edits nas skills
└── validator.go  # Valida proposta em held-out tasks
```

**Failure patterns detectados** (`analyzer.go`):

| Pattern | Detectado por | Fix típico |
|---------|--------------|------------|
| `premature-exit` | verifier rejeita mas agente marcou como done | Adicionar verificação obrigatória no SKILL.md |
| `wrong-scope` | agente modifica arquivos fora do spec | Adicionar constraint de escopo no frontmatter |
| `missing-verification` | execute completa sem chamar verify | Tornar verify obrigatório no gate |
| `context-overflow` | tokens usados > 90% antes de verify | Reduzir contexto inicial da skill |
| `retry-loop` | mesmo erro 3x seguidas | Adicionar exit condition mais cedo |

### `internal/fleet/`

```
internal/fleet/
├── coordinator.go  # Orquestra múltiplos agentes
├── roles.go        # Definições de roles (Planner/Implementer/Verifier)
├── store.go        # Shared context store
└── resolver.go     # Conflict resolution
```

---

## 3. Novos Comandos CLI

### `radiant context`

```
radiant context detect              # detecta domain + tier + signals
radiant context assemble            # monta CONTEXT.md mínimo
radiant context assemble --budget=8000  # budget-aware assembly
radiant context compress            # comprime fases completadas
radiant context compress --from=execute  # comprime fase específica
```

### `radiant boot`

```
radiant boot                        # manifest Markdown (human-friendly)
radiant boot --json                 # manifest JSON (machine-readable)
radiant boot --agent=cursor         # manifest adaptado para Cursor
radiant boot --agent=claude         # manifest adaptado para Claude Code
```

### `radiant loop`

```
radiant loop start "<goal>"         # inicia loop com budget padrão (50K)
radiant loop start "<goal>" --budget=100000 --max-iter=20
radiant loop start "<goal>" --profile=lean|standard|thorough
radiant loop status                 # estado atual (fase, iter, tokens)
radiant loop resume                 # retoma loop interrompido
```

### `radiant trace`

```
radiant trace show <run-id>         # visualiza trace formatado
radiant trace show <run-id> --json  # trace em JSON
radiant trace list                  # lista todos os runs
```

### `radiant budget`

```
radiant budget estimate <spec-dir>  # estima tokens antes de rodar
radiant budget report <run-id>      # relatório de custo pós-run
```

### `radiant improve`

```
radiant improve                     # analisa todos os traces, propõe melhorias
radiant improve --skill=credit-risk # foca em skill específica
radiant improve --dry-run           # mostra propostas sem aplicar
radiant improve --apply             # aplica propostas validadas
```

### `radiant fleet`

```
radiant fleet start "<goal>" --agents=3
radiant fleet status
radiant fleet stop
```

---

## 4. Mudanças em Packages Existentes

### `internal/skill/` (refactor)

**Antes**: 60 skills embedadas completas no binário (~500KB de texto).  
**Depois**: Apenas metadados embedados (~3KB); corpo lazy-loaded do filesystem.

```go
// Antes
//go:embed skills/*/SKILL.md
//go:embed skills/*/frontmatter.yaml

// Depois
//go:embed skills/*/frontmatter.yaml  // apenas metadata (~3KB)
// SKILL.md carregado on-demand de .radiant-harness/skills/<name>/SKILL.md
```

**API pública não muda** — apenas o loading interno.

### `internal/scaffold/templates/hooks/load-context.mjs` (v2)

**Antes**: Carrega `docs/STATE.md` + spec ativa (flat, sem budget awareness).  
**Depois**:
1. Verifica se `.radiant-harness/CONTEXT.md` existe (produzido pelo `context assemble`)
2. Se sim, carrega apenas CONTEXT.md (≤2KB) em vez do conjunto completo de skills
3. Se não, roda `radiant context assemble` silenciosamente e carrega o resultado
4. Registra tokens estimados no trace

### `internal/scaffold/adapters.go` (enhance)

Todos os 6 adapters ganham:
- Referência a `radiant boot` como first step
- Referência a `radiant loop start` como entry point para tasks
- Token budget guidance (limite recomendado por tipo de IDE)

### `internal/harness/state.go` (integrate)

Loop engine usa o mesmo mecanismo de state persistence (`.radiant-harness/progress.json`) — sem duplicação de persistência.

---

## 5. Skill Schema v2.0

Adições backwards-compatible ao schema atual (`docs/SKILL-SCHEMA.md`):

```yaml
# Campos novos (opcionais — defaults aplicados se ausentes)

token_budget:
  estimate_tokens: 8000      # estimativa para skill completa
  context_tier: minimal      # minimal | standard | full
  lazy_load: true            # corpo carregado on-demand (default: true)

loop:
  supported: true            # skill funciona dentro do loop engine
  phases: [plan, execute, verify]  # fases que a skill usa
  exit_on_fail: 3            # falhas consecutivas antes de abort
  verifier_required: true    # verifier adversarial é obrigatório
```

Skills existentes funcionam sem esses campos — defaults são aplicados:
- `token_budget.lazy_load: true` (todos os skills)
- `loop.supported: false` (skills antigas não entram no loop automaticamente)

---

## 6. Estrutura de Diretórios Completa (v2.0)

```
radiant-harness/
├── cmd/radiant/main.go              # CLI entrypoint (cobra, 31→48 commands)
├── internal/
│   ├── boot/                        # NEW: bootstrap protocol
│   │   ├── boot.go
│   │   └── manifest.go
│   ├── context/                     # NEW: context engine
│   │   ├── detector.go
│   │   ├── assembler.go
│   │   ├── compressor.go
│   │   └── summarizer.go
│   ├── loop/                        # NEW: loop engine
│   │   ├── cycle.go
│   │   ├── verifier.go
│   │   ├── budget.go
│   │   └── trace.go
│   ├── improve/                     # NEW: self-improvement
│   │   ├── analyzer.go
│   │   ├── proposer.go
│   │   └── validator.go
│   ├── fleet/                       # NEW: multi-agent (Sprint 39)
│   │   ├── coordinator.go
│   │   ├── roles.go
│   │   ├── store.go
│   │   └── resolver.go
│   ├── skill/                       # REFACTOR: lazy loading
│   │   ├── registry.go              # metadata-only embed
│   │   ├── loader.go                # on-demand body load
│   │   └── skill.go                 # (existing, enhanced)
│   ├── harness/                     # ENHANCE: integrate loop
│   │   ├── state.go                 # (existing)
│   │   └── protocols.go             # (existing)
│   ├── scaffold/                    # ENHANCE: richer adapters
│   │   ├── adapters.go              # (enhanced)
│   │   └── templates/
│   │       └── hooks/
│   │           ├── load-context.mjs # v2: token-aware
│   │           ├── post-tool.mjs    # NEW: trace recorder
│   │           └── pre-tool.mjs     # NEW: budget check
│   ├── engine/                      # (existing)
│   ├── llm/                         # (existing)
│   ├── policy/                      # (existing)
│   ├── quality/                     # (existing)
│   └── spec/                        # (existing)
└── docs/
    ├── ROADMAP.md                   # (updated: links to ROADMAP-V2.md)
    ├── ROADMAP-V2.md                # NEW: this roadmap
    ├── HARNESS-V2-PLAN.md           # NEW: this plan
    ├── CONTEXT-ENGINE.md            # NEW: Sprint 33 docs
    ├── LOOP-ENGINE.md               # NEW: Sprint 35 docs
    ├── MIGRATION-V2.md              # NEW: Sprint 40 docs
    └── SKILL-SCHEMA.md              # UPDATED: v2.0 additions
```

---

## 7. Breaking Changes (v0.7 → v1.0)

| Change | Migration |
|--------|-----------|
| `radiant init` não extrai mais todos os skills | Rodar `radiant context assemble` para extrair skills relevantes |
| Skills em `.radiant-harness/skills/` não são mais atualizadas automaticamente | Usar `radiant update` explicitamente |
| AGENTS.md v2 referencia `radiant boot` | Agentes que liam o AGENTS.md antigo precisam adaptar o workflow |
| `load-context.mjs` v2 carrega CONTEXT.md (não skills individuais) | Nenhuma ação necessária — upgrade automático ao regenerar views |

Detalhes completos em `docs/MIGRATION-V2.md` (gerado no Sprint 40).

---

## 8. Ordem de Implementação Recomendada

Os sprints foram ordenados por impacto × dependência:

```
Sprint 33 (Context Engine)
    ↓ depende de: nothing (novo package)
Sprint 34 (Bootstrap)
    ↓ depende de: Sprint 33 (usa context detect)
Sprint 35 (Loop Engine)
    ↓ depende de: Sprint 33 (usa budget), Sprint 34 (usa boot)
Sprint 36 (Hooks + IDEs)
    ↓ depende de: Sprint 33 (usa CONTEXT.md), Sprint 35 (usa loop)
Sprint 37 (Token Budget)
    ↓ depende de: Sprint 35 (integra no loop)
Sprint 38 (Self-Improvement)
    ↓ depende de: Sprint 35 (usa traces)
Sprint 39 (Multi-Agent)
    ↓ depende de: Sprint 35 (usa loop cycle), Sprint 37 (usa budget)
Sprint 40 (Hardening)
    ↓ depende de: todos os anteriores
```

---

## 9. Testes por Componente

| Package | Tipo | Target |
|---------|------|--------|
| `internal/context/detector` | Unit | 15 testes (1 por tipo de projeto detectável) |
| `internal/context/assembler` | Unit | 10 testes (skill recommendation por domain) |
| `internal/context/compressor` | Unit | 5 testes (edge cases: already compressed, zero budget) |
| `internal/boot` | Integration | 8 testes (manifest JSON, Markdown, por agente) |
| `internal/loop/cycle` | Unit | 12 testes (todas as transições de state machine) |
| `internal/loop/verifier` | Unit | 5 testes (approved, rejected, timeout) |
| `internal/loop/budget` | Unit | 8 testes (warn, exceeded, stress) |
| `internal/loop/trace` | Unit | 5 testes (write, read, concurrent) |
| `internal/improve/analyzer` | Unit | 8 testes (cada failure pattern) |
| `internal/improve/proposer` | Unit | 5 testes (diff proposal format) |
| `internal/improve/validator` | Integration | 5 testes (apply + rollback) |
| `internal/fleet` | Integration | 10 testes (2 agentes paralelos, conflict, merge) |
| E2E (boot → loop → verify) | E2E | 5 testes completos |

**Total mínimo**: +101 testes novos → ≥425 total

---

## 10. Definição de Done (v1.0.0)

O harness v1.0.0 está pronto quando:

- [ ] `radiant boot` emite manifest ≤500 tokens para qualquer projeto
- [ ] `radiant context assemble` carrega ≤10 skills relevantes (não todas as 60)
- [ ] `radiant loop start "<goal>"` roda ciclo completo sem intervenção humana
- [ ] Ciclo verifica resultado com agente adversarial separado
- [ ] Budget hard limit funciona (loop para antes de estourar)
- [ ] `radiant improve --from-traces` propõe melhorias a partir de falhas reais
- [ ] v2.0 usa ≤60% dos tokens do v0.7.0 para mesma tarefa (benchmark com spec de referência)
- [ ] `go test ./... -race -count=1` ≥425 testes passando, zero falhas
- [ ] `make release` produz 6/6 targets clean
- [ ] Novato vai de zero a loop rodando em <5 minutos seguindo README
- [ ] Nenhuma feature funciona apenas com Claude (tudo funciona com qualquer LLM OpenAI-compatible)
