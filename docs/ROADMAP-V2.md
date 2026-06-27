# Radiant Harness v2.0 — Roadmap

> **Data**: 2026-06-26  
> **Versão alvo**: v2.0.0  
> **Status**: Planejado — aprovado para implementação

---

## Motivação

O harness atual (v0.7.0) é sólido em execução: 60 skills, 31 comandos, 6 plataformas, state machine crash-safe, MCP server, 3-layer security. Mas tem três lacunas estruturais que impedem de ser um harness verdadeiramente topo de linha:

1. **Skills carregadas de forma monolítica** — todos os 60 skills são embedados e extraídos independente do contexto. Um projeto de blockchain não precisa das skills de solvência atuarial na janela de contexto.

2. **Sem loop autônomo** — o agente precisa de intervenção humana entre fases. Não existe um ciclo Discover → Plan → Execute → Verify → Persist que rode sozinho até o budget ou a meta.

3. **Sem auto-melhoria** — o harness não aprende com seus próprios erros. Traces existem mas nada os analisa para melhorar instruções futuras.

A v2.0 resolve os três sem quebrar nada do que funciona hoje.

---

## Princípios (mantidos da v1, não-negociáveis)

1. **Zero vendor lock-in.** Nenhum LLM, IDE ou provedor é privilegiado.
2. **Vendor-neutral LLM.** Qualquer modelo OpenAI-compatible funciona.
3. **Cross-platform real.** 6 targets, build limpo a cada release.
4. **Sem SDKs pesados.** HTTP puro via `net/http`.
5. **Skills como contrato machine-readable.** Qualquer LLM parseia.
6. **Detecção em runtime.** Nada hardcoded.

### Novos princípios v2.0

7. **Context-First, Lazy Assembly.** O harness carrega apenas o que o contexto exige. Menos tokens = melhor performance de agente.
8. **Loop-Native.** Todo workflow tem um ciclo Discover → Plan → Execute → Verify → Persist como primitiva. Sem loop, não é workflow.
9. **Adversarial Verification.** Nenhum agente avalia o próprio trabalho. O verificador é sempre separado.
10. **Budget-Aware.** Token budget é cidadão de primeira classe — estimado antes, monitorado durante, reportado depois.
11. **Self-Improving.** O harness analisa traces de falha e propõe melhorias nas próprias instruções.
12. **Bootstrap Universal.** Qualquer LLM ou IDE entra pelo mesmo ponto — `radiant boot` — sem knowledge prévio do projeto.

---

## Visão de Arquitetura v2.0

```
┌─────────────────────────────────────────────────────────────┐
│                    QUALQUER AGENTE / LLM                    │
│          (Claude, Cursor, Copilot, Gemini, Codex…)          │
└─────────────────────┬───────────────────────────────────────┘
                      │  radiant boot (entrada universal)
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   BOOTSTRAP LAYER                           │
│  • Detecta projeto e domínio                                │
│  • Emite manifest <500 tokens (JSON ou Markdown)            │
│  • Recomenda 3-5 skills relevantes (não todas as 60)        │
│  • Instrui o agente sobre o loop a seguir                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   CONTEXT ENGINE                            │
│  detector → assembler → compressor                          │
│  • Lê sinais do projeto (arquivos, deps, specs existentes)  │
│  • Monta CONTEXT.md mínimo (apenas skills relevantes)       │
│  • Comprime quando token budget > 70%                       │
│  • Lazy load: metadados embedados, corpo carregado on-demand│
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    LOOP ENGINE                              │
│                                                             │
│   Discover → Plan → Execute → Verify → Persist → Repeat    │
│       │                           │                         │
│       │                    [adversarial verifier]           │
│       │                    [separado do executor]           │
│       │                                                     │
│   Budget Manager                Trace Recorder              │
│   • token limit hard            • JSONL por run             │
│   • iter limit                  • reasoning + evidence      │
│   • compress auto               • auditável                 │
│   • exit conditions             • alimenta self-improve     │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                 SELF-IMPROVEMENT ENGINE                     │
│  • Lê traces de falha                                       │
│  • Identifica padrões de erro                               │
│  • Propõe edits nas instruções de skills                    │
│  • Valida em tasks não vistas antes de aplicar              │
└─────────────────────────────────────────────────────────────┘
```

