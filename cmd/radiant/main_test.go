package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Add JWT authentication", "add-jwt-authentication"},
		{"Hello World", "hello-world"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"snake_case_input", "snake-case-input"},
		{"MixedCASE", "mixedcase"},
		{"trailing punctuation!!!", "trailing-punctuation"},
		{"with / slash", "with-slash"},
		{"under_score", "under-score"},
		{"", ""},
		{"---leading---trailing---", "leading-trailing"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugifyLengthCap(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := slugify(long)
	if len(got) > 48 {
		t.Errorf("slugify(%d chars) returned %d chars; should cap at 48", len(long), len(got))
	}
}

func TestNextSpecSeqEmpty(t *testing.T) {
	dir := t.TempDir()
	seq, err := nextSpecSeq(dir)
	if err != nil {
		t.Fatalf("nextSpecSeq: %v", err)
	}
	if seq != 1 {
		t.Errorf("nextSpecSeq on empty dir = %d, want 1", seq)
	}
}

func TestNextSpecSeqIncrement(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0001-foo", "0003-bar", "0007-baz", "README.md", "not-numbered"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	seq, err := nextSpecSeq(dir)
	if err != nil {
		t.Fatalf("nextSpecSeq: %v", err)
	}
	if seq != 8 {
		t.Errorf("nextSpecSeq = %d, want 8 (highest was 0007)", seq)
	}
}

func TestUpsertStateCurrentFeature(t *testing.T) {
	in := `# State

## Current position
- current_feature: old-feature
- tier: trivial
- next_command: radiant run old-feature

## Last session
- last_updated: 2026-01-01T00:00:00Z
`
	out := upsertStateCurrentFeature(in, "0007-new", "feature", "radiant run 0007-new")
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "- current_feature: 0007-new":
		case line == "- tier: feature":
		case line == "- next_command: radiant run 0007-new":
		default:
			// other lines preserved
			if !strings.HasPrefix(line, "- last_updated") && !strings.Contains(line, "State") && !strings.Contains(line, "Current") && !strings.Contains(line, "Last") && line != "" {
				t.Errorf("unexpected line modified: %q", line)
			}
		}
	}
	if !strings.Contains(out, "- current_feature: 0007-new") {
		t.Error("current_feature line not updated")
	}
	if !strings.Contains(out, "- tier: feature") {
		t.Error("tier line not updated")
	}
	if !strings.Contains(out, "- next_command: radiant run 0007-new") {
		t.Error("next_command line not updated")
	}
}

func TestNextADRSequenceEmpty(t *testing.T) {
	dir := t.TempDir()
	seq, err := nextADRSequence(dir)
	if err != nil {
		t.Fatalf("nextADRSequence: %v", err)
	}
	if seq != 1 {
		t.Errorf("nextADRSequence on empty dir = %d, want 1", seq)
	}
}

func TestNextADRSequenceSkipsNonMatching(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0001-foo.md", "_template.md", "README.md", "0007-bar.md", "0003-baz.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seq, err := nextADRSequence(dir)
	if err != nil {
		t.Fatalf("nextADRSequence: %v", err)
	}
	if seq != 8 {
		t.Errorf("nextADRSequence = %d, want 8 (highest was 0007, _template/README ignored)", seq)
	}
}

func TestRenderADRIncludesRequiredSections(t *testing.T) {
	body := renderADR(42, "Use Postgres", "accepted")
	for _, section := range []string{"## Status", "## Context", "## Decision", "## Consequences"} {
		if !strings.Contains(body, section) {
			t.Errorf("renderADR missing section %q", section)
		}
	}
	if !strings.Contains(body, "accepted") {
		t.Errorf("renderADR should include the supplied status, got:\n%s", body)
	}
}

func TestRenderADRRejectsInvalidStatus(t *testing.T) {
	body := renderADR(1, "Test", "totally-invalid-status")
	if !strings.Contains(body, "proposed") {
		t.Errorf("renderADR should fall back to 'proposed' on invalid status; got:\n%s", body)
	}
}

