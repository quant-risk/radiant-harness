---
name: metricas
description: Track Lead Time, Throughput, CD maturity, and bottlenecks.
---

# Skill: Metrics (Delivery Flow Health)

Measures flow health: Lead Time, Throughput, and Continuous Delivery maturity.
Uses git, CI, and PM tool data to **find bottlenecks** — not to rank people.

## Phase 1 — Gather data sources (Research)

1. Define the measurement period (last sprint, last month, last quarter).
2. **Git data:** `terminal: git log --since="<date>" --merges --format="%H %ci %s"` — merged PRs with timestamps.
3. **CI data:** if MCP connected (`mcp__github__*`), fetch workflow run durations and failure rates.
4. **PM data:** if MCP connected (`mcp__jira__*` or `mcp__linear__*`), fetch issues with created/resolved timestamps.
5. `read_file` `docs/engineering/metrics.md` — previous measurements for trend comparison.

> Delegate data gathering to a subagent. Raw git/Jira logs are large — bring only the computed summary into context.

## Phase 2 — Compute Lead Time (Plan)

Lead Time = time from spec creation (or first commit, or issue creation) to production deploy.

1. For each completed item, calculate:
   - **Spec → first commit** (planning time).
   - **First commit → merge** (development time).
   - **Merge → deploy** (deploy queue time).
2. Report **median** and **p85** — the tail shows where flow stalls.
3. Identify which segment dominates. Examples:
   - Spec → commit is long → planning bottleneck, spec ambiguity.
   - Commit → merge is long → review bottleneck, PR too large.
   - Merge → deploy is long → deploy process bottleneck, manual gates.

## Phase 3 — Compute Throughput (Plan)

1. Count items that reached "done"/prod in the period.
2. Compare to previous period (from `metrics.md`): increasing, stable, declining?
3. Note team size context — throughput per contributor, not absolute numbers.

## Phase 4 — Assess CD maturity (Plan)

Score Continuous Delivery / Deployment practices:

| Practice | Current state | How to check | Gap to advance |
|----------|--------------|--------------|----------------|
| Always deployable (CD) | yes/partial/no | CI green on main? Deploy blocked by manual steps? | |
| Auto-deploy (CDepl) | yes/partial/no | Does merge to main trigger auto-deploy? | |
| Rollback capability | yes/no | Can we revert in < 5 min? | |

2. Code quality metrics (from CI):
   - Coverage: current %, trend vs last period.
   - Static analysis: blocking findings count, trend.

## Phase 5 — Generate report (Implement)

1. Update `docs/engineering/metrics.md` with all computed values and trends.
2. Highlight **top bottleneck** with a specific recommendation:
   > "Lead Time median is 8 days. 5 of those are commit→merge. PRs average 600 lines.
   > Recommendation: split features into smaller specs (target < 300 lines/PR)."
3. Present the report to the user. Ask: "Does this match your perception? Any context I'm missing?"

## Phase 6 — Update state

1. Update `docs/STATE.md`: metrics reviewed, date, top bottleneck noted.
2. If a bottleneck relates to a roadmap item, note the cross-reference.

## Rules

- **Find bottlenecks, don't rank people.** Never report individual contributor metrics.
- Use median and p85 — averages hide the tail where problems live.
- Trend matters more than absolute numbers. One snapshot is noise.
- If data sources are incomplete (no MCP, sparse git history), note the gap and report what's available.
- Never present metrics without context — a number without a story is noise.