---

## Sprints v2.0

### Sprint 33 — Context Engine (v0.8.0)

**Tema**: Context-First — o harness carrega apenas o que o contexto exige.

**Problema**: Hoje `radiant init` extrai todos os 60 skills para `.radiant-harness/skills/`. Um projeto de frontend não precisa de 12 skills de risco de crédito na janela de contexto.

**Solução**: Skill registry com metadados embedados (~3KB total); corpo das skills carregado on-demand apenas quando solicitado.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `internal/context/detector.go` — lê sinais do projeto (go.mod, package.json, pom.xml, specs/, docs/) e determina domínio + tier | M | `radiant context detect` identifica corretamente ≥5 tipos de projeto |
| 2 | `internal/context/assembler.go` — monta CONTEXT.md mínimo com apenas skills relevantes | M | CONTEXT.md gerado tem ≤10 skills para qualquer projeto; skills irrelevantes ausentes |
| 3 | `internal/context/compressor.go` — comprime contexto quando token count > 70% do budget | S | `radiant context compress --budget=8000` comprime CONTEXT.md para ≤5600 tokens |
| 4 | Refactor `internal/skill/registry.go` — metadados embedados; corpo lazy | M | `go test ./internal/skill/...` verde; skill body NÃO é carregado se não usado |
| 5 | `radiant context detect` command | S | Retorna JSON com `domain`, `tier`, `signals`, `recommended_skills` |
| 6 | `radiant context assemble [--budget=N]` command | S | Gera `.radiant-harness/CONTEXT.md` com apenas skills relevantes |
| 7 | `radiant context compress` command | S | Sumariza fases completadas, descarta contexto stale |
| 8 | 15+ testes novos | S | `go test ./internal/context/... -race` verde |

**Files novos**: `internal/context/{detector,assembler,compressor}.go`, `internal/skill/registry.go` (refactor)  
**Files alterados**: `cmd/radiant/main.go`, `internal/skill/skill.go`

---

### Sprint 34 — Bootstrap Protocol (v0.8.1)

**Tema**: Entrada universal — qualquer LLM/IDE descobre o harness pelo mesmo ponto.

**Problema**: Hoje não existe um ponto de entrada único que qualquer agente possa chamar para entender o que o harness é e o que fazer a seguir. O agente precisa ter knowledge prévio.

**Solução**: `radiant boot` — comando que emite um manifest <500 tokens descrevendo o projeto, as skills relevantes, e o loop a seguir.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `internal/boot/` package — gera manifest em Markdown ou JSON | M | Manifest ≤500 tokens independente do tamanho do projeto |
| 2 | `radiant boot` command (Markdown, human-friendly) | S | Output legível; inclui domínio, tier, 3-5 skills recomendadas, próximos passos |
| 3 | `radiant boot --json` (JSON, machine-readable para LLMs) | S | JSON válido com campos `domain`, `tier`, `skills`, `loop`, `budget_estimate` |
| 4 | `radiant boot --agent=<name>` — manifest específico por IDE | S | Output adaptado ao formato preferido do agente (Claude, Cursor, etc.) |
| 5 | AGENTS.md v2 — referencia `radiant boot` como entry point | S | AGENTS.md gerado instrui agente a rodar `radiant boot` primeiro |
| 6 | Todos os 6 IDE adapters atualizados com referência ao boot | S | Cada adapter inclui instrução de bootstrap |
| 7 | Token budget estimate incluído no manifest | S | `budget_estimate` reflete custo esperado da tarefa em tokens |
| 8 | 10+ testes novos | S | `go test ./internal/boot/... -race` verde |

**Files novos**: `internal/boot/boot.go`, `internal/boot/manifest.go`  
**Files alterados**: `internal/scaffold/adapters.go`, todos os templates de IDE

