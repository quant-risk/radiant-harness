# radiant-harness Docs

This directory documents the harness as a reusable agent-control system,
not a single task run.

## What the project does

radiant-harness installs a local `radiant` CLI and MCP server that lets a
host agent run work through a governed Discover -> Plan -> Execute ->
Verify -> Persist loop. When the host supports MCP sampling, the harness
drives the agent phase by phase. When sampling is unavailable, it falls
back to self-driven scaffolds with explicit handoff markers so the host
agent can continue with its own native tools.

## Main docs

| File | Purpose |
|------|---------|
| `ARCHITECTURE.md` | high-level system and module map |
| `HOST-AGENTS.md` | host-agent integration model |
| `LOOP-ENGINE.md` | loop execution model |
| `ONTOLOGY.md` | world model and domain vocabulary |
| `STATE.md` | current project state and next actions |
| `glossary.md` | canonical terms |
| `ROADMAP.md` | current Now/Next/Later backlog |
| `engineering/TESTING.md` | verification strategy |

## Validation

Run the project validation entrypoint:

```bash
./scripts/run.sh
```

For the public drop-in path specifically:

```bash
make test-dropin
```
