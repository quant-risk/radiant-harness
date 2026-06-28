# Sprint 62 — Fleet Status melhorado + fleet summary (v2.10.0)

> **Status**: Shipped ✅  
> **Version target**: v2.10.0

Melhora a observabilidade da fleet com contadores por status, preview de
evidências e worktree, e um novo subcomando `fleet summary` que consolida
resultados de tasks concluídas.

---

## O que foi construído

### `FormatStatus` melhorado — `internal/fleet/coordinator.go`

| Antes | Depois |
|-------|--------|
| Apenas tabela de tasks | + linha de contadores: N total, N pending, N assigned, N done, N failed |
| Coluna Agent | + coluna Worktree/Evidence (worktree para assigned; preview de 40 chars de evidence para done) |
| Sem hint quando vazio | Hint "run `radiant fleet plan <run-id>` first" quando tasks = 0 |

### `FormatSummary` — `internal/fleet/coordinator.go`

Nova função que agrega evidências das tasks concluídas:

```
Summary — Fleet: fleet-1234567890
Goal: build rate limiter

2/3 tasks completed

── task-01: Research
   A clear implementation plan exists...
   worktree: /tmp/worktrees/task-01

── task-02: Implement
   All 42 tests pass and coverage is 88%

1 task(s) failed:
  ✗ task-03: Verify
```

### `fleet summary <run-id>` — `cmd/radiant/cmd_fleet.go`

Novo subcomando que chama `FormatSummary(coord.Status())`.

### `internal/fleet/sprint62_test.go` — 9 novos testes

FormatStatus: contadores, hint sem tasks, preview de evidence, worktree dir.
FormatSummary: sem tasks done, contagem N/total, evidence, failed, goal.

---

## Fluxo completo da fleet

```bash
radiant fleet start "build rate limiter" --agents 3
radiant fleet plan     fleet-<id> --model claude-sonnet-4-6
radiant fleet dispatch fleet-<id> --model claude-sonnet-4-6 --auto-route
radiant fleet status   fleet-<id>   # progresso em tempo real
radiant fleet summary  fleet-<id>   # consolidação final
```
