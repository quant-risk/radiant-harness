# Light/Full — Operator Guide (v2.42.0)

> **TL;DR:** Comportamento emerge do **subcomando**, não de flag.
> - `radiant mcp serve` → **Light** (MCP sampling, sem API key)
> - Qualquer outro subcomando (`loop start`, `run`, `fleet start`, `init`, ...) → **Full** (HTTP direto, API key necessária)

Sem `--mode` flag. Sem `RADIANT_MODE` env. Sem `mode:` no `.radiant.yaml`. Sem `radiant mode show/set`.

---

## O que é Light

**Light** = "o harness possui o agente". Você roda o harness como MCP server, e quando ele precisa de inference ele pede via `sampling/createMessage` ao host agent (Claude Code, Hermes, Cursor, ...). **O host paga pelos tokens** — você não precisa de API key.

**Quando usar:** você já tem Claude Code (ou outro host MCP) rodando. Quer que o harness te ajude, mas sem configurar billing.

**Como usar:**

```bash
# 1. Registra o harness como MCP server no Claude Code
claude mcp add radiant -- /usr/local/bin/radiant mcp serve

# 2. Pronto. A partir daí o Claude Code spawna `radiant mcp serve`
# automaticamente. Toda inference vem do Claude Code via sampling.
```

**O que você NÃO pode fazer em Light:**
- Rodar `radiant loop start`, `radiant run`, `radiant fleet start` direto do terminal. Essas subcomandos são **Full** — sempre.

---

## O que é Full

**Full** = "o harness é autônomo". O harness chama LLM HTTP endpoints diretamente (OpenRouter, OpenAI, Anthropic, Groq, Mistral, xAI). **Você paga pelos tokens** — API key necessária.

**Quando usar:** CI/CD, scripts, automação, ou quando você quer que o harness rode standalone (sem Claude Code).

**Como usar:**

```bash
# Configure a API key
export OPENROUTER_API_KEY=sk-...
# ou
export OPENAI_API_KEY=sk-...
# ou
export ANTHROPIC_API_KEY=sk-...

# Rode o que quiser
radiant loop start "implement the login feature"
radiant run specs/0001-foo
radiant fleet start "migrate the auth system" --agents 5
```

**O que você NÃO pode fazer em Full:**
- Spawnar como MCP server. Se você quer MCP, use `radiant mcp serve` (sempre Light).

---

## Auto-detect na prática

Não tem detecção. O subcomando já define tudo:

| Subcomando | Mode | API key? |
|------------|------|----------|
| `radiant mcp serve` | Light (sampling) | Não |
| `radiant loop start` | Full (HTTP) | Sim |
| `radiant run` | Full (HTTP) | Sim |
| `radiant fleet start` | Full (HTTP) | Sim |
| `radiant init`, `validate`, `spec`, ... | Full | Não (não chamam LLM) |

**Regra simples:** se o subcomando pode chamar LLM e não é `mcp serve`, é Full.

---

## Tentei misturar os dois — o que acontece?

Se você rodar `radiant loop start` sem API key: erro claro no primeiro LLM call, dizendo qual env var setar.

Se você rodar `radiant mcp serve` num TTY (terminal interativo): warning de "isso não vai receber JSON-RPC requests", mas o processo continua rodando. Útil pra debug.

Se você tentar passar `--mode=light` ou `--mode=full`: erro "unknown flag". Não tem.

Se você setar `RADIANT_MODE=light` no env: ignorado. Não tem.

Se você setar `mode: light` no `.radiant.yaml`: ignorado. Não tem.

---

## Por que separamos Light e Full por subcomando?

Antes (v2.37.0) era um único binário com mode flag + env var + config. Operador tinha que saber qual combinação usar. Cada um errava em lugares diferentes:
- Alguns esqueciam de setar `--mode=light` quando queriam usar Claude Code
- Outros setavam `RADIANT_MODE=full` num host onde Claude Code esperava sampling
- A flag criava ambiguidade: `radiant loop start --mode=light` não fazia sentido (loop é Full)

A solução: **o nome do subcomando já diz tudo**. Não tem como errar.

- `radiant mcp serve` literalmente diz "esse comando é um MCP server" → sampling.
- `radiant loop start` literalmente diz "esse comando roda um loop autônomo" → HTTP.

---

## Para MCP host authors (Claude Code, Hermes, ...)

Se você está integrando `radiant` no seu host MCP via setup:

```json
{
  "mcpServers": {
    "radiant": {
      "command": "radiant",
      "args": ["mcp", "serve"]
    }
  }
}
```

Quando o host invoca `radiant mcp serve`, o harness:
1. Detecta que stdin é pipe (não TTY) → fica quieto
2. Lê JSON-RPC do stdin
3. Quando precisa de LLM, emite `sampling/createMessage` de volta
4. Host responde com a completion
5. Harness devolve o resultado pro host como `tools/call` response

Não tem flag, não tem env. Funciona out-of-the-box.

---

## FAQ

**P: Por que remover `--mode`?**
R: Era fonte constante de confusão. O subcomando já define o contexto.

**P: E se eu quiser rodar `loop` mas em Light?**
R: Não dá. `loop` precisa de LLM autônomo. Pra usar Claude Code como inference, rode `mcp serve` e use as tools expostas.

**P: E se eu quiser que `mcp serve` use API key?**
R: Não dá. `mcp serve` é sempre sampling. Se você quer autonomia, use `loop start` etc.

**P: E se eu quiser 2 instâncias de `mcp serve` (uma em cada projeto)?**
R: Funciona normalmente. Cada uma é um processo independente com seu próprio MCP server.

**P: Backwards compatibility?**
R: v2.42.0 remove `--mode`, `RADIANT_MODE`, `mode:`, e o subcomando `radiant mode`. Operadores que usavam esses precisam atualizar. Veja CHANGELOG v2.42.0 para a migration.

---

## See also

- `docs/SPRINT72-PLAN.md` — implementation of related MCP sampling work
- `internal/llm/sampling.go` — `SamplingBackend` implementation
- `internal/llm/backend.go` — `Backend` interface (HTTPBackend + SamplingBackend)
- `cmd/radiant/cmd_audit.go` — `mcp serve` registration
- `internal/mode/mode.go` — Light/Full constants (used in trace metadata)