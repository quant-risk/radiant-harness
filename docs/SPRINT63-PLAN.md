# Sprint 63 — loop status com trace progress (v2.11.0)

> **Status**: Shipped ✅  
> **Version target**: v2.11.0

Adiciona observabilidade de progresso em tempo real ao loop runner:
`radiant loop status <run-id>` lê o JSONL trace e mostra iteração atual,
fase, tokens totais e última ação — sem precisar do processo vivo.

---

## O que foi construído

### `TracePath(projectDir, runID)` — `internal/loop/trace.go`

Helper que retorna o caminho esperado do JSONL trace para um dado run-id:
`.radiant-harness/traces/<runID>.jsonl`

### `FormatProgress(runID, events)` — `internal/loop/trace.go`

Renderiza um resumo compacto derivado do event stream:

```
Run:      my-run-2026-06-27
Elapsed:  4m32s  (14:01:00 → 14:05:32)
Iteration: 3
Phase:     execute
Tokens:    1430 total (980 in / 450 out)
Events:    12

Last action: write auth middleware → ✓ ok
Evidence:    all tests pass, coverage 91%
```

Derivação puramente do stream de eventos — não precisa do Cycle vivo.

### `loop status [run-id]` extendido — `cmd/radiant/cmd_loop.go`

Sem run-id: comportamento anterior (estado do cycle ativo).  
Com run-id: lê o trace e chama `FormatProgress`.

```bash
radiant loop status                        # estado ativo
radiant loop status my-run-2026-06-27      # trace do run específico
```

### `internal/loop/sprint63_test.go` — 13 novos testes

- `TracePath`: formato e unicidade
- `FormatProgress`: events vazios, run ID, phase, last action, tokens acumulados,
  zero tokens, contagem de iterações, evidence, evidence longa truncada, elapsed
- Round-trip: write trace → ReadTrace → FormatProgress

---

## Referências

- `internal/loop/trace.go` — `TracePath`, `FormatProgress`
- `cmd/radiant/cmd_loop.go` — `loop status [run-id]`
- `internal/loop/sprint63_test.go` — 13 testes
