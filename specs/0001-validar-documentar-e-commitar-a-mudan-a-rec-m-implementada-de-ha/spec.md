# Spec — validar, documentar e commitar handoff self-driven

Status: closed.

## Goal

Validar, documentar e commitar a mudança de handoff self-driven no
`radiant_possess` para hosts sem MCP sampling.

## Acceptance criteria

- Testes relevantes passam.
- README/AGENTS/INSTALL descrevem o comportamento operacional.
- O commit contém apenas mudanças relacionadas ao harness.

## Non-goals

- Resolver casos externos do usuário.

## Verification

- `go test ./cmd/radiant`
- `make test-dropin`
- commits `e6950c3`, `62096da`, `501f272`, `f0f8186`, `f706801`
