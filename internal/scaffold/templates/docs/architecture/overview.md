---
name: architecture-overview
description: System architecture across 5 axes + security and operational. Pull when working on architecture, infra, quality, observability, or security.
alwaysApply: false
---

# System Architecture

> **Consolidated** view of the system by 5 axes (+ security and operational). Each section is a
> **short summary + link** to detail (ADRs, context-map, diagrams, TESTING). Generated/updated
> in `/kickoff`. **Keep lean** — detail lives in linked docs, this is the map.

## 1. Tech stack
<Languages, frameworks, runtime, package management, target versions.>

## 2. Base architecture
<Style (modular monolith / services / serverless), layers (DDD), main bounded contexts.>

## 3. Infra
<Cloud/provider, environments (dev/stg/prod), deploy model, IaC, cost.>

## 4. Quality
<Test strategy (pyramid), minimum coverage, lint/format, static analysis (type-check/complexity/SAST), review policy.>

## 5. Observability
<Structured logs, metrics, tracing, alerts and SLO/SLI of the system.>

## 6. Security
<Authentication and authorization, controls and policies, data protection (PII/encryption),
compliance (LGPD/GDPR/…), secrets management.>

## 7. Operational
<Deploy and rollback, monitoring and alerts (who is paged), backup and recovery,
incident runbook. Links to Infra (3) and Observability (5).>
