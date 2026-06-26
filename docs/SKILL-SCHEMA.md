# Skill Schema v2.0

**Version**: 2.0.0
**Status**: Current (v1.0.0-final)

This document describes the SKILL.md frontmatter format used by radiant-harness v1.0.0.

## Backwards Compatibility

v2.0 is backwards-compatible with v1.0 skills. New fields are optional.
A v1 skill with no `token_budget`, `context_tier`, or `lazy_load` will:
- Use `token_budget: auto` (estimated from content length)
- Be included in `context_tier: domain` (loaded when domain matches)
- Default `lazy_load: true`

---

## Schema

```yaml
# SKILL.md frontmatter (delimited by --- ... ---)
---
id: <kebab-case unique identifier>
version: "2.0"                       # v2.0 marker; omit for v1 compat
name: <display name>
description: <one-sentence description used in CONTEXT.md>
domain:
  - <domain>                         # one or more: backend, frontend, ml, finance,
                                     #   blockchain, ops, systems, science, general
tier:
  - <tier>                           # one or more: trivial, feature, architecture, product
tags:
  - <tag>                            # arbitrary; used for --skills=<tag> filter

# v2.0 additions (all optional):
token_budget: <N>                    # max tokens this skill may consume in context.
                                     #   "auto" = estimate from content length.
                                     #   Enforced by the assembler when over-budget.
context_tier: <tier_name>           # which context assembly tier includes this skill.
                                     #   "core" = always loaded (nova-feature, validar, adr)
                                     #   "domain" = loaded when domain matches
                                     #   "tier" = loaded when work tier matches
                                     #   "explicit" = only when requested via --skills=<id>
lazy_load: <bool>                   # default true. false = load full body, not just frontmatter.
                                     #   Set false for tiny skills (<100 tokens) or
                                     #   skills the assembler can't summarize well.
---
```

## Field Reference

### Required

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique kebab-case identifier. Used in `--skills=<id>` flag. |
| `name` | string | Human-readable display name. |
| `description` | string | One sentence. Shown in CONTEXT.md instead of full skill body. |
| `domain` | list | Domains where this skill is relevant. |
| `tier` | list | Work tiers where this skill is relevant. |

### Optional (v2.0)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | string | — | Set to `"2.0"` to enable strict v2 validation. |
| `tags` | list | `[]` | Arbitrary tags for `--skills=<tag>` filter. |
| `token_budget` | int\|"auto" | `"auto"` | Max tokens for this skill's content in context. |
| `context_tier` | string | `"domain"` | When to auto-include this skill. |
| `lazy_load` | bool | `true` | Whether to defer body loading. |

---

## Context Tiers

| `context_tier` | When loaded |
|---------------|-------------|
| `core` | Always — regardless of domain or tier |
| `domain` | When project domain matches `domain:` list |
| `tier` | When work tier matches `tier:` list |
| `explicit` | Only via `--skills=<id>` or `--skills=<tag>` |

Skills with `context_tier: explicit` are never auto-included; they must be
explicitly requested. Good for niche skills (e.g., `solidity-audit`) that
would add noise to most projects.

---

## Token Budget Rules

1. `auto`: estimated as `ceil(len(description) / 4) + 20` tokens.
2. If the assembler would exceed the total budget, it drops skills in reverse
   priority order (explicit → tier → domain → core).
3. Core skills are never dropped.
4. When a skill is dropped, its `id` appears in the "skills omitted" line of CONTEXT.md.

---

## Examples

### v1 skill (backwards-compatible)
```yaml
---
id: nova-feature
name: Nova Feature
description: Start any new feature with clean spec, tests, and handoff checklist.
domain:
  - general
tier:
  - feature
  - architecture
  - product
tags:
  - core
  - feature
---
```

### v2 core skill
```yaml
---
id: nova-feature
version: "2.0"
name: Nova Feature
description: Start any new feature with clean spec, tests, and handoff checklist.
domain:
  - general
tier:
  - feature
  - architecture
  - product
tags:
  - core
token_budget: 150
context_tier: core
lazy_load: true
---
```

### v2 domain-specific skill
```yaml
---
id: finance-risk
version: "2.0"
name: Finance Risk Assessment
description: IFRS9 ECL, VaR, Basel compliance checks for financial software.
domain:
  - finance
tier:
  - feature
  - architecture
tags:
  - finance
  - risk
token_budget: 300
context_tier: domain
lazy_load: true
---
```

### v2 explicit-only skill (never auto-loaded)
```yaml
---
id: solidity-audit
version: "2.0"
name: Solidity Audit
description: Smart contract security audit: reentrancy, overflow, access control.
domain:
  - blockchain
tier:
  - architecture
tags:
  - audit
  - security
token_budget: 500
context_tier: explicit
lazy_load: false
---
```

---

## Validation

Run `radiant validate-file skills/<id>/SKILL.md` to check schema compliance.

Errors reported:
- Missing required fields
- Unknown `context_tier` value
- `token_budget` exceeded by actual content length
- `lazy_load: false` on a skill with >300 tokens body (warning)
