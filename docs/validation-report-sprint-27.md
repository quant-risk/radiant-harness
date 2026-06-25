# Sprint 27 validation — 4 domain skills (api, security, blockchain, iot)

**Commit (this report):** TBD
**Re-validates:** `66af799` (Sprint 26 — 3 skills)
**Version:** v0.6.3 (in source)

## What shipped

Four new domain-specific skills bundled with the CLI:

| Skill | Lines | Coverage |
|-------|-------|----------|
| `api` | 540 | REST/GraphQL/gRPC selection, versioning, auth, rate limits, errors, observability, deprecation |
| `security` | 601 | Threat modelling (STRIDE), secret management, authn/authz, input validation, audit logging, incident response, compliance |
| `blockchain` | 567 | Chain selection, smart-contract architecture, threat model, audit, gas, upgrade patterns, oracle integration, monitoring |
| `iot` | 585 | Hardware/firmware, MCU + connectivity trade-offs, power budget, OTA, secure boot, key provisioning, failure-mode testing |

**Total: 2293 lines** across 8 files (4 frontmatter.yaml + 4 SKILL.md).

## Iteration discipline

**Zero issues caught in this sprint.** All 4 new skills validated
cleanly on the first run of `TestAllBundledSkillsValidateCleanly`.
The schema discipline established in Sprint 26 (use only
`string, number, enum, object, path` for input types) carried
over. First-pass green across 28 skills.

## Skill design consistency

Each skill follows the same template:

- **frontmatter.yaml** (130-150 lines): name, version, when_to_use,
  inputs (typed), outputs (artifact paths), gates (release-blocking),
  context_provides, related_skills, anti_patterns, author, license
- **SKILL.md** (400-460 lines): decision tree, step-by-step workflow,
  tables of patterns, 2-3 worked examples, anti-patterns (❌),
  failure modes (table), related skills

## Domain coverage (post-Sprint 27)

| Domain | Skill |
|--------|-------|
| Product/process | nova-feature, nova-product, kickoff, roadmap, handoff, incident |
| Discovery/design | clarificar, validar, mapear, diagramar, adr, metricas |
| Quality/correctness | auditar, evals, revisar-pr, camada-agentica |
| Infrastructure | setup-ci, integracoes |
| **Domain — Mobile** | mobile |
| **Domain — Data** | data |
| **Domain — Frontend** | frontend |
| **Domain — ML** | ml |
| **Domain — Game** | game |
| **Domain — CLI** | cli |
| **Domain — API** | api (NEW — REST/GraphQL/gRPC design) |
| **Domain — Security** | security (NEW — threat modelling + compliance) |
| **Domain — Blockchain** | blockchain (NEW — smart contracts + web3) |
| **Domain — IoT** | iot (NEW — embedded systems + edge) |

**10 domain skills total.** Future candidates: `radiant-os` (operating
systems), `radiant-devops`, `radiant-quantum`, `radiant-robotics`,
`radiant-ar-vr`, `radiant-audio`.

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| `TestAllBundledSkillsValidateCleanly` | **28/28 skills pass** (was 24) |
| Tests | **337 PASS, 0 FAIL** |
| Data races | **0** |
| Cross-compile | **6/6** |

## Final tally (post-everything)

- **21 CLI commands** + **28 bundled skills** (was 24, +4) + **1 open MIT schema spec**
- **337 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**

## Stopping point

This is a strong stopping point. **28 skills** covering the full
project lifecycle:

- **6 process** skills (kickoff through incident)
- **6 discovery/design** skills (clarify through ADR)
- **4 quality/correctness** skills (audit through camada-agentica)
- **2 infrastructure** skills (setup-ci, integracoes)
- **10 domain** skills (mobile, data, frontend, ml, game, cli,
  api, security, blockchain, iot)

That's a comprehensive skill catalog for shipping a wide range
of products end-to-end. Each domain skill has:
- A signed-off architecture pattern
- Step-by-step workflow
- Topic-specific tables
- 2-3 worked examples
- Anti-patterns
- Failure modes
- Cross-references to other skills

Remaining candidates:
- More domain skills (`os`, `devops`, `quantum`, `robotics`, `ar-vr`, `audio`)
- Tag v0.6.3 for real via `radiant release v0.6.3 --interactive`