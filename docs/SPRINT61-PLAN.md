# Sprint 61 — Fleet Plan: decomposição automática de goal em tasks (v2.9.0)

> **Status**: Shipped ✅  
> **Version target**: v2.9.0

Fecha o fluxo end-to-end da fleet: antes do `dispatch`, o usuário agora
tem um passo de planejamento que gera tasks automaticamente.

---

## O que foi construído

### `internal/fleet/planner.go` — `Plan(ctx, goal, client)`

Duas estratégias, com fallback automático:

| Estratégia | Quando | Resultado |
|-----------|--------|-----------|
| LLM | `client != nil` e chamada bem-sucedida | 2–6 tasks parseadas do JSON |
| Heurística | `client == nil` ou LLM falha | 3 tasks: research → implement → verify |

**Interface `PlannerClient`** — desacopla de `*llm.Client` para testabilidade:
```go
type PlannerClient interface {
    Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error)
}
```

**JSON parsing** — strips markdown code fences, skips entradas incompletas,
todos os tasks saem com `Status: TaskPending`.

### `fleet plan <run-id>` — `cmd/radiant/cmd_fleet.go`

Novo subcomando que lê `store.Snapshot().Goal` e chama `fleet.Plan()`:

```bash
# Heurístico (sempre funciona)
radiant fleet plan fleet-1234567890

# LLM-assistido
radiant fleet plan fleet-1234567890 --model claude-sonnet-4-6

# Fluxo completo
radiant fleet start "build rate limiter" --agents 3
radiant fleet plan   fleet-<id> --model claude-sonnet-4-6
radiant fleet dispatch fleet-<id> --model claude-sonnet-4-6 --auto-route
radiant fleet status fleet-<id>
```

Flags: `--model`, `--api-key`.

### `internal/fleet/planner_test.go` — 11 novos testes

- Heurística: 3 tasks, todos pending, IDs sequenciais, goal refletido, title curto, DoneWhen preenchido
- Fallback: LLM com erro → retorna heurística sem propagar erro
- LLM sucesso: parsing correto, todos pending, markdown fences stripped, entradas incompletas ignoradas

---

## Referências

- `internal/fleet/planner.go` — `Plan`, `planHeuristic`, `planWithLLM`, `PlannerClient`
- `cmd/radiant/cmd_fleet.go` — `fleetPlanCmd`
- `internal/fleet/planner_test.go` — 11 testes
