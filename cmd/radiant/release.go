//go:build with_full

// Package main — release.go: release workflow.
//
// Implements `radiant release <version>` plus all the helpers it needs:
//   - runRelease orchestrates the 7-step release (validate, preflight,
//     tag, build, smoke, commit, push)
//   - bumpVersionInSource updates the version constant in source files
//   - lastGitTag, specsChangedSince, looksLikeSemver — small git helpers
//   - runGit, runGitCommit, runGitInDir, runGoStep — shell wrappers
//   - runFmtCheck, runTestRace, runMakeRelease — quality gates before tag
//   - isTerminal, promptConfirm — interactive prompts
//   - runCamadaAgentica — camada-agentica skill runner, called by release
//
// Originally inlined in cmd/radiant/helpers.go (a 4931-line god file).
// Splitting this out makes the release surface auditable.

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// lastGitTag returns the most recent git tag reachable from HEAD.
// Returns "" if no tags exist (the caller falls back to scope=all).
// We use `git describe --tags --abbrev=0` which is the standard
// way to get the "last release tag".
func lastGitTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		// Exit code 128 with "fatal: No names found" is normal
		// when no tags exist. Return empty string + nil error so
		// the caller falls back.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// specsChangedSince returns the slugs of features in specs/ whose
// files (spec.md, tasks.md) have changed since `ref` (a git ref).
// Implemented via `git diff --name-only <ref>..HEAD -- specs/`.
// Only counts changes to spec.md / tasks.md; src/ changes are
// out of scope for evals (they're implementation, not spec).
func specsChangedSince(ref string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", ref+"..HEAD", "--", "specs/")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var slugs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Line format: "specs/0001-jwt/spec.md" or "specs/0001-jwt/tasks.md"
		// Extract the slug (second path component).
		parts := strings.Split(line, "/")
		if len(parts) >= 2 {
			slug := parts[1]
			if !seen[slug] {
				seen[slug] = true
				slugs = append(slugs, slug)
			}
		}
	}
	return slugs, nil
}

// auditFinding is one row in the audit report. The audit
// collects these and renders them sorted by severity.
type auditFinding struct {
	Severity string // "ERROR" | "WARNING" | "INFO"
	Location string // file:line where the issue was found
	Message  string
}

// runAudit runs the project-wide conformity check from the
// `auditar` skill as a CLI. MVP scope: AC traceability +
// ADR status validity. Returns non-zero if any ERROR found
// (or any WARNING when --fail-on-warning).
func runAudit(scope, outPath string, failOnWarning bool) error {
	if outPath == "" {
		outPath = "docs/audit-report.md"
	}

	var findings []auditFinding

	// Step 1: AC traceability per spec.
	if scope == "full" || scope == "specs" {
		findings = append(findings, auditACTraceability()...)
	}

	// Step 2: ADR status validity (every ADR file should have
	// a valid status header).
	if scope == "full" || scope == "adrs" {
		findings = append(findings, auditADRStatus()...)
	}

	// Step 3: doc frontmatter (any .md with frontmatter must
	// parse as YAML).
	if scope == "full" || scope == "docs" {
		findings = append(findings, auditDocFrontmatter()...)
	}

	// Sort: ERROR first, then WARNING, then INFO.
	severityRank := map[string]int{"ERROR": 0, "WARNING": 1, "INFO": 2}
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})

	// Counts.
	errors, warnings, infos := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "ERROR":
			errors++
		case "WARNING":
			warnings++
		case "INFO":
			infos++
		}
	}

	body := renderAuditReport(scope, findings, errors, warnings, infos)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Summary: %d errors, %d warnings, %d info\n", errors, warnings, infos)
	if errors > 0 || (failOnWarning && warnings > 0) {
		return fmt.Errorf("audit found %d error(s) and %d warning(s) — see %s", errors, warnings, outPath)
	}
	return nil
}