---

### Sprint 35 — Loop Engine (v0.9.0)

**Tema**: Autonomia — o harness roda o ciclo completo sem intervenção humana.

**Problema**: Hoje `radiant run` executa uma spec, mas o ciclo Discover → Plan → Execute → Verify → Persist é manual. O agente precisa de um humano entre fases.

**Solução**: Loop engine com ciclo autônomo, verificação adversarial, budget controls, e traces de raciocínio.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `internal/loop/cycle.go` — state machine do ciclo completo | L | 5 estados: Discover, Plan, Execute, Verify, Persist. Transições válidas enforçadas. |
| 2 | `internal/loop/verifier.go` — verificador adversarial (agente separado) | M | Verifier usa modelo diferente ou prompt adversarial; nunca o mesmo agente que executou |
| 3 | `internal/loop/budget.go` — token + iteration budget manager | M | Hard limit: para o ciclo antes de estourar. Emite warning a 70%. |
| 4 | `internal/loop/trace.go` — reasoning trace recorder em JSONL | M | Cada ação registra: timestamp, agente, prompt hash, resultado, tokens usados, evidência |
| 5 | `radiant loop start "<goal>" [--budget=N] [--max-iter=N]` | M | Inicia ciclo autônomo; persiste estado em `.radiant-harness/loop.json` |
| 6 | `radiant loop status` — estado atual do loop em andamento | S | Exibe fase atual, iter count, tokens usados, tokens restantes |
| 7 | `radiant loop resume` — retoma loop interrompido | S | Lê `.radiant-harness/loop.json`; continua de onde parou |
| 8 | `radiant trace show <run-id>` — visualiza reasoning trace | S | Exibe trace formatado; suporta `--json` para output machine-readable |
| 9 | Exit conditions determinísticos | M | Para em: sucesso (verifier aprova), max-iter, budget esgotado, falha crítica (3x consecutivas) |
| 10 | 20+ testes novos | M | `go test ./internal/loop/... -race` verde; stress test de budget overflow |

**Files novos**: `internal/loop/{cycle,verifier,budget,trace}.go`  
**Files alterados**: `cmd/radiant/main.go`, `internal/harness/state.go` (integração)

---

### Sprint 36 — Enhanced Hooks + IDE Adapters (v0.9.1)

**Tema**: Eficiência por IDE — cada ferramenta recebe o melhor da sua plataforma.

**Problema**: O hook atual é apenas SessionStart com load-context.mjs simples. Não há hooks reativos (PostToolCall, PreToolCall), não há compressão de contexto triggered por evento, e os adapters de IDE não exploram capacidades específicas de cada plataforma.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `hooks/post-tool.mjs` — registra cada tool call no trace | S | Todo tool call produz entrada no JSONL trace |
| 2 | `hooks/pre-tool.mjs` — verifica budget antes de cada tool | S | Tool call bloqueada se budget < 10% restante |
| 3 | `hooks/load-context.mjs` v2 — token-aware, lazy loading | M | Carrega apenas CONTEXT.md (não skills completas); ≤2KB de overhead |
| 4 | Claude Code adapter v2 — settings.json com permissions, allowlists, PostToolCall, PreToolCall | M | settings.json gerado inclui hooks reativos; permissions explícitas |
| 5 | Cursor adapter v2 — formato `.cursor/rules/*.mdc` (novo formato MDC) | M | Regras geradas no formato MDC correto; sem usar AGENTS.md deprecated |
| 6 | Copilot adapter v2 — `.github/copilot-instructions.md` enriquecido | S | Inclui bootstrap reference e loop instructions |
| 7 | Gemini adapter v2 — `GEMINI.md` com escape correto + budget hints | S | Arquivo gerado sem erros de parse; inclui token budget guidance |
| 8 | Windsurf + Codex adapters atualizados | S | Mesma qualidade dos outros adapters |
| 9 | `radiant views --diff` — mostra diferenças antes de regenerar | S | Diff legível antes de sobrescrever qualquer view |
| 10 | 10+ testes novos | S | `go test ./internal/scaffold/... -race` verde |

