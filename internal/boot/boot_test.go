package boot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_BasicProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/myservice\ngo 1.22\n")

	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if m.Version != "2.0" {
		t.Errorf("version = %q, want %q", m.Version, "2.0")
	}
	if m.Project.Name != "myservice" {
		t.Errorf("project name = %q, want %q", m.Project.Name, "myservice")
	}
	if len(m.Skills) == 0 {
		t.Error("expected at least one recommended skill")
	}
	if m.Loop.Pattern == "" {
		t.Error("expected loop pattern to be set")
	}
}

func TestGenerate_BudgetProfiles(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		profile string
		minCtx  int
	}{
		{"lean", 100},
		{"standard", 1000},
		{"thorough", 5000},
		{"", 1000}, // default is standard
	}
	for _, tt := range tests {
		m, err := Generate(dir, Options{BudgetProfile: tt.profile})
		if err != nil {
			t.Fatalf("profile=%q: %v", tt.profile, err)
		}
		if m.Budget.ContextMin < tt.minCtx {
			t.Errorf("profile=%q: ContextMin=%d < %d", tt.profile, m.Budget.ContextMin, tt.minCtx)
		}
	}
}

func TestRenderMarkdown_UnderTokenLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/api\ngo 1.22\n")

	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	md := RenderMarkdown(m, FlavorGeneric)

	// Rough token estimate: 500 tokens ≈ 2000 chars (at 4 chars/token)
	if len(md) > 3000 {
		t.Errorf("markdown manifest too long: %d chars (want ≤3000)", len(md))
	}
	if !strings.Contains(md, "Radiant Boot") {
		t.Error("manifest should contain 'Radiant Boot' header")
	}
	if !strings.Contains(md, "radiant context assemble") {
		t.Error("manifest should contain context assemble command")
	}
}

func TestRenderMarkdown_Flavors(t *testing.T) {
	dir := t.TempDir()
	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		flavor AgentFlavor
		hint   string
	}{
		{FlavorClaude, "Claude Code"},
		{FlavorCursor, "Cursor"},
		{FlavorCopilot, "Copilot"},
		{FlavorGeneric, "radiant loop"},
	}
	for _, tt := range tests {
		md := RenderMarkdown(m, tt.flavor)
		if !strings.Contains(md, tt.hint) {
			t.Errorf("flavor=%s: expected %q in markdown", tt.flavor, tt.hint)
		}
	}
}

func TestRenderJSON_Valid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"frontend-app"}`)

	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	jsonStr, err := RenderJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	// Must be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonStr)
	}

	// Required fields
	for _, field := range []string{"version", "project", "recommended_skills", "loop", "budget_estimate"} {
		if _, ok := parsed[field]; !ok {
			t.Errorf("JSON missing required field %q", field)
		}
	}
}

func TestGenerate_ActiveSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-auth")
	os.MkdirAll(specDir, 0o755)
	writeFile(t, filepath.Join(dir, "specs", "0001-auth"), "spec.md", "# Auth Feature\n")

	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if m.ActiveSpec == "" {
		t.Error("expected active spec to be detected")
	}
	if !strings.HasPrefix(m.ActiveSpec, "specs/") {
		t.Errorf("active spec = %q, should start with specs/", m.ActiveSpec)
	}
}

func TestGenerate_ContextFileHint(t *testing.T) {
	dir := t.TempDir()

	// Before context assemble: should show "not yet generated"
	m, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.ContextFile, "not yet generated") && !strings.Contains(m.ContextFile, "CONTEXT.md") {
		t.Errorf("unexpected context file hint: %q", m.ContextFile)
	}

	// After creating CONTEXT.md: should show relative path
	os.MkdirAll(filepath.Join(dir, ".radiant-harness"), 0o755)
	os.WriteFile(filepath.Join(dir, ".radiant-harness", "CONTEXT.md"), []byte("# Context\n"), 0o644)

	m2, err := Generate(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m2.ContextFile, "CONTEXT.md") {
		t.Errorf("expected CONTEXT.md in context file path, got %q", m2.ContextFile)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
