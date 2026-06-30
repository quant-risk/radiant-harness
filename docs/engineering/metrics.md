---
name: metrics
description: Delivery metrics — Lead Time, Throughput, Continuous Delivery/Deployment maturity, and code quality (coverage, static analysis). Pull when reviewing flow or planning. Updated by /metricas.
alwaysApply: false
---

# Delivery Metrics

> Flow health: **Lead Time**, **Throughput** and **Continuous Delivery/Deployment**.
> Updated by `/metricas`. Use to **find bottlenecks**, not to rank people.

**Period:** <cycle / dates> · **Updated on:** <YYYY-MM-DD>

## Lead Time — time to production
> From start (spec / issue / 1st commit) to prod deploy. Report **median** and **p85**.

## Throughput — items completed in the cycle
> How many items reached "done"/prod in the period.

## Continuous Delivery / Deployment
| Practice                                   | Current state        | Gap to advance |
|--------------------------------------------|----------------------|----------------|
| Continuous Delivery (always deployable)    | yes / partial / no   | <…>            |
| Continuous Deployment (auto deploy)        | yes / partial / no   | <…>            |

## Code quality
> Traceable evidence of the **result**: coverage and static analysis. Trend, not isolated number.

### Coverage
| Scope              | Current | Minimum | Trend       |
|--------------------|---------|---------|-------------|
| Global             | <X%>    | <Y%>    | <↑ / → / ↓> |

### Static analysis
| Category               | Findings | Blocking | Trend       |
|------------------------|----------|----------|-------------|
| Type-check             | <n>      | <n>      | <↑ / → / ↓> |
| Complexity / smells    | <n>      | <n>      | <↑ / → / ↓> |
| Security (SAST)        | <n>      | <n>      | <↑ / → / ↓> |
