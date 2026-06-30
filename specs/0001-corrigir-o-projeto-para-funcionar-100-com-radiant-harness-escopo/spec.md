# Spec — corrigir o projeto para funcionar com radiant-harness

Status: closed by v3.7.5 validation and follow-up cleanup on 2026-06-30.

## Goal

Corrigir o projeto para funcionar com radiant-harness, incluindo MCP/Codex,
possess/self-driven, ontologia utilizável, preservação de mudanças do usuário
e evidências objetivas de verificação.

## Acceptance criteria

- MCP setup, `radiant doctor`, `radiant mcp self-test`, and drop-in
  installation path are documented and testable.
- `radiant_possess` has a self-driven fallback for hosts without sampling.
- The ontology and project docs identify where context, skills, specs, and
  architecture live.
- Verification evidence exists in tests and release artifacts.

## Non-goals

- Guarantee every possible third-party host runtime without host-specific setup.
- Preserve unrelated user cases in this repository.

## Verification

- `make test-dropin`
- `go test ./cmd/radiant ./internal/...`
- `radiant mcp self-test`