**Files novos**: `hooks/post-tool.mjs`, `hooks/pre-tool.mjs`  
**Files alterados**: `internal/scaffold/adapters.go`, todos os templates de hooks e IDE

---

### Sprint 37 — Token Budget & Compression (v0.9.2)

**Tema**: Eficiência de tokens como cidadão de primeira classe.

**Problema**: Hoje não existe estimativa de custo antes de rodar, nem controle de compressão automática. Agents desperdiçam tokens repetindo contexto já processado.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `radiant budget estimate <spec-dir>` — estima tokens antes de rodar | M | Retorna estimativa por fase (Plan, Execute, Verify) com range min/max |
| 2 | `radiant budget report <run-id>` — relatório de custo pós-run | S | Exibe tokens por fase, custo estimado por provider, comparação com estimativa |
| 3 | Auto-compressão de contexto a 70% do budget | M | `--compress=auto` ativa compressão automática; fases completadas viram sumário |
| 4 | Context summarizer — fases completadas → compact summary | M | Sumário de fase completada ≤20% dos tokens originais; informação crítica preservada |
| 5 | `radiant context compress --from=<phase>` — comprime fases específicas | S | Comprime contexto de fase específica on-demand |
| 6 | Token tracking integrado ao loop engine (Sprint 35) | M | `radiant loop status` exibe tokens usados/restantes em tempo real |
| 7 | Budget profiles: `--profile=lean|standard|thorough` | S | `lean` = 20K tokens; `standard` = 50K; `thorough` = 200K |
| 8 | 10+ testes novos | S | `go test ./internal/context/... -race` verde com foco em compressor |

**Files novos**: `internal/context/summarizer.go`, `internal/context/budget_profiles.go`  
**Files alterados**: `internal/loop/budget.go`, `cmd/radiant/main.go`

---

### Sprint 38 — Self-Improvement Engine (v1.0.0-beta)

**Tema**: O harness aprende com os próprios erros.

**Problema**: Traces de falha existem mas ninguém os analisa. Skills com baixa taxa de sucesso não são detectadas nem melhoradas automaticamente.

**Insights de referência**: Self-Harness (Data Science Dojo) — melhoria de até 21pp de performance só ajustando instruções sem trocar o modelo.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | `internal/improve/analyzer.go` — analisa traces JSONL, agrupa falhas por padrão | M | Identifica ≥3 categorias de falha: premature-exit, wrong-scope, missing-verification |
| 2 | `internal/improve/proposer.go` — propõe edits nas skills baseado em padrões | M | Proposta é diff específico no `SKILL.md` ou `frontmatter.yaml` |
| 3 | `internal/improve/validator.go` — testa proposta em tasks held-out antes de aplicar | L | Proposta só é aplicada se melhora success rate; regressões são rejeitadas |
| 4 | `radiant improve --from-traces [--skill=<name>]` | M | Analisa traces da skill especificada (ou todas); propõe melhorias |
| 5 | `radiant improve --dry-run` — mostra propostas sem aplicar | S | Output legível mostrando o diff proposto por skill |
| 6 | `radiant improve --apply` — aplica propostas validadas | S | Aplica apenas propostas que passaram na validação; backup do original |
| 7 | Improvement history em `.radiant-harness/improvements.jsonl` | S | Cada melhoria aplicada tem registro: trace-ids que motivaram, diff, before/after success rate |
| 8 | 10+ testes novos | M | `go test ./internal/improve/... -race` verde |

**Files novos**: `internal/improve/{analyzer,proposer,validator}.go`  
**Files alterados**: `cmd/radiant/main.go`

---

### Sprint 39 — Multi-Agent Coordination (v1.0.0)

**Tema**: Platform layer — coordenação de múltiplos agentes especializados.