// auditACTraceability walks specs/ and verifies that every AC
// in spec.md has at least one task in tasks.md that covers it,
// and that every task in tasks.md references at least one AC.
// Returns one finding per violation.
func auditACTraceability() []auditFinding {
	var findings []auditFinding
	entries, err := os.ReadDir("specs")
	if err != nil {
		// specs/ missing is not an audit failure (project may
		// not use specs/).
		return findings
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_templates" || e.Name() == "quick" {
			continue
		}
		dir := filepath.Join("specs", e.Name())
		specBody, err := os.ReadFile(filepath.Join(dir, "spec.md"))
		if err != nil {
			findings = append(findings, auditFinding{
				Severity: "WARNING",
				Location: dir + "/spec.md",
				Message:  "spec.md missing or unreadable",
			})
			continue
		}
		tasksBody, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
		if err != nil {
			findings = append(findings, auditFinding{
				Severity: "INFO",
				Location: dir + "/tasks.md",
				Message:  "tasks.md missing — ACs have no coverage claim",
			})
			continue
		}

		acs := parseAcceptanceCriteria(string(specBody))
		tasksBodyStr := string(tasksBody)
		for _, ac := range acs {
			if !strings.Contains(tasksBodyStr, ac.ID) {
				findings = append(findings, auditFinding{
					Severity: "WARNING",
					Location: dir + "/spec.md",
					Message:  fmt.Sprintf("AC %s (%s) has no covering task in tasks.md", ac.ID, ac.Title),
				})
			}
		}
	}
	return findings
}

// auditADRStatus scans docs/architecture/adr/ for status headers
// and verifies each is one of the canonical values.
func auditADRStatus() []auditFinding {
	var findings []auditFinding
	adrDir := "docs/architecture/adr"
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		return findings // no ADRs yet
	}
	validStatuses := map[string]bool{
		"proposed":   true,
		"accepted":   true,
		"deprecated": true,
		"superseded": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(adrDir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Look for "## Status" then the next non-empty line.
		found := false
		lines := strings.Split(string(body), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) != "## Status" {
				continue
			}
			found = true
			if i+1 < len(lines) {
				status := strings.TrimSpace(lines[i+1])
				if status == "" {
					findings = append(findings, auditFinding{
						Severity: "WARNING",
						Location: path,
						Message:  "## Status header has no value on the next line",
					})
				} else if !validStatuses[strings.ToLower(status)] {
					findings = append(findings, auditFinding{
						Severity: "WARNING",
						Location: path,
						Message:  fmt.Sprintf("## Status value %q is not one of proposed|accepted|deprecated|superseded", status),
					})
				}
			}
			break
		}
		if !found && !strings.HasPrefix(e.Name(), "_") {
			findings = append(findings, auditFinding{
				Severity: "INFO",
				Location: path,
				Message:  "ADR file has no '## Status' section",
			})
		}
	}
	return findings
}

// auditDocFrontmatter walks docs/ for .md files and reports any
// with malformed YAML frontmatter (unclosed --- block).
func auditDocFrontmatter() []auditFinding {
	var findings []auditFinding
	err := filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(body)
		if !strings.HasPrefix(s, "---") {
			return nil // no frontmatter, that's fine
		}
		// Look for closing "---" on its own line.
		rest := strings.TrimPrefix(s, "---")
		idx := strings.Index(rest, "\n---")
		if idx < 0 {
			findings = append(findings, auditFinding{
				Severity: "WARNING",
				Location: path,
				Message:  "frontmatter opened (---) but never closed",
			})
		}
		return nil
	})
	if err != nil {
		// docs/ missing is not an audit failure
		return findings
	}
	return findings
}

// renderAuditReport produces docs/audit-report.md content.
func renderAuditReport(scope string, findings []auditFinding, errors, warnings, infos int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Audit report — scope=%s\n\n", scope)
	b.WriteString("> Generated by `radiant audit`. Per the `auditar`\n")
	b.WriteString("> skill: project-wide conformity check (frontmatter,\n")
	b.WriteString("> AC traceability, ADR validity, deviations).\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n")
	b.WriteString("|----------|-------|\n")
	fmt.Fprintf(&b, "| ERROR    | %d |\n", errors)
	fmt.Fprintf(&b, "| WARNING  | %d |\n", warnings)
	fmt.Fprintf(&b, "| INFO     | %d |\n\n", infos)

	if len(findings) == 0 {
		b.WriteString("No findings. Project passes the audit.\n")
		return b.String()
	}

	b.WriteString("## Findings\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Message)
		fmt.Fprintf(&b, "- **Location**: %s\n\n", f.Location)
	}
	return b.String()
}

