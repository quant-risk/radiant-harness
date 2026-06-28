# Sprint 65 — Cost tracking em tempo real (v2.13.0)

> **Status**: Shipped ✅

`EstimateCost(model, in, out)` + `FormatCost(usd)` integrados em
`FormatProgress`, `TraceInfo` e `FormatTraceList`. Qualquer `loop status <run-id>`
com `--model` mostra custo acumulado em USD.

## Principais mudanças

- `pricing.go`: `CostPer1KInput` adicionado; 32 modelos com preços de input e output
- `EstimateCost(modelID, tokensIn, tokensOut) (usd, ok)` — retorna 0,false para modelo desconhecido
- `FormatCost(usd)` — `$0.0042` ou `< $0.0001`
- `FormatProgress(runID, modelID, events)` — linha "Cost: $X" quando modelo conhecido
- `TraceInfo.CostUSD` populado via `Meta["model"]` nos eventos
- `FormatTraceList` ganha coluna COST
- 16 novos testes
