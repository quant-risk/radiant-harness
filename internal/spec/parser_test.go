package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeACID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"AC-1", "AC-1"},
		{"AC1", "AC-1"},
		{"AC_1", "AC-1"},
		{"ac-1", "AC-1"},
		{"AC 1", "AC-1"},
		{"AC-42", "AC-42"},
		{"ac_42", "AC-42"},
	}
	for _, c := range cases {
		got := NormalizeACID(c.in)
		if got != c.want {
			t.Errorf("NormalizeACID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestACTokensNormalizesVariants(t *testing.T) {
	tokens := ACTokens("Covers AC1, AC_2 and ac-3; also AC-4")
	want := []string{"AC-1", "AC-2", "AC-3", "AC-4"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], want[i])
		}
	}
}

func TestParseSpecHandlesFrontmatterAndACs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	body := `---
name: collect-feedback
description: Golden example spec
---

# Spec — Collect feedback

## Summary
The widget sends feedback (text + context) and the system stores it.

## Acceptance criteria

### AC-1: valid feedback is accepted
- **Given** non-empty text and a context
- **When** the widget sends
- **Then** the feedback is stored and returns an id

### AC-2: empty feedback is rejected
- **Given** empty text (or only spaces)
- **When** sending
- **Then** returns validation error
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "collect-feedback" {
		t.Errorf("name lost: %q", s.Name)
	}
	if s.Summary == "" {
		t.Error("summary not parsed")
	}
	if len(s.ACs) != 2 {
		t.Fatalf("expected 2 ACs, got %d", len(s.ACs))
	}
	if s.ACs[0].ID != "AC-1" || s.ACs[1].ID != "AC-2" {
		t.Errorf("AC IDs not normalized: %+v", s.ACs)
	}
	if s.ACs[0].Given == "" || s.ACs[0].When == "" || s.ACs[0].Then == "" {
		t.Errorf("AC-1 missing G/W/T: %+v", s.ACs[0])
	}
}

func TestParseSpecAcceptsAC1WithoutDash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	body := `---
name: test
---

### AC1: one
- Given: x
- When: y
- Then: z
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ACs) != 1 || s.ACs[0].ID != "AC-1" {
		t.Errorf("AC1 not normalized to AC-1: %+v", s.ACs)
	}
}

func TestParseSpecHandlesAndClause(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	body := `---
name: test
---

### AC-1: multi-clause
- Given: x
- When: y
- Then: z
- And: w
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(s.ACs[0].Then, "and w") {
		t.Errorf("And clause not appended to Then: %q", s.ACs[0].Then)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (s[len(s)-len(sub):] == sub || s[:len(sub)] == sub || hasSubstr(s, sub))))
}

func hasSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParseTasksNormalizesACReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	body := `| # | Task | Covers AC | Depends on | Gate | Status |
|---|------|-----------|------------|------|--------|
| 1 | Validate input | AC2, ac-3 · AC_4 | — | npm test | done |
| 2 | Store | AC1 | 1 | npm test | done |
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(p.Tasks))
	}
	if len(p.Tasks[0].CoversACs) != 3 {
		t.Errorf("expected 3 ACs in task 1, got %v", p.Tasks[0].CoversACs)
	}
	for _, ac := range p.Tasks[0].CoversACs {
		if ac[2] != '-' {
			t.Errorf("AC not normalized: %q", ac)
		}
	}
}

func TestParseTasksGroupsParallelPhases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	body := `| # | Task | Covers AC | Depends on | Gate | Status |
|---|------|-----------|------------|------|--------|
| 1 | Build A | AC-1 | — | npm test | done |
| 2 | [P] Build B | AC-2 | — | npm test | done |
| 3 | [P] Build C | AC-3 | — | npm test | done |
| 4 | Wrap up | AC-4 | 2,3 | npm test | done |
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Phases) < 3 {
		t.Errorf("expected at least 3 phases, got %d", len(p.Phases))
	}
	// Phase 2 should contain the parallel tasks B and C.
	if len(p.Phases) >= 2 {
		parallelPhase := p.Phases[1]
		if len(parallelPhase.Tasks) != 2 {
			t.Errorf("expected 2 parallel tasks in phase 2, got %d", len(parallelPhase.Tasks))
		}
	}
}

func TestParseTasksSkipsHeaderAndSeparator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	body := `Some intro paragraph.

| # | Task | Covers AC | Depends on | Gate | Status |
|---|------|-----------|------------|------|--------|
| 1 | Only task | AC-1 | — | npm test | done |
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(p.Tasks))
	}
}
