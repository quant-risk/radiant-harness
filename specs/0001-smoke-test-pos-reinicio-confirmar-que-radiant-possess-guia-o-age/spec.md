# Spec — smoke test pós-reinício do radiant_possess self-driven

Status: closed.

## Goal

Confirmar após reinício que `radiant_possess` guia o agente em modo
self-driven quando sampling não existe.

## Acceptance criteria

- MCP server inicia e lista os tools esperados.
- `radiant_possess` retorna handoff self-driven em host sem sampling.
- O host consegue preencher os artefatos e executar a verificação.

## Non-goals

- Testar todos os hosts documentados em um único smoke test.

## Verification

- `radiant mcp self-test`
- `make test-dropin`