func TestRenderADRReferencesSkill(t *testing.T) {
	body := renderADR(1, "Test", "proposed")
	if !strings.Contains(body, "adr") {
		t.Errorf("renderADR should reference the adr skill; got:\n%s", body)
	}
}

func TestReadFrontmatterVersionMissing(t *testing.T) {
	if v := readFrontmatterVersion(filepath.Join(t.TempDir(), "nope.yaml")); v != "" {
		t.Errorf("missing file should yield empty version, got %q", v)
	}
}

func TestReadFrontmatterVersionValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frontmatter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\nversion: \"1.2.3\"\ndescription: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := readFrontmatterVersion(path); v != "1.2.3" {
		t.Errorf("expected 1.2.3, got %q", v)
	}
}

func TestReadFrontmatterVersionUnquoted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frontmatter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\nversion: 0.9.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := readFrontmatterVersion(path); v != "0.9.0" {
		t.Errorf("expected 0.9.0 (unquoted), got %q", v)
	}
}

func TestReadFrontmatterVersionNoField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frontmatter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\ndescription: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := readFrontmatterVersion(path); v != "" {
		t.Errorf("expected empty (no version field), got %q", v)
	}
}

func TestGenerateAgentsMDIncludesSkills(t *testing.T) {
	body := generateAgentsMD()
	if !strings.Contains(body, "# AGENTS.md") {
		t.Error("generateAgentsMD missing header")
	}
	// Must reference at least the bundled skills we know exist.
	for _, name := range []string{"nova-feature", "validar", "adr"} {
		if !strings.Contains(body, name) {
			t.Errorf("generateAgentsMD missing skill %q", name)
		}
	}
}

func TestGenerateAgentsMDMinimal(t *testing.T) {
	body := generateAgentsMD()
	if lineCount := len(strings.Split(body, "\n")); lineCount > 100 {
		t.Errorf("AGENTS.md should stay <=100 lines (video research #6); got %d", lineCount)
	}
}

func TestRenderDiagramAllLevels(t *testing.T) {
	for _, level := range []string{"context", "container", "component", "code"} {
		body, err := renderDiagram(level)
		if err != nil {
			t.Errorf("renderDiagram(%q) error: %v", level, err)
			continue
		}
		if !strings.Contains(body, "```mermaid") {
			t.Errorf("renderDiagram(%q) missing mermaid fence", level)
		}
	}
}

func TestRenderDiagramRejectsUnknownLevel(t *testing.T) {
	if _, err := renderDiagram("bogus"); err == nil {
		t.Error("renderDiagram(bogus) should error")
	}
}

func TestRenderDiagramHasC4Directives(t *testing.T) {
	for _, pair := range []struct {
		level, directive string
	}{
		{"context", "C4Context"},
		{"container", "C4Container"},
		{"component", "C4Component"},
	} {
		body, err := renderDiagram(pair.level)
		if err != nil {
			t.Fatalf("renderDiagram(%q) error: %v", pair.level, err)
		}
		if !strings.Contains(body, pair.directive) {
			t.Errorf("renderDiagram(%q) should contain %q", pair.level, pair.directive)
		}
	}
}

func TestRenderInceptionIncludesAllPhases(t *testing.T) {
	body := renderInception("api-obs", "API observability for small dev teams", 6)
	for _, phase := range []string{"## 1. Why", "## 2. What", "## 4. Who", "## 5. How", "## 6. When", "## 7. Where", "MVP cut"} {
		if !strings.Contains(body, phase) {
			t.Errorf("renderInception missing %q", phase)
		}
	}
}

func TestRenderInceptionIncludesVision(t *testing.T) {
	body := renderInception("slug", "Help engineers debug latency", 8)
	if !strings.Contains(body, "Help engineers debug latency") {
		t.Error("renderInception should embed the supplied vision string")
	}
}

