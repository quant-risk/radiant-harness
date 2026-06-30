# Spec — fallback self-driven para Codex sem MCP sampling

Status: closed.

## Goal

Quando MCP sampling/createMessage não existir, `radiant_possess` deve guiar o
host agent em modo self-driven em vez de encerrar como falha crítica.

## Acceptance criteria

- Erros JSON-RPC `-32601` de sampling viram handoff self-driven.
- O estado registra modo self-driven e arquivos pendentes.
- A resposta MCP informa claramente os arquivos e o próximo passo do host.
- Há regressão cobrindo o handoff sem sampling.

## Non-goals

- Fazer o harness editar arquivos sozinho fora do host agent.
- Remover suporte para hosts que possuem sampling.

## Verification

- `go test ./cmd/radiant -run TestRadPossessJSONRPCRegression`
- `make test-dropin`