**Problema**: O harness gerencia um agente por vez. Tarefas grandes (refactor de sistema, migração de dados) precisam de múltiplos agentes em paralelo com papéis distintos.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | Agent role system: Planner, Implementer, Verifier, Summarizer | M | Cada role tem prompt template e budget específico |
| 2 | Shared context store — agentes leem/escrevem contexto compartilhado | M | Sem race conditions; locking via flock; atomic writes |
| 3 | `radiant fleet start "<goal>" --agents=N` — orquestra N agentes | L | Agentes trabalham em worktrees isolados; resultados mergeados |
| 4 | Inter-agent trust boundaries | M | Agente não pode escrever em worktree de outro; sandbox path enforçado |
| 5 | `radiant fleet status` — estado de todos os agentes em andamento | S | Tabela com agente, fase, tokens usados, última ação |
| 6 | Conflict resolution — dois agentes editam mesmo arquivo | M | Detecção de conflito; verifier decide qual versão prevalece |
| 7 | Fleet trace — trace unificado de todos os agentes do fleet | S | `radiant trace show --fleet <run-id>` exibe timeline unificada |
| 8 | 15+ testes novos | M | `go test ./internal/fleet/... -race` verde; stress test com 4 agentes paralelos |

**Files novos**: `internal/fleet/{coordinator,roles,store,resolver}.go`  
**Files alterados**: `cmd/radiant/main.go`, `internal/harness/state.go`

---

### Sprint 40 — Hardening + Documentação (v1.0.0-final)

**Tema**: Produção-ready — testes completos, docs atualizadas, migração de v0.7.

| # | Deliverable | Esforço | Critério de aceitação |
|---|-------------|---------|----------------------|
| 1 | End-to-end integration tests — todos os 6 IDE adapters | M | CI verde em darwin/arm64 e linux/amd64 para todos os adapters |
| 2 | Performance benchmark — token efficiency v2.0 vs v0.7.0 | M | v2.0 usa ≤60% dos tokens do v0.7.0 para mesmo projeto (medido com spec de referência) |
| 3 | Skill Schema v2.0 — adiciona `token_budget`, `context_tier`, `lazy_load` | M | `docs/SKILL-SCHEMA.md` atualizado; validação backwards-compatible com skills v1 |
| 4 | Migração guide — v0.7 → v1.0 | S | `docs/MIGRATION-V2.md` cobre todos os breaking changes com exemplos |
| 5 | Docs completas do Context Engine | S | `docs/CONTEXT-ENGINE.md` com exemplos de cada tipo de projeto detectado |
| 6 | Docs completas do Loop Engine | S | `docs/LOOP-ENGINE.md` com diagrama do state machine e exemplos |
| 7 | README.md atualizado — Quick Start para v2.0 | M | Novato consegue ir de zero a `radiant loop start` em <5 minutos |
| 8 | `go test ./... -race -count=1` com ≥400 testes passando | M | Zero falhas; zero races |
| 9 | `make release` produz 6/6 targets clean | S | Sem regressão de cross-compile |
| 10 | CHANGELOG.md com v1.0.0 entry completa | S | Todas as mudanças desde v0.7.0 documentadas |

**Status**: ✅ Entregue — v1.0.0

---

### Sprint 41 — Ontology / World Model (v1.1.0)

**Tema**: Formalizar o grafo de entidades e relações que o harness conhece.

**Status**: ✅ Entregue — v1.1.0

Entregas: `internal/ontology/` (10 entity kinds, 10 relation kinds, 4 axioms, 22 tests),
`internal/context/ontology_bridge.go` (anti-drift test), `docs/ONTOLOGY.md`,
CLI `radiant ontology export/validate/skills`, `radiant boot --world-model`.

---

### Sprint 42 — Worktree Isolation (v1.1.0)

**Tema**: Cada agente paralelo trabalha em checkout isolado real.

**Status**: ✅ Entregue — v1.1.0

Entregas: `internal/worktree/` (Manager sobre `git worktree`, 7 tests),
`internal/fleet/isolation.go` (ClaimIsolated com rollback em race, 5 tests),
CLI `radiant worktree add/list/remove/prune`.

