# Sprint 48 — Loop Runner Wiring: `loopStartCmd` → `loop.Run()` (v1.6.0)

> **Status**: Shipped ✅  
> **Version target**: v1.6.0

---

## Background

Sprint 47 built `loop.Run()` — the autonomous loop with real LLM calls. Sprint 48
wires it into the CLI so `radiant loop start <goal>` actually runs inference end-to-end.

---

## What was built

### `cmd/radiant/main.go` — `loopStartCmd.RunE` rewritten

The command body now:

1. **Resolves LLM credentials** via `resolveLoopLLMCreds()` — vendor-neutral order:
   `RADIANT_OPENROUTER_API_KEY` → `OPENROUTER_API_KEY` → `RADIANT_OPENAI_API_KEY` →
   `OPENAI_API_KEY` → `RADIANT_ANTHROPIC_API_KEY` → `ANTHROPIC_API_KEY`

2. **Resolves model** — flag `--model` > env `RADIANT_MODEL` > default `claude-sonnet-4-6`

3. **Builds `loop.RunConfig`** with all Sprint 44–47 brakes:
   `BudgetConfig`, `StallPatience`, `VerifierConfig.Quorum`, `ReviewPanel`, `Ground`

4. **Calls `loop.Run()`** — returns `RunResult` with exit reason, iterations, elapsed, tokens, cost

5. **Prints result** — clear summary; `ExitNeedsHuman` prompts `radiant loop review`

### New flags (Sprint 48)

| Flag | Description |
|------|-------------|
| `--verifier-model <id>` | Separate model for the verifier (default = executor model) |
| `--base-url <url>` | Override LLM endpoint (e.g. `http://localhost:11434/v1` for Ollama) |
| `--dry-run` | Print config and exit — no LLM calls. Safe to run without API key. |

### `resolveLoopLLMCreds(baseURLOverride)` helper

New function at bottom of `main.go`. Returns `(apiKey, baseURL)` by scanning env vars
in vendor-neutral order. When `--base-url` is passed, it overrides the derived URL.

---

## Behaviour

```bash
# Needs OPENROUTER_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY
radiant loop start "add unit tests for internal/loop/budget.go"

# Preview without calling LLM
radiant loop start "my goal" --dry-run

# Full config
radiant loop start "fix the race condition in scheduler" \
  --model claude-opus-4-8 \
  --verifier-model claude-opus-4-8 \
  --max-time 20m \
  --max-cost 1.00 \
  --stall-patience 3 \
  --quorum-k 2 \
  --ground \
  --review-restarts 2
```

On success:
```
✓ Loop finished
  Exit:       success
  Iterations: 2
  Elapsed:    47s
  Tokens:     12450
  Cost:       $0.0374
```

On escalation:
```
✓ Loop finished
  Exit:       needs_human

Action required: radiant loop review
```

---

## What was NOT done (intentional)

- No streaming output per iteration — LLM responses arrive as a completed string.
  Streaming is a UX improvement for a future sprint.
- No `--verifier-base-url` — verifier uses the same base URL as executor.
  Separate routing is a future improvement.

---

## References

- `internal/loop/runner.go` — `loop.Run()`, `RunConfig`, `RunResult` (Sprint 47)
- `internal/llm/client.go` — `llm.Model{Model, APIKey, BaseURL}`, `SimpleChat()`
- `internal/loop/pricing.go` — `PriceFor(modelID)`
