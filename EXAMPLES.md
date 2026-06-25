# Examples

End-to-end worked examples for the radiant CLI. Every example
runs against a real fixture in `internal/scaffold/templates/examples/`.

---

## Pulse — feedback collector (the canonical example)

`internal/scaffold/templates/examples/pulse/` is a worked SDD
project: a tiny "feedback collector" SaaS that captures user
feedback, stores it in a JSON file, and serves it via HTTP.

It demonstrates every command in the CLI:

```bash
cd internal/scaffold/templates/examples/pulse

# Lean Inception artifact (docs/product/)
cat docs/product/vision.md
cat docs/product/features.md

# Feature spec (specs/)
cat specs/0001-collect-feedback/spec.md
cat specs/0001-collect-feedback/tasks.md

# Implementation (src/)
cat src/feedback.mjs
cat src/feedback.test.mjs
```

The test file is the canonical example of "every AC has a test":
4 ACs in spec.md, 4 tests in feedback.test.mjs, 1:1 mapping.

### Running radiant against Pulse

```bash
# validate
radiant validate specs/0001-collect-feedback --gates

# measure fidelity
radiant evals

# audit
radiant audit
```

Expected output: 100% fidelity on Pulse because the test file
explicitly covers every AC.

---

## End-to-end walkthrough (from scratch)

This is the canonical first-day-with-radiant flow. Run in a
fresh empty directory:

### 1. Initialize

```bash
mkdir my-app && cd my-app
radiant init . --all --yes
```

Produces:
- `AGENTS.md` — universal project index
- `.radiant-harness/skills/` — 17 bundled skills
- `state.md` — initial session state

### 2. Discover the product

```bash
radiant product "API observability for small dev teams" --mvp-weeks=6
```

Produces:
- `docs/product/inception.md` — 6-phase Lean Inception template
- `docs/product/personas.md` — 3 persona slots

Fill in the sections following the nova-product skill (it's the
canonical "how to do a Lean Inception" guide).

### 3. Cut the MVP

After the inception, choose 3-5 features for the MVP:

```markdown
## MVP cut (in docs/product/inception.md)

The 3-7 features we ship first (in priority order):

1. **HTTP request tracing** — covers "Solo backend dev" persona's top job
2. **Latency percentile dashboards** — covers p95 < 200ms requirement
3. **Error rate alerts** — covers on-call engineer's top job
```

### 4. Spec the first feature

```bash
radiant spec "HTTP request tracing with p95 latency dashboard" \
  --tier=feature \
  --ac="AC1: every HTTP request is recorded with method+path+status+duration" \
  --ac="AC2: dashboard shows p50, p95, p99 latency for the last hour" \
  --task="1:Add OpenTelemetry SDK" \
  --task="2:Implement middleware" \
  --task="3:Build dashboard endpoint" \
  --gate="go test ./tracing/..." \
  --gate="go build ./..." \
  --covers="1:AC1,AC2" \
  --covers="2:AC1,AC2" \
  --covers="3:AC2"
```

Produces:
- `specs/0001-http-tracing/spec.md` with 2 ACs
- `specs/0001-http-tracing/tasks.md` with 3 tasks + 2 gates
- Coverage check: every AC must appear in at least one Covers cell

### 5. Implement (LLM-driven)

```bash
# configure the LLM
radiant config --provider=openrouter --model=anthropic/claude-sonnet-4.5

# run the implementation (the LLM drives this)
radiant run specs/0001-http-tracing --model=anthropic/claude-sonnet-4.5
```

The CLI invokes the LLM in 4 phases: planner → implementer →
correct → validator. Each phase uses a different system prompt
loaded from the bundled skills.

### 6. Verify

```bash
# static UAT (spec/code/tests alignment)
radiant validate specs/0001-http-tracing --gates

# PR review scaffold
radiant review-pr specs/0001-http-tracing --run-gates -o pr-review.md

# AC→test coverage
radiant evals

# project-wide audit
radiant audit
```

### 7. CI

```bash
radiant setup-ci --provider=github
```

Produces `.github/workflows/esteira.yml` with 4 gates: validate,
audit, tests, build.

### 8. Pause between sessions

```bash
radiant handoff \
  --feature=0001-http-tracing \
  --tier=feature \
  --next-command="radiant run specs/0001-http-tracing" \
  --note="implemented AC1+AC2; tests passing; ready for AC3"
```

Writes `.radiant-harness/state.md` atomically. Next session starts
with `radiant state` to see the resume point.

### 9. Cut a release

```bash
radiant release v0.1.0 --dry-run   # preview
radiant release v0.1.0             # actually cut
```

The release command runs:
1. Pre-flight (clean tree)
2. Version validation
3. Quality gates (build/vet/fmt/test-race)
4. Version bump in cmd/radiant/main.go
5. Cross-compile (6/6 targets)
6. Commit + git tag

---

## MCP server (for agents that prefer it)

```bash
# start the server (stdio transport)
radiant mcp serve
```

Tools exposed:
- `radiant_spec`
- `radiant_adr`
- `radiant_product`
- `radiant_evals`
- `radiant_audit`
- `radiant_release` (dry-run only, for safety)

Configure your MCP client to point at `radiant mcp serve`. See
the `radiant-mcp.json` example in this directory.

---

## Real-world scenario: greenfield B2B SaaS

This is the pattern most radiant users will follow:

```
Week 0:  Lean Inception via radiant product
Week 1:  MVP feature specs via radiant spec
Week 2-6: Implementation via radiant run (per feature)
Week 6:  MVP release via radiant release v0.1.0
Week 7-12: Growth features (radiant spec + radiant run)
Week 12: 0.2.0 release via radiant release v0.2.0
...
```

See `internal/scaffold/templates/examples/pulse/` for the
smallest possible worked example.

---

## See also

- [README](../README.md) — overview
- [INSTALL](../INSTALL.md) — installation
- [Methodology merge report](METHODOLOGY-MERGE-FINAL.md) — context
- [Skill schema](SKILL-SCHEMA.md) — open spec for new skills