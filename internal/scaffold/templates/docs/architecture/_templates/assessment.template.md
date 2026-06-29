---
name: assessment
description: As-is portrait (brownfield). Pull when mapping or evaluating the codebase.
alwaysApply: false
---

# Assessment (as-is) — <project name>

> Map of the current state of a project **already running** (brownfield).
> Objective: understand what exists before proposing changes. Photograph, don't judge yet.

## Overview
<What the system does today, is in production, how many users/services, age of the code.>

## Detected stack
| Layer              | Current technology          | Notes |
|--------------------|-----------------------------|-------|
| Language/runtime   | <…>                         |       |
| Frameworks         | <…>                         |       |
| Persistence        | <…>                         |       |
| Infra/deploy       | <…>                         |       |

## Current architecture
<Real style (monolith, services, big ball of mud?), layers, dangerous couplings.>

## Maturity across 5 axes
| Axis           | Current state                    | Gap vs SDD standard     | Risk |
|----------------|----------------------------------|------------------------|------|
| Tech stack     | <…>                              | <…>                    | low/med/high |
| Architecture   | <…>                              | <…>                    |      |
| Infra          | <…>                              | <…>                    |      |
| Quality        | <tests? coverage? static analysis?> | <…>                 |      |
| Observability  | <logs/metrics/tracing/SLO?>      | <…>                    |      |

## Main debts and risks
1. <biggest risk>

## Historical decisions to capture as ADR
- [ ] <e.g. "use of X as database" — why it was chosen, if it still holds>
