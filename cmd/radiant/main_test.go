package main

import (
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