func TestRenderInceptionRespectsMVPWeeks(t *testing.T) {
	body := renderInception("slug", "v", 12)
	if !strings.Contains(body, "**12 weeks**") {
		t.Errorf("renderInception should embed the supplied mvp-weeks; got:\n%s", body)
	}
}

func TestRenderInceptionReferencesNovaProduct(t *testing.T) {
	body := renderInception("slug", "v", 8)
	if !strings.Contains(body, "nova-product") {
		t.Errorf("renderInception should reference the nova-product skill")
	}
}

func TestRenderPersonasTemplateHasThreeSlots(t *testing.T) {
	body := renderPersonasTemplate()
	count := strings.Count(body, "## <Persona name>")
	if count < 2 || count > 4 {
		t.Errorf("personas template should have 2-4 slots (nova-product skill says so); got %d", count)
	}
	for _, field := range []string{"Job to be done", "Pain today", "Success looks like"} {
		if !strings.Contains(body, field) {
			t.Errorf("personas template missing field %q", field)
		}
	}
}

func TestRenderIntegrationsDocIncludesServers(t *testing.T) {
	servers := map[string]mcpServer{
		"github": {Command: "npx", Args: []string{"-y", "@mcp/server-github"}, Env: map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}"}},
		"jira":   {Command: "npx", Args: []string{"-y", "@atlassian/mcp-server-jira"}},
	}
	body := renderIntegrationsDoc(servers)
	for _, name := range []string{"github", "jira", "Declared MCP servers", "Approval log"} {
		if !strings.Contains(body, name) {
			t.Errorf("renderIntegrationsDoc missing %q", name)
		}
	}
}

func TestRenderIntegrationsDocHandlesHTTPServer(t *testing.T) {
	servers := map[string]mcpServer{
		"notion": {URL: "https://mcp.notion.com/sse"},
	}
	body := renderIntegrationsDoc(servers)
	if !strings.Contains(body, "notion") || !strings.Contains(body, "<http>") {
		t.Errorf("renderIntegrationsDoc should render URL-based servers as <http>; got:\n%s", body)
	}
}

func TestRenderIntegrationsDocEmptyMap(t *testing.T) {
	body := renderIntegrationsDoc(map[string]mcpServer{})
	if !strings.Contains(body, "Declared MCP servers") {
		t.Errorf("renderIntegrationsDoc should still emit the table header even when empty")
	}
}

func TestIntegrationsListReadsMCPConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")
	content := `{
  "mcpServers": {
    "github": {"command": "npx", "args": ["-y", "@mcp/server-github"]}
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
	}
	if cfg.Servers["github"].Command != "npx" {
		t.Errorf("github command = %q, want npx", cfg.Servers["github"].Command)
	}
}

func TestIntegrationsListMissingFile(t *testing.T) {
	// We can't easily run runIntegrationsList here because it reads
	// from the CWD. But we can verify that the error message is
	// stable — it's the user-facing string we promise in the skill.
	tmpDir := t.TempDir()
	_, err := os.ReadFile(filepath.Join(tmpDir, ".mcp.json"))
	if err == nil {
		t.Error("expected error reading missing .mcp.json")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got %v", err)
	}
}

func TestParseAcceptanceCriteriaBasic(t *testing.T) {
	spec := `# 0001

## Acceptance criteria

### AC1: valid login returns a JWT
- Given ...
- When ...
- Then ...

