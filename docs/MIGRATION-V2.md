# Migration Guide: v0.7.0 → v1.0.0

This guide covers all breaking changes between radiant-harness v0.7.0 (pre-v2.0 roadmap)
and v1.0.0-final, with before/after examples for each.

---

## Summary of Breaking Changes

| Area | v0.7.0 | v1.0.0 |
|------|--------|--------|
| Entry point | Any skill | `radiant boot` → CONTEXT.md → loop |
| Skill loading | All skills in context | Lazy: 3–10 skills from registry |
| Context file | Full SKILL.md bodies | `.radiant-harness/CONTEXT.md` (≤2KB) |
| Hooks | SessionStart only | SessionStart + PreToolUse + PostToolUse |
| Loop | Manual task-by-task | `radiant loop start "<goal>"` (autonomous) |
| Token tracking | None | Per-phase budget with warn/exceeded states |
| IDE adapters | Static files | Generated via `radiant views --agent=<list>` |
| Verification | Self-graded | Adversarial verifier (separate agent call) |

---

## 1. Entry Point

### Before (v0.7.0)
```
# Agent reads all skills at session start
cat .claude/skills/nova-feature/SKILL.md
cat .claude/skills/validar/SKILL.md
# ... loads 20–60 skills
```

### After (v1.0.0)
```bash
# Generate project-specific context (<500 tokens)
radiant boot

# Or assemble minimal CONTEXT.md
radiant context assemble

# Then start autonomous loop
radiant loop start "implement login feature"
```

---

## 2. Skill Loading

### Before
All skills were loaded at session start regardless of project type.
This consumed 20K–55K tokens before any work began.

### After
```bash
# Context Engine detects domain and picks 3–10 skills
radiant context detect          # → domain: backend, tier: feature
radiant context assemble        # → .radiant-harness/CONTEXT.md (~300 tokens)

# Skills loaded: nova-feature, validar, adr (core)
#              + handoff, guard (feature tier)
#              + refactor, api-design (backend domain)
```

Token savings: ~80–95% reduction in context overhead.

---

## 3. Hook Configuration

### Before (settings.json)
```json
{
  "hooks": {
    "SessionStart": [{ "command": "node .claude/hooks/load-context.mjs" }]
  }
}
```

### After (settings.json)
```json
{
  "hooks": {
    "SessionStart": [{ "command": "node hooks/load-context.mjs" }],
    "PreToolUse":  [{ "command": "node hooks/pre-tool.mjs" }],
    "PostToolUse": [{ "command": "node hooks/post-tool.mjs" }]
  },
  "permissions": {
    "allow": ["Bash(go build *)", "Bash(go test *)", "Bash(git *)", "Bash(radiant *)"]
  }
}
```

**Regenerate** with:
```bash
radiant views --agent=claude --force
```

---

## 4. Loop Commands

### Before
No autonomous loop. Work was manual and session-local.

### After
```bash
radiant loop start "refactor payment module"  # start
radiant loop status                            # check phase + budget
radiant loop resume                            # resume after interrupt
radiant trace show <run-id>                    # inspect reasoning
radiant trace list                             # list all runs
```

---

## 5. Budget Profiles

### Before
No token tracking. Agents had no visibility into remaining context.

### After
```bash
# Estimate before running
radiant budget estimate spec/0042-login/spec.md --profile=standard

# Check during run
radiant loop status    # shows tokens used/remaining

# Report after run
radiant budget report <run-id>
```

Profiles:
- `--profile=lean` → 10K tokens (simple tasks)
- `--profile=standard` → 50K tokens (most features) **default**
- `--profile=thorough` → 200K tokens (architecture changes)

---

## 6. IDE Adapters

### Before
IDE files (`.cursor/rules/`, `.github/copilot-instructions.md`, etc.) were
written manually or via `radiant init` with no update mechanism.

### After
```bash
# Check what would change
radiant views --agent=cursor --diff

# Regenerate a single IDE adapter
radiant views --agent=copilot --force

# Regenerate all adapters
radiant views --agent=all --force
```

Copilot now includes bootstrap reference. Cursor gets `alwaysApply: true`.
Gemini gets token budget guidance.

---

## 7. Self-Improvement

### Before
Failure traces were not analyzed. Skills had static instructions.

### After
```bash
# Analyze trace failures
radiant improve --from-traces

# Preview proposed changes
radiant improve --from-traces --dry-run

# Apply validated improvements
radiant improve --from-traces --apply

# View history
radiant improve history
```

---

## 8. Multi-Agent (new in v1.0.0)

```bash
# Start a fleet of specialized agents
radiant fleet start "migrate database schema" --agents=3

# Check fleet progress
radiant fleet status <fleet-run-id>
```

---

## File Layout Changes

| v0.7.0 | v1.0.0 |
|--------|--------|
| `.claude/hooks/load-context.mjs` | `hooks/load-context.mjs` (v2) |
| _(none)_ | `hooks/pre-tool.mjs` |
| _(none)_ | `hooks/post-tool.mjs` |
| _(none)_ | `.radiant-harness/CONTEXT.md` |
| _(none)_ | `.radiant-harness/loop.json` |
| _(none)_ | `.radiant-harness/traces/<run-id>.jsonl` |
| _(none)_ | `.radiant-harness/improvements.jsonl` |
| _(none)_ | `.radiant-harness/fleet/<run-id>.json` |

---

## Quick Start After Migration

```bash
# 1. Regenerate all IDE adapters
radiant views --agent=all --force

# 2. Assemble context for your project
radiant context assemble

# 3. Boot (≤500 tokens entry point for any LLM)
radiant boot

# 4. Start first autonomous loop
radiant loop start "describe your goal here"
```
