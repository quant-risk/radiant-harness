# Sprint 55 — LLM Planning no loop (v2.3.0)

> **Status**: Shipped ✅  
> **Version target**: v2.3.0

Responde à observação crítica 5 do assessment GLM 5.2: o loop pulava Planning
sem chamar LLM — "planning no loop autônomo é básico comparado ao engine SDD."

---

## Design

O planner é **opt-in** (`--plan`). Sem a flag, o loop roda exatamente como
antes. Com ela, o LLM é chamado na fase Plan antes de cada executor:

```
Discover → Plan (LLM: decompõe goal → numbered steps) → Execute (com PLAN: injetado) → Verify → Persist
```

**Fail-open**: se o planner falhar (sem key, timeout, etc.), o loop continua
sem plano. O executor nunca é bloqueado por falha do planner.

**Modelo separado** (`--planner-model`): um modelo mais barato (ex: haiku) pode
planejar enquanto sonnet/opus executa. Zero por padrão → usa o mesmo que `--model`.

---

## O que foi construído

### `internal/loop/runner.go`

**`RunConfig.Plan bool`** — opt-in. Default `false` = comportamento anterior.

**`RunConfig.PlannerModel llm.Model`** — zero value → resolve para ExecutorModel.

**`BuildPlannerPrompt(goal string, iteration int) string`** — exportado para
testes. Na iteração 0, apenas pede decomposição do goal. Nas re-tentativas,
menciona que tentativas anteriores falharam e pede foco nas lacunas.

**`plannerSystemPrompt()`** — instrui o LLM a produzir uma lista numerada de
passos concretos, máximo 10 itens, sem implementar.

**`buildExecutorPrompt` — nova assinatura**: `(goal, groundBlock, planOutput string, priorReviewFindings []string)`.
O `planOutput` é injetado entre GOAL e PRIOR REVIEW FINDINGS como bloco `PLAN:`.

**Fluxo dentro de `Run()` quando `Plan=true`:**

```go
if cfg.Plan && len(lastReviewFindings) == 0 {
    planResp, planErr := planClient.SimpleChat(ctx, plannerSystemPrompt(), planPrompt)
    // fail-open: se planErr != nil, planOutput fica ""
    if planErr == nil { planOutput = planResp }
}
execPrompt := buildExecutorPrompt(goal, groundBlock, planOutput, lastReviewFindings)
```

**Por que `len(lastReviewFindings) == 0`**: quando há findings do reviewer,
o executor já recebe instruções específicas do que corrigir — replanejar seria
ruído, não sinal. O planner roda só em iterações "limpas".

### `cmd/radiant/main.go` (via `cmd_loop.go`)

```bash
# Planner usa o mesmo modelo que o executor
radiant loop start "add rate limiting" --model claude-sonnet-4-6 --plan

# Planner mais barato, executor mais capaz
radiant loop start "refactor scheduler" \
  --model claude-opus-4-8 \
  --planner-model claude-haiku-4-5-20251001 \
  --plan
```

Flags adicionadas em `loopStartCmd` e `loopResumeCmd`:
- `--plan` — habilita LLM planning
- `--planner-model` — modelo separado para o planner

### `internal/loop/sprint55_test.go` — 11 novos testes

- `RunConfig.Plan` default false, assignable
- `RunConfig.PlannerModel` default zero value
- `BuildPlannerPrompt` contém goal, sem "Prior" na iter 0, com "iteration N" nas re-tentativas
- `buildExecutorPrompt` com/sem plan — PLAN: aparece/não aparece, posição após GOAL:
- `Run()` com `Plan=true` falha-open quando planner não tem key
- `Run()` com `Plan=false` continua funcionando

---

## Invariantes

- **Fail-open**: planner error → `planOutput = ""` → executor roda sem plano
- **Plan=false (default)**: comportamento idêntico ao anterior — zero custo extra
- **Verifier nunca planeja**: somente o executor recebe o plano
- **Sem plano em re-iterações com findings**: o reviewer já diz o que corrigir

---

## Comparação com SDD Engine

O SDD Engine (`internal/engine`) tem planning estruturado baseado em spec.md +
tasks.md + ACs. O planner do loop é mais leve: planeja a partir do goal em
linguagem natural. Os dois coexistem — loop é para goals free-form, engine é
para specs formais.

---

## Referências

- GLM 5.2 assessment — ponto 5
- `internal/loop/runner.go` — `BuildPlannerPrompt`, `plannerSystemPrompt`, `buildExecutorPrompt`
- `cmd/radiant/cmd_loop.go` — `--plan`, `--planner-model`