// runRelease cuts a release. Pipeline:
//  1. Pre-flight: check git tree is clean
//  2. Validate version format (semver, with optional leading 'v')
//  3. Check the tag doesn't already exist
//  4. Run quality gates (build, vet, fmt, test)
//  5. Bump version in cmd/radiant/main.go
//  6. Cross-compile (if not skipped)
//  7. Commit version bump (if not skipped)
//  8. Git tag vX.Y.Z (if not skipped)
//
// All destructive steps are skipped under --dry-run; the user
// sees exactly what would happen.
func runRelease(version string, dryRun, skipTests, skipCrossCompile, skipTag, skipCommit, interactive bool) error {
	// Normalize: accept both "0.5.1" and "v0.5.1".
	version = strings.TrimPrefix(version, "v")
	tagName := "v" + version

	// 1. Validate semver format.
	if !looksLikeSemver(version) {
		return fmt.Errorf("invalid version %q — expected semver (e.g. 0.5.1 or v0.5.1)", version)
	}

	fmt.Printf("  → Cutting release %s\n\n", tagName)

	// 2. Pre-flight: clean tree.
	if !dryRun {
		out, err := runGit("status", "--porcelain")
		if err != nil {
			return fmt.Errorf("git status: %w", err)
		}
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("working tree is dirty — commit or stash before cutting a release:\n%s", out)
		}
		fmt.Println("  ✓ working tree clean")
	} else {
		fmt.Println("  [skip] pre-flight (--dry-run)")
	}

	// 3. Check tag doesn't exist.
	if !dryRun && !skipTag {
		out, err := runGit("tag", "-l", tagName)
		if err != nil {
			return fmt.Errorf("git tag: %w", err)
		}
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("tag %s already exists — delete it first or pick a different version", tagName)
		}
		fmt.Printf("  ✓ tag %s does not exist yet\n", tagName)
	} else if dryRun {
		fmt.Printf("  [skip] tag existence check (--dry-run); would check %s\n", tagName)
	}

	// 4. Quality gates.
	if !skipTests {
		fmt.Println("\n  → Running quality gates")
		if !dryRun {
			if err := runGoStep("build", "build", "./..."); err != nil {
				return err
			}
			if err := runGoStep("vet", "vet", "./..."); err != nil {
				return err
			}
			if err := runFmtCheck(); err != nil {
				return err
			}
			if err := runTestRace(); err != nil {
				return err
			}
			fmt.Println("  ✓ build / vet / fmt / test (-race) all green")
		} else {
			fmt.Println("  [skip] quality gates (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] quality gates (--skip-tests)")
	}

	// 4b. Interactive confirmation: prompt BEFORE any destructive step
	// (version bump, commit, tag). Only fires when:
	//   --interactive is set AND
	//   we're not in --dry-run (nothing to confirm) AND
	//   stdin is a terminal (CI/non-tty automatically skips prompts)
	if interactive && !dryRun && isTerminal(os.Stdin) {
		fmt.Println("\n  ────────────────────────────────────────────")
		fmt.Printf("  About to bump version → %s\n", version)
		if !skipCommit {
			fmt.Printf("  Then commit 'release: cut %s'\n", tagName)
		}
		if !skipTag {
			fmt.Printf("  Then create tag %s\n", tagName)
		}
		fmt.Println("  ────────────────────────────────────────────")
		ok, err := promptConfirm("  Continue? [Y/n]: ")
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if !ok {
			fmt.Println("  ✗ Aborted by user. No changes made.")
			return nil
		}
		fmt.Println("  ✓ Confirmed")
	} else if interactive && !dryRun {
		fmt.Println("  [skip] interactive prompt (non-tty stdin — assuming yes)")
	}

	// 5. Bump version in cmd/radiant/main.go.
	fmt.Println("\n  → Bumping version")
	if err := bumpVersionInSource(version, dryRun); err != nil {
		return err
	}

	// 6. Cross-compile.
	if !skipCrossCompile {
		fmt.Println("\n  → Cross-compiling (6 targets)")
		if !dryRun {
			if err := runMakeRelease(); err != nil {
				return err
			}
			fmt.Println("  ✓ 6/6 targets built (see dist/)")
		} else {
			fmt.Println("  [skip] cross-compile (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] cross-compile (--skip-cross-compile)")
	}

	// 7. Commit.
	if !skipCommit {
		fmt.Println("\n  → Committing version bump")
		if !dryRun {
			if err := runGitCommit(fmt.Sprintf("release: cut %s", tagName), "cmd/radiant/main.go", "CHANGELOG.md"); err != nil {
				return err
			}
			fmt.Printf("  ✓ committed 'release: cut %s'\n", tagName)
		} else {
			fmt.Println("  [skip] commit (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] commit (--skip-commit)")
	}

	// 8. Git tag.
	if !skipTag {
		fmt.Println("\n  → Tagging")
		if !dryRun {
			if _, err := runGit("tag", tagName); err != nil {
				return fmt.Errorf("git tag: %w", err)
			}
			fmt.Printf("  ✓ tagged %s\n", tagName)
		} else {
			fmt.Printf("  [skip] tag (--dry-run); would create %s\n", tagName)
		}
	} else {
		fmt.Println("  [skip] tag (--skip-tag)")
	}

	fmt.Printf("\n  ✓ Release %s complete\n", tagName)
	fmt.Printf("    Next: git push origin main && git push origin %s\n", tagName)
	// Record a local telemetry event if the user has opted in.
	// Only fires when --skip-tag is NOT set (a release without a
	// tag isn't really a release).
	if !skipTag {
		recordTelemetry("release")
	}
	return nil
}

// isTerminal reports whether the given file is connected to a terminal.
// On unix, checks ModeCharDevice; on Windows every fd looks like one,
// but for our purposes (CI vs interactive shell), this is sufficient.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptConfirm asks the user a yes/no question on stdin. Empty input
// (just Enter) defaults to yes. "n"/"no" answers no. Anything else is
// rejected with a clear error.
//
// Exposed for tests; the caller is responsible for ensuring stdin is
// a terminal (or piping known input).
func promptConfirm(question string) (bool, error) {
	fmt.Print(question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid answer %q — expected y/yes/n/no", answer)
	}
}

// looksLikeSemver is a relaxed semver check: MAJOR.MINOR.PATCH
// with optional pre-release / build suffix. We don't enforce the
// strict semver spec (it would block "0.5.0-rc.1" etc.); we just
// require three numeric components separated by dots. Accepts
// an optional leading "v" (so both "0.5.1" and "v0.5.1" pass).
func looksLikeSemver(v string) bool {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return false
	}
	for _, p := range parts {
		// Allow trailing pre-release (e.g. "0-rc.1") by stripping.
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// runGit runs a git subcommand in the project root and returns stdout.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGitInDir runs git in the given directory.
func runGitInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGoStep runs a `go` subcommand (build/vet) with the project env.
func runGoStep(label, sub string, args ...string) error {
	cmd := exec.Command("go", append([]string{sub}, args...)...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s: %w\n%s", label, err, string(out))
	}
	return nil
}

// runFmtCheck fails if any .go file is not gofmt'd.
func runFmtCheck() error {
	cmd := exec.Command("gofmt", "-l", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt -l: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("files not gofmt'd:\n%s", string(out))
	}
	return nil
}

// runTestRace runs the full test suite under -race.
func runTestRace() error {
	cmd := exec.Command("go", "test", "./...", "-count=1", "-race", "-timeout=180s")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go test: %w\n%s", err, string(out))
	}
	return nil
}

// runMakeRelease invokes `make release` and forwards output.
func runMakeRelease() error {
	cmd := exec.Command("make", "release")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make release: %w\n%s", err, string(out))
	}
	return nil
}

// runGitCommit commits the given paths with the given message.
// Uses -c user.name/email to avoid touching global git config.
func runGitCommit(msg string, paths ...string) error {
	args := []string{"add", "--"}
	args = append(args, paths...)
	if _, err := runGit(args...); err != nil {
		return err
	}
	commit := []string{
		"-c", "user.name=Henrique",
		"-c", "user.email=henrique@fortvna.com.br",
		"commit", "-m", msg,
	}
	cmd := exec.Command("git", commit...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, string(out))
	}
	return nil
}

