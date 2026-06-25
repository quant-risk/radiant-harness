# Skill: security

> Threat modelling, secret management, auth, input validation,
> audit logging, incident response. Security is a property of
> the system; retrofitting it is 100x more expensive than
> designing it in.

## Decision tree

```
Project starts (handles sensitive data, exposed to untrusted input,
                or must meet a compliance regime)
        │
        ▼
[Step 1] Threat model             ── docs/security/threat-model.md
        │                              (STRIDE; reviewed BEFORE code)
        ▼
[Step 2] Data classification      ── what data, what sensitivity
        │
        ▼
[Step 3] Auth design              ── docs/security/auth-design.md
        │
        ▼
[Step 4] Secret management        ── docs/security/secret-management.md
        │
        ▼
[Step 5] Input validation         ── boundaries + sanitisation
        │
        ▼
[Step 6] Dependency hygiene       ── pinned, scanned, updated
        │
        ▼
[Step 7] Audit logging            ── security events captured
        │
        ▼
[Step 8] Incident response        ── docs/security/incident-response.md
        │                              (with tabletop exercises)
        ▼
[Step 9] Compliance check         ── SOC2 / ISO / PCI / HIPAA / GDPR
        │
        ▼
[Step 10] Continuous monitoring   ── SIEM, alerts, scans
```

## Workflow

### Step 1: Threat model

`docs/security/threat-model.md` answers:

1. **What are we protecting?** (assets: data, services, reputation)
2. **Who would attack it?** (adversaries: opportunistic, motivated,
   insider)
3. **How would they attack?** (attack surfaces: API, UI, network,
   supply chain)
4. **What mitigations do we have?** (existing controls)
5. **What's the residual risk?** (gaps to address)

STRIDE is a useful framework:

| Category | Threat | Example |
|----------|--------|---------|
| **S**poofing | Pretending to be someone else | Forged JWT, stolen session cookie |
| **T**ampering | Modifying data in transit or at rest | MITM, SQL injection |
| **R**epudiation | Denying an action without proof | Missing audit log |
| **I**nformation disclosure | Leaking data | PII in error messages, verbose stack traces |
| **D**enial of service | Disrupting service | Resource exhaustion, expensive queries |
| **E**levation of privilege | Gaining unauthorized access | IDOR, broken access control |

For each threat, document: likelihood, impact, existing mitigations,
gap, owner, deadline.

### Step 2: Data classification

Classify every data category:

| Class | Examples | Handling |
|-------|----------|----------|
| **Public** | Marketing copy, public docs | No special handling |
| **Internal** | Org chart, internal docs | Need-to-know basis |
| **Confidential** | Source code, business metrics | Encrypted at rest; access logged |
| **PII** | Name, email, phone, address | Encrypted; access logged; consent required |
| **PHI** | Health records | HIPAA: encrypted; access logged; BAA required |
| **PCI** | Card numbers, CVV | Never stored; tokenised via PCI-compliant vault |

The classification drives every downstream decision: encryption,
access control, retention, audit, residency.

### Step 3: Authentication + authorization

**Authentication** (authn): who you are.

| Method | Strengths | Weaknesses | When |
|--------|-----------|------------|------|
| **Password** | Universal | Weak defaults; reuse; phishable | Avoid as sole factor |
| **TOTP / SMS MFA** | Adds factor | SMS is sim-swappable; TOTP requires app | Standard second factor |
| **Passkey (WebAuthn)** | Phishing-resistant; convenient | Adoption uneven | Modern consumer apps |
| **OAuth2 / OIDC** | Standard; delegated | Implementation complexity | B2B; third-party integrations |
| **mTLS** | Strong service identity | Cert management overhead | Service-to-service in zero-trust |
| **SSO (SAML / OIDC)** | Centralised; IT controls | SSO provider becomes critical dependency | Enterprise customers |

**Authorization** (authz): what you can do.

- **RBAC** (role-based): "admin" can do X. Simple, common.
- **ABAC** (attribute-based): "user with attr X can do Y in
  context Z". Expressive but complex.
- **ReBAC** (relationship-based): "user is the owner of this
  resource". Modern; good for multi-tenant.

Authorization must be **enforced server-side**, not just in the UI.
"Users can't see the button" is not access control; "server returns
403 if the user lacks permission" is.

### Step 4: Secret management

**Secrets**: API keys, database passwords, signing keys, OAuth
client secrets, encryption keys, webhook secrets, third-party
credentials.

