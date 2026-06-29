//go:build with_full

package main

// ── security posture audit ─────────────────────────────────────────────────
//
// `radiant security [--scope=secrets|perms|all] [--output=...] [--fail-on-warning]`
// scans the project for two classes of issues:
//   1. Hardcoded secrets in source code (regex-based — covers the
//      most common formats seen in real leaks).
//   2. Sensitive files with overly permissive permissions (group
//      or world access).
//
// Output: markdown report at docs/security-report.md (configurable
// via --output). Exit code: 0 if clean, 1 if errors found, 1 if
// warnings and --fail-on-warning is set.
//
// This file was extracted from helpers.go in Sprint 74 (v2.44.0)
// as part of the helpers.go debt-reduction effort. The cmd_audit.go
// command registration was previously inlined; it's now here alongside
// the implementation.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// securityFinding is one row in the security report.
type securityFinding struct {
	Severity string // "ERROR" | "WARNING" | "INFO"
	Location string // file:line or path
	Message  string
}

// secretPattern matches a plausible secret in source. The list
// covers the most common formats seen in real leaks. False
// positives are accepted (a secret-shaped string in a test file
// is still worth flagging — humans can ignore if appropriate).
type secretPattern struct {
	Name        string
	Regex       *regexp.Regexp
	Description string
}

// registerSecurityCmd registers the `radiant security` subcommand.
//
// `radiant security [--scope=secrets|perms|all] [--output=...]`
// scans the project for common security issues:
//   - Hardcoded secrets (API keys, tokens) in source code
//   - Sensitive files with overly permissive file permissions
//
// MVP scope: secrets + permissions. Dependency-CVE scanning and
// config-CORS checks are deferred to future work.
func registerSecurityCmd(root *cobra.Command) {
	securityCmd := &cobra.Command{
		Use:   "security",
		Short: "Security posture audit: hardcoded secrets + sensitive file perms",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _ := cmd.Flags().GetString("scope")
			outPath, _ := cmd.Flags().GetString("output")
			failOnWarn, _ := cmd.Flags().GetBool("fail-on-warning")
			return runSecurity(scope, outPath, failOnWarn)
		},
	}
	securityCmd.Flags().String("scope", "all", "scan scope: secrets | perms | all")
	securityCmd.Flags().StringP("output", "o", "", "output path (default: docs/security-report.md)")
	securityCmd.Flags().Bool("fail-on-warning", false, "exit non-zero on warnings (default: only errors)")
	root.AddCommand(securityCmd)
}

// runSecurity scans the project for security issues per the
// `--scope` flag. Scopes: "secrets" (regex-based secret scan),
// "perms" (sensitive file permission check), "all" (both).
func runSecurity(scope, outPath string, failOnWarning bool) error {
	if outPath == "" {
		outPath = "docs/security-report.md"
	}

	var findings []securityFinding
	if scope == "all" || scope == "secrets" {
		findings = append(findings, scanSecrets()...)
	}
	if scope == "all" || scope == "perms" {
		findings = append(findings, scanPerms()...)
	}

	severityRank := map[string]int{"ERROR": 0, "WARNING": 1, "INFO": 2}
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})

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

	body := renderSecurityReport(scope, findings, errors, warnings, infos)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Summary: %d errors, %d warnings, %d info\n", errors, warnings, infos)
	if errors > 0 || (failOnWarning && warnings > 0) {
		return fmt.Errorf("security scan found %d error(s) and %d warning(s) — see %s", errors, warnings, outPath)
	}
	return nil
}

