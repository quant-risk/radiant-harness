---
name: pulse-example
description: Golden example — fictitious product taken through the pipeline end-to-end.
alwaysApply: false
---

# Golden Example — Pulse

Fictitious product (in-app feedback widget) built **with the pipeline itself** to prove the
end-to-end flow: **discovery** (vision/features) → **spec** (acceptance criteria) → **tasks**
→ **implementation** → **tests** → **eval**.

Run from repo root:

```bash
node scripts/audit-esteira.mjs examples/pulse        # structural conformity
node scripts/eval-spec-fidelity.mjs examples/pulse   # spec→code fidelity
node --test examples/pulse/src/feedback.test.mjs       # tests pass
```
