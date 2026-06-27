# Sprint 56 — Fleet Dispatcher: processos reais por worktree (v2.4.0)

> **Status**: Shipped ✅  
> **Version target**: v2.4.0

Responde à observação crítica 3 do assessment GLM 5.2: o Fleet Coordinator
não spawna processos reais — era scaffolding de estado.

---

## O que foi construído

### `internal/fleet/dispatch.go` — nova camada de execução real

**`Dispatcher`** — spawna um processo OS por tarefa, cada um em seu próprio
worktree git isolado. Paralelo via `sync.WaitGroup`.

```go
d, _ := fleet.NewDispatcher(iso, fleet.DispatchConfig{
    Binary:  "/path/to/radiant",
    Timeout: 30 * time.Minute,
})
results, _ := d.RunAll(ctx, []string{"--model", "claude-sonnet-4-6"})
```

**`DispatchConfig`**:
- `Binary` — path do executável (default: `os.Executable()`)
- `Env` — environment para os processos filhos (default: herda do pai)
- `Stdout / Stderr` — saída agregada de todos os agentes
- `Timeout` — limite por processo (0 = sem timeout)

**`RunAll(ctx, extraArgs)`**:
1. Claim de todas as tarefas pendentes via `Isolator.ClaimIsolated`
2. Uma goroutine por tarefa → `exec.CommandContext` com:
   - `cmd.Dir = worktree.Path` — processo isolado por checkout
   - `RADIANT_WORKTREE_DIR`, `RADIANT_AGENT_ID`, `RADIANT_TASK_ID` no env
   - Args: `<binary> loop start <task.DoneWhen> [extraArgs...]`
3. Ao terminar: `store.CompleteTask(success/failed)` + `iso.Release(wt)`
4. Retorna `[]AgentResult` com ExitCode, Err, Elapsed por agente

**`spawnAgent`** — lida com `exec.ExitError` separadamente de erros de dispatch
(ex: binary not found). Exit não-zero de um agente não interrompe os outros.

### `internal/fleet/coordinator.go`

Removido o comentário `// does NOT spawn real processes`. O Coordinator agora
é descrito como gerenciador de estado, complementado pelo Dispatcher.

### `internal/fleet/dispatch_test.go` — 8 novos testes

- `DispatchConfig` defaults
- `AgentResult` zero value
- `NewDispatcher` resolve executable
- `RunAll` sem tarefas → [] vazio
- `RunAll` com processo que exit 0 → TaskDone no store
- `RunAll` com processo que exit 1 → TaskFailed no store
- `RunAll` com context cancelado → processo morto, result retornado
- Branch cleanup via `t.Cleanup` → zero branches órfãs após cada teste

---

## Arquitetura Fleet completa

```
Store          — estado compartilhado entre agentes (JSON atômico)
Coordinator    — gerencia AgentRecords, prompts por role, status table
Isolator       — ClaimIsolated: worktree git real por tarefa
Dispatcher  ★  — RunAll: spawna processo OS por tarefa em seu worktree
```

O fluxo de ponta a ponta para fleet multi-agente:

```bash
# 1. Inicializar store com tarefas
store.SetTasks(tasks)

# 2. Criar dispatcher
iso := fleet.NewIsolator(store, repoDir)
d := fleet.NewDispatcher(iso, fleet.DispatchConfig{Binary: "radiant"})

# 3. Spawnar todos — paralelo, isolado, com timeout
results := d.RunAll(ctx, []string{"--model", "claude-sonnet-4-6", "--plan"})
```

---

## Referências

- GLM 5.2 assessment — ponto 3
- `internal/fleet/dispatch.go` — Dispatcher, DispatchConfig, AgentResult
- `internal/fleet/isolation.go` — Isolator (worktrees reais)
- `internal/fleet/coordinator.go` — estado (complementa Dispatcher)
