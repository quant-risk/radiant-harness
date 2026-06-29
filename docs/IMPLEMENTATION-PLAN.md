# Plano de Implementação — Light/Full Release

> Status: em execução. Branch: `feature/light-full-release`.
> Versão alvo: **v2.37.0** (radiant-harness).

## Contexto

O radiant-harness já é um catálogo vivo de loop engineering patterns (ReACT, Reflection, Evaluator-Optimizer, Circuit Breaker, Bounded Execution, Multi-Agent Supervisor, etc.). O que falta:

1. **Abstração clara de modos operacionais** — o usuário escolhe entre o harness possuir o agente (Light / sampling) ou ser autônomo via API key (Full / HTTP).
2. **Semantic model** — camada "o que significa aqui" que mapeia termos de negócio pra estruturas concretas. Crítico pra Fortvna (CMN 4.966, IFRS 9, Basileia).
3. **Skill `lazy-executor`** — portar a ladder do ponytail como skill interna, com filtro por intensity.
4. **Tool Use formal** — passar de "LLM emite código, depois parseamos" pra "LLM chama tools estruturadas com schema validado".
5. **Higiene de código** — quebrar `helpers.go` (4931 linhas), consolidar pricing table (3 cópias), corrigir `pathIsSafe`, organizar docs.

## Os dois modos

### Light — harness possui o agent

- **Quem usa:** times que já pagam Claude Code / Cursor / Copilot / Codex e querem **reutilizar a assinatura do agente hospedeiro** pra inferência. Sem API key própria.
- **Como funciona:** o radiant roda como **MCP server**; quando precisa de uma chamada LLM, emite `sampling/createMessage` pro agente hospedeiro. O agente executa a inferência com as credenciais dele. Volta a resposta via JSON-RPC.
- **Vantagem:** zero custo duplicado, zero chave de API adicional, o agente "já tem" o contexto do usuário.
- **Requisito:** o agente precisa implementar MCP sampling (Claude Code, Hermes, Cursor, etc. suportam).
- **Setup:** `radiant setup-mcp --agent=claude --sampling`

### Full — autônomo, sem agente

- **Quem usa:** times que querem rodar o radiant como **worker autônomo** — CI, cron, batch, ou simplesmente quem prefere gerenciar a própria chave.
- **Como funciona:** o radiant faz chamadas HTTP diretas ao provedor LLM (OpenRouter, OpenAI, Anthropic, Groq, Mistral, xAI, ou qualquer OpenAI-compatible). Tem sua própria API key, seu próprio orçamento, seu próprio trace.
- **Vantagem:** rodável em CI sem agente, governança total de custo, controle total de modelo.
- **Requisito:** `OPENROUTER_API_KEY` (ou `OPENAI_API_KEY` / `ANTHROPIC_API_KEY`) no env ou em `.radiant.yaml`.
- **Setup:** `export OPENROUTER_API_KEY=sk-... && radiant config --provider=openrouter --model=claude-sonnet-4-6`

### Auto-detect

- `radiant doctor` detecta o modo mais provável e reporta.
- Regra: se `CLAUDE_CONFIG_DIR` existe OU `~/.claude/settings.json` tem entrada `radiant` MCP → sugere Light. Senão → Full.
- Usuário sempre pode forçar via `radiant loop start --mode=full|light` ou `RADIANT_MODE=full`.

## Fases

### ✅ Fase 0 — Setup
- Branch `feature/light-full-release`
- `.gitignore` cobre `.radiant-harness/` e `radiant` binário

### 🔨 Fase 1 — Abstração de Modos
- Novo pacote `internal/mode/` com `Mode` enum (Light, Full, Auto).
- Resolução: flag CLI > env (`RADIANT_MODE`) > config (`.radiant.yaml`) > auto-detect.
- Auto-detect: presença de MCP config do radiant → Light; senão → Full.
- Flag `--mode=light|full|auto` em `loop`, `fleet`, `run`.
- `radiant doctor` reporta modo resolvido + razão.
- Doc nova: `docs/MODES.md` (Light vs Full, quando usar cada).
- README atualizado com a seção "Choose your mode".
- Versão bump: `v2.37.0`.

### 🔨 Fase 2 — Consolidar pricing
- Novo pacote `internal/pricing/` — fonte única de verdade.
- Carrega de `internal/pricing/pricing.yaml` (YAML versionado).
- Gera 3 tabelas compiladas no build: `PresetModels`, `PricePerMTokensUSD`, `providerPricing`.
- Comando `radiant pricing` mostra tabela atual, avisa se desatualizada.
- Mantém compatibilidade: `loop/pricing.go` re-exporta de `internal/pricing`.

