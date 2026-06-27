# Sprint 50 — Trace Integration: LLM calls gravados em JSONL (v1.8.0)

> **Status**: Shipped ✅  
> **Version target**: v1.8.0

---

## Background

O `Tracer` existia desde sprints anteriores e o `radiant trace show` já funcionava,
mas `loop.Run()` nunca gravava eventos nele. Cada chamada LLM passava sem registro.
Sprint 50 fecha esse gap: toda chamada ao LLM dentro do loop agora gera um
`TraceEvent` em `.radiant-harness/traces/<runID>.jsonl`.

---

## O que foi construído

### `internal/loop/runner.go`

**`traceCall(tr, runID, phase, agent, model, prompt, response, tokens, err)`**

Helper nil-safe que grava um `TraceEvent` após cada `SimpleChat`:
- `PromptHash`: `sha256(prompt)[0:4]` em hex — 8 chars, identifica prompts únicos sem expor conteúdo
- `TokensIn` / `TokensOut`: `tokens/2` cada (estimativa simétrica quando provider não retorna contagem)
- `Result`: `"ok"` | `"failed"` com evidência de erro quando falha
- `Meta["model"]`: nome do modelo usado nessa chamada
- Nil-safe: `tr == nil` → retorna sem fazer nada (stall brake, dry-run, etc.)

**Tracer auto-criado em `Run()`**

```go
tr := cfg.Trace
if tr == nil {
    tr, err = NewTracer(projectDir, runID)
    defer tr.Close()
}
```

- `cfg.Trace` non-nil → reutiliza tracer externo (compartilhamento com caller)
- nil → cria automaticamente; `defer tr.Close()` garante flush no exit

**Eventos gravados por iteração:**

| Chamada | Agent | Phase |
|---------|-------|-------|
| `execClient.SimpleChat()` | `executor` | `execute` |
| `verClient.SimpleChat()` (verify) | `verifier` | `verify` |
| `verClient.SimpleChat()` (review) | `reviewer` | `verify` |

**`RunConfig.Trace *Tracer`** — novo campo opcional.

### `internal/loop/sprint50_test.go` — 10 novos testes

- `TestTraceCallNilTracer` / `TestTraceCallNilTracerWithError` — nil-safety
- `TestTraceCallRecordsOkEvent` — campos `result`, `agent`, `phase`, `meta`, `prompt_hash`, tokens
- `TestTraceCallRecordsFailedEvent` — `result=failed`, `evidence=<error message>`
- `TestTraceCallMultipleEvents` — 3 eventos em ordem, agentes corretos
- `TestTraceCallHashDiffersForDifferentPrompts` — hashes únicos por prompt
- `TestRunConfigTraceFieldDefault` / `TestRunConfigTraceFieldAssignable`
- `TestRunCreatesTraceFile` — arquivo criado em disco mesmo quando LLM falha
- `TestTraceCallTimestamp` — timestamp dentro da janela `[before, after]`

---

## Resultado no disco

Após `radiant loop start "goal" --model claude-sonnet-4-6`:

```
.radiant-harness/traces/run-1751234567.jsonl
```

Cada linha é um JSON:
```json
{
  "ts": "2026-06-27T14:05:12Z",
  "run": "run-1751234567",
  "phase": "execute",
  "action": "llm_call",
  "agent": "executor",
  "prompt_hash": "a3f8c2d1",
  "tokens_in": 320,
  "tokens_out": 320,
  "result": "ok",
  "meta": {"model": "claude-sonnet-4-6"}
}
```

Inspeccionável com: `radiant trace show run-1751234567`

---

## Referências

- `internal/loop/runner.go` — `traceCall()`, `RunConfig.Trace`, tracer auto-criação
- `internal/loop/trace.go` — `Tracer`, `TraceEvent`, `NewTracer`, `Record`
