# Radiant Harness — Research Analysis (14 Videos)

Videos de Valdemar Neto (Tech Leads Club) sobre Context Engineering,
Spec Driven Development, Harness Engineering e arquitetura modular.

## Insights Extraídos (por relevância para o radiant-harness)

### 1. RPI Framework (Research → Plan → Implement)

O padrão mais importante. Em vez de fazer tudo numa janela de contexto:

- **Research**: abre o leque, pesquisa, descobre. Pode usar contexto grande.
- **Plan**: salva em markdown, cria spec + design + tasks. O plano tem TODOS
  os detalhes que a IA precisa para implementar sem pesquisar de novo.
- **Implement**: pega o plano e executa. Contexto pequeno, focado.

**Impacto no harness**: O `/kickoff` já faz discovery, mas o `/nova-feature`
precisa de uma fase de Research explícita antes do Plan. O STATE.md já é
o "progress file" — mas precisa ser mais estruturado.

### 2. Progressive Disclosure + On-Demand Loading

- **CLAUDE.md/rules**: só o básico (always-apply). ~15k tokens.
- **Docs sob demanda**: linkados, carregados quando a tarefa exige.
- **Skills**: para tarefas repetitivas (criar task, revisar PR, etc).
- **MCPs**: para conhecimento remoto (Jira, Confluence, GitHub).

**Impacto no harness**: Já fazemos isso bem. Mas o hook deveria ser mais
inteligente — não só STATE + vision + roadmap, mas também ler o frontmatter
`description` dos docs para saber o que mais carregar.

### 3. Context Window Management

- **Smart zone**: até 40% da janela de contexto.
- **Dumb zone**: acima de 60% — a IA alucina.
- **200k tokens** é a janela ideal (não 1M — mais tokens = mais alucinação).
- **Subagents**: para manter contexto limpo. Cada subagent recebe só o que precisa.
- **Nova janela**: sempre abrir contexto novo para implementação após pesquisa.

**Impacto no harness**: O `/clarificar` (uma pergunta por vez) já ajuda.
Mas precisa de um mecanismo explícito de "salvar plano e abrir novo contexto".

### 4. Harness Engineering (o próximo passo além de Spec Driven)

Spec Driven é um TIPO de harness. Harness completo inclui:

- **Feed Forward**: spec, rules, guidelines, skills (preventivo)
- **Feedback**: linters, testes, type-checkers, review agent (corretivo)
- **Progress files**: STATE.md, log de decisões entre sessões
- **Bootstrap scripts**: que reconstruem contexto rapidamente
- **Agent contracts**: implementador e tester concordam no que fazer ANTES
- **Separate agents**: um implementa, outro valida (não subagents do mesmo processo)
- **Executable verification**: testes rodados como gate, não inspeção visual

**Impacto no harness**: O radiant-harness já tem muito disso (spec, design,
tasks, gates, eval). Falta: agent contracts, progress files mais estruturados,
e a separação implementador/validador.

### 5. Architecture Criticism (Clean Architecture vs Pragmatismo)

- Clean architecture RITUALÍSTICA (muitas camadas, interfaces para tudo)
  é RUIM para IA — cada arquivo extra = tokens extra.
- **Navigation Paradox paper (2026)**: DI/DI container cria conexões
  escondidas. IA acerta só 76% dos arquivos necessários.
- **Modular monolith** com fronteiras explícitas > microsserviços para IA.
- Princípios > cerimônia: lógica isolada, DI, testabilidade — sem as camadas.
- IA sugere arquitetura complexa porque o treinamento é enviesado (blogs > simplicidade).

**Impacto no harness**: O template `src/` com 4 camadas (domain/application/
infrastructure/interfaces) pode ser excessivo para projetos menores. Deveria
ter um template simplificado (2 camadas: core + adapters) para features triviais.

### 6. Agents.md Effectiveness (paper Universidade de Zurique)

- Agents.md gerado por IA: -3% sucesso, +20% custo vs não ter nada.
- Agents.md gerado por humano: +4% sucesso, +19% custo vs não ter nada.
- **Conclusão**: agents.md é NECESSÁRIO mas tem custo. Manter enxuto.
- Sem agents.md: IA alucina mais, deleta testes, ignora convenções.
- **Regra**: só o necessário no principal, linkar o resto sob demanda.

**Impacto no harness**: Já fazemos isso (CLAUDE.md enxuto + docs on-demand).
Validado pelo paper.

### 7. Skills Valdemar Neto (Top 5)

