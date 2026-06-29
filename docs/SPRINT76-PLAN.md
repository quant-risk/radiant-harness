# Sprint 76 — v2.46.0 — `cmd_setup_mcp.go` split: main + per_agent

## Goal

`cmd/radiant/cmd_setup_mcp.go` grew from 464 LOC (post-Sprint 73) to
**781 LOC** (post-Sprint 75) after adding Hermes + Kimi + OpenClaw +
Cline. Continuing the same "debt-reduction" rhythm we used in
Sprint 74 (helpers.go extractions), split this file into two
themed files following the same pattern.

## Current shape (post-Sprint 75)

```
cmd_setup_mcp.go  (781 LOC total)
├── package + imports                                  ~30 LOC
├── registerSetupMCPCmd (cobra command)                ~70 LOC
├── radiantBinaryPath                                   ~14 LOC
├── resolveMCPAgents (auto-detect)                     ~55 LOC
├── type mcpEntry                                      ~5  LOC
├── mcpConfigFor (routing switch)                      ~120 LOC
├── mergeClaudeSettings, mergeMCPJSON, mergeZedSettings ~70 LOC  ← generic JSON
├── writeMCPConfig                                     ~14 LOC
├── Codex (TOML): radiantBlockPattern, tomlQuote,
│   mergeCodexTOML                                    ~80  LOC  ← per-agent
├── OpenCode: openCodeServer, openCodeConfig,
│   mergeOpenCodeConfig                                ~85  LOC  ← per-agent
├── Hermes: hermesEntry, mergeHermesConfig             ~60  LOC  ← per-agent
├── Kimi: mergeKimiMCP                                 ~45  LOC  ← per-agent
├── OpenClaw: openClawServer, mergeOpenClawJSONConfig  ~80  LOC  ← per-agent
└── Cline: clineEntry, mergeClineConfig                ~55  LOC  ← per-agent
```

## Target shape (post-Sprint 76)

```
cmd_setup_mcp.go  (~280 LOC)
├── package + minimal imports (encoding/json, fmt, os, os/exec,
│   path/filepath, strings, github.com/spf13/cobra)
├── registerSetupMCPCmd
├── radiantBinaryPath
├── resolveMCPAgents
├── type mcpEntry
├── mcpConfigFor         ← still the single source of truth for routing
├── mergeClaudeSettings, mergeMCPJSON, mergeZedSettings
└── writeMCPConfig

cmd_setup_mcp_per_agent.go  (~520 LOC)
├── package + agent-specific imports (regexp, gopkg.in/yaml.v3,
│   plus shared stdlib above)
├── Codex TOML:        radiantBlockPattern, tomlQuote, mergeCodexTOML
├── OpenCode JSON:     openCodeServer, openCodeConfig, mergeOpenCodeConfig
├── Hermes YAML:       hermesEntry, mergeHermesConfig
├── Kimi JSON:         mergeKimiMCP
├── OpenClaw JSON:     openClawServer, mergeOpenClawJSONConfig
└── Cline JSON:        clineEntry, mergeClineConfig
```

## Why this split (not finer, not coarser)

- **Coarser** (everything stays in one file): not enough leverage;
  781 LOC stays, no surface area reduction in `cmd_setup_mcp.go`.
- **Finer** (one file per agent = 6 new files): over-splitting — each
  file would be 50–85 LOC of mostly-related code. Harder to scan when
  reading one agent's merge.
- **This split:** The "main" file gets down to ~280 LOC, focused on
  the routing + detection + generic JSON merges. The "per_agent" file
  is a flat 6-agent reference that's easy to extend (add a 12th
  agent: append one stanza, done).

## Imports discipline

`cmd_setup_mcp.go` keeps:
- encoding/json, fmt, os, os/exec, path/filepath, strings
- github.com/spf13/cobra

`cmd_setup_mcp_per_agent.go` adds:
- regexp             (Codex TOML pattern)
- gopkg.in/yaml.v3   (Hermes YAML)

Both files stay in `package main` — internal Go file split, not a
new package. Tests in `cmd_setup_mcp_test.go` continue to reference
both because they're all in `package main`.

## Test impact

**Zero.** The 18 setup-mcp tests reference exported and unexported
functions like `mergeCodexTOML`, `mergeHermesConfig`, etc., all of
which stay callable (same package, same identifiers, just living in
different files now). All 18 tests must keep passing without any
edit.

## Files

- `cmd/radiant/cmd_setup_mcp.go` — trim from 781 → ~280 LOC
- `cmd/radiant/cmd_setup_mcp_per_agent.go` — NEW, ~520 LOC
- `cmd/radiant/main.go` — version bump (`2.45.0` → `2.46.0`)
- `CHANGELOG.md` — v2.46.0 entry
- `RELEASE-NOTES.md` — v2.46.0 notes

## What's NOT in this sprint

- **No new agents.** This is purely a refactor; agent count stays at 11.
- **No behaviour change.** Pure code movement. Same routing, same output.
- **No `helpers.go` extractions.** Next debt-reduction candidate is a
  separate sprint (Sprint 77 candidates: PR review ~400 LOC,
  integrations ~150 LOC, incident ~150 LOC, ADR scaffolds ~100 LOC).