Rules:

| Rule | Why |
|------|-----|
| Never in git | Git history is forever; scanners exist |
| Never in env dumps | Logs often capture env; users share env |
| Never in CLI args | `ps aux` shows args; shell history logs them |
| Never in error messages | Stack traces are public when leaked |
| Short TTL where possible | Limits blast radius of a leak |
| Rotate on suspicion | Without proof; assume compromise |
| Distinct per environment | dev, staging, prod each have different secrets |

**Storage**: use a secret manager.

- Cloud: AWS Secrets Manager, GCP Secret Manager, Azure Key Vault
- Self-hosted: HashiCorp Vault, Bitnami Sealed Secrets (k8s)
- Local dev: `.env` (gitignored) or `direnv` + `.envrc` (gitignored)

**Access**: principle of least privilege. Service tokens should
have the minimum scopes they need, scoped to specific resources.
Admins should have short-lived credentials, not standing admin
access.

### Step 5: Input validation

Every input from outside the trust boundary must be validated.
The boundary is the **system edge** (API handler, message consumer,
file parser) — not deep in business logic.

Validation principles:

1. **Allow-list, not block-list**: define what's allowed, reject
   the rest. Block-lists miss new attacks.
2. **Validate type, length, range, format**: use a schema
   validator (zod, pydantic, validator, jsonschema).
3. **Size-limit everything**: request body size, file upload size,
   field length. Default deny.
4. **Sanitise for context**: HTML escaping for HTML output; SQL
   parameterisation for SQL; shell escaping for shell commands.
5. **Reject, don't fix**: if input doesn't match schema, return
   400. Don't try to coerce or "fix" bad input.

Common attack vectors:

| Vector | Mitigation |
|--------|------------|
| SQL injection | Parameterised queries; ORM; never string-concat SQL |
| XSS | Output encoding; CSP headers; sanitisation library |
| Command injection | Avoid shell execution; if needed, use argv arrays |
| Path traversal | Validate path components; resolve and check prefix |
| Deserialisation | Use safe formats (JSON, protobuf); never `pickle`/`eval` |
| SSRF | Validate URLs against allow-list; block internal IPs |
| XXE | Disable external entities in XML parsers |
| File upload | Validate MIME type AND content; store outside webroot |

### Step 6: Dependency hygiene

Supply chain attacks are real and increasing.

| Practice | Why |
|----------|-----|
| Pin versions (lockfiles) | Reproducible builds; no silent upgrades |
| Scan for CVEs | Dependabot / Renovate + Trivy / Snyk |
| Minimal dependency tree | Fewer things to break; faster builds |
| Verify integrity (hashes) | Detect tampering; use `go mod verify`, `npm ci` |
| Prefer maintained packages | Unmaintained deps are a future CVE |
| Audit new deps | Before adding, check maintainer, last release, vulns |

Update cadence:
- **Security patches**: within 24h of disclosure
- **Patch versions**: weekly
- **Minor versions**: monthly (review changelog)
- **Major versions**: scheduled; may have breaking changes

### Step 7: Audit logging

What to log:

| Event | Fields |
|-------|--------|
| Authentication | user_id, source IP, success/failure, timestamp, MFA method |
| Authorization failure | user_id, target resource, attempted action, reason |
| Privilege change | actor_id, target_user_id, before/after roles, timestamp |
| Secret access | actor_id, secret name, action (read/write/rotate) |
| Admin action | actor_id, action, target, before/after state |
| Data export | actor_id, dataset, row count, destination |
| Configuration change | actor_id, change, before/after |

Log structure:
- JSON (parseable; queryable)
- Tamper-evident (write-only; append-only; signed)
- Time-synced (NTP; UTC)
- Sent to a centralised system (SIEM, log aggregator)

**Never log**: passwords, API keys, full PAN, CVV, session tokens,
PII beyond what the access purpose requires, request bodies (in
general — log specific fields with care).

### Step 8: Incident response

`docs/security/incident-response.md`:

```
DETECT  → Automated alert, user report, external notification
TRIAGE  → Severity (Sev1 / Sev2 / Sev3 / Sev4); on-call paged
CONTAIN → Stop the bleeding (revoke creds, isolate host,
          disable endpoint, freeze account)
ERADICATE → Remove attacker access; patch the vulnerability;
          rotate compromised secrets
RECOVER  → Restore service; verify integrity; monitor
POSTMORTEM → What broke, how, why; corrective actions;
          owner + deadline for each
```

