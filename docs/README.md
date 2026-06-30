# docs/README.md

> Templated by `radiant-harness` self-driven mode on 2026-06-30.

## What this project does

```
Validar e corrigir o fluxo drop-in para outro agente: quando o usuário disser 'Resolva esse case usando https://github.com/quant-risk/radiant-harness', o agente deve conseguir instalar/configurar o harness, expor MCP, entrar em possess/self-driven quando sampling não existir e concluir sem erro operacional. Se houver lacunas em README/INSTALL/install.sh/MCP setup/testes, corrigir, validar, commitar e subir.
```


## Layout produced by the harness

| Path | Origin | Status |
|---|---|---|
| `AGENTS.md` | generated (radiant-harness v3.6.0+) | templated |
| `docs/README.md` | generated this file | templated |
| `docs/CONTEXT.md` | generated (moved to .radiant-harness/CONTEXT.md in self-driven mode) | templated |
| `specs/0001-validar-e-corrigir-o-fluxo-drop-in-para-outro-agente-quando-o-us/spec.md` | templated | templated |
| `specs/0001-validar-e-corrigir-o-fluxo-drop-in-para-outro-agente-quando-o-us/tasks.md` | templated | templated |
| `scripts/run.sh` | templated entrypoint | templated |

## Next step

The host agent should read every templated file, replace each
`[host-agent: fill in — task_id=1a433c01a123b633 phase=docs]` marker with the real content, and then run the entrypoint
`./scripts/run.sh` to validate end-to-end.

---
[host-agent: fill in — task_id=1a433c01a123b633 phase=execute]
