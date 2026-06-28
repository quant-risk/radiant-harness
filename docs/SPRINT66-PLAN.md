# Sprint 66 — fleet watch (v2.14.0)

> **Status**: Shipped ✅

`radiant fleet watch <run-id> [--interval N]` faz polling do store a cada N
segundos (default 10), limpa tela e re-imprime `FormatStatus` até todas as
tasks atingirem estado terminal (done/failed). 8 novos testes.

## Uso

```bash
radiant fleet watch fleet-1234567890
radiant fleet watch fleet-1234567890 --interval 5
```