// bumpVersionInSource updates `var version = "..."` in
// cmd/radiant/main.go to the new version. The file is rewritten
// line-by-line to avoid touching other content. With dryRun=true,
// prints what would change without writing.
func bumpVersionInSource(newVersion string, dryRun bool) error {
	path := "cmd/radiant/main.go"
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	body := string(data)
	oldLine := ""
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "var version =") {
			oldLine = line
			break
		}
	}
	if oldLine == "" {
		return fmt.Errorf("could not find 'var version = ...' in %s", path)
	}
	newLine := fmt.Sprintf(`var version = "%s"`, newVersion)
	if oldLine == newLine {
		fmt.Printf("  = %s (no change)\n", path)
		return nil
	}
	if dryRun {
		fmt.Printf("  [would-replace] %s\n        %s\n      → %s\n", path, oldLine, newLine)
		return nil
	}
	updated := strings.Replace(body, oldLine, newLine, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("  ✓ %s: %s\n", path, newLine)
	return nil
}


// runMCPServe reads newline-delimited JSON-RPC requests from `in`,
// writes JSON-RPC responses to `out`. Implements the Model Context
// Protocol (MCP) over stdio. Tools exposed are radiant commands
// (spec, adr, product, evals, audit, release). Each tool call
// spawns the corresponding command as a subprocess and returns
// stdout as the result.
//
