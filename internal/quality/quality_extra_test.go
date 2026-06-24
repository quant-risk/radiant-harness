package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractACDetailsWithAndClause(t *testing.T) {
	content := `---
name: test
---

### AC-1: multi-clause
- Given: x
- When: y
- Then: z
- And: w

### AC-2: simple
- Given: a
- When: b
- Then: c
`
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = path
	acs := extractACDetails(content)
	if len(acs) != 2 {
		t.Fatalf("expected 2 ACs, got %d", len(acs))
	}
	if !strings.Contains(acs[0].Then, "and w") {
		t.Errorf("AC-1 should have And appended to Then, got %q", acs[0].Then)
	}
	if acs[1].Then != "c" {
		t.Errorf("AC-2 Then = %q, want 'c'", acs[1].Then)
	}
}

func TestExtractACDetailsEmpty(t *testing.T) {
	acs := extractACDetails("")
	if len(acs) != 0 {
		t.Errorf("expected 0 ACs from empty input, got %d", len(acs))
	}
}

func TestExtractACDetailsMultipleACs(t *testing.T) {
	content := `### AC-1: first
- Given: a
- When: b
- Then: c

### AC-2: second
- Given: d
- When: e
- Then: f

### AC-3: third
- Given: g
- When: h
- Then: i
`
	acs := extractACDetails(content)
	if len(acs) != 3 {
		t.Fatalf("expected 3 ACs, got %d", len(acs))
	}
	if acs[0].ID != "AC-1" || acs[2].ID != "AC-3" {
		t.Errorf("AC IDs not in order: %+v", acs)
	}
}

func TestExtractACDetailsMalformed(t *testing.T) {
	// AC header without Given/When/Then
	content := `### AC-1: incomplete
Just a description, no clauses.
`
	acs := extractACDetails(content)
	if len(acs) != 1 {
		t.Fatalf("expected 1 AC, got %d", len(acs))
	}
	if acs[0].ID != "AC-1" {
		t.Errorf("ID = %q, want AC-1", acs[0].ID)
	}
	// The ValidateFeature check on this would fail; the parser is
	// permissive and accepts incomplete ACs.
}

func TestExtractACDetailsCaseInsensitive(t *testing.T) {
	content := `### AC-1: lowercase
- given: a
- when: b
- then: c
`
	acs := extractACDetails(content)
	if len(acs) != 1 {
		t.Fatalf("expected 1 AC, got %d", len(acs))
	}
	if acs[0].Given != "a" || acs[0].When != "b" || acs[0].Then != "c" {
		t.Errorf("clauses not parsed: %+v", acs[0])
	}
}

func TestSplitShellTokensQualityPackage(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"echo ok", []string{"echo", "ok"}},
		{`echo "build-ok"`, []string{"echo", "build-ok"}},
		{"a || b", []string{"a", "|", "|", "b"}},
		{"a;b", []string{"a", ";", "b"}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitShellTokens(c.in)
		if !equalSlices(got, c.want) {
			t.Errorf("splitShellTokens(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsShellOpQualityPackage(t *testing.T) {
	yes := []string{"&&", "||", "|", ";", "&", ">", "<", "(", ")"}
	no := []string{"echo", "test", "a", "b"}
	for _, s := range yes {
		if !isShellOp(s) {
			t.Errorf("%q should be a shell op", s)
		}
	}
	for _, s := range no {
		if isShellOp(s) {
			t.Errorf("%q should NOT be a shell op", s)
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExtractGatesWithMultipleTasks(t *testing.T) {
	content := `| # | Task | AC | Dep | Gate | Status |
|---|------|----|-----|------|--------|
| 1 | Build | AC-1 | — | ` + "`npm test`" + ` | done |
| 2 | Test | AC-2 | 1 | ` + "`go test ./...`" + ` | done |
| 3 | Re-run | AC-1 | — | ` + "`npm test`" + ` | done |  // duplicate
`
	gates := extractGates(content)
	if len(gates) != 2 {
		t.Errorf("expected 2 unique gates, got %d: %v", len(gates), gates)
	}
}

func TestExtractGatesEmpty(t *testing.T) {
	gates := extractGates("")
	if len(gates) != 0 {
		t.Errorf("expected 0 gates from empty content, got %d", len(gates))
	}
}

func TestExtractGatesIncludesAllBacktickedText(t *testing.T) {
	// All backticked text in a tasks.md is treated as a potential gate —
	// allowlist validation rejects what's not a real binary. This way
	// `true`, `pwd`, and `npm test` all get parsed; the allowlist decides
	// which run.
	content := `Inline code: ` + "`const`" + ` might be a gate.
And this is real: ` + "`echo ok`" + ` should match.`
	gates := extractGates(content)
	if len(gates) != 2 {
		t.Errorf("expected 2 backticked items, got %d: %v", len(gates), gates)
	}
}

func TestRunGatesReadsTasksFile(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-test")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasksContent := "| # | Task | AC | Dep | Gate | Status |\n" +
		"|---|------|----|-----|------|--------|\n" +
		"| 1 | Test | AC-1 | — | `true` | todo |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasksContent), 0o644); err != nil {
		t.Fatal(err)
	}
	results := RunGates(dir, specDir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("`true` should pass, got: %+v", results[0])
	}
}

func TestRunGatesHandlesMissingTasksFile(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-missing")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	results := RunGates(dir, specDir)
	if len(results) != 1 {
		t.Fatalf("expected 1 error result for missing tasks.md, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("missing tasks.md should produce a failed result")
	}
}

func TestRunGatesRejectsForbiddenBinary(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-evil")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasksContent := "| # | Task | AC | Dep | Gate | Status |\n" +
		"|---|------|----|-----|------|--------|\n" +
		"| 1 | Evil | AC-1 | — | `rm -rf /` | todo |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasksContent), 0o644); err != nil {
		t.Fatal(err)
	}
	results := RunGates(dir, specDir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("rm -rf / should be skipped by allowlist")
	}
	if results[0].Reason == "" {
		t.Error("skipped gate should have a reason")
	}
}

func TestSplitOnLogicalOpsQualityPackage(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a && b", []string{"a ", " b"}},
		{"a || b || c", []string{"a ", " b ", " c"}},
		{"echo hi", []string{"echo hi"}},
	}
	for _, c := range cases {
		got := splitOnLogicalOps(c.in)
		if !equalSlices(got, c.want) {
			t.Errorf("splitOnLogicalOps(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
