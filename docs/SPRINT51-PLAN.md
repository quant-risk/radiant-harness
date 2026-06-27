# Sprint 51 — Context Injection: CONTEXT.md no executor (v1.9.0)

> **Status**: Shipped ✅  
> **Version target**: v1.9.0

---

## Background

`loop.Run()` chamava o executor sem nenhum conhecimento do codebase. O package
`internal/context` já sabia detectar o projeto e montar um `CONTEXT.md` — mas
nunca era chamado dentro do loop. Sprint 51 conecta os dois: quando
`--context-budget N` é passado, o executor recebe o contexto do projeto no system
prompt de cada iteração.

---

## O que foi construído

### `internal/loop/runner.go`

**`RunConfig.ContextBudgetTokens int`** — novo campo.
- `0` (default) → context assembly desabilitado; loop roda exatamente como antes.
- `> 0` → `assembleContextBlock()` é chamado uma vez no início do run.

**`assembleContextBlock(projectDir string, contextBudgetTokens int) string`**

Detecta o projeto, monta `CONTEXT.md` e retorna o conteúdo prefixado com
`## PROJECT CONTEXT\n\n`. Fail-open: qualquer erro retorna `""`, nunca
bloqueia o loop.

```go
func assembleContextBlock(projectDir string, contextBudgetTokens int) string {
    det, err := radctx.Detect(projectDir)    // domain, tier, skills
    path, _, err := radctx.Assemble(...)     // writes .radiant-harness/CONTEXT.md
    data, err := os.ReadFile(path)
    return "## PROJECT CONTEXT\n\n" + strings.TrimSpace(string(data))
}
```

**`executorSystemPrompt(contextBlock string) string`** — assinatura estendida.
- `""` → retorna o base prompt sem modificação.
- `non-""` → appends `"\n\n" + contextBlock` ao base prompt.

O context block aparece *depois* do base prompt, nunca antes.

**Fluxo dentro de `Run()`:**

```
startup:  projectCtxBlock = assembleContextBlock(projectDir, cfg.ContextBudgetTokens)
per-iter: execClient.SimpleChat(ctx, executorSystemPrompt(projectCtxBlock), execPrompt)
```

Montado uma vez, injetado em todas as iterações — sem I/O por iteração.

### `cmd/radiant/main.go`

**`--context-budget <n>`** — novo flag em `loopStartCmd`.

```bash
radiant loop start "add rate limiting to the API" \
  --model claude-sonnet-4-6 \
  --context-budget 6000
```

Mapeia para `RunConfig.ContextBudgetTokens`.

### `internal/loop/sprint51_test.go` — 11 novos testes

- `assembleContextBlock` — dir inexistente, dir vazio, prefixo correto
- `executorSystemPrompt` — sem context, com context, base preservado, context após base
- `RunConfig.ContextBudgetTokens` — default zero, assignable
- `Run()` integração — CONTEXT.md não criado quando budget=0; criado quando budget=4000

### Correção: `sprint47_test.go`

`executorSystemPrompt()` → `executorSystemPrompt("")` — assinatura mudou, 1 linha.

---

## Invariantes

- **Fail-open**: `assembleContextBlock` retorna `""` silenciosamente em qualquer erro
  (dir inexistente, Detect falha, OS error) — o loop continua sem contexto.
- **Uma montagem por run**: `Detect + Assemble` são caros (I/O); executados uma vez
  no startup, não por iteração.
- **Zero = desabilitado**: sem `--context-budget`, o loop não toca `internal/context`.

---

## Referências

- `internal/context/detector.go` — `Detect(projectDir)`
- `internal/context/assembler.go` — `Assemble(projectDir, det, opts)`
- `internal/loop/runner.go` — `assembleContextBlock`, `executorSystemPrompt(string)`
