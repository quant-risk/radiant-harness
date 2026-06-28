# Sprint 67 — Integração MCP: loop tools (v2.15.0)

> **Status**: Shipped ✅

Três novas tools expostas pelo servidor MCP (`radiant mcp serve`):

| Tool | Descrição |
|------|-----------|
| `radiant_loop_start` | Inicia loop autônomo com goal, model, max_iter, auto_route |
| `radiant_loop_status` | Progresso de um run pelo trace (run_id opcional) |
| `radiant_loop_list` | Lista todos os runs com event count e custo |

Qualquer agente MCP (Claude, Cursor, Copilot…) pode agora invocar o loop
diretamente sem sair do fluxo conversacional. 8 novos testes em sprint67_mcp_test.go.
