# docs/README.md

> Templated by `radiant-harness` self-driven mode on 2026-06-30.

## What this project does

```
Remover a spec MenuFlex do repositório porque pertence a outro contexto do usuário, não ao backlog do radiant-harness. Em seguida avaliar o restante das pendências reais: specs placeholder, docs de produto/roadmap/glossário e próximos passos para consolidar o harness.
```


## Layout produced by the harness

| Path | Origin | Status |
|---|---|---|
| `AGENTS.md` | generated (radiant-harness v3.6.0+) | templated |
| `docs/README.md` | generated this file | templated |
| `docs/CONTEXT.md` | generated (moved to .radiant-harness/CONTEXT.md in self-driven mode) | templated |
| `specs/0001-remover-a-spec-menuflex-do-reposit-rio-porque-pertence-a-outro-c/spec.md` | templated | templated |
| `specs/0001-remover-a-spec-menuflex-do-reposit-rio-porque-pertence-a-outro-c/tasks.md` | templated | templated |
| `scripts/run.sh` | templated entrypoint | templated |

## Next step

The host agent should read every templated file, replace each
task-specific marker with the real content, and then run the entrypoint
`./scripts/run.sh` to validate end-to-end.

---
This task removes the unrelated MenuFlex case and leaves broader radiant-harness backlog cleanup for a separate pass.