Severity ladder:

| Severity | Definition | Response time | Examples |
|----------|------------|---------------|----------|
| **Sev1** | Active breach; data exfil in progress; production down | Immediate (page on-call) | Credential leak in git; ransomware; active intrusion |
| **Sev2** | Confirmed vulnerability; potential impact | Within hours | High-CVEs in production deps; unauthorised access attempt |
| **Sev3** | Suspicious activity; needs investigation | Within 1 business day | Anomalous login pattern; failed access spikes |
| **Sev4** | Hygiene / best-practice gap | Within 1 week | Outdated TLS; missing audit log; weak password policy |

Tabletop exercise: quarterly. Walk through a scenario (compromised
admin credentials, leaked API key, DDoS). Test that the runbook
is reachable, contacts are current, and the team knows what to do.

### Step 9: Compliance

| Regime | Key controls | Audit cadence |
|--------|--------------|---------------|
| **SOC2** | Access control, change mgmt, monitoring, incident response | Annual |
| **ISO 27001** | ISMS, risk treatment, statement of applicability | Annual surveillance; re-cert every 3y |
| **PCI DSS** | Network segmentation, encryption, vulnerability scans | Quarterly ASV scans; annual ROC |
| **HIPAA** | PHI safeguards, BAA, breach notification (60d) | Annual risk assessment |
| **GDPR** | Lawful basis, data subject rights, DPIA, breach notification (72h) | Ongoing |
| **LGPD** | Similar to GDPR; Brazilian equivalent | Ongoing |

Compliance is a snapshot in time. Plan for the audit; don't wait
until the week before.

### Step 10: Continuous monitoring

Detection without response is just noise.

| Layer | Tooling | What it catches |
|-------|---------|-----------------|
| SAST (static) | Semgrep, CodeQL, SonarQube | Code-level vulns in CI |
| DAST (dynamic) | OWASP ZAP, Burp | Runtime vulns in deployed app |
| SCA (deps) | Dependabot, Renovate, Trivy, Snyk | Known CVEs in deps |
| Secret scan | gitleaks, trufflehog | Leaked credentials in commits |
| Container scan | Trivy, Grype | Vulns in container images |
| Runtime | Falco, Tetragon, cloud-native tools | Anomalous process / network behaviour |
| SIEM | Splunk, Elastic, Datadog | Aggregated security events |
| IDS / IPS | Snort, Suricata | Network attacks |

Wire all of these into the same alerting pipeline so on-call sees
one stream.

## Security-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Secret in git | Permanent leak; scanner bait | Pre-commit hook + CI gate; rotate on discovery |
| SQL injection | Full DB compromise | Parameterised queries; never string-concat SQL |
| IDOR (Insecure Direct Object Reference) | Horizontal privilege escalation | Per-resource authz check server-side |
| XSS | Session hijack; data theft | Output encoding; CSP; sanitisation |
| Missing auth on endpoint | Anonymous access to admin functions | Auth check on EVERY endpoint; deny by default |
| Outdated dependency with CVE | Known attack vector | Dependabot + weekly updates |
| No audit log | Can't detect or investigate incidents | Log security events from day 1 |
| Broken access control | Privilege escalation; data leak | Server-side authz; test for cross-tenant access |
| Overly permissive CORS | Cross-origin data leak | Allow-list specific origins; no `*` for credentials |
| No rate limiting | Brute force; DoS | Per-IP, per-account, per-endpoint limits |

## Examples

### Example 1: B2C fintech app (regulated)

```
Threat model:   public-internet, motivated adversary
Data:           PII + PCI scope subset
Compliance:     PCI DSS + SOC2 + GDPR
Deployment:     saas-public (multi-region)
Auth:           OAuth2 + WebAuthn (passkey)

Threats addressed:
  - Credential stuffing: rate limit + MFA required
  - Account takeover: passkey + device binding
  - Card data theft: tokenisation via PCI vault; PAN never stored
  - Insider threat: audit logs; least-privilege access

Secret management:
  - HashiCorp Vault; service tokens with TTL ≤ 1h
  - DB credentials rotated weekly; rotate on demand
  - No secrets in env files; only in vault

Input validation:
  - JSON Schema at API gateway; reject malformed
  - Request body size limit: 1MB default
  - File upload: MIME sniff + size + virus scan

Audit log:
  - All auth events; privilege changes; admin actions
  - Tamper-evident (write-only bucket); 7-year retention
  - SIEM with detection rules

Incident response:
  - Sev1: page CISO + on-call + legal (within 15 min)
  - PCI breach: notify acquirer within 24h
  - GDPR breach: notify DPA within 72h
```

