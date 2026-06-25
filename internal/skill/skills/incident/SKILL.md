# Skill: incident

> Incident response: triage, mitigate, communicate, post-mortem.
> The first 15 minutes matter; this skill structures the response.

## Decision tree

```
Alert fires / customer reports / security incident detected
        │
        ▼
[0-2 min] Acknowledge + assign severity
        │
        ├── sev1 (full outage, major impact) → page on-call, name commander
        ├── sev2 (degraded, partial)          → page on-call
        ├── sev3 (minor, workaround)          → ticket, async response
        └── sev4 (cosmetic)                   → backlog
        │
        ▼
[2-5 min] Name incident commander
        │
        ▼
[5-15 min] Mitigate (restore service, stop the bleeding)
        │
        ├── Known playbook exists? → follow it
        └── Unknown? → hypothesis-driven debugging
        │
        ▼
[15-30 min] Communicate
        │
        ├── Update status page (sev1/sev2)
        ├── Notify stakeholders (engineering, support, leadership)
        └── Start public incident document
        │
        ▼
[During] Continue investigation (find root cause, not just symptom)
        │
        ▼
[Resolution] Apply permanent fix, deploy, verify
        │
        ▼
[+24-48h] Post-mortem (blameless, structured)
        │
        ▼
[+5 days] Action items owned + tracked in roadmap
```

## Workflow

### Step 1: Triage (0-2 min)

1. **Acknowledge** the alert — silence is worse than a wrong
   guess. Acknowledge in PagerDuty / Opsgenie / Slack.
2. **Assign severity**:
   - **sev1**: full outage, major customer impact, no workaround
   - **sev2**: degraded, partial impact, workaround exists
   - **sev3**: minor, single feature affected
   - **sev4**: cosmetic, no customer impact
3. **Open a war room** (Slack channel / Zoom / phone bridge)
   for sev1 and sev2.

### Step 2: Name commander (2-5 min)

The incident commander is NOT the person debugging. The
commander:
- Coordinates across teams
- Decides when to escalate
- Owns external comms
- Declares "we're mitigating" vs "we're investigating"

If you're senior and the only one awake, you're the commander.
Hand off as soon as someone else can take it.

### Step 3: Mitigate (5-15 min)

**Goal: stop the bleeding. Not: find root cause.**

Playbook order:
1. Roll back the most recent deploy (always safe, often enough)
2. Scale up (if load-related)
3. Failover to backup (if region-related)
4. Disable the broken feature (feature flag)
5. Manually intervene (last resort, slow)

If none of these work in 15 min, escalate. Don't keep
flailing.

### Step 4: Communicate (15-30 min)

For sev1/sev2:

1. **Status page** — what customers see. Be honest about
   impact; don't sugarcoat.
2. **Internal stakeholders** — engineering leads, support,
   leadership. Slack #incidents channel.
3. **Affected customers** — direct outreach for sev1, especially
   paying customers.

Template:

> We're investigating [symptom]. Some users may experience
> [impact]. Our team is actively investigating. We'll update
> within [timeframe].

### Step 5: Investigate (during)

Once service is restored, find the root cause. Use:

- **Logs** — search for errors, exceptions, anomalies
- **Metrics** — dashboards, alerts that fired
- **Recent changes** — deploys, config changes, feature flags
- **Hypotheses** — what's your best guess? Test it. Iterate.

Don't rest until you know WHY, not just WHAT.

### Step 6: Permanent fix + post-mortem

After mitigation:

1. Apply the permanent fix (code change, config update, etc.)
2. Deploy + verify
3. Schedule post-mortem within 5 business days
4. Use the `roadmap` skill to track action items

## Post-mortem template

The post-mortem is the most valuable output of the incident.
Run it within 5 business days, while memory is fresh. Use the
`blameless post-mortem` template:

```markdown
# Post-mortem: <NNNN> — <summary>

**Severity**: sev1 / sev2 / sev3 / sev4
**Date**: <YYYY-MM-DD>
**Duration**: <HH:MM> (from detection to resolution)
**Impact**: <customer-facing impact>
**Commander**: <name>
**Author**: <name>

## Timeline (UTC)

- HH:MM — detection (alert fired / customer report)
- HH:MM — commander named
- HH:MM — mitigation started (rollback / scale / failover)
- HH:MM — service restored
- HH:MM — root cause identified
- HH:MM — permanent fix deployed

## Root cause

What happened, and WHY. Not the symptom — the cause. Include
the chain of events that led to the failure.

## Contributing factors

- What monitoring missed
- What tests didn't catch
- What process / runbook was unclear
- What communication failed

## What went well

- Fast detection
- Quick rollback
- Clear comms
- Good escalation

## Action items

| # | Action | Owner | Due | Tracked in |
|---|--------|-------|-----|------------|
| 1 | Add monitoring for X | @alice | 2026-07-01 | roadmap |
| 2 | Improve runbook for Y | @bob   | 2026-07-15 | roadmap |
| 3 | Add regression test for Z | @carol | 2026-07-01 | roadmap |
```

## Examples

### Example 1: sev1 — API outage from a bad deploy

```
14:23 UTC — alert: 5xx rate > 5% on /v1/users
14:24 UTC — engineer acknowledges, severity = sev1
14:26 UTC — commander named (CTO, the only one awake)
14:28 UTC — identified recent deploy at 14:22; rolled back
14:32 UTC — 5xx rate back to <0.1%; service restored
14:45 UTC — root cause: new migration script locked a table
15:30 UTC — permanent fix deployed (script rewritten + tested)
[+3 days] — post-mortem with action items
```

### Example 2: sev2 — slow database queries

```
09:15 UTC — alert: p95 latency on /api/orders > 2s (threshold 500ms)
09:17 UTC — engineer acknowledges, severity = sev2
09:25 UTC — investigated; suspect missing index after recent schema change
09:35 UTC — added index in staging, verified query plan
10:00 UTC — applied to production; latency back to <100ms
[+2 days] — post-mortem: lessons about index review in code review
```

## Anti-patterns

### ❌ No severity assigned

Without severity, you can't prioritize. "We'll figure it out"
isn't a strategy.

### ❌ No incident commander

Multiple responders with no coordinator = chaos. One person
must own: who decides what, who communicates what, who's
debugging.

### ❌ Mitigate without investigating

Restoring service is half the work. If you stop at "we rolled
back", the same deploy will land next week and break again.
Find the root cause.

### ❌ Blame people

"We should never have merged that PR" → wrong frame. The
system let the PR through. Why? No review? No test? No
rollback mechanism? Fix the system.

### ❌ Skip the post-mortem

"I already know what happened" → you don't. The team needs
shared understanding, and the action items prevent recurrence.

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/roadmap` | After post-mortem; tracks action items |
| `/validar` | After applying permanent fix; verifies regression test |
| `/auditar` | After sev1/sev2; project layout may have contributed |

## Failure modes

| Failure | Recovery |
|---------|----------|
| Severity too low assigned; impact bigger than expected | Re-classify to higher severity; re-page; ack to stakeholders |
| Commander overwhelmed; debugging instead of coordinating | Hand off commander role; senior takes over coordination |
| Mitigation restores service but no root cause known | Continue investigation in parallel; do NOT declare resolved |
| Post-mortem becomes a blame session | Reset the frame: "what about the system allowed this?"; re-run |
| Customer comms delayed >30 min | Default to over-communicating; even a brief "we're on it" buys trust |