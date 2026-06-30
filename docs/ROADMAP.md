# Roadmap — radiant-harness

## Roadmap objective

Make radiant-harness a reliable drop-in governance layer for host agents:
installable from GitHub, usable through MCP, auditable through persisted
state, and clear enough for another agent to complete real project work.

## Now

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Close placeholder specs | Removes false open work | S | Maintainers | Current repo audit | Done: no tracked spec/doc placeholders remain |
| Align async docs with code | Prevents wrong operational guidance | S | Maintainers | Existing tests | Done: MCP descriptions match shipped offline primitives |
| Stabilize root validation | Gives agents one command to trust | S | Maintainers | Local binary installed | Done: `./scripts/run.sh` passes |

## Next

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Broaden host matrix | Reduces install surprises | M | Maintainers | Access to host CLIs | Matrix covers each documented host path |
| Improve status UX | Easier recovery after partial runs | M | Maintainers | State schema stable | Status shows next action and stale placeholders clearly |
| Release v3.7.x cleanup | Publishes doc/backlog cleanup | S | Maintainers | Now items complete | Tag and GitHub release created |

## Later

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| True background subprocess async | Better long-running sync-host support | L | Maintainers | Concrete host need | Async runs continue independently and stream/poll robustly |
| Fleet async primitives | More predictable parallel orchestration | L | Maintainers | Stable loop async | Fleet has the same status/retry guarantees as loop |
| Richer ontology tooling | Better scope discovery and skill routing | M | Maintainers | Glossary/ontology adoption | Ontology can be validated against specs and skills |
