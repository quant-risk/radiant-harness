<div align="center">

# Radiant Harness

**Spec-Driven Development para agentes de IA — escrito em Go.**

Scaffold e execute pipelines SDD para Claude Code, Codex, Cursor, Copilot, Gemini CLI e Windsurf.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Tests](https://img.shields.io/badge/tests-57%2B%20passing-brightgreen?style=flat-square)
![CI](https://img.shields.io/badge/CI-GitHub_Actions-blue?style=flat-square)

*Parte dos instrumentos da [Fortvna Risk Solutions](https://github.com/Fortvna-Risk-Solutions)*

</div>

---

## O que é

Radiant Harness é um **harness** — não apenas um scaffold. Ele fecha o loop
entre especificação e execução com validação automática, auto-correção e
persistência crash-safe.

```
Spec (O QUE construir)          Harness (COMO executar)
─────────────────────         ─────────────────────────
spec.md                       orchestrator.go
tasks.md                      validator.go
design.md                     feedback.go (auto-correção)
CLAUDE.md                     state.go (atômico + flock)
skills/                       context.go (RPI budget)
templates/                    agent.go (allowlist + timeout)
```

### Por que Go?

Single binary (zero runtime deps), concorrência nativa (goroutines, não
async), e tipagem forte para um domínio regulado. O
[ADR-0002](docs/adr-0002-go-rewrite.md) documenta o trade-off completo.

## Instalação

```bash
# Via go install
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# Ou build from source
git clone https://github.com/Fortvna-Risk-Solutions/radiant-harness.git
cd radiant-harness
go build -o radiant ./cmd/radiant/
```

Ou via Docker:

```bash
docker build -t radiant .
docker run --rm -v $(pwd):/work radiant init --all
```

## Uso

```bash
# Scaffold um projeto para um ou mais agentes (vendor-neutral)
radiant init --agent=claude,codex,cursor

# Ou todos os 6 adapters suportados
radiant init --all

# Validar conformidade do pipeline (audit + fidelity)
radiant validate

# Validar + executar gates das tasks (UAT de verdade)
radiant validate --gates

# Executar o harness em uma feature (modo agent — chama qualquer CLI da allowlist)
radiant run specs/0001-collect-feedback/ --agent=codex --retries=3

# Executar via LLM API direta (sem agente instalado, multi-provider)
radiant run specs/0001-collect-feedback/ \
  --provider=openrouter \
  --model=gpt-5 \
  --api-key=$OPENROUTER_API_KEY

# Ou direto na OpenAI, Anthropic, etc
radiant run specs/0001-collect-feedback/ \
  --provider=openai \
  --model=o3 \
  --api-key=$OPENAI_API_KEY

# Listar modelos disponíveis (presets curados)
radiant models
```

### Modelos suportados (10 presets)

Os presets cobrem os principais laboratórios e podem ser usados com qualquer
provedor OpenAI-compatible (OpenRouter, OpenAI, Anthropic via proxy, custom
BaseURL). Default `MaxTokens` de 32k; override por modelo.

| Família | Presets |
|---|---|
| Anthropic | `claude-opus-4.1`, `claude-sonnet-4.5`, `claude-sonnet-4` |
| OpenAI | `gpt-5`, `gpt-5-codex`, `gpt-4o` |
| Google | `gemini-2.5-pro` |
| DeepSeek | `deepseek-v4-pro`, `deepseek-v4-flash` |
| Xiaomi | `mimo-v2.5-pro` |

## Camada Spec (feed-forward)

| Componente | Descrição |
|---|---|
| **15 skills** | Comandos slash para o ciclo SDD completo |
| **7 templates** | spec, tasks, product, design, domain, lean, agent-contract |
| **Quality gates** | Audit, fidelity, mermaid, validate (com `--gates`) |
| **CI workflow** | GitHub Actions (lint + test + cross-build em Go 1.22–1.24) |
| **6 adaptadores** | Claude, Codex, Cursor, Copilot, Gemini, Windsurf |

## Camada Harness (feedback)

| Componente | Descrição |
|---|---|
| **Orchestrator** | Implementação + validação como processos separados |
| **Validator** | Contexto isolado, não subagente do implementador |
| **Auto-correction** | Falha → fix → re-teste (retries configuráveis) |
| **Agent teams** | Goroutines + semáforo (cap em 4 paralelos) |
| **State machine** | 8 estados, transições guardadas, persistência atômica |
| **Advisory flock** | `radiant run` concorrentes no mesmo projeto serializam |
| **Command allowlists** | Agentes e gates restritos a conjunto fechado |
| **Timeouts** | 10 min agent, 5 min gate, propagação via context |

## O Framework RPI

Toda feature segue **Research → Plan → Implement**, cada um em sua própria
janela de contexto:

1. **Research** — descobrir o que construir. Salvar em markdown.
2. **Plan** — definir como construir. Spec + design + tasks.
3. **Implement** — construir. Contexto fresco. Executar, verificar, ship.

Orçamento de tokens: 30% Research / 20% Plan / 50% Implement. Smart zone
< 40%, dumb zone > 60% — abra nova janela antes de passar.

## Comandos

| Comando | O que faz |
|---|---|
| `radiant init [dir]` | Scaffold do pipeline SDD |
| `radiant validate [dir]` | Validar conformidade (audit + fidelity) |
| `radiant validate --gates` | Validar + executar os gates das tasks |
| `radiant run <spec-dir>` | Executar o harness em uma feature |
| `radiant config` | Configurar provedor LLM |
| `radiant models` | Listar modelos disponíveis |

## Templates

| Template | Propósito |
|---|---|
| `spec.template.md` | Critérios de aceitação (Given/When/Then) |
| `tasks.template.md` | Decomposição de tarefas com gates |
| `product.template.md` | PRD-lite (por quê e para quem) |
| `design.template.md` | Documento de Design Técnico |
| `domain.template.md` | Modelo de domínio DDD |
| `lean-architecture.template.md` | Alternativa de 2 camadas |
| `agent-contract.template.md` | Acordo implementador ↔ validador |

## Skills (15)

| Skill | Propósito |
|---|---|
| `/kickoff` | Constituição do projeto |
| `/integracoes` | Ferramentas da equipe + MCPs |
| `/mapear` | Mapear codebase existente |
| `/diagramar` | Arquitetura Mermaid |
| `/roadmap` | Now/Next/Later |
| `/camada-agentica` | Regras, subagentes, skills, CI |
| `/nova-feature` | Loop RPI |
| `/clarificar` | Entrevista incansável |
| `/validar` | UAT com gates |
| `/revisar-pr` | Gate de conformidade SDD |
| `/setup-ci` | Pipeline CI/CD |
| `/metricas` | Lead Time, Throughput |
| `/auditar` | Auditoria do pipeline |
| `/evals` | Scoring de fidelidade da spec |
| `/handoff` | Continuidade de sessão |

## Arquitetura

Veja [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) para o documento completo.

```
cmd/radiant/          CLI principal (cobra)
internal/
  ├── engine/         Motor de execução (modo LLM-API)
  ├── harness/        Orchestrator + state + agent runner (modo agent)
  ├── llm/            Cliente universal (OpenRouter, OpenAI, Anthropic, custom)
  ├── quality/        Audit, fidelity, mermaid, validate, gates
  ├── scaffold/       Templates embutidos + adaptadores (embed.FS)
  ├── spec/           Parsers robustos de spec.md / tasks.md
  ├── benchmark/      Benchmarks de performance
  └── plugin/         Sistema de plugins
vscode-extension/      Tree views + status bar + CodeLens para gates
.github/workflows/    CI: lint + test em Go 1.22, 1.23, 1.24
```

## Segurança

- **Agent allowlist** — só `claude`, `codex`, `cursor`, `copilot`, `gemini`
  podem ser invocados como agente, independente do que a spec pedir.
- **Gate allowlist** — tasks.md gates são tokenizados; cada binário precisa
  estar em `{node, npm, pnpm, yarn, bun, go, make, pytest, python, cargo,
  rustc, jest, vitest, tsc, eslint, shellcheck}`. `rm`, `curl`, `wget`,
  etc. são rejeitados.
- **Path sandboxing** — code blocks emitidos pelo LLM são verificados
  contra o diretório do projeto antes de serem escritos.
- **Timeouts** — 10 min por agent run, 5 min por gate run; propagação
  via `context.Context`.

## Desenvolvimento

```bash
go build ./...       # build todos os pacotes
go test ./...        # executar todos os testes
make build           # build binário
make test            # testes com verbose
make lint            # go vet
make release         # binários cross-platform (linux/darwin/windows)
make smoke           # smoke test do CLI
```

CI roda `gofmt`, `go vet`, build, test e smoke em Go 1.22, 1.23 e 1.24
em todo PR. Veja `.github/workflows/ci.yml`.

## Referências

- [OpenAI: Harness Engineering](https://openai.com/index/harness-engineering/)
- [Anthropic: Harness Design](https://www.anthropic.com/engineering/harness-design-long-running-apps)
- [Martin Fowler: Harness Engineering](https://martinfowler.com/articles/harness-engineering.html)
- [Navigation Paradox paper (2026)](https://arxiv.org/html/2602.20048v1)
- [AGENTS.md effectiveness study](https://arxiv.org/pdf/2602.11988)
- [TLC Spec Driven](https://agent-skills.techleads.club/)

## Licença

MIT

---

<div align="center">

**[Fortvna Risk Solutions](https://github.com/Fortvna-Risk-Solutions)** · *Audentes Fortvna Iuvat*

</div>
