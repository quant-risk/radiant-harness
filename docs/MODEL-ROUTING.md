# Model Routing Engine — Design Document

> **Status:** Draft (pre-implementation)
> **Target version:** v2.1.0
> **Sprint:** 56+ (Sprint 55 in progress by other agent)
> **Author:** Design session 2026-06-27
> **Predecessor:** AutoRoute (Sprint 47), Loop Runner (Sprint 47–52),
> Plan Phase (Sprint 55)

---

## 1. Problem

Every coding agent on the market uses **one model per session**. The user
picks Sonnet, or Opus, or GPT-5 — and that single model does research,
planning, code generation, and review. This is wasteful:

- **Opus planning a 3-line bugfix** burns 15x more tokens than needed.
- **Haiku trying to architect a distributed system** produces garbage.
- **Sonnet reviewing its own code** has a built-in conflict of interest.

The harness already has separate executor/verifier/planner clients
(Sprint 47/55) and an `AutoRoute` function (Sprint 47). But
`AutoRoute` only works in **direct API mode** — it does nothing when
the user runs the harness inside Claude Code, Codex CLI, Gemini CLI,
or any other host agent.

Additionally, the codebase has **three model tables that are out of
sync** (see section 8), limiting which models the harness can actually
use.

This document specifies:
(a) a model ID reconciliation (Sprint 56 prerequisite)
(b) an agent-aware, tier-based Model Routing Engine (Sprint 56–58)

---

## 2. Design Principles

1. **Agent-aware.** Detects which agent hosts the session and emits
   routing artifacts that agent can actually consume.

2. **Capability-driven.** If an agent can't switch models mid-session,
   degrades gracefully to advisory text.

3. **Tier mapping per family.** Every family has three tiers: **top**
   (research, verify), **mid** (plan, implement), **budget**
   (summarize). Data-driven, not hardcoded.

4. **Direct API mode is the golden path.** `radiant loop start` gets
   full multi-model routing. IDE agents get best-effort static config.

5. **No vendor lock-in, no new dependencies.** Pure Go, no SDK.

---

## 3. Routing Strategies

Five strategies. Each agent maps to exactly one.

### 3.1 `direct_api` (golden path)

**Agents:** radiant loop, radiant run

Full multi-model routing. Executor, verifier, and planner are separate
`llm.Client` instances (already implemented in Sprint 47/55).

### 3.2 `subagent_delegation`

**Agents:** Claude Code

Claude Code's `Task` tool spawns subagents with model override. Emits
`.claude/settings.json` + `.claude/commands/radiant-route.md`.

### 3.3 `delegate_task_routing`

**Agents:** Hermes Agent

Emits `.radiant-harness/routing-hermes.yaml` for Hermes'
`delegate_task` mechanism.

### 3.4 `config_per_role`

**Agents:** OpenCode, Roo Code, Kilo Code, Cline

Emits agent-specific config JSON with per-role model slots.

### 3.5 `single_model_advisory` (fallback)

**Agents:** Codex CLI, Gemini CLI, Cursor, Copilot, Windsurf

Injects `## Model Routing Advisory` into the agent's instructions file.

---

## 4. Tier Matrix

### 4.1 Model families -> tiers (2026-06-27)

| Family    | Top (research/verify) | Mid (plan/implement)   | Budget (summarize)  |
|-----------|-----------------------|------------------------|---------------------|
| Claude    | opus-4-8              | sonnet-4-6             | haiku-4-5           |
| OpenAI    | gpt-5                 | gpt-5-mini             | gpt-5-nano          |
| Gemini    | gemini-2.5-pro        | gemini-2.5-flash       | gemini-2.5-flash    |
| Xiaomi    | mimo-v2.5-pro         | mimo-v2.5-pro          | mimo-v2.5-lite      |
| DeepSeek  | deepseek-v4-pro       | deepseek-v4-flash      | deepseek-v4-flash   |
| Mistral   | mistral-large-2       | codestral-22b          | codestral-22b       |
| GLM/Z.AI  | glm-5.2               | glm-5.2-air            | glm-5.2-air         |
| Kimi      | kimi-k2               | kimi-k2                | kimi-k2-flash       |
| MiniMax   | minimax-m1            | minimax-text-01        | abab-7              |
| Qwen      | qwen-3-coder-plus     | qwen-3-coder-plus      | qwen-2.5-coder-plus |
| Llama     | llama-4-scout-70b     | llama-4-scout-17b      | llama-4-scout-17b   |
| Groq      | groq-llama-3.3-70b    | groq-llama-3.3-70b     | groq-llama-3.3-8b   |

### 4.2 Phase -> tier mapping

| Phase     | Tier  | Rationale                                      |
|-----------|-------|------------------------------------------------|
| Research  | Top   | Deep analysis, needs strongest reasoning       |
| Plan      | Top   | Architecture decomposition, expensive mistakes |
| Implement | Mid   | Code generation, high volume, mid-tier suffices|
| Correct   | Mid   | Fixing bugs in generated code, same tier       |
| Verify    | Top   | Adversarial review, must be >= executor        |
| Persist   | Budget| Writing checkpoint JSON, trivial               |
| Summarize | Budget| Compact handoff text, trivial                  |

### 4.3 Agent -> strategy mapping