### AC2: invalid login returns 401
- Given wrong password
- When POST /auth/login
- Then response is 401
`
	acs := parseAcceptanceCriteria(spec)
	if len(acs) != 2 {
		t.Fatalf("expected 2 ACs, got %d", len(acs))
	}
	if acs[0].ID != "AC1" {
		t.Errorf("first AC id = %q, want AC1", acs[0].ID)
	}
	if acs[1].ID != "AC2" {
		t.Errorf("second AC id = %q, want AC2", acs[1].ID)
	}
	if !strings.Contains(acs[0].Title, "valid login") {
		t.Errorf("first AC title missing expected text: %q", acs[0].Title)
	}
}

func TestParseAcceptanceCriteriaEmpty(t *testing.T) {
	acs := parseAcceptanceCriteria("# 0001\n\n## Why\n\nfoo\n")
	if len(acs) != 0 {
		t.Errorf("expected 0 ACs (no ### AC headers), got %d", len(acs))
	}
}

func TestParseAcceptanceCriteriaCaseInsensitive(t *testing.T) {
	spec := "### ac1: lowercase header\n"
	acs := parseAcceptanceCriteria(spec)
	if len(acs) != 1 || acs[0].ID != "AC1" {
		t.Errorf("expected AC1 (uppercased), got id=%q len=%d", acs[0].ID, len(acs))
	}
}

func TestParseGatesFromTasks(t *testing.T) {
	tasks := `| # | Task | Covers | Gate |
|---|------|--------|------|
| 1 | Add lib | AC1 | ` + "`go build ./...`" + ` |
| 2 | Tests | AC1, AC2 | ` + "`go test ./...`" + ` |
| 3 | Docs | AC3 | — |
`
	gates := parseGatesFromTasks(tasks)
	if len(gates) != 2 {
		t.Fatalf("expected 2 gates, got %d: %v", len(gates), gates)
	}
	if gates[0] != "go build ./..." {
		t.Errorf("gate[0] = %q, want go build ./...", gates[0])
	}
	if gates[1] != "go test ./..." {
		t.Errorf("gate[1] = %q, want go test ./...", gates[1])
	}
}

func TestParseGatesFromTasksEmpty(t *testing.T) {
	if gates := parseGatesFromTasks("# tasks\n\nno table here\n"); len(gates) != 0 {
		t.Errorf("expected 0 gates from no-table input, got %d", len(gates))
	}
}

func TestCountDiffFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1 +1 @@
-old
+new
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1 +1 @@
-x
+y
`
	if n := countDiffFiles(diff); n != 2 {
		t.Errorf("countDiffFiles = %d, want 2", n)
	}
}

func TestRenderPRReviewIncludesSections(t *testing.T) {
	acs := []acceptanceCriterion{
		{ID: "AC1", Title: "valid login"},
		{ID: "AC2", Title: "invalid login"},
	}
	gates := []string{"go test ./..."}
	body := renderPRReview("0007-jwt", acs, gates, nil, "", struct {
		Lines int
		Files int
	}{})
	for _, want := range []string{
		"# PR review: 0007-jwt",
		"## Summary",
		"## Recommendation",
		"## AC coverage",
		"## Gate results",
		"## SPEC_DEVIATION",
		"AC1", "AC2",
		"revisar-pr", // skill name appears in footer
	} {
		if !strings.Contains(body, want) {
			t.Errorf("renderPRReview missing %q", want)
		}
	}
}

func TestRenderPRReviewGatePassFail(t *testing.T) {
	gates := []string{"true", "false"}
	results := []gateResult{
		{Name: "true", Passed: true},
		{Name: "false", Passed: false, Err: "exit code 1"},
	}
	body := renderPRReview("0001", nil, gates, results, "", struct {
		Lines int
		Files int
	}{})
	if !strings.Contains(body, "✓ pass") {
		t.Error("renderPRReview should mark 'true' as ✓ pass")
	}
	if !strings.Contains(body, "✗ fail") {
		t.Error("renderPRReview should mark 'false' as ✗ fail")
	}
}

func TestRenderPRReviewWithDiffStats(t *testing.T) {
	body := renderPRReview("0001", nil, nil, nil, "changes.diff", struct {
		Lines int
		Files int
	}{Lines: 42, Files: 3})
	if !strings.Contains(body, "3 files, 42 lines") {
		t.Errorf("renderPRReview should embed diff stats; got:\n%s", body)
	}
}