### 🔨 Fase 3 — Refatorar `helpers.go`
- Quebrar em pacotes temáticos:
  - `cmd/radiant/mcp.go` — `handleMCPRequest`, `mcpRunFull`, `callMCPTool`
  - `cmd/radiant/release.go` — `runRelease`, `bumpVersionInSource`
  - `cmd/radiant/ci.go` — `renderGitHubActions`, `renderGitLabCI`, `renderCircleCI`
  - `cmd/radiant/diagram.go` — `renderDiagram`, `codeDiagram`, etc.
  - `cmd/radiant/spec.go` — `renderSpecMD`, `renderTasksMD`, `nextSpecSeq`
- `helpers.go` sobra só com helpers genéricos (< 500 linhas).

### 🔨 Fase 4 — Skill `lazy-executor`
- Novo skill em `internal/skill/skills/lazy-executor/`:
  - `SKILL.md` — ladder do ponytail em PT-BR, otimizado pro contexto radiant
  - `frontmatter.yaml` — schema-compliant
- `intensity` field em `RunConfig` (lite/full/ultra).
- Flag CLI `--intensity=lite|full|ultra`.
- `loop/runner.go` injeta skill filtrada no system prompt do executor.
- Default: `full`. Same `ponytail` ladder, com nota: "no radiant, lazy = menos iteração necessária porque o verifier já corta".

### 🔨 Fase 5 — Semantic Model
- Novo pacote `internal/semantic/`:
  - `schema.go` — `Metric`, `Rule`, `Relationship`, `Scope`, `Expression`
  - `resolver.go` — `Resolve("RWA Corporate")` → expression tree
  - `loader.go` — carrega de YAML
  - `injector.go` — monta bloco pro system prompt
- Domínio `credit-risk` em `internal/semantic/metrics/credit-risk.yaml`:
  - `PD`, `LGD`, `EAD`, `RWA`, `ExpectedLoss`
  - `provision_min_ifrs9`, `provision_min_cmn4966`
  - References: tabela CMN 4.966 §4.2.1, IFRS 9 §5.5
- CLI: `radiant semantic resolve "<query>"`, `radiant semantic list`, `radiant semantic explain <metric>`.
- Loop runner injeta automaticamente se o domínio detectado for `credit-risk`.

### 🔨 Fase 6 — Tool Use formal
- Novo pacote `internal/tools/`:
  - `registry.go` — registry de tools com schema JSON
  - `executor.go` — executa tool calls com validação
  - `tools.go` — tools built-in: `read_file`, `write_file`, `search_code`, `run_gate`
- LLM passa a emitir `{"tool": "read_file", "args": {...}}` em vez de code blocks.
- Tracing separado para tool calls.
- Mantém compatibilidade com code blocks (parseados como fallback).

### 🔨 Fase 7 — Fix `pathIsSafe`
- Resolve symlinks antes de comparar (com `filepath.EvalSymlinks`).
- Rejeita target que escapa `projectDir` mesmo via symlink.
- Test: cria symlink fora, tenta escrever dentro, deve rejeitar.

### 🔨 Fase 8 — Docs
- README reescrito com seções: Quickstart, Modes (Light/Full), Commands, Architecture, Contributing.
- `docs/MODES.md` — guia detalhado dos dois modos.
- `docs/IMPLEMENTATION-PLAN.md` — este arquivo.
- `docs/ARCHITECTURE.md` atualizado com `internal/mode/` e `internal/semantic/`.
- `CHANGELOG.md` — entrada v2.37.0 com tudo.
- `RELEASE-NOTES.md` — notas de release pra v2.37.0.

### 🔨 Fase 9 — Validação final
- `go vet ./...` clean.
- `go test ./...` passa (suite completa).
- Smoke build: `make build && ./bin/radiant doctor && ./bin/radiant --version`.
- Cross-compile check: `GOOS=linux GOARCH=amd64 go build`.
- Tag `v2.37.0`.

## Princípios

1. **Compatibilidade retroativa total.** Quem usa `radiant loop start` sem flag continua funcionando. Modos são additive.
2. **Cada commit compila e testa verde.** Nada de "WIP" no main.
3. **Skills são conteúdo, não código.** Pattern do scaffold existente — embedded via `//go:embed`.
4. **Semantic model começa pequena.** Um domínio (credit-risk), umas 6 métricas. Provar valor antes de expandir.
5. **Lazy-executor é opt-in.** Default continua sendo "concrete complete implementation". Quem quiser `--intensity=ultra` ativa.

## Métricas de sucesso

- `radiant doctor` reporta modo corretamente em 100% dos casos.
- Loop com `--intensity=ultra` gera código menor que `--intensity=full` (métrica: tokens por iteração).
- `radiant semantic resolve "RWA Corporate"` retorna expression válida em < 100ms.
- Build cross-platform limpo nos 6 targets.
- Suite de testes: zero regressão, +30 testes novos (modes, pricing, semantic, lazy, tools, path).