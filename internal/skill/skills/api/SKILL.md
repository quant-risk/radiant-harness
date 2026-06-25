# Skill: api

> API design and operations: REST vs GraphQL vs gRPC, versioning,
> auth, rate limits, errors, observability. An API without a
> contract is a hope; an API without deprecation policy is a trap.

## Decision tree

```
Project starts (or pivots to API-first)
        │
        ▼
[Step 1] Protocol choice       ── REST | GraphQL | gRPC | mixed
        │
        ▼
[Step 2] Design rationale      ── docs/api/design-rationale.md
        │
        ▼
[Step 3] Contract              ── docs/api/contract.md
        │                          (OpenAPI / SDL / protobuf)
        ▼
[Step 4] Auth + authz          ── OAuth2 / mTLS / API key / JWT
        │
        ▼
[Step 5] Error shape           ── stable, versioned, documented
        │
        ▼
[Step 6] Pagination + filtering
        │
        ▼
[Step 7] Rate limits + quotas
        │
        ▼
[Step 8] Observability         ── metrics + traces + logs
        │
        ▼
[Step 9] Deprecation policy    ── timeline + sunset
        │
        ▼
[Step 10] Operations runbook   ── docs/api/operations-runbook.md
```

## Workflow

### Step 1: Protocol choice

| Protocol | Best for | Strengths | Weaknesses |
|----------|----------|-----------|------------|
| **REST** | Resource-oriented public APIs; CRUD; cacheable responses | Universally understood; HTTP semantics; easy caching | Over-fetching; many endpoints; versioning pain |
| **GraphQL** | Aggregating many resources; client-driven queries; mobile + web | Single endpoint; client gets what it asks for | Complexity (N+1, caching, query cost); tooling immaturity |
| **gRPC** | Service-to-service; low-latency; polyglot; streaming | Typed contract (protobuf); streaming; fast | Browser support requires grpc-web proxy; not human-readable |
| **WebSocket / SSE** | Real-time push; chat; live dashboards | Persistent connection; low overhead per message | Stateful; no HTTP semantics; harder to scale |
| **tRPC** | TypeScript-only service-to-service | Type-safe end-to-end; no schema | Not for cross-language; not for public APIs |

Decision factors:
- **Audience**: third-party devs → REST or GraphQL (universally known).
  Internal services → gRPC or tRPC.
- **Data shape**: simple CRUD → REST. Aggregations, mobile clients →
  GraphQL. Streaming or high RPS → gRPC.
- **Tooling**: REST has the best tooling. GraphQL has Apollo,
  Hasura, etc. gRPC has protoc + Buf.

### Step 2: Design rationale

`docs/api/design-rationale.md` answers, in plain text:

1. **Why this protocol?** Not "REST is popular" — what concrete
   property drove the choice?
2. **Why this auth model?** OAuth2? mTLS? API keys? Why this
   one over the alternatives?
