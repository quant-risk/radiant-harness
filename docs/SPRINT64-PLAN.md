# Sprint 64 — loop list + trace list rica (v2.12.0)

> **Status**: Shipped ✅  
> **Version target**: v2.12.0

Adiciona listagem rica de runs com event count, fase, resultado e timestamp —
tanto via `radiant loop list` (novo) quanto via `radiant trace list` (melhorado).

---

## O que foi construído

### `TraceInfo` + `ListTraceInfos` — `internal/loop/trace.go`

```go
type TraceInfo struct {
    RunID      string
    EventCount int
    LastPhase  Phase
    LastResult string
    LastAction string
    UpdatedAt  time.Time
}
func ListTraceInfos(projectDir string) ([]TraceInfo, error)
```

Lê o último evento de cada trace para preencher o resumo. Ordena newest-first
por `UpdatedAt` (insertion sort; zero-times ficam por último).

### `FormatTraceList(infos)` — `internal/loop/trace.go`

Tabela com colunas RUN-ID / EVENTS / PHASE / RESULT / UPDATED:

```
RUN-ID                                EVENTS  PHASE       RESULT    UPDATED
--------------------------------------------------------------------------------
my-run-2026-06-27-001                      8  execute     ok        2026-06-27 14:30
my-run-2026-06-26-002                      3  plan        failed    2026-06-26 09:12
```

### `loop list` — `cmd/radiant/cmd_loop.go`

Novo subcomando com flag `--plain` (IDs apenas, um por linha).

### `trace list` melhorado — `cmd/radiant/cmd_loop.go`

Usa `FormatTraceList` por padrão; `--plain` preserva comportamento anterior.

### `internal/loop/sprint64_test.go` — 11 novos testes

`ListTraceInfos`: dir vazio, single trace, newest-first, múltiplos eventos (último usado).  
`FormatTraceList`: empty, run ID, event count, phase+result, truncamento, timestamp, header.

---

## Referências

- `internal/loop/trace.go` — `TraceInfo`, `ListTraceInfos`, `FormatTraceList`
- `cmd/radiant/cmd_loop.go` — `loop list`, `trace list` melhorado
- `internal/loop/sprint64_test.go` — 11 testes