| Agent         | Strategy                  | Detection                     |
|---------------|---------------------------|-------------------------------|
| radiant loop  | direct_api                | loop.json exists              |
| radiant run   | direct_api                | internal                      |
| Claude Code   | subagent_delegation       | `.claude/` dir exists         |
| Hermes Agent  | delegate_task_routing     | `~/.hermes/` dir exists       |
| OpenCode      | config_per_role           | `.opencode/` dir exists       |
| Codex CLI     | single_model_advisory     | `codex` on $PATH + AGENTS.md  |
| Gemini CLI    | single_model_advisory     | `gemini` on $PATH             |
| Cursor        | single_model_advisory     | `.cursor/rules/` exists       |
| Copilot       | single_model_advisory     | `.github/copilot-*` exists    |
| Windsurf      | single_model_advisory     | `.windsurf/` exists           |
| (unknown)     | single_model_advisory     | default fallback              |

---

## 5. Package Layout

```
internal/routing/
├── routing.go          Strategy, RoutingPlan, PhaseRouting types
├── capability.go       Agent -> strategy detection
├── matrix.go           Family tiers + phase->tier mapping
├── resolver.go         Anchor + family + agent -> resolved Plan
├── emitter.go          Strategy-specific artifact generation
├── override.go         .radiant-harness/routing.yaml override
├── *_test.go           Table-driven tests
```

### Core types

```go
package routing

type Strategy string
const (
    StrategyDirectAPI           Strategy = "direct_api"
    StrategySubagentDelegation  Strategy = "subagent_delegation"
    StrategyDelegateTask        Strategy = "delegate_task_routing"
    StrategyConfigPerRole       Strategy = "config_per_role"
    StrategySingleModelAdvisory Strategy = "single_model_advisory"
)

type Phase string
const (
    PhaseResearch  Phase = "research"
    PhasePlan      Phase = "plan"
    PhaseImplement Phase = "implement"
    PhaseCorrect   Phase = "correct"
    PhaseVerify    Phase = "verify"
    PhasePersist   Phase = "persist"
    PhaseSummarize Phase = "summarize"
)

type RoutingPlan struct {
    Agent    AgentID
    Strategy Strategy
    Anchor   string
    Family   string
    Phases   map[Phase]PhaseRouting
}

type PhaseRouting struct {
    Phase string
    Model string  // e.g. "claude-opus-4-8"
    Tier  string  // "top", "mid", "budget"
    Via   string  // "main", "subagent", "api", "advisory"
}
```

---

## 6. CLI Surface

### New: `radiant models route`

```bash
radiant models route                              # auto-detect
radiant models route --agent=claude --anchor=claude-sonnet-4-6
radiant models route --dry-run                    # show plan, no writes
radiant models route --apply                      # emit artifacts
radiant models route --json                       # machine-readable
```

### Extended: `radiant loop start`

Auto-resolves executor/verifier/planner from anchor when not explicit.

### Extended: `radiant boot`

Manifest gains `routing` section.

### Extended: `radiant init`

Prints routing hint after scaffolding.

---

## 7. Migration from AutoRoute

`llm.AutoRoute` preserved as thin wrapper calling `routing.Resolve`.
Import cycle avoided via function pointer: routing receives a
`familyLookup func(string) FamilyTier` from the caller, not the llm
package itself.

---

## 8. Model ID Reconciliation (Sprint 56 prerequisite)

### Current state: three tables, three naming conventions

| Table | Location | Format | Example | Status |
|-------|----------|--------|---------|--------|
| PresetModels | llm/client.go | dotted, OpenRouter path | claude-opus-4.1 | STALE |
| providerPricing | loop/pricing.go | dashed, native | claude-opus-4-8 | CURRENT |
| PricePerMTokensUSD | llm/routing.go | dotted | claude-opus-4.1 | STALE |

### Target: one canonical ID per model, used in all three tables

**Canonical IDs** (using the dashed format already in pricing.go):

```
Claude:    claude-opus-4-8, claude-sonnet-4-6, claude-haiku-4-5
OpenAI:    gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-codex
Gemini:    gemini-2.5-pro, gemini-2.5-flash
Xiaomi:    mimo-v2.5-pro, mimo-v2.5-lite
DeepSeek:  deepseek-v4-pro, deepseek-v4-flash, deepseek-r1
Mistral:   mistral-large-2, codestral-22b
GLM/Z.AI:  glm-5.2, glm-5.2-air
Kimi:      kimi-k2, kimi-k2-flash
MiniMax:   minimax-m1, minimax-text-01, abab-7
Qwen:      qwen-3-coder-plus, qwen-2.5-coder-plus
Groq:      groq-llama-3.3-70b, groq-llama-3.3-8b
```

### Action

1. Update PresetModels: replace all entries with canonical IDs.
   Each entry maps the canonical ID to the correct OpenRouter path:
   `claude-opus-4-8` -> `anthropic/claude-opus-4-8`
2. Update PricePerMTokensUSD: same canonical IDs.
3. providerPricing already uses canonical IDs — add missing entries
   (GLM, Kimi, MiniMax, Qwen, gpt-5-mini, gpt-5-nano).
4. AutoRoute and tierByPreset: update prefix matching to handle
   new dashed format.

---

## 9. Testing Strategy

| Test file              | What it covers                                |
|------------------------|-----------------------------------------------|
| routing_test.go        | Resolve() returns correct per-phase models    |
| matrix_test.go         | Every family has all 3 tiers filled           |
| matrix_test.go         | Phase->tier mapping is complete               |
| capability_test.go     | Detection returns correct strategy per artifact|
| capability_test.go     | Priority order enforced                       |
| emitter_test.go        | Each strategy emits valid JSON/YAML           |
| emitter_test.go        | Idempotent (Emit twice = identical output)    |

All tests table-driven, deterministic, temp directories.