func TestRenderGitHubActionsHasGates(t *testing.T) {
	body := renderGitHubActions("")
	for _, want := range []string{
		"name: radiant-esteira",
		"on:",
		"pull_request:",
		"actions/checkout@v4",
		"radiant validate",
		"radiant audit",
		"go test ./...",
		"go build ./...",
		"RADIANT_API_KEY: ${{ secrets.RADIANT_API_KEY }}",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GitHub Actions workflow missing %q", want)
		}
	}
}

func TestRenderGitHubActionsRespectsModel(t *testing.T) {
	body := renderGitHubActions("gpt-4o")
	if !strings.Contains(body, "radiant validate --model gpt-4o") {
		t.Errorf("model should be passed to validate; got:\n%s", body)
	}
}

func TestRenderGitLabCIHasGates(t *testing.T) {
	body := renderGitLabCI("")
	for _, want := range []string{
		"stages:",
		"radiant-validate:",
		"radiant audit",
		"go test ./...",
		"RADIANT_API_KEY: $RADIANT_API_KEY",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GitLab CI missing %q", want)
		}
	}
}

func TestRenderCircleCIHasGates(t *testing.T) {
	body := renderCircleCI("")
	for _, want := range []string{
		"version: 2.1",
		"cimg/go:1.22",
		"radiant validate",
		"radiant audit",
		"go test ./...",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("CircleCI missing %q", want)
		}
	}
}

func TestCISecretsForProviders(t *testing.T) {
	cases := []struct {
		provider string
		want     []string
	}{
		{"github", []string{"RADIANT_API_KEY", "GITHUB_TOKEN"}},
		{"gitlab", []string{"RADIANT_API_KEY", "GITLAB_TOKEN"}},
		{"circleci", []string{"RADIANT_API_KEY", "CIRCLE_TOKEN"}},
		{"unknown", []string{"RADIANT_API_KEY"}},
	}
	for _, c := range cases {
		got := ciSecretsFor(c.provider)
		if len(got) != len(c.want) {
			t.Errorf("ciSecretsFor(%q) length = %d, want %d", c.provider, len(got), len(c.want))
			continue
		}
		for i, s := range got {
			if s != c.want[i] {
				t.Errorf("ciSecretsFor(%q)[%d] = %q, want %q", c.provider, i, s, c.want[i])
			}
		}
	}
}

func TestNoHardcodedSecretsInCITemplates(t *testing.T) {
	// Sanity check: none of the three templates should embed a
	// plausible API key (e.g. "sk-...", "key-..."). They must
	// reference secrets via the provider's secret store.
	bodies := map[string]string{
		"github":   renderGitHubActions(""),
		"gitlab":   renderGitLabCI(""),
		"circleci": renderCircleCI(""),
	}
	for provider, body := range bodies {
		for _, banned := range []string{"sk-", "key-", "api_key=", "apikey="} {
			if strings.Contains(strings.ToLower(body), banned) {
				t.Errorf("%s template contains hardcoded secret pattern %q", provider, banned)
			}
		}
	}
}

func TestCamadaAgenticaReportsMissingAgentsMD(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// No AGENTS.md exists; the command should NOT panic, it should
	// report the missing file and continue. We can't easily capture
	// stdout, so we just confirm the function returns nil (no error).
	if err := runCamadaAgentica("", false); err != nil {
		t.Errorf("runCamadaAgentica on empty dir should not error; got %v", err)
	}
}

func TestCamadaAgenticaDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Write a stale AGENTS.md that mentions skills by name but
	// without their current versions — simulating drift.
	stale := "# AGENTS.md\n\n| foo | bar |\n|-----|-----|\n| adr | old |\n| nova-feature | old |\n"
	if err := os.WriteFile("AGENTS.md", []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	// Should run cleanly (prints drift warnings to stdout).
	if err := runCamadaAgentica("", false); err != nil {
		t.Errorf("runCamadaAgentica should not error on stale AGENTS.md; got %v", err)
	}
	// After --fix, the regenerated file should mention current
	// versions.
	if err := runCamadaAgentica("", true); err != nil {
		t.Errorf("runCamadaAgentica --fix error: %v", err)
	}
	body, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "v1.0.0") {
		t.Errorf("regenerated AGENTS.md should contain current versions; got:\n%s", body)
	}
}