### Example 2: internal admin tool (zero-trust)

```
Threat model:   zero-trust (assume breach)
Data:           confidential (internal business data)
Compliance:     SOC2
Deployment:     self-hosted (k8s)
Auth:           SSO via corporate IdP (SAML)

Threats addressed:
  - Stolen session cookie: short TTL (4h); re-auth for sensitive ops
  - Lateral movement: mTLS service-to-service; per-service authz
  - Insider data theft: audit log all data exports

Secret management:
  - Sealed Secrets for k8s; rotated by GitOps
  - DB credentials in Vault; injected as sidecar

Input validation:
  - All admin endpoints validate against OpenAPI schema
  - File upload restricted to specific MIME types; size cap

Audit log:
  - All admin actions; privilege grants; data exports
  - Centralised in SIEM; 1-year retention
```

### Example 3: healthcare SaaS (HIPAA)

```
Threat model:   public-internet, motivated adversary (medical data is valuable)
Data:           PHI (medical records)
Compliance:     HIPAA + SOC2 + state privacy laws
Deployment:     saas-private (single tenant per customer)
Auth:           SSO + MFA + WebAuthn

Threats addressed:
  - PHI breach: encryption at rest (AES-256); TLS 1.3 in transit
  - Wrong patient record: per-resource authz (patient_id binding)
  - Insider snooping: audit log all PHI access; anomaly detection

Secret management:
  - Cloud KMS for encryption keys; key rotation annual
  - Per-tenant encryption keys; customer-managed keys (CMK)

Input validation:
  - All API input schema-validated
  - HL7 / FHIR messages: validate against profile; sanitise

Audit log:
  - All PHI access: who, what patient, when, why (purpose-of-use)
  - 6-year retention; tamper-evident
  - Real-time anomaly detection (e.g. doctor accessing celebrity records)

Incident response:
  - HIPAA breach: notify HHS within 60d (large breaches: within 60d, media within 60d)
  - BAA with all sub-processors
```

## Anti-patterns

### ❌ "We'll add security later"

Security is a property of the architecture. Threat-model BEFORE
feature work. Catching a flaw at design costs 1x; at code 10x; in
production 100x.

### ❌ Secrets in code / env / logs

Git history is forever. Logs are searchable. Env dumps leak.
Use a secret manager with rotation. CI gate for secret scanning.

### ❌ Trusting client input

Any input from outside the trust boundary must be validated.
SQL injection, path traversal, XSS, command injection are all
"we trusted the input" bugs. Validate at the edge; reject if
invalid.

### ❌ Custom auth / crypto / session

Don't roll your own. Use vetted libraries (Argon2id for passwords,
JWT libraries for tokens, OPA / Casbin for authz). Custom crypto
is broken crypto.

### ❌ No audit logging

If you can't see it, you can't detect it. Log security events
from day 1: auth, authz failures, privilege changes, secret
access, admin actions.

### ❌ Standing admin access + unrotated credentials

Service tokens with admin scope and no expiry are breach bait.
Short TTLs; rotate regularly; revoke on departure.

### ❌ Security by obscurity

Custom binary protocols, undocumented endpoints, hidden admin
panels. Real attackers find them; you just made incident response
harder.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Secret leaked in git | Rotate immediately (assume compromised); purge history; post-mortem |
| Vulnerability CVE published | Triage within 24h; patch in 7d; communicate to customers if exploited |
| Active intrusion detected | Isolate affected systems; preserve evidence; engage IR firm; notify legal |
| Audit log tampering detected | Switch to write-only / append-only storage; investigate; rotate any exposed secrets |
| Compliance audit failure | Remediate gaps; document timeline; coordinate with auditor |
| Insider data theft suspected | Revoke access; preserve logs; engage legal + law enforcement |
| DDoS | Activate CDN / WAF rate limits; communicate via status page; post-mortem |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/api` | API auth design; auth on endpoints; rate limits |
| `/cli` | Secret handling in argv; auth in CLI tools |
| `/setup-ci` | SAST / SCA / secret scanning in CI |
| `/incident` | Severity ladder; on-call; rollback |
| `/data` | Encryption at rest; PII handling; retention |