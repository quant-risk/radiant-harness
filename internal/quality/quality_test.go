package quality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditPipeline(t *testing.T) {
	dir := t.TempDir()

	// Valid doc
	validDoc := "---\nname: test\ndescription: Test doc\nalwaysApply: false\n---\nBody"
	os.WriteFile(filepath.Join(dir, "test.md"), []byte(validDoc), 0o644)

	result := AuditPipeline(dir)
	if !result.OK {
		t.Errorf("expected OK, got errors: %v", result.Errors)
	}
}

func TestAuditMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.md"), []byte("# No frontmatter"), 0o644)

	result := AuditPipeline(dir)
	if result.OK {
		t.Error("expected failure for missing frontmatter")
	}
}

func TestAuditMissingAlwaysApply(t *testing.T) {
	dir := t.TempDir()
	doc := "---\nname: test\ndescription: Test\n---\nBody"
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte(doc), 0o644)

	result := AuditPipeline(dir)
	if result.OK {
		t.Error("expected failure for missing alwaysApply")
	}
}

func TestEvalSpecFidelity(t *testing.T) {
	dir := t.TempDir()

	// Create spec with ACs
	specDir := filepath.Join(dir, "specs", "0001-test")
	os.MkdirAll(specDir, 0o755)

	specContent := "### AC-1: test\nGiven X\nWhen Y\nThen Z"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	// Tasks covering AC-1
	taskContent := "| 1 | Task | AC-1 | — | test | todo |"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	result := EvalSpecFidelity(dir)
	if !result.OK {
		t.Errorf("expected OK, got errors: %v", result.Errors)
	}
}

func TestEvalMissingTaskCoverage(t *testing.T) {
	dir := t.TempDir()

	specDir := filepath.Join(dir, "specs", "0001-test")
	os.MkdirAll(specDir, 0o755)

	specContent := "### AC-1: test\nGiven X\nWhen Y\nThen Z\n\n### AC-2: other\nGiven A\nWhen B\nThen C"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	// Tasks only cover AC-1
	taskContent := "| 1 | Task | AC-1 | — | test | todo |"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	result := EvalSpecFidelity(dir)
	if result.OK {
		t.Error("expected failure — AC-2 has no task coverage")
	}
}
