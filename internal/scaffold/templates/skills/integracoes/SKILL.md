---
name: integracoes
description: Discover team tools and connect MCPs with lock protocol.
---

# Skill: Integrations (MCP Discovery and Connection)

Discovers team tools, validates MCP connections, and generates `docs/engineering/integrations.md`.
Read-first approach: connect, read to validate, then lock the account/workspace before any write.

## Phase 1 — Discover team tools (Research)

1. `read_file` `docs/product/vision.md` and `docs/STATE.md` for context on what tools are in use.
2. Ask the user: "Which tools does the team use for project management, docs, code/CI, cloud, observability?"
3. If an existing `integrations.md` exists, load it — this is a re-run; verify which entries are still valid.
4. Detect MCP servers already present in the session (`mcp__<server>__*` tools available).

## Phase 2 — Propose MCP connections (Plan)

For each team tool, map to a candidate MCP server:

| Tool category | Candidate MCP | Enables |
|---------------|--------------|---------|
| Project mgmt (Jira/Linear) | Atlassian / Linear official | read/create issues |
| Docs (Confluence/Notion) | Confluence / Notion MCP | read/search docs |
| Code (GitHub/GitLab) | GitHub / GitLab MCP | PRs, issues, code review |
| Cloud (AWS/GCP) | Cloud provider MCP | resource inventory, logs |

2. Present the proposed connections table. Confirm which to proceed with.

## Phase 3 — Connect and validate (Implement)

For each approved connection, follow the **lock protocol**:

1. **Connect:** run `claude mcp add <server>` or edit `.mcp.json` (project-scoped, no secrets in file).
2. **Read-lock:** confirm the account/workspace with the user before reading anything.
3. **Validate read:** execute one read call (e.g. `mcp__jira__list_issues`). Verify the workspace matches.
4. **Write-lock:** before any write, **re-confirm** the account/workspace with the user.
   Write is opt-in per skill — "active connection does NOT authorize use."

5. Update the MCP table in `CLAUDE.md` with validated account/workspace and consuming skills.
6. Generate `docs/engineering/integrations.md` from the template — fill team tools, proposed/connected MCPs, status.

## Phase 4 — Routing

For each connected MCP, document which skills consume it:

| MCP | Consuming skills |
|-----|-----------------|
| `mcp__jira__*` | `/nova-feature` (fetch issues), `/metricas` (cycle time) |
| `mcp__github__*` | `/revisar-pr` (PR check), `/metricas` (lead time) |

## Rules

- **No secrets in files.** Tokens go in env vars or `claude mcp add`, never in `.mcp.json` or docs.
- **Re-executable:** re-running `/integracoes` re-validates existing connections and adds new ones.
- Read before write. Confirm before read. Re-confirm before write.
- Follow `CLAUDE.md` conventions — only use MCPs listed in the session.
