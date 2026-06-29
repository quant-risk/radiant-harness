//go:build !light_only

// Package main — ci.go: CI workflow generation for GitHub Actions,
// GitLab CI, and CircleCI. Used by `radiant setup-ci`.
//
// Each renderer returns a self-contained YAML with all five gates
// (validate, audit, security, tests, build) wired in. Secrets are
// referenced via the provider's secret store (${{ secrets.X }} /
// $VARIABLE / context.env), never hardcoded.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runSetupCI generates the CI workflow for the chosen provider.
func runSetupCI(provider, outPath, model string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if outPath == "" {
		switch provider {
		case "github":
			outPath = ".github/workflows/esteira.yml"
		case "gitlab":
			outPath = ".gitlab-ci.yml"
		case "circleci":
			outPath = ".circleci/config.yml"
		default:
			return fmt.Errorf("unknown provider %q — choose: github | gitlab | circleci", provider)
		}
	}

	var body string
	switch provider {
	case "github":
		body = renderGitHubActions(model)
	case "gitlab":
		body = renderGitLabCI(model)
	case "circleci":
		body = renderCircleCI(model)
	default:
		return fmt.Errorf("unknown provider %q — choose: github | gitlab | circleci", provider)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(outPath); err == nil {
		// Refuse to overwrite — the user must explicitly --force or
		// pick a different path. Existing CI configs are precious.
		return fmt.Errorf("%s already exists; pass --output=<new-path> or remove it first", outPath)
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Next steps:\n")
	fmt.Printf("    1. Review the generated file — verify the gates match your project.\n")
	fmt.Printf("    2. Set the required secrets in your CI provider:\n")
	for _, s := range ciSecretsFor(provider) {
		fmt.Printf("       - %s\n", s)
	}
	fmt.Printf("    3. Push to trigger the first run.\n")
	return nil
}

// ciSecretsFor returns the list of secret names that the
// generated workflow references. Used by runSetupCI to print
// a helpful "set these secrets" reminder.
func ciSecretsFor(provider string) []string {
	common := []string{"RADIANT_API_KEY"}
	switch provider {
	case "github":
		return append(common, "GITHUB_TOKEN")
	case "gitlab":
		return append(common, "GITLAB_TOKEN")
	case "circleci":
		return append(common, "CIRCLE_TOKEN")
	default:
		return common
	}
}

// renderGitHubActions produces a .github/workflows/esteira.yml
// that runs validate → audit → security → tests → build on every
// PR (5 gates; Sprint 17 added `radiant security`).
// RADIANT_API_KEY is referenced via secrets, not hardcoded.
func renderGitHubActions(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("          radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`name: radiant-esteira

on:
  pull_request:
    branches: [main, master]
  push:
    branches: [main, master]

jobs:
  radiant-gates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Install radiant
        run: go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
      - name: Validate (spec/code alignment)
        env:
          RADIANT_API_KEY: ${{ secrets.RADIANT_API_KEY }}
        run: |
%s          radiant validate
      - name: Audit (project layout conformity)
        run: radiant audit
      - name: Security (hardcoded secrets + sensitive file perms)
        run: radiant security --fail-on-warning
      - name: Tests
        run: go test ./... -count=1 -race
      - name: Build
        run: go build ./...
`, modelArg)
}

// renderGitLabCI produces a .gitlab-ci.yml with the same five
// gates. Secrets via CI/CD variables (the GitLab idiom).
func renderGitLabCI(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("        radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`stages:
  - radiant
  - build

radiant-validate:
  stage: radiant
  image: golang:1.22
  variables:
    RADIANT_API_KEY: $RADIANT_API_KEY
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant validate%s

radiant-audit:
  stage: radiant
  image: golang:1.22
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant audit

radiant-security:
  stage: radiant
  image: golang:1.22
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant security --fail-on-warning

tests:
  stage: build
  image: golang:1.22
  script:
    - go test ./... -count=1 -race

build:
  stage: build
  image: golang:1.22
  script:
    - go build ./...
`, modelArg)
}

// renderCircleCI produces a .circleci/config.yml with the same
// five gates. Secrets via context (the CircleCI idiom).
func renderCircleCI(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("          radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`version: 2.1

jobs:
  radiant-esteira:
    docker:
      - image: cimg/go:1.22
    steps:
      - checkout
      - run:
          name: Install radiant
          command: go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
      - run:
          name: Validate (spec/code alignment)
          command: |
%s            radiant validate
      - run:
          name: Audit (project layout conformity)
          command: radiant audit
      - run:
          name: Security (hardcoded secrets + sensitive file perms)
          command: radiant security --fail-on-warning
      - run:
          name: Tests
          command: go test ./... -count=1 -race
      - run:
          name: Build
          command: go build ./...

workflows:
  version: 2
  radiant:
    jobs:
      - radiant-esteira
`, modelArg)
}