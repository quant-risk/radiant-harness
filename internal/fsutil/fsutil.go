// Package fsutil contains filesystem utilities shared across packages
// without creating import cycles. Currently hosts PathIsSafe, which is
// depended on by both internal/engine (legacy code-block path) and
// internal/tools/fs (write_file tool).
//
// Kept deliberately small — anything more than path/byte safety
// belongs in a domain-specific package (engine, fs, etc.).
package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// PathIsSafe reports whether `candidate` (a project-relative path)
// resolves to a location inside `projectDir` after symlink resolution.
//
// Symlinks are resolved before the boundary check, so a symlink inside
// the project that points outside is still rejected. The original
// implementation in internal/engine only checked the textual path,
// which let an attacker (or a confused LLM) create "../../etc/foo" via
// a symlink and bypass the gate.
//
// Strategy: resolve both the project root and the longest existing
// prefix of the candidate path. If the file doesn't exist yet (LLM
// proposing a new file), walk up the path until we find a directory
// that does, resolve that, and check the prefix. This catches the
// symlink-escape case where e.g. `evil/target.txt` passes the lexical
// check but `evil` is a symlink pointing outside the project.
func PathIsSafe(projectDir, candidate string) bool {
	if candidate == "" {
		return false
	}
	absProj, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}

	// Resolve the project root if it exists.
	if fileExists(absProj) {
		if r, err := filepath.EvalSymlinks(absProj); err == nil {
			absProj = r
		}
	}

	// Resolve the candidate path by walking up until we find something
	// that exists. For "evil/target.txt" where evil is a symlink but
	// target.txt doesn't exist, we resolve "evil" and detect the escape.
	full := filepath.Join(absProj, candidate)
	resolved := resolveLongestExistingPrefix(full)

	rel, err := filepath.Rel(absProj, resolved)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// resolveLongestExistingPrefix walks up `full` until it finds a path
// that exists (file, dir, or symlink), then returns the EvalSymlinks
// result of that path. If no prefix exists, returns `full` unchanged.
func resolveLongestExistingPrefix(full string) string {
	cur := full
	for {
		if fileExists(cur) {
			if r, err := filepath.EvalSymlinks(cur); err == nil {
				return r
			}
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root without finding anything; give up.
			return full
		}
		cur = parent
	}
}

// fileExists reports whether p exists (file, dir, symlink — anything).
func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}