1. **TLC Spec Driven** — melhor em testes, pior em levantamento de requisitos
2. **Identificar domínios** — DDD para código legado
3. **Analisar acoplamento** — baseado no livro Balanced Coupling
4. **Resolver comentários GitHub** — priorizar e resolver review comments
5. **Criar design docs** — entrevista antes de gerar

**Impacto no harness**: Skills 2 e 3 são gaps nossos. Skill 4 já temos (`/revisar-pr`).
Skill 5 já temos (templates de design). Skill 1 é a inspiração principal.

### 8. Spec Driven Frameworks Benchmark ($2000 em tokens)

| Framework | Requisitos | Testes | Geral | Custo (tokens) |
|-----------|-----------|--------|-------|-----------------|
| TLC Spec Driven | 0.92 | melhor | 1º | 31M |
| GitHub Spec Kit | 0.96 | médio | 2º | 36M |
| OpenSpec | 0.96 | baixo | 3º | 24M |
| Superpowers | 0.94 | baixo | 4º | 31M |
| Sem framework (Opus) | — | — | 0.89 | 18M |

**Conclusões**:
- Modelo bom (Opus) sem framework faz 0.89 — quase tão bom quanto frameworks.
- Frameworks ATRAPALHAM modelos bons quando têm harness fraco de testes.
- TLC ganha por causa do hardness de testes (obriga AC→teste→verificação).
- Spec Kit gasta mais tokens por criar mais arquivos.
- Superpowers é bom em requisitos mas fraco em testes (surpreendente).

**Impacto no harness**: Nosso eval-spec-fidelity.mjs já faz AC→task→teste.
Mas falta: obrigação de rodar testes como gate (não opcional), e
verificação automática de que cada AC tem teste correspondente.

### 9. Think it / Build it / Ship it (Incremental Architecture)

Framework do Spotify para evoluir arquitetura sem over-engineering:

- **Think it**: exploração, premissas, decisões básicas
- **Build it**: MVP interno, experimentação, RFCs
- **Ship it**: rollout gradual para usuários externos
- **Ticket it**: melhorias incrementais baseadas em feedback

**Impacto no harness**: O `/kickoff` já suporta greenfield/brownfield.
Mas falta o conceito de "fases" no roadmap — Now/Next/Later não é
suficiente. Precisa de MVP → Ship → Iterate explícito.

### 10. UUIDv7/ULID para Performance

- UUIDv4 é randômico → destrói índices B-tree.
- UUIDv7/ULID: time-ordered → preserva performance do índice.
- Shopify usou esse approach para melhorar sistema de pagamentos.

**Impacto no harness**: Adicionar ao template de domain.md uma nota sobre
ID strategy (UUIDv7/ULID como padrão, não UUIDv4).

## Ações para o Radiant Harness

### Alta prioridade (melhoram diretamente o produto)

1. **Adicionar fase de Research ao `/nova-feature`** — antes de criar spec,
   fazer discovery. Salvar em markdown. Depois criar spec/tasks.

2. **Progress file mais estruturado** — STATE.md precisa de seções mais
   rígidas: decisions log, blockers, next actions, context bookmarks.

3. **Agent contracts** — template para "implementador concorda com validador"
   antes de começar. Checklist compartilhado.

4. **Executable verification obrigatória** — no DoD, os gates DEVEM ser
   executados (não apenas listados). O `/validar` já faz isso, mas o
   template de tasks deveria forçar.

5. **Template simplificado** — além do template completo (4 camadas DDD),
   oferecer um template "lean" (core + adapters) para features triviais.

### Média prioridade (melhoram o fluxo)

6. **Context window budget** — no CLAUDE.md, adicionar seção sobre quando
   abrir nova janela de contexto (acima de 40% → novo contexto).

7. **Subagent strategy** — documentar quando usar subagents (tarefas
   paralelas, pesquisa isolada) vs quando não usar.

8. **Git worktrees** — documentar como usar worktrees para paralelizar
   tasks independentes.

9. **Model routing** — documentar que modelo usar para quê (plano vs
   implementação). Opus para plan, Sonnet para implement.

10. **ID strategy no domain template** — UUIDv7/ULID como padrão.

### Baixa prioridade (refinamentos)

11. **Skill de análise de acoplamento** — inspirada na skill do Valdemar.
12. **Skill de identificação de domínios** — para código legado.
13. **Benchmark de frameworks** — comparar radiant-harness com TLC Spec Driven.
