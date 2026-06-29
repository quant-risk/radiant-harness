package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathIsSafe_NormalPaths(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		candidate string
		want      bool
	}{
		{"src/main.go", true},
		{"docs/spec.md", true},
		{"a/b/c/d.txt", true},
		{"", false},
		{"../escape.txt", false},
		{"../../etc/passwd", false},
	}
	for _, c := range cases {
		if got := PathIsSafe(dir, c.candidate); got != c.want {
			t.Errorf("PathIsSafe(%q, %q) = %v, want %v", dir, c.candidate, got, c.want)
		}
	}
}

func TestPathIsSafe_SymlinkEscape(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()

	// Symlink inside project pointing outside.
	link := filepath.Join(project, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported in test env: %v", err)
	}

	if PathIsSafe(project, "evil/target.txt") {
		t.Errorf("PathIsSafe should reject writes through symlink that escapes project")
	}
	if !PathIsSafe(project, "src/main.go") {
		t.Errorf("PathIsSafe should accept a normal in-project path")
	}
}

func TestPathIsSafe_NestedSymlinkedProjectRoot(t *testing.T) {
	// The project root itself is reached via a symlink. Writes should
	// still be confined to the resolved root.
	real := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "linked-project")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	if !PathIsSafe(link, "src/main.go") {
		t.Errorf("PathIsSafe should accept writes under a symlinked project root")
	}
	// Writes that escape the symlink target should still be rejected.
	other := t.TempDir()
	otherLink := filepath.Join(real, "out")
	if err := os.Symlink(other, otherLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	if PathIsSafe(link, "out/target.txt") {
		t.Errorf("PathIsSafe should reject writes through nested escape symlink")
	}
}