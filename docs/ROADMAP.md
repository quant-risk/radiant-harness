# Roadmap — Vendor-Neutral SDD Harness

> Este roadmap é vivo e orientado por **princípios**, não por backlog
> de features. Tudo aqui é escrito pensando em **vendor neutrality**
> (nenhum LLM, IDE ou provedor é privilegiado), **agent agnosticism**
> (qualquer agente moderno consome o workflow sem plugin), e
> **cross-platform** (Linux, macOS, Windows; amd64, arm64).

O plano completo (com fases, deliverables e critérios de aceitação)
vive em [`docs/HARNESS-PLAN.md`](./HARNESS-PLAN.md). O schema aberto
de skills vive em [`docs/SKILL-SCHEMA.md`](./SKILL-SCHEMA.md).

---

## Princípios (não negociáveis)

1. **Zero Claude-centrism.** Skills, artifacts e configuração vivem em
   `.radiant-harness/` e `AGENTS.md` (universal). Views nativas
   (`.claude/`, `.cursor/`, etc.) são **opt-in** via `--agent=<list>`.
2. **Vendor-neutral LLM.** Presets são opcionais. Qualquer modelo
   OpenAI-compatible funciona via `--model=...`. Nenhum viés "Claude
   first" no código, na documentação, nem nos defaults.
3. **Cross-platform real.** 6 targets (linux/amd64, linux/arm64,
   darwin/amd64, darwin/arm64, windows/amd64, windows/arm64) buildam
   limpo a cada release.
4. **Sem SDKs pesados.** HTTP puro via `net/http`. Adicionar provedor
   é uma entrada em `PresetModels` + (se preciso) um `baseURL()`.
5. **Skills como contrato machine-readable.** Toda skill tem
   `frontmatter.yaml` com inputs/outputs/gates explícitos. Qualquer
   LLM parseia sem convenção proprietária.
6. **Detecção em runtime.** PATH, $HOME, env vars. Nada de hardcoded
   `/usr/local/bin` ou `~/Library`.

---

## Sprints concluídos

### Sprint 0 — Fundação (commit `cfe074f`, v0.2.0)
- Segurança (allowlists, atomic state, sandboxing, flock, timeouts)
- Vendor-neutral scaffolds
- VS Code extension skeleton
- CI workflow

### Sprint 1 — Roadmap (commit `974d513`)
- `docs/ROADMAP.md` criado

### Sprint 2 — Validação empírica (commit `6a50cdd`, 118 testes)
- 5 bugs reais corrigidos via validação empírica
- 7 provedores LLM (Mistral, Groq, xAI)
- `radiant doctor`, `radiant bench`

### Sprint 3 — Cross-platform + auto-routing (commit `a505b87`, 150 testes)
- Cross-platform lock via atomic rename
- `--auto-route` + `llm.AutoRoute()`
- Pricing table USD

### Sprint 4 — Cost + rate limit + manifests (commit `313a591`, 157 testes)
- Token accounting (mutex, 50-goroutine stress)
- Cost display no `run`
- Rate-limit awareness (`Retry-After`)
- Homebrew/Scoop/AUR manifests

### Sprint 5 — Anthropic nativo (commit `653c51e`, 164 testes)
- `internal/llm/anthropic.go` — POST `/v1/messages`, SSE streaming
- `radiant eval` — comparador de providers

### Sprint 6 — Multi-agent + tracing + CodeLens (commit `7fb5262`, 168 testes)
- `--planner` / `--implementer` flags
- `engine.TraceEvent` + per-phase summary
- VS Code CodeLens em `tasks.md`

### Sprint 7 — Planner fires + JSONL + race fix (commit `f20e94e`, 172 testes)
- `runPlannerAdvisory` — planner LLM agora é chamado de verdade
- `--trace-out` — JSONL export atômico
- Race fix em `Engine.currentTaskID`
- Cross-compile: adicionado `linux/arm64` + `windows/arm64`

### Sprint 8 — Gate output cap (commit `7fb5b54`, 176 testes)
- `--max-gate-output` (10 MiB default)
- `io.LimitReader` em todos os gate runners (3 packages × POSIX + Windows)
- Truncation marker + broken-pipe kill

### Sprint 9 — Allowlist dedup (commit `a9614b7`, 188 testes)
- Novo package `internal/policy/`
- Single source of truth: `AgentCommands`, `GateBinaries`, `ValidateGateCommand`
- Eliminadas 3 cópias paralelas em `engine/`, `harness/`, `quality/`

---

## Sprints planejados

### Sprint 10 — Skill runtime + spec authoring (PRÓXIMO)

**Tema**: Fundação metodológica. Skills universais, spec authoring,
tier system, state machine.

