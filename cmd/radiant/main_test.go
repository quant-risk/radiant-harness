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
