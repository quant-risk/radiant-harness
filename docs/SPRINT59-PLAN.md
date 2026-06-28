# Sprint 59 — AutoRoute integrado no loop runner (v2.7.0)

> **Status**: Shipped ✅  
> **Version target**: v2.7.0

Fecha o arco do model routing engine: `AutoRoute` agora é funcional no loop.

---

## O que foi construído

### `RunConfig.AutoRoute bool` — `internal/loop/runner.go`

Novo campo opt-in (default `false`). Quando `true`, o `Run()` deriva
modelos por fase a partir do anchor (`ExecutorModel`) usando `llm.AutoRoute`:

| Fase | Tier alvo | Exemplo (anchor = sonnet) |
|------|-----------|--------------------------|
| Research / Verify | TierTop | `claude-opus-4-8` |
| Plan | TierMid | `claude-sonnet-4-6` (anchor) |
| Execute | anchor | `claude-sonnet-4-6` |

Quando a família do anchor não tem um sibling mais forte, o anchor é
usado para todas as fases (fail-safe). `VerifierModel` e `PlannerModel`
explícitos só têm efeito quando `AutoRoute=false`.

### Flags no CLI — `cmd/radiant/cmd_loop.go`

```
radiant loop start "goal" --auto-route
radiant loop resume <run-id> --auto-route
```

`--auto-route` é combinável com `--model` (define o anchor) e com `--plan`
(LLM planning permanece independente do routing).

### `internal/loop/sprint59_test.go` — 10 novos testes

- `AutoRoute` default false, assignable
- `autoRoutedModels` helper: disabled → todos no anchor
- Sonnet anchor → verifier sobe para opus, execute fica em sonnet
- Opus anchor → verifier fica em opus, plan desce para sonnet
- Unknown anchor → todos ficam no anchor (fail-safe)
- BaseURL e APIKey propagados para modelos derivados
- `Run()` com `AutoRoute=true` — fail-open quando sem API key

---

## Exemplo de uso

```bash
# Research usa claude-opus-4-8, Execute/Plan ficam em claude-sonnet-4-6
radiant loop start "refactor the auth module" \
  --model claude-sonnet-4-6 \
  --auto-route \
  --plan

# Família desconhecida → todos usam o custom model
radiant loop start "build the API" \
  --model my-custom-model \
  --auto-route  # noop seguro
```

---

## Referências

- `internal/loop/runner.go` — `RunConfig.AutoRoute`, bloco de derivação de clientes
- `internal/llm/routing.go` — `AutoRoute(anchor, phase)`, `tierByPreset`
- `cmd/radiant/cmd_loop.go` — flags `--auto-route` em start e resume
