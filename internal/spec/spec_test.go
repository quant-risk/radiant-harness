package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpec(t *testing.T) {
	dir := t.TempDir()
	specContent := `---
name: test-spec
description: A test spec
alwaysApply: true
---

# Spec — Test Feature

## Summary
The system collects feedback.

## Acceptance criteria

### AC-1: valid feedback is accepted
- **Given** non-empty text
- **When** the widget sends
- **Then** feedback is stored and returns an id

### AC-2: empty feedback is rejected
- **Given** empty text
- **When** sending
- **Then** returns validation error

## Out of scope
- Moderation
`
	specFile := filepath.Join(dir, "spec.md")
	os.WriteFile(specFile, []byte(specContent), 0o644)

	spec, err := ParseSpec(specFile)
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if spec.Name != "test-spec" {
		t.Errorf("expected name 'test-spec', got '%s'", spec.Name)
	}
	if spec.Summary != "The system collects feedback." {
		t.Errorf("unexpected summary: %s", spec.Summary)
	}
	if len(spec.ACs) != 2 {
		t.Fatalf("expected 2 ACs, got %d", len(spec.ACs))
	}
	if spec.ACs[0].ID != "AC-1" {
		t.Errorf("expected AC-1, got %s", spec.ACs[0].ID)
	}
	if spec.ACs[0].Given != "non-empty text" {
		t.Errorf("unexpected Given: %s", spec.ACs[0].Given)
	}
	if spec.ACs[1].ID != "AC-2" {
		t.Errorf("expected AC-2, got %s", spec.ACs[1].ID)
	}
}

func TestACTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"AC-1 and AC-2", 2},
		{"AC_1 duplicate AC-1", 1},
		{"no acs here", 0},
		{"AC-1 AC-2 AC-3", 3},
	}
	for _, tt := range tests {
		got := ACTokens(tt.input)
		if len(got) != tt.want {
			t.Errorf("ACTokens(%q) = %d, want %d", tt.input, len(got), tt.want)
		}
	}
}
