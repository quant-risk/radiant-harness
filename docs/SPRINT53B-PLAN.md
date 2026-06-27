# Sprint 53B — main.go Split + Token Estimation (v2.1.0)

> **Status**: Shipped ✅  
> **Version target**: v2.1.0

Responde às observações críticas 1 e 4 do assessment GLM 5.2.

---

## Problema 1: `cmd/radiant/main.go` monolítico (7.117 linhas)

### O que foi feito

O corpo de `main()` foi extraído em 10 funções `registerXxxCmds(root *cobra.Command)`,
cada uma em seu próprio arquivo dentro de `cmd/radiant/`:

| Arquivo            | Conteúdo                                               | Linhas |
|--------------------|--------------------------------------------------------|--------|
| `cmd_run.go`       | init, validate, run, bench, doctor, config, models, eval | 472  |
| `cmd_spec.go`      | spec, adr, diagramar, product, integrations, views, review-pr, setup-ci | 392 |
| `cmd_audit.go`     | camada-agentica, evals, release, audit, mcp, security  | 159    |
| `cmd_telemetry.go` | telemetry, stats, causal, model, predict, train, evaluate, drift, autodata, validate-file, profile, incident | 299 |
| `cmd_ops.go`       | update                                                 | 110    |
| `cmd_session.go`   | state, handoff                                         | 90     |
| `cmd_skills.go`    | skills, boot                                           | 134    |
| `cmd_context.go`   | context + subcomandos, ontology + subcomandos          | 218    |
| `cmd_fleet.go`     | worktree, fleet, improve, budget                       | 353    |
| `cmd_loop.go`      | loop (start/status/resume/schedule/review), trace      | 449    |

### `main()` resultante — 26 linhas de corpo

```go
func main() {
    root := &cobra.Command{ ... }
    registerRunCmds(root)
    registerSpecCmds(root)
    // ... 8 mais ...
    registerLoopCmds(root)
    root.SetVersionTemplate("{{.Version}}\n")
    if err := root.Execute(); err != nil { os.Exit(1) }
}
```

**Nota**: `main.go` ainda contém ~4500 linhas de funções helpers compartilhadas
entre múltiplos arquivos do package. Mover para `helpers.go` é Sprint 54.

---

## Problema 4: `estimateTokens` usa `len/4` (subestima CJK, português)

```go
// antes
return (len(prompt) + len(response)) / 4

// depois
runes := utf8.RuneCountInString(prompt) + utf8.RuneCountInString(response)
return (runes*10 + 34) / 35  // ≈ runes / 3.5, integer-only
```

- `len()` conta bytes; `RuneCountInString()` conta code points (o que tokenizadores usam)
- 3.5 chars/token > conservador que 4.0; margem extra para budget checks
- `len("ção") == 5` bytes vs `RuneCountInString("ção") == 3` runes — correto

---

## Referências

- GLM 5.2 assessment — pontos 1 e 4
- `cmd/radiant/cmd_*.go` — 10 novos arquivos de registro de comandos
- `internal/loop/runner.go:estimateTokens` — unicode/utf8