---

### Sprint 43 — Schedule Stage (v1.1.0)

**Tema**: Fechar o loop: …→Persist→**Schedule**↺.

**Status**: ✅ Entregue — v1.1.0

Entregas: `internal/schedule/` (Evaluate puro, DetectSignals, LoadState/SaveState
atômico, 18 tests), `docs/SCHEDULE.md`, CLI `radiant loop schedule`.

---

### Sprint 44 — Loop Hardening: Human Checkpoint + Brakes (v1.2.0)

**Tema**: Fechar os 3 gaps de hard-stop confirmados pelo código de `awesome-loop-engineering`.

**Status**: ✅ Shipped — commit `a2f232e` | 61 testes | 82% cobertura

| Gap | Entregue | Arquivo |
|-----|----------|---------|
| Human escalation (`Escalate`) | ✅ | `verifier.go` + `cycle.go` (inbox, `PhaseAwaitingHuman`, `ExitNeedsHuman`) |
| No-progress brake (stall) | ✅ | `brake.go` (`StallBrake`, ring buffer, `Record`/`Reset`) |
| Time + cost budget | ✅ | `budget.go` (`CheckTime`, `CheckCost`, `EstimatedCostUSD`) + `pricing.go` (14 modelos) |

---

### Sprint 45 — Verifier Hardening: Review Panel + Quorum + Grounding (v1.3.0)

**Tema**: Tornar o verifier honesto — múltiplas lentes, mean geométrica, memória de contexto.

**Status**: ✅ Shipped — commit `f8f8a62` | 84 testes | 84.2% cobertura

| Gap | Código de referência | Descrição |
|-----|---------------------|-----------|
| Review panel pós-convergência | `jonny981/loops:loop.ts:config.review()` | Segunda camada roda APÓS convergência; falha reabre loop com `lastReview` |
| Quorum k-of-n | `jonny981/loops:condition.ts:quorum()` | N juízes paralelos; erro = voto "não"; K devem passar |
| Geometric mean por dimensão | `jonny981/loops:condition.ts:geometricMean()` | Um eixo zero → score zero; mais honesto que média aritmética |
| Commit-log grounding | `jonny981/loops:ground.ts:groundingText()` | Injeta últimos N commits no prompt de cada iteração limpa (evita amnésia) |
| Cláusulas anti-cheat | `awesome-loop-engineering:FIELD-NOTES.md #8` | Verifier detecta e rejeita: tests deletados, stubs, scope widening |

Entregas: `internal/loop/review.go`, extend `internal/loop/verifier.go` (quorum + dimensions + anti-cheat),
`internal/loop/ground.go`, flags `--quorum-n/--quorum-k/--review-restarts/--ground`.
≥20 testes. Version bump → v1.3.0.

---

## Sumário de Entregas por Sprint

| Sprint | Versão | Tema | Novos Cmds | Novos Pkgs | Testes Mínimos | Status |
|--------|--------|------|-----------|-----------|----------------|--------|
| 33 | v0.8.0 | Context Engine | 2 | `context/` | +15 | ✅ |
| 34 | v0.8.1 | Bootstrap Protocol | 3 | `boot/` | +10 | ✅ |
| 35 | v0.9.0 | Loop Engine | 4 | `loop/` | +20 | ✅ |
| 36 | v0.9.1 | Enhanced Hooks + IDEs | 1 | — | +10 | ✅ |
| 37 | v0.9.2 | Token Budget | 3 | — | +10 | ✅ |
| 38 | v1.0.0-beta | Self-Improvement | 3 | `improve/` | +10 | ✅ |
| 39 | v1.0.0 | Multi-Agent | 3 | `fleet/` | +15 | ✅ |
| 40 | v1.0.0-final | Hardening + Docs | 0 | — | +20 | ✅ |
| 41 | v1.1.0 | Ontology / World Model | 3 | `ontology/` | +22 | ✅ |
| 42 | v1.1.0 | Worktree Isolation | 4 | `worktree/` | +12 | ✅ |
| 43 | v1.1.0 | Schedule Stage | 1 | `schedule/` | +18 | ✅ |
| 44 | v1.2.0 | Loop Hardening (brakes + escalate) | 2 | `loop/` | +30 | ✅ |
| 45 | v1.3.0 | Verifier Hardening (review + quorum + grounding) | 3 | `loop/` | +23 | ✅ |

