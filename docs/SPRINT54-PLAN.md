# Sprint 54 — helpers.go: extração das funções auxiliares (v2.2.0)

> **Status**: Shipped ✅  
> **Version target**: v2.2.0

Continua o refactor do Sprint 53B — finaliza a separação do `main.go` monolítico.

---

## O que foi feito

As 99 funções helpers que ficaram em `main.go` após o Sprint 53B foram movidas
para `cmd/radiant/helpers.go`. O `main.go` ficou com 36 linhas.

### Estrutura resultante de `cmd/radiant/`

| Arquivo          | Responsabilidade                        | Linhas |
|------------------|-----------------------------------------|--------|
| `main.go`        | Entry point: root + register calls      | 36     |
| `helpers.go`     | 99 funções auxiliares compartilhadas    | 4562   |
| `cmd_run.go`     | init, validate, run, bench, doctor, config, models, eval | 472 |
| `cmd_spec.go`    | spec, adr, diagramar, product, integrations, views, review-pr, setup-ci | 392 |
| `cmd_audit.go`   | camada-agentica, evals, release, audit, mcp, security | 159 |
| `cmd_telemetry.go` | telemetry, stats, causal, model, predict, train, evaluate, drift, autodata, validate-file, profile, incident | 299 |
| `cmd_ops.go`     | update                                  | 110    |
| `cmd_session.go` | state, handoff                          | 90     |
| `cmd_skills.go`  | skills, boot                            | 134    |
| `cmd_context.go` | context + subcomandos, ontology + subcomandos | 218 |
| `cmd_fleet.go`   | worktree, fleet, improve, budget        | 353    |
| `cmd_loop.go`    | loop (start/status/resume/schedule/review), trace | 449 |

### `main.go` final

```go
package main

import "github.com/spf13/cobra"

var version = "1.1.0"

func main() {
    root := &cobra.Command{
        Use:     "radiant",
        Short:   "Universal SDD harness for any AI model or agent",
        Version: version,
    }
    registerRunCmds(root)
    registerSpecCmds(root)
    registerAuditCmds(root)
    registerTelemetryCmds(root)
    registerOpsCmds(root)
    registerSessionCmds(root)
    registerSkillsCmds(root)
    registerContextCmds(root)
    registerFleetCmds(root)
    registerLoopCmds(root)
    root.SetVersionTemplate("{{.Version}}\n")
    if err := root.Execute(); err != nil { os.Exit(1) }
}
```

---

## Invariantes

- Todos os arquivos são `package main` — zero mudança de visibilidade
- `go test ./... -count=1` → 19/19 green, 651 testes
- `goimports` gerencia todos os imports — nenhum import manual
- Ponto 1 do assessment GLM 5.2 completamente resolvido

---

## Próximo passo natural

`helpers.go` ainda tem 4562 linhas. Um Sprint futuro pode agrupá-las por
domínio (ex: `helpers_release.go`, `helpers_telemetry.go`, `helpers_audit.go`)
seguindo o mesmo padrão dos `cmd_*.go`. O ganho marginal é menor — `helpers.go`
é um arquivo de suporte, não um entry point, e qualquer editor de código
navega funções por símbolo, não por linha.
