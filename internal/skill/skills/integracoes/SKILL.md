# Skill: integracoes

> Discovers and validates MCPs / tools. Critical safety: the
> account/workspace boundary matters. NEVER auto-configures.

## Decision tree

```
User mentions a tool, or kickoff calls integracoes
        │
        ▼
Tool exists in the local MCP catalogue? (env var present?)
        │
        ├── no ──► Mark as "candidate, not installed"
        │          Suggest installation instructions
        │
        └── yes ──► Ask: business account or personal?
                       │
                       ├── business ──► Validate with read query
                       │
                       └── personal ──► WARN: don't use for
                                          work artifacts

        ▼
Test with a read-only query (list / search / get)
        │
        ├── Pass ──► Mark "validated, ready"
        │
        └── Fail ──► Mark "validated, failing" + reason
                     Suggest fix or removal
        │
        ▼
User approves writing .mcp.json?
        │
        ├── yes ──► Write .mcp.json (only approved servers)
        │
        └── no ──► Just record in docs/engineering/integrations.md
```

## Workflow

### Step 1: enumerate

Ask user what tools they use. Common categories:
- Issue tracking: Jira, Linear, GitHub Issues
- Docs: Confluence, Notion, GitHub Wiki
- Code: GitHub, GitLab, Bitbucket
- Cloud: AWS, GCP, Azure
- Communication: Slack, Teams

Don't propose tools — let the user say what they use. Don't
default to "use GitHub" because the project is on GitHub.

### Step 2: account boundary

For each tool, ask explicitly: business or personal account?
This is not optional. A personal Notion has personal notes that
shouldn't appear in work artifacts.

### Step 3: validate

For each MCP/tool, run a read-only query. Examples:
- Jira: list 1 project
- Notion: search for "test"
- GitHub: list 1 repo

If the query fails, mark as "failing" and ask the user. Don't
silently mark as "ready".

### Step 4: ask before writing .mcp.json

`.mcp.json` changes agent behavior globally for the project.
ALWAYS ask before writing. The user might want the integration
documented but not active yet.

## Examples

### Example 1: GitHub MCP, business account

**Inputs**: scope=all, user mentions "GitHub"

**Validation**: `gh repo list` returns the project repos.

**Approval**: "Yes, write .mcp.json for GitHub"

**Output**: `docs/engineering/integrations.md` has GitHub entry,
`.mcp.json` has the github MCP server.

### Example 2: Notion MCP, personal account

**Inputs**: scope=all, user mentions "Notion"

**Warning**: "You said Notion. Is this your business workspace or
personal? Personal notes shouldn't be referenced in this project."

**If user says personal**: Mark as "personal, do not use for work
artifacts. Listed for awareness only."

## Anti-patterns

- ❌ Auto-writing .mcp.json. Always ask.
- ❌ Treating "installed" as "ready". Always test.
- ❌ Ignoring personal vs business. Default safety = personal.
  Force user to opt-in to business.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `account-boundary-clarified` | User says "I don't know" | Default to personal. Don't proceed without clarification. |
| `validated` | Read query fails | Surface error; ask user to fix MCP config. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `kickoff` | Calls integracoes before starting interviews. |
| `nova-feature` | May pull data from MCPs during research. |