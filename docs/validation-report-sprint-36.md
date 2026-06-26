# Validation Report — Sprint 36: Enhanced Hooks + IDE Adapters

**Date:** 2026-06-26
**Sprint:** 36 of 40
**Status:** PASSED

---

## Deliverables

| File | Type | Purpose |
|------|------|---------|
| `hooks/load-context.mjs` v2 | Hook | Token-aware SessionStart; fast path via CONTEXT.md (≤2KB), legacy fallback |
| `hooks/post-tool.mjs` | Hook | PostToolUse — records every tool call into active loop trace (JSONL) |
| `hooks/pre-tool.mjs` | Hook | PreToolUse — blocks tool calls when token budget < 10% remaining |
| `internal/scaffold/templates/settings.json` | Template | Updated with PreToolUse + PostToolUse hooks and `permissions.allow` allowlist |
| `internal/scaffold/scaffold.go` | Go | `DiffViews()`, `FormatDiff()`, `EnrichContent()` — IDE-specific enrichment |
| `internal/scaffold/sprint36_test.go` | Go tests | 13 new tests for diff, format, enrich |
| `cmd/radiant/main.go` | CLI | `radiant views --diff` flag |

---

## Test Results

```
ok  github.com/quant-risk/radiant-harness/internal/scaffold  0.510s
```

**13 new tests pass (20 total in scaffold package).**

### DiffViews (4 tests)
- `TestDiffViews_AllNew` — empty directory produces status=new for all views
- `TestDiffViews_UnchangedAfterWrite` — status=unchanged after writing exact generated content
- `TestDiffViews_DetectsChange` — status=changed when on-disk content differs from generated
- `TestDiffViews_UnknownAgent` — returns 0 diffs for unregistered agent

### FormatDiff (3 tests)
- `TestFormatDiff_AllNew` — `+ path` markers for new files
- `TestFormatDiff_AllUnchanged` — "Nothing to update" message
- `TestFormatDiff_Mixed` — `+ new`, `~ changed`, `= N unchanged` all present

### EnrichContent (6 tests)
- `TestEnrichContent_Copilot_AddsBootstrapRef` — bootstrap reference + loop commands added
- `TestEnrichContent_Gemini_AddsBudgetHints` — budget profiles (10K/50K/200K) added
- `TestEnrichContent_Cursor_AddsAlwaysApply` — `alwaysApply: true` injected into MDC frontmatter
- `TestEnrichContent_Cursor_NoDoubleAlwaysApply` — existing alwaysApply not duplicated
- `TestEnrichContent_Claude_Passthrough` — Claude content unchanged
- `TestEnrichContent_Unknown_Passthrough` — unknown agent content unchanged

---

## Architecture

### Hooks (Sprint 36 additions)

**`hooks/load-context.mjs` v2 (SessionStart):**
- Fast path: reads `.radiant-harness/CONTEXT.md` assembled by `radiant context assemble`
- Hard cap at 2048 bytes to enforce ≤2KB overhead guarantee
- Fallback to legacy BASE docs when no CONTEXT.md exists

**`hooks/post-tool.mjs` (PostToolUse):**
- Reads tool event from stdin (JSON: tool_name, tool_input, tool_response)
- Appends JSONL entry to `.radiant-harness/traces/<run-id>.jsonl`
- Skips silently when no active loop (no loop.json)

**`hooks/pre-tool.mjs` (PreToolUse):**
- Reads budget from `loop.json`
- Blocks tool call (exit 2 + JSON `{decision: "block", reason: "..."}`) when < 10% remaining
- Skips silently when no active loop or unlimited budget

### settings.json v2

```json
{
  "hooks": {
    "SessionStart":  [{ "command": "node hooks/load-context.mjs" }],
    "PreToolUse":   [{ "command": "node hooks/pre-tool.mjs" }],
    "PostToolUse":  [{ "command": "node hooks/post-tool.mjs" }]
  },
  "permissions": {
    "allow": ["Bash(go build *)", "Bash(go test *)", "Bash(git *)", "Bash(radiant *)"]
  }
}
```

### `radiant views --diff`

Shows what would change before any file is written:
```
  [claude]
  + CONVENTIONS.md (new)
  ~ .claude/settings.json (changed)
  = 61 file(s) unchanged
```

---

## Regression

All prior sprint packages remain green:
```
ok  github.com/quant-risk/radiant-harness/internal/context   0.581s
ok  github.com/quant-risk/radiant-harness/internal/boot      0.833s
ok  github.com/quant-risk/radiant-harness/internal/loop      1.167s
ok  github.com/quant-risk/radiant-harness/internal/scaffold  0.510s
```

---

## Next: Sprint 37

Token Budget & Compression:
- `radiant budget estimate <file>` — estimate tokens before sending to LLM
- Auto-compression triggers on context assemble when over threshold
- Context summarizer for long loop traces