| 46 | v1.4.0 | CLI Wiring (loop start flags + loop review) | 1 | `cmd/` | 0 | ✅ |
| 47 | v1.5.0 | Loop Runner — `loop.Run()` conecta LLM ao ciclo autônomo | 1 | `internal/loop/` | 21 | ✅ |
| 48 | v1.6.0 | Loop Runner Wiring — `loopStartCmd` → `loop.Run()` end-to-end | 1 | `cmd/` | 0 | ✅ |
| 49 | v1.7.0 | Status cost display + `loop resume` → `loop.Run()` | 2 | `internal/loop/`, `cmd/` | 0 | ✅ |

**Total entregue (Sprints 1–49)**: 19 packages, 104 testes no loop package, suite completa green  
**Versão atual**: v1.7.0

---

## Métricas de Sucesso (por sprint)

- [ ] `go test ./... -race -count=1` 100% verde
- [ ] `go vet ./...` zero warnings
- [ ] `gofmt -l .` zero unformatted files
- [ ] `make release` produz 6/6 targets
- [ ] Validation report commitado em `docs/validation-report-sprint-N.md`
- [ ] Nenhuma regressão de vendor-neutrality
- [ ] Token efficiency: cada sprint novo NÃO piora o custo de tokens do bootstrap

### Métricas específicas v2.0

- [ ] Sprint 33: `radiant context detect` identifica ≥5 tipos de projeto corretamente
- [ ] Sprint 34: `radiant boot` emite manifest ≤500 tokens para qualquer projeto
- [ ] Sprint 35: loop roda ciclo completo sem intervenção humana em spec de referência
- [ ] Sprint 37: v2.0 usa ≤60% dos tokens do v0.7.0 para mesma tarefa
- [ ] Sprint 38: self-improvement melhora success rate ≥10pp em skills testadas
- [ ] Sprint 40: novato vai de zero a `radiant loop start` em <5 minutos

---

## Anti-backlog v2.0

Itens **explicitamente fora do roadmap** v2.0:

- ❌ Fine-tuning ou training de modelos (o harness melhora instruções, não pesos)
- ❌ Interface gráfica (CLI é o canônico; TUI pode vir depois)
- ❌ Cloud-hosted harness (tudo roda local; remote é responsabilidade do agente)
- ❌ Vendor lock-in de qualquer tipo (incluindo OpenRouter como obrigatório)
- ❌ Recursos que só funcionam com Claude (qualquer feature deve funcionar em qualquer LLM)
- ❌ Dependências além de `cobra` e `yaml.v3` no binário principal

---

## Referências

Os insights que fundamentam esta v2.0 foram extraídos de:

- **Loop Engineering** (Rajesh Mane): sistema > prompt; Discover → Isolate → Verify → Persist → Schedule
- **Harness Engineering** (TheAIEngineering): scope + state + verification + budget + exit conditions + observability
- **Self-Harness** (Data Science Dojo): agente assiste próprias falhas → propõe correções → valida antes de aplicar
- **Code as Agent Harness** (Elisa Terumi / UIUC + Meta + Stanford): 3 camadas — Interface → Mechanisms → Scaling
- **Loop Engineering Paper** (João Cláudio / Google): Act → Observe → Learn → Repeat dentro da janela de contexto
- **Autodata** (arxiv:2606.25996): agentes especializados > sistema monolítico; hierarquia > flat; iteração > one-shot
- **Alexandre Dubugras**: mapear critério de sucesso com clareza antes de iniciar o loop
- **André Lindenberg**: termination criteria + state management tradeoffs + measurable outcomes
