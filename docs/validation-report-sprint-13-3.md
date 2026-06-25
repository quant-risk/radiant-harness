# Validation Report — Sprint 13 third batch (v0.4.8)

**Date:** 2026-06-25
**Version:** 0.4.8
**Commit under validation:** (pending — this commit)
**Sprint:** 13 — PR + Multi-agent Views (CI scaffold)
**Scope:** `radiant setup-ci` CLI + 3 provider templates + 6 tests.

---

## 1. Build hygiene

```
$ go build ./...
(clean)

$ go vet ./...
(clean)

$ gofmt -l .
(clean)
```

**Result:** ✅ Pass.

## 2. Race-detector tests

```
$ CGO_ENABLED=0 go test ./... -count=1 -race -timeout=180s
?   	github.com/quant-risk/radiant-harness/internal         [no test files]
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.477s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  2.082s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.922s
ok  	github.com/quant-risk/radiant-harness/internal/harness    5.742s
ok  	github.com/quant-risk/radiant-harness/internal/llm        7.050s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.274s
ok  	github.com/quant-risk/radiant-harness/internal/quality    2.499s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   3.428s
ok  	github.com/quant-risk/radiant-harness/internal/skill      1.620s
ok  	github.com/quant-risk/radiant-harness/internal/spec       1.309s
```

**Total:** 260 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build ... -o dist/radiant-linux-amd64
GOOS=linux   GOARCH=arm64 go build ... -o dist/radiant-linux-arm64
GOOS=darwin  GOARCH=amd64 go build ... -o dist/radiant-darwin-amd64
GOOS=darwin  GOARCH=arm64 go build ... -o dist/radiant-darwin-arm64
GOOS=windows GOARCH=amd64 go build ... -o dist/radiant-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build ... -o dist/radiant-windows-arm64.exe
✓ Release binaries in dist/
```

| Target | Status |
|---|---|
| linux/amd64 | ✅ |
| linux/arm64 | ✅ |
| darwin/amd64 | ✅ |
| darwin/arm64 | ✅ |
| windows/amd64 | ✅ |
| windows/arm64 | ✅ |

**Result:** ✅ 6/6 targets build clean.

## 4. End-to-end — all 3 providers

### Provider 1: GitHub Actions (default)

```
$ radiant setup-ci
  ✓ wrote .github/workflows/esteira.yml

  Next steps:
    1. Review the generated file — verify the gates match your project.
    2. Set the required secrets in your CI provider:
       - RADIANT_API_KEY
       - GITHUB_TOKEN
    3. Push to trigger the first run.
```

- Triggers: PR + push to main/master ✓
- Gates: validate, audit, tests, build ✓
- Secrets: `${{ secrets.RADIANT_API_KEY }}` ✓

### Provider 2: GitLab CI

```
$ radiant setup-ci --provider=gitlab
  ✓ wrote .gitlab-ci.yml
```

- Two stages (`radiant`, `build`) ✓
- 4 jobs: radiant-validate, radiant-audit, tests, build ✓
- Secrets: `$RADIANT_API_KEY` (GitLab CI/CD variables) ✓

### Provider 3: CircleCI

```
$ radiant setup-ci --provider=circleci
  ✓ wrote .circleci/config.yml
```

- Single job, docker image (cimg/go:1.22) ✓
- All 4 gates inline ✓
- Workflow `radiant` triggers the job ✓

### Edge cases

| Case | Behaviour |
|------|-----------|
| Re-run on existing file | `Error: .github/workflows/esteira.yml already exists; pass --output=<new-path> or remove it first` |
| `--output=custom.yml` | Writes to custom path; default location untouched |
| `--provider=bogus` | `Error: unknown provider "bogus" — choose: github | gitlab | circleci` |
| `--model=gpt-4o` (GitHub) | Adds `radiant validate --model gpt-4o` to validate step |

**Result:** ✅ All 3 providers + 4 edge cases work.

## 5. Safety guarantee verified

The setup-ci skill's anti-pattern: "Hardcoding secrets. Always use
the CI provider's secret store." Verified by:

- `TestNoHardcodedSecretsInCITemplates` — none of the 3 templates
  contain patterns like `sk-`, `key-`, `api_key=`, `apikey=`.
- All secrets are referenced via the provider's secret store:
  - GitHub: `${{ secrets.RADIANT_API_KEY }}`
  - GitLab: `$RADIANT_API_KEY` (CI/CD variable)
  - CircleCI: context-based (no literal in template; configured
    via CircleCI UI)

## 6. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | 240 |
| Sprint 13 batch 1 | +5 | 245 |
| Sprint 13 batch 2 | +9 | 254 |
| **Sprint 13 batch 3** | **+6** | **260** |

Sprint 13.3 tests:

- `TestRenderGitHubActionsHasGates` — all required gates + actions
  present + secret reference correct.
- `TestRenderGitHubActionsRespectsModel` — `--model=gpt-4o` flows
  into the validate step.
- `TestRenderGitLabCIHasGates` — stages, jobs, secret reference.
- `TestRenderCircleCIHasGates` — version, image, gates.
- `TestCISecretsForProviders` — per-provider secret list matches
  the documented set.
- `TestNoHardcodedSecretsInCITemplates` — safety guard: no banned
  patterns in any of the 3 templates.

All 6 pass in `-race` mode.

## 7. Iteration discipline recap

First build attempt failed with `modelArg declared and not used`
in `renderGitLabCI`. The variable was declared but the format
string didn't reference it. Caught by `go build` before the binary
was produced. Fix: pass `modelArg` as the trailing argument to
`fmt.Sprintf`. **Lesson:** when introducing an optional format
substitution, immediately check that the closing `)` passes it.

## 8. Decisions

- ✅ Sprint 13 third batch is **READY TO MERGE** at v0.4.8.
- ✅ No `--force` flag for setup-ci — overwriting an existing CI
  config silently would be dangerous. User must opt-in with
  `--output=<new-path>` or remove the file first.
- ✅ All 4 radiant gates (validate, audit, tests, build) included
  by default. User can edit the file to remove gates they don't
  want — the template is a starting point, not a constraint.
- ✅ `--model` is optional; defaults to "no explicit model" (the
  validate command picks up its default from config).

## 9. End-to-end flow now complete (11 steps)

```
1. radiant product "..."          ← Lean Inception (v0.4.4)
2. radiant spec "<feature>"       ← AC→test mapping (v0.4.2)
3. radiant run specs/<NNNN>       ← implementation (v0.3.x)
4. radiant adr "<decision>"       ← Nygard ADR (v0.4.3)
5. radiant diagramar <level>      ← C4 Mermaid (v0.4.3)
6. radiant integrations list      ← MCP discovery (v0.4.5)
7. radiant handoff --feature=...  ← session pause (v0.4.2)
8. radiant update [--force]       ← skill refresh (v0.4.3)
9. radiant views --agent=<list>   ← native agent views (v0.4.6)
10. radiant review-pr <spec>      ← PR review scaffold (v0.4.7)
11. radiant setup-ci              ← CI workflow (v0.4.8) ← NEW
```

## 10. What Sprint 13 will continue to tackle

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 13.4 | `radiant camada-agentica` | `camada-agentica` | Generate the agentic layer config. |
| 13.5 | `radiant evals` | `evals` | Run AC→test coverage metrics. |

After Sprint 13, the radiant CLI is feature-complete against the
original HARNESS-PLAN.md scope.