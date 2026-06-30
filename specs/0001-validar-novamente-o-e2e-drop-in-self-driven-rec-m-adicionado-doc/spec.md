# Spec — revalidar E2E drop-in self-driven

Status: closed.

## Goal

Revalidar o E2E drop-in self-driven, documentar como rodar o teste e commitar
a documentação final.

## Acceptance criteria

- `make test-dropin` passa.
- `go test ./...` passa.
- `radiant mcp self-test` passa.
- A documentação referencia o teste.
- A árvore de trabalho fica limpa após commit.

## Non-goals

- Adicionar novo comportamento funcional além da validação/documentação.

## Verification

- `make test-dropin`
- `go test ./...`
- `radiant mcp self-test`
