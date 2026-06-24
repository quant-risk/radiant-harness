---
name: integrations
description: Team tools and MCPs. Pull when integrating Jira/Confluence/cloud.
alwaysApply: false
---

# Integrations and MCPs — <project name>

> Tools the team already uses and proposed **MCP servers** to connect them to the AI agent.
> Generated in the kickoff.

## Team tools
| Category           | Tool                    | Process / observation        |
|--------------------|------------------------|------------------------------|
| Project management | <Jira/Trello/Linear>   | <Scrum / Kanban / Waterfall> |
| Documentation      | <Confluence/Notion>    | <where living docs live>     |
| Code & CI          | <GitHub/GitLab>        | <PR/MR flow>                 |
| Cloud              | <AWS/GCP/Azure>        | <regions, accounts>          |
| Observability      | <Datadog/Sentry/Grafana> | <where alerts land>       |

## Proposed MCPs
| Tool           | MCP server (proposed)    | Account/workspace (validated) | What it enables          | Status |
|----------------|-------------------------|-------------------------------|--------------------------|--------|
| Jira           | Atlassian (official)    | <project workspace>           | read/create issues       | proposed |
| GitHub         | GitHub (official)       | <org/repo>                    | PRs, issues, code review | proposed |

## How to connect
- **Project-scoped:** `.mcp.json` at repo root — shareable with team. **No secrets.**
- **Secrets:** via env var or `claude mcp add`. **Never** commit tokens.
