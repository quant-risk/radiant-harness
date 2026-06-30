# Spec — corrigir fluxo drop-in para outro agente

Status: closed by v3.7.5 drop-in E2E.

## Goal

Quando um usuário pedir a outro agente para resolver um case usando
`https://github.com/quant-risk/radiant-harness`, esse agente deve conseguir
instalar, configurar MCP, entrar em possess/self-driven quando necessário e
concluir sem erro operacional conhecido.

## Acceptance criteria

- `install.sh` instala a release correta.
- `radiant setup-mcp` e instruções por host estão documentadas.
- O MCP expõe os tools esperados.
- Host sem sampling recebe handoff self-driven utilizável.
- O E2E público simula instalação e execução.

## Non-goals

- Garantir execução sem reinício quando o host exige reload de MCP.
- Garantir hosts não documentados.

## Verification

- `make test-dropin`
- `make test-agents`
- `make audit-install`
- release `v3.7.5`
