# Sprint 60 — Fleet Dispatch com AutoRoute (v2.8.0)

> **Status**: Shipped ✅  
> **Version target**: v2.8.0

Fecha a ponte entre AutoRoute (Sprint 59) e a fleet real de processos (Sprint 56):
o `fleet dispatch` agora spawna agentes reais e propaga `--model` e `--auto-route`
para cada subprocesso.

---

## O que foi construído

### `fleet dispatch <run-id>` — `cmd/radiant/cmd_fleet.go`

Novo subcomando que chama `Dispatcher.RunAll()` com extraArgs construídos
a partir das flags da CLI:

```
radiant fleet dispatch fleet-1234567890
radiant fleet dispatch fleet-1234567890 --model claude-sonnet-4-6 --auto-route
radiant fleet dispatch fleet-1234567890 --timeout 30
```

**Flags:**

| Flag | Default | Descrição |
|------|---------|-----------|
| `--model` | `""` | Modelo forwarded para cada `loop start` |
| `--auto-route` | `false` | Habilita AutoRoute em cada agente |
| `--timeout` | `0` | Timeout por agente em minutos |

**Fluxo completo:**

```
radiant fleet start "goal" --agents 3   # cria store + coordinator
radiant fleet dispatch <run-id>          # spawna processos reais
radiant fleet status <run-id>            # acompanha progresso
```

### `internal/fleet/sprint60_test.go` — 4 novos testes

Cada teste usa `captureArgsBinary` — script de shell que printa `$@` — para
verificar que os extraArgs chegam verbatim nos subprocessos:

- `TestDispatchExtraArgs_ModelForwarded`: `--model claude-sonnet-4-6 --auto-route` → presentes na saída
- `TestDispatchExtraArgs_NoExtraArgs`: nil extraArgs → sem erro
- `TestDispatchExtraArgs_AutoRouteFlag`: `--auto-route` → presente na saída
- `TestDispatchMultiTask_ExtraArgsOnAll`: 2 tarefas → `--model` aparece ≥2x

`captureWriter` usa `sync.Mutex` para ser safe com `-race` (múltiplos
goroutines escrevem em paralelo via `RunAll`).

---

## Referências

- `cmd/radiant/cmd_fleet.go` — `fleetDispatchCmd`
- `internal/fleet/dispatch.go` — `Dispatcher.RunAll`, `spawnAgent`
- `internal/fleet/sprint60_test.go` — testes de forwarding de extraArgs