// scanSecrets walks .go / .md / .yml / .json / .sh / .env / .ts / .js
// files and reports any line that matches a known secret pattern.
// Skips vendor / node_modules / .git / dist directories and
// .test.go files (test fixtures commonly contain fake secrets).
func scanSecrets() []securityFinding {
	patterns := []secretPattern{
		{Name: "AWS Access Key", Regex: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), Description: "AWS access key ID"},
		{Name: "GitHub Token", Regex: regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`), Description: "GitHub personal access token"},
		{Name: "GitHub Fine-Grained Token", Regex: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`), Description: "GitHub fine-grained PAT"},
		{Name: "Slack Token", Regex: regexp.MustCompile(`xox[abpr]-[A-Za-z0-9-]{10,}`), Description: "Slack API token"},
		{Name: "OpenAI Key", Regex: regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`), Description: "OpenAI / OpenAI-compatible API key"},
		{Name: "Anthropic Key", Regex: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`), Description: "Anthropic API key"},
		{Name: "Google API Key", Regex: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), Description: "Google API key"},
		{Name: "Generic Bearer", Regex: regexp.MustCompile(`Bearer\s+[A-Za-z0-9_\-\.=]{20,}`), Description: "Bearer token in source"},
	}
	skipDirs := map[string]bool{
		".git":                    true,
		"node_modules":            true,
		"vendor":                  true,
		"dist":                    true,
		".radiant-harness/skills": true, // bundled skills have example secrets in docs
	}
	var findings []securityFinding
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Top-level skips.
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" {
				return filepath.SkipDir
			}
			if skipDirs[path] {
				return filepath.SkipDir
			}
			return nil
		}
		// Only scan known file types.
		ext := filepath.Ext(path)
		switch ext {
		case ".go", ".md", ".yml", ".yaml", ".json", ".sh", ".env",
			".ts", ".js", ".py", ".rb", ".toml":
		default:
			return nil
		}
		// Skip test files (test fixtures commonly contain fake secrets).
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".test.ts") ||
			strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, "_test.py") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, p := range patterns {
				if p.Regex.MatchString(line) {
					findings = append(findings, securityFinding{
						Severity: "ERROR",
						Location: fmt.Sprintf("%s:%d", path, i+1),
						Message:  fmt.Sprintf("Possible %s (%s)", p.Name, p.Description),
					})
					break // one finding per line is enough
				}
			}
		}
		return nil
	})
	if err != nil {
		// Walking the tree shouldn't fail; if it does, return what we have.
		return findings
	}
	return findings
}

// scanPerms checks for sensitive files with overly permissive
// (world-readable/writable) permissions. Targets: .env, *.key,
// *.pem, id_rsa, id_dsa, id_ecdsa, id_ed25519, *.p12, *.pfx.
func scanPerms() []securityFinding {
	sensitiveNames := map[string]bool{
		".env":            true,
		".env.local":      true,
		".env.production": true,
		"id_rsa":          true,
		"id_dsa":          true,
		"id_ecdsa":        true,
		"id_ed25519":      true,
	}
	sensitiveExts := map[string]bool{
		".key": true, ".pem": true, ".p12": true, ".pfx": true,
	}
	var findings []securityFinding
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !sensitiveNames[base] && !sensitiveExts[filepath.Ext(base)] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		mode := info.Mode().Perm()
		// Group or world has any access (read/write/execute).
		if mode&0o077 != 0 {
			findings = append(findings, securityFinding{
				Severity: "WARNING",
				Location: path,
				Message:  fmt.Sprintf("sensitive file has permissive mode %04o (group/world can access); chmod 600 recommended", mode),
			})
		}
		return nil
	})
	if err != nil {
		return findings
	}
	return findings
}

// renderSecurityReport produces the docs/security-report.md content.
func renderSecurityReport(scope string, findings []securityFinding, errors, warnings, infos int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Security report — scope=%s\n\n", scope)
	b.WriteString("> Generated by `radiant security`. MVP scope:\n")
	b.WriteString("> hardcoded secret scan + sensitive file permissions.\n")
	b.WriteString("> Dep-CVE scanning and config-CORS checks are future work.\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n")
	b.WriteString("|----------|-------|\n")
	fmt.Fprintf(&b, "| ERROR    | %d |\n", errors)
	fmt.Fprintf(&b, "| WARNING  | %d |\n", warnings)
	fmt.Fprintf(&b, "| INFO     | %d |\n\n", infos)

	if len(findings) == 0 {
		b.WriteString("No findings. Project passes the security scan.\n")
		return b.String()
	}

	b.WriteString("## Findings\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Message)
		fmt.Fprintf(&b, "- **Location**: %s\n\n", f.Location)
	}
	return b.String()
}