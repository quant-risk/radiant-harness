package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
)

// ── DiffViews ────────────────────────────────────────────────────────────────

func TestDiffViews_AllNew(t *testing.T) {
	dir := t.TempDir()
	diffs := DiffViews(radiant.AgentClaude, dir)
	if len(diffs) == 0 {
		t.Fatal("expected diffs, got none")
	}
	for _, d := range diffs {
		if d.Status != "new" {
			t.Errorf("file %q: expected status=new (empty dir), got %q", d.Path, d.Status)
		}
	}
}

func TestDiffViews_UnchangedAfterWrite(t *testing.T) {
	dir := t.TempDir()

	// Write the generated files to disk
	views := GenerateViewsForAgent(radiant.AgentClaude)
	for _, v := range views {
		dest := filepath.Join(dir, v.Path)
		os.MkdirAll(filepath.Dir(dest), 0o755)
		os.WriteFile(dest, []byte(v.Content), 0o644)
	}

	// Now diff should show all unchanged
	diffs := DiffViews(radiant.AgentClaude, dir)
	for _, d := range diffs {
		if d.Status != "unchanged" {
			t.Errorf("file %q: expected unchanged after write, got %q", d.Path, d.Status)
		}
	}
}

func TestDiffViews_DetectsChange(t *testing.T) {
	dir := t.TempDir()

	views := GenerateViewsForAgent(radiant.AgentCopilot)
	if len(views) == 0 {
		t.Fatal("no views generated for copilot")
	}

	// Write first view with modified content
	dest := filepath.Join(dir, views[0].Path)
	os.MkdirAll(filepath.Dir(dest), 0o755)
	os.WriteFile(dest, []byte("old content that differs"), 0o644)

	diffs := DiffViews(radiant.AgentCopilot, dir)
	found := false
	for _, d := range diffs {
		if d.Path == views[0].Path && d.Status == "changed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected status=changed for modified file %q", views[0].Path)
	}
}

func TestDiffViews_UnknownAgent(t *testing.T) {
	dir := t.TempDir()
	diffs := DiffViews("nonexistent", dir)
	if len(diffs) != 0 {
		t.Errorf("unknown agent should return 0 diffs, got %d", len(diffs))
	}
}

// ── FormatDiff ───────────────────────────────────────────────────────────────

func TestFormatDiff_AllNew(t *testing.T) {
	diffs := []ViewDiff{
		{Path: "a.md", Status: "new"},
		{Path: "b.md", Status: "new"},
	}
	out := FormatDiff(diffs)
	if !strings.Contains(out, "+ a.md") {
		t.Errorf("expected new file marker, got: %s", out)
	}
	if strings.Contains(out, "Nothing to update") {
		t.Error("should not say 'nothing to update' when there are new files")
	}
}

func TestFormatDiff_AllUnchanged(t *testing.T) {
	diffs := []ViewDiff{
		{Path: "a.md", Status: "unchanged"},
		{Path: "b.md", Status: "unchanged"},
	}
	out := FormatDiff(diffs)
	if !strings.Contains(out, "Nothing to update") {
		t.Errorf("expected 'Nothing to update', got: %s", out)
	}
}

func TestFormatDiff_Mixed(t *testing.T) {
	diffs := []ViewDiff{
		{Path: "a.md", Status: "new"},
		{Path: "b.md", Status: "changed"},
		{Path: "c.md", Status: "unchanged"},
	}
	out := FormatDiff(diffs)
	if !strings.Contains(out, "+ a.md") {
		t.Errorf("expected new file in output, got: %s", out)
	}
	if !strings.Contains(out, "~ b.md") {
		t.Errorf("expected changed file in output, got: %s", out)
	}
	if !strings.Contains(out, "1 file(s) unchanged") {
		t.Errorf("expected unchanged count, got: %s", out)
	}
}

// ── EnrichContent ─────────────────────────────────────────────────────────────

func TestEnrichContent_Copilot_AddsBootstrapRef(t *testing.T) {
	base := "# Base content\nsome instructions"
	enriched := EnrichContent(base, radiant.AgentCopilot)
	if !strings.Contains(enriched, "radiant boot") {
		t.Errorf("Copilot enrichment should add bootstrap reference, got:\n%s", enriched)
	}
	if !strings.Contains(enriched, "radiant loop start") {
		t.Errorf("Copilot enrichment should add loop commands, got:\n%s", enriched)
	}
}

func TestEnrichContent_Gemini_AddsBudgetHints(t *testing.T) {
	base := "# Gemini instructions"
	enriched := EnrichContent(base, radiant.AgentGemini)
	if !strings.Contains(enriched, "Token Budget") {
		t.Errorf("Gemini enrichment should add token budget guidance, got:\n%s", enriched)
	}
	if !strings.Contains(enriched, "standard (50K)") {
		t.Errorf("Gemini enrichment should list budget profiles, got:\n%s", enriched)
	}
}

func TestEnrichContent_Cursor_AddsAlwaysApply(t *testing.T) {
	base := "---\ndescription: rules\n---\n# Content"
	enriched := EnrichContent(base, radiant.AgentCursor)
	if !strings.Contains(enriched, "alwaysApply: true") {
		t.Errorf("Cursor enrichment should add alwaysApply, got:\n%s", enriched)
	}
}

func TestEnrichContent_Cursor_NoDoubleAlwaysApply(t *testing.T) {
	base := "---\ndescription: rules\nalwaysApply: true\n---\n# Content"
	enriched := EnrichContent(base, radiant.AgentCursor)
	count := strings.Count(enriched, "alwaysApply:")
	if count != 1 {
		t.Errorf("expected exactly 1 alwaysApply, got %d", count)
	}
}

func TestEnrichContent_Claude_Passthrough(t *testing.T) {
	base := "# Claude content"
	enriched := EnrichContent(base, radiant.AgentClaude)
	if enriched != base {
		t.Errorf("Claude should pass through unchanged, got: %s", enriched)
	}
}

func TestEnrichContent_Unknown_Passthrough(t *testing.T) {
	base := "# some content"
	enriched := EnrichContent(base, "unknown-agent")
	if enriched != base {
		t.Errorf("unknown agent should pass through unchanged")
	}
}