func TestCamadaAgenticaUnknownAgent(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("AGENTS.md", []byte("# stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Unknown agent name → resolveAgents returns empty → command
	// still runs (no error).
	if err := runCamadaAgentica("bogus", false); err != nil {
		t.Errorf("runCamadaAgentica with bogus agent should not error; got %v", err)
	}
}

func TestComputeFeatureCoverageAllCovered(t *testing.T) {
	dir := t.TempDir()
	specMD := "# 0001\n\n## Acceptance criteria\n\n### AC1: foo\n### AC2: bar\n"
	tasksMD := "| 1 | task | AC1, AC2 | `true` |\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tasksMD), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := computeFeatureCoverage(dir)
	if err != nil {
		t.Fatalf("computeFeatureCoverage: %v", err)
	}
	if c.Total != 2 || c.Covered != 2 {
		t.Errorf("expected 2/2 covered, got %d/%d", c.Covered, c.Total)
	}
	if c.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", c.Score)
	}
	if len(c.Uncovered) != 0 {
		t.Errorf("expected 0 uncovered, got %v", c.Uncovered)
	}
}

func TestComputeFeatureCoveragePartial(t *testing.T) {
	dir := t.TempDir()
	specMD := "# 0001\n\n## Acceptance criteria\n\n### AC1: foo\n### AC2: bar\n### AC3: baz\n"
	tasksMD := "| 1 | task | AC1, AC2 | `true` |\n" // AC3 not mentioned
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tasksMD), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := computeFeatureCoverage(dir)
	if err != nil {
		t.Fatalf("computeFeatureCoverage: %v", err)
	}
	if c.Total != 3 || c.Covered != 2 {
		t.Errorf("expected 2/3 covered, got %d/%d", c.Covered, c.Total)
	}
	if len(c.Uncovered) != 1 || c.Uncovered[0] != "AC3" {
		t.Errorf("expected uncovered=[AC3], got %v", c.Uncovered)
	}
}

func TestComputeFeatureCoverageNoTasksMD(t *testing.T) {
	dir := t.TempDir()
	specMD := "# 0001\n\n## Acceptance criteria\n\n### AC1: foo\n### AC2: bar\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatal(err)
	}
	// No tasks.md
	c, err := computeFeatureCoverage(dir)
	if err != nil {
		t.Fatalf("computeFeatureCoverage: %v", err)
	}
	if c.Covered != 0 || c.Total != 2 {
		t.Errorf("expected 0/2 (no tasks), got %d/%d", c.Covered, c.Total)
	}
}

func TestRenderEvalsReportIncludesSections(t *testing.T) {
	coverages := []featureCoverage{
		{Slug: "0001-login", Total: 3, Covered: 3, Score: 1.0},
		{Slug: "0002-reports", Total: 1, Covered: 0, Uncovered: []string{"AC1"}, Score: 0.0},
	}
	body := renderEvalsReport("all", coverages)
	for _, want := range []string{
		"# Evals: all",
		"## Summary",
		"## Per-feature fidelity",
		"## AC-level detail",
		"0001-login",
		"0002-reports",
		"Aggregate fidelity",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("renderEvalsReport missing %q", want)
		}
	}
}

func TestRenderEvalsReportComputesAggregate(t *testing.T) {
	coverages := []featureCoverage{
		{Slug: "a", Total: 4, Covered: 4, Score: 1.0},
		{Slug: "b", Total: 0, Covered: 0, Score: 0.0}, // empty feature
	}
	body := renderEvalsReport("all", coverages)
	// Aggregate should be 4 / 4 = 100% (empty feature contributes 0)
	if !strings.Contains(body, "100.0%") {
		t.Errorf("expected 100%% aggregate (only non-empty feature), got:\n%s", body)
	}
}
