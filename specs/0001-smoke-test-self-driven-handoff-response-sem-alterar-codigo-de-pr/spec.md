# Spec — smoke test do handoff self-driven

Status: closed.

## Goal

Validar a resposta de handoff self-driven sem alterar código de produto.

## Acceptance criteria

- A resposta cita modo self-driven.
- A resposta aponta arquivos gerados e pendências.
- Nenhuma alteração funcional de produto é necessária para o smoke test.

## Non-goals

- Resolver uma tarefa de domínio real.

## Verification

- `go test ./cmd/radiant -run TestRadPossessJSONRPCRegression`
