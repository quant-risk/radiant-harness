# spec.md — 0001-implementar-fallback-self-driven-para-codex-quando-mcp-sampling

> Templated by `radiant-harness` self-driven mode (v3.6.0+) on 2026-06-30.
> Replace the [host-agent: fill in] markers with real acceptance criteria.

## Goal

```
Implementar fallback self-driven para Codex quando MCP sampling/createMessage não for suportado. Requisitos: detectar erro JSON-RPC -32601/method sampling.createMessage ou sampling/createMessage; não encerrar como critical_failure nesses casos; registrar estado/trace como self-driven/awaiting_host_action; permitir que o fluxo seja retomável via resume/status; preservar compatibilidade com hosts que suportam sampling; adicionar/ajustar verificação mínima para provar que o erro vira modo self-driven em vez de falha crítica.
```


## Acceptance criteria

- AC1: [host-agent: fill in — task_id=43e8c85f6936619e phase=plan] (high-level — refine below)
- AC2: [host-agent: fill in — task_id=43e8c85f6936619e phase=plan] (high-level — refine below)
- AC3: [host-agent: fill in — task_id=43e8c85f6936619e phase=plan] (high-level — refine below)

## Non-goals

- [host-agent: fill in — task_id=43e8c85f6936619e phase=plan] (sketch; expand)

## Profile

- thorough

---
[host-agent: fill in — task_id=43e8c85f6936619e phase=plan]