| # | Deliverable | Effort |
|---|-------------|--------|
| 1 | Skill schema v1 documentado (`docs/SKILL-SCHEMA.md`) | S |
| 2 | 3 skills escritas do zero: `nova-feature`, `clarificar`, `validar` | M |
| 3 | Skills bundled no CLI binary; `init` extrai pra `.radiant-harness/skills/` | M |
| 4 | `AGENTS.md` gerado pelo `init` | S |
| 5 | `radiant spec <intent>` — entrevista interativa | M |
| 6 | `--tier` flag + auto-detect (trivial/feature/architecture) | S |
| 7 | `radiant state` + `radiant handoff` | S |
| 8 | Native view generation opt-in via `--agent=<list>` | M |

Ver detalhes completos em [`docs/HARNESS-PLAN.md` §5.1](./HARNESS-PLAN.md).

### Sprint 10 — DONE (2026-06-24)

All 8 deliverables shipped across 3 batches (`f0f4546`, `b98e503`,
this batch). See `docs/validation-report-sprint-10-batch1.md`,
`docs/validation-report-sprint-10-batch2.md`, and the third-batch
report (this commit) for per-deliverable acceptance.

Total: **216 tests, 16 skills bundled, 0 races, 6/6 cross-compile**.

### Sprint 11 — Discovery + Design (Lean + DDD + RFC)

**Tema**: `radiant product`, `radiant adr`, `radiant diagramar`,
`radiant update`, skills `kickoff` + `integrations`.

### Sprint 12 — Brownfield + Governance

**Tema**: `radiant mapear`, `radiant audit`, `radiant metrics`,
skills `mapear` + `audit` + `metricas`.

### Sprint 13 — PR + Multi-agent views

**Tema**: `radiant review-pr`, native views para 6 agentes
(Claude/Cursor/Codex/Copilot/Gemini/Windsurf), `radiant setup-ci`,
`radiant camada-agentica`, `radiant evals`.

---

## Métricas de sucesso

A cada sprint:

- [ ] `go test ./... -race -count=1` 100% verde
- [ ] `go vet ./...` zero warnings
- [ ] `gofmt -l .` zero unformatted files
- [ ] `make release` produz os 6 targets (linux/{amd64,arm64},
      darwin/{amd64,arm64}, windows/{amd64,arm64})
- [ ] Smoke (`init` + `validate`) verde
- [ ] Validation report commitado em
      `docs/validation-report-sprint-N.md`
- [ ] Nenhuma regressão de vendor-neutrality — se uma nova view
      nativa de agente aparecer, ela é gerada a partir de
      `.radiant-harness/skills/`, não duplicada
- [ ] Teste de fogo: `rm -rf .claude .cursor .windsurf .gemini .github/copilot-instructions.md AGENTS.md.specialized` deve deixar
      o projeto ainda funcional via `AGENTS.md` + `.radiant-harness/skills/`

A cada fase:

- [ ] Sprint 10: skill schema ratificado, 3 skills prontas, qualquer
      LLM pode consumir o projeto sem nada proprietário
- [ ] Sprint 11: discovery (greenfield + brownfield) ponta a ponta
- [ ] Sprint 12: governance (audit + metrics) ponta a ponta
- [ ] Sprint 13: 6 views nativas geradas, PR review automatizado

---

## Anti-backlog

Itens **explicitamente fora do roadmap** até segunda ordem:

- ❌ Suporte preferencial a Claude (qualquer feature nova deve
  funcionar igual em qualquer agente que consuma `AGENTS.md`)
- ❌ Claude Code hooks como dependência obrigatória (são opt-in
  via `--agent=claude`)
- ❌ `CLAUDE.md` como arquivo de namespace (o arquivo se chama
  `AGENTS.md`, é universal)
- ❌ Slash commands como única entry point (CLI commands são o
  canônico, slash commands são view opcional)
- ❌ Vendor lock-in de qualquer tipo (open spec, MIT, qualquer
  implementação é bem-vinda)

---

## Mudança de direção registrada (2026-06-24)

Antes deste sprint, o projeto seguia um roadmap focado em
"multi-platform/multi-LLM hardening". Após a comparação com
[spec-driven](https://github.com/igoruehara/spec-driven) (137⭐,
45 forks, comunidade ativa), o rumo muda:

- **Antes**: CLI que executa SDD quando a spec já existe.
- **Agora**: CLI que entrega o workflow SDD completo (Lean
  Inception → DDD → TDD/RFC → SDD → CODE → PR), vendor-neutral,
  agent-agnostic, com skill schema aberto.

Esta mudança preserva todo o trabalho de Sprints 0-9 (engine,
execução, segurança, multi-LLM, tracing) — o roadmap novo
**acrescenta** a camada metodológica sobre essa base.

O plano detalhado (4 fases, 4 sprints, 24 deliverables) vive em
[`docs/HARNESS-PLAN.md`](./HARNESS-PLAN.md).