3. **Why this pagination model?** Offset? Cursor? Page-based?
4. **Why this versioning scheme?** URL path? Header? Query param?
5. **What are the non-goals?** (e.g. "not a real-time API"; "no
   third-party consumers in v1")

This document prevents the "we built the wrong API" rework. It's
read by the next team that inherits the project.

### Step 3: Contract

`docs/api/contract.md` is the source of truth:

- **OpenAPI 3.x** for REST (or Swagger 2.0 if tooling requires)
- **GraphQL SDL** for GraphQL (with federation directives if used)
- **protobuf** + Buf module for gRPC
- **JSON Schema** for plain JSON APIs (rare but exists)

The contract MUST be:
- Versioned (every endpoint has a version; or the whole API is
  versioned, with semantic versioning)
- Lintable (use a linter: Spectral for OpenAPI, eslint-plugin-graphql,
  buf for protobuf)
- Generated into client SDKs (don't hand-write clients)
- Tested (contract tests / snapshot tests / schema diff in CI)

### Step 4: Auth + authz

Auth (who you are) and authz (what you can do) are different:

| Auth model | When | Trade-offs |
|------------|------|------------|
| **API keys** | Server-to-server; partner integrations | Simple; no expiry; need rotation strategy |
| **OAuth2 client credentials** | B2B; first-party SDK auth | Standard; tokens expire; need client_id/secret storage |
| **OAuth2 authorization code + PKCE** | Third-party apps acting on behalf of user | Full delegated auth; complex flows; consent UX |
| **mTLS** | Service-to-service in zero-trust network | Strong identity; requires cert management |
| **JWT** | Stateless auth between services | Hard to revoke; clock skew issues; payload size |

Authz:
- **Scopes / roles**: declared in the token; checked per endpoint.
- **ABAC**: attributes (user, resource, action, context); checked
  per request. More expressive; harder to debug.
- **ReBAC**: relationship-based (e.g. "is the user the owner of
  this resource?"). Modern but tooling-immature.

Pick auth BEFORE publishing. Migrating from API keys to OAuth is a
breaking change for every consumer.

### Step 5: Error shape

Stable, parseable, versioned. Example:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Field 'email' must be a valid email address",
    "details": [
      {
        "field": "email",
        "code": "invalid_format",
        "message": "must be a valid email"
      }
    ],
    "request_id": "req_abc123",
    "documentation_url": "https://api.example.com/errors/validation_failed"
  }
}
```

HTTP status codes (semantic):
- `200` / `204` — success
- `400` — client error (validation, malformed)
- `401` — not authenticated
- `403` — authenticated but not authorized
- `404` — resource not found
- `409` — conflict (duplicate, version mismatch)
- `422` — semantic validation failure
- `429` — rate limited (with `Retry-After` header)
- `500` — server error (your fault)
- `503` — temporarily unavailable (with `Retry-After`)

The error shape MUST NOT change between API versions. Adding fields
is OK; removing or renaming is a breaking change.

### Step 6: Pagination + filtering

| Pattern | Best for | Trade-offs |
|---------|----------|------------|
| **Offset + limit** | Small datasets; UI pages | Simple; slow for deep offsets; inconsistency under writes |
| **Cursor (opaque)** | Large datasets; feeds | Fast; consistent under writes; cursor is opaque |
| **Page-based (page=N)** | Documents; reports | Human-readable URLs; doesn't scale well |
| **Keyset** | Time-series; ordered data | Most efficient; complex cursors |

Default to **cursor-based** for any list endpoint that might grow
beyond a few hundred items. Document the cursor's stability
guarantee (does it survive writes?).

Filtering: declare which fields are filterable, and which operators
(`eq`, `in`, `gt`, `lt`, `contains`). Don't try to build a query
language.

### Step 7: Rate limits

Documented AND enforced. Per consumer tier:

| Tier | Requests/sec | Burst | Daily quota |
|------|--------------|-------|-------------|
| Free | 1 | 5 | 10k |
| Standard | 100 | 200 | 1M |
| Premium | 1000 | 2000 | 100M |
| Partner | 5000 | 10000 | Unlimited |

Limits in headers on every response:
- `X-RateLimit-Limit`: total budget
- `X-RateLimit-Remaining`: remaining in window
- `X-RateLimit-Reset`: epoch when window resets
- `Retry-After`: seconds to wait (on 429)

Enforcement options: token bucket, leaky bucket, sliding window,
fixed window. Token bucket is the common choice.

### Step 8: Observability

Three pillars: **metrics, traces, logs**.

**Metrics** (RED method for request-driven services):
- **R**ate: requests/sec per endpoint
- **E**rrors: error rate (5xx + 4xx for client errors)
- **D**uration: latency p50 / p95 / p99

Plus: saturation (CPU, memory, queue depth), traffic shape.

**Traces**: distributed tracing with W3C trace context. Every
request has a `trace_id` and a `span_id`; propagate to downstream
calls. Tools: OpenTelemetry, Jaeger, Zipkin, Datadog APM.

**Logs**: structured (JSON) with `request_id`, `trace_id`,
`user_id`, `endpoint`, `status`, `latency_ms`. **DO NOT log**:
request bodies (PII), auth headers (secrets), API keys, passwords.

A `request_id` appears in:
- Every log line for the request
- Every error response
- The `Server-Timing` response header
- Distributed trace metadata

### Step 9: Deprecation policy

For public APIs:

1. **Announce**: mark fields/endpoints as `@deprecated` in the
   OpenAPI / SDL. Add a `Sunset` HTTP header (RFC 8594) on
   responses.
2. **Document**: migration guide on the developer portal.
3. **Notify**: email + changelog + in-product banner for active
   consumers.
4. **Wait**: minimum 6 months before removal (12 for major
   partners).
5. **Sunset**: remove the endpoint; return `410 Gone` if hit
   before the removal date.

Breaking changes require a new API version (not just deprecation).
v1 stays online until v2 is GA + 6 months.

### Step 10: Operations runbook

`docs/api/operations-runbook.md`:

- **SLOs**: latency (p99 < 200ms), availability (99.9% / 99.99%)
- **Alerting**: page on call when SLO burns faster than budget
- **Dashboards**: per-endpoint RED; per-tenant rate-limit usage
- **Incident response**: rollback, feature flag, status page
- **On-call rotation**: who, when, escalation
- **Postmortem template**: what broke, why, how to prevent

## API-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Unversioned breaking change | Consumers break silently | Contract tests in CI; version every endpoint |
| No rate limit enforcement | One bad consumer DoSes everyone | Server-side limits; per-tenant quotas |
| PII in logs | Compliance incident (GDPR, CCPA) | Log only request_id + endpoint + status; redact bodies |
| Error string contains stack trace | Leaks internals; consumer can't parse | Stable error shape; debug info only in `details` |
| Auth migration surprise | Consumers can't log in | Communicate deprecation; dual-auth window; version |
| 5xx on validation error | Misleading; consumer thinks it's transient | Use 4xx for client errors; reserve 5xx for server faults |
| No pagination defaults | Consumer fetches 1M rows; OOM | Require pagination; default page size; max page size |

## Examples

### Example 1: public REST API (B2C, payment-adjacent)

```
Protocol: REST (OpenAPI 3.1)
Audience: public (third-party developers)
Scale:    medium (10k req/s peak)
Sensitivity: regulated (PCI scope subset)
Latency: p99 < 300ms

Contract:
  - OpenAPI 3.1; lint with Spectral
  - All endpoints versioned: /v1/, /v2/
  - Stable error shape across versions

Auth: OAuth2 client credentials + JWT bearer
Authz: scopes per endpoint; resource-level ownership checks

Limits:
  - Free: 10 req/s, 100k/day
  - Standard: 100 req/s, 10M/day
  - Premium: 1000 req/s, 1B/day
  - Enforced: token bucket per API key

Observability:
  - OpenTelemetry traces; W3C trace context propagated
  - Logs: JSON with request_id; bodies NEVER logged
  - Metrics: RED per endpoint; saturation per service

Deprecation: 12 months notice for breaking; sunset header;
             v1 stays online 6 months after v2 GA
```

### Example 2: internal gRPC mesh (microservices)

```
Protocol: gRPC (protobuf + Buf)
Audience: internal (other services in same org)
Scale:    large (500k req/s aggregate)
Sensitivity: PII (user data)
Latency: p99 < 50ms

Contract:
  - protobuf in git; Buf module with lint + breaking-change checks
  - Per-service semantic versioning; major bumps require migration
  - Generated Go / Rust / Python clients in the same monorepo

Auth: mTLS (service mesh: Istio / Linkerd)
Authz: service identity + per-method authorization policies

Limits:
  - Per-method rate limit in service mesh config
  - Bulkheads per upstream service

Observability:
  - Distributed tracing via service mesh
  - Metrics: RED per RPC method
  - Logs: structured with trace_id; never log payload bodies

Deprecation: 3-month notice (internal); old services run
             alongside new during migration
```

### Example 3: GraphQL gateway (mobile + web client)

```
Protocol: GraphQL (Apollo Federation)
Audience: internal (mobile app + web app)
Scale:    medium (5k req/s)
Sensitivity: PII
Latency: p99 < 150ms

Contract:
  - SDL in git; lint with eslint-plugin-graphql
  - Federation directives (@key, @requires, @provides)
  - Subgraph schemas per service; supergraph composed at build

Auth: JWT bearer from client; user identity in context
Authz: field-level directives (@auth); data-loader for batching

Limits:
  - Query cost analysis (graphql-query-complexity); reject > N
  - Per-client rate limit; depth limit; aliasing limits

Observability:
  - Apollo Studio / GraphQL Hive for query analytics
  - Per-resolver timing; N+1 detection
  - Logs: query hash + variables (no PII)

Deprecation: schema directives; minimum 3 months for breaking;
             changelog via Hive
```

## Anti-patterns

### ❌ Unversioned breaking changes

Removing a field, renaming a parameter, or changing a type
silently breaks every consumer. Version the API; contract tests
in CI catch drift before merge.

### ❌ Free-form error strings

"Something went wrong" forces consumers to string-match. Use a
stable error shape with codes (`validation_failed`, `not_found`,
`rate_limited`).

### ❌ No rate limits (or unenforced limits)

A single bad consumer (or your own buggy client) can DoS the API.
Limits must be enforced server-side, per consumer, with headers
on every response.

### ❌ Logging request bodies

PII, secrets, and PHI in logs is a compliance incident. Log
request_id, endpoint, status, latency. Redact everything else.

### ❌ Auth decided after launch

Migrating auth schemes is a breaking change for every consumer.
Decide auth BEFORE the API is published. Document it.

### ❌ "We'll add pagination later"

If your endpoint returns a list, it needs pagination on day 1.
Default cursor-based with a small page size. Max page size
enforced server-side.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Breaking change released | Revert; coordinate migration; communicate |
| Consumer overload (429s spiking) | Throttle at edge; notify consumer; review quota |
| Auth outage (token signing key rotated) | Dual-key window; document rotation procedure |
| PII leak in logs | Redact pipeline; rotate any exposed secrets; notify DPO |
| Rate limit too tight (legit users throttled) | Per-tier review; consumer-tier upgrade path |
| Contract drift between services | Generated clients; CI blocking merge on schema diff |
| Unversioned deprecation discovered | Pin version in client; freeze old API; version bump |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial API scoping (uses api inputs) |
| `/roadmap` | Track deprecations, major version timelines |
| `/security` | Auth model, secret handling, audit logs |
| `/setup-ci` | Contract tests, schema diff, breaking-change gates |
| `/evals` | Eval framework for API behaviour; golden-request regression |
| `/incident` | API outage; consumer-impacting regression; rollback |