package fs

import (
	"path/filepath"
)

// absProjectDir returns the absolute, symlink-resolved path of the
// project root. Used by both read_file and search_code so they share
// the same path canonicalisation.
func absProjectDir(projectDir string) (string, error) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return "", err
	}
	// Resolve symlinks at the project root if it exists. This matches
	// the behaviour of fsutil.PathIsSafe so both functions agree on
	// what "inside the project" means.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// joinPath joins an absolute project root with a project-relative
// candidate path. Separated from filepath.Join so callers don't
// accidentally double-join (e.g. when projectDir is already absolute).
func joinPath(absProject, candidate string) string {
	return filepath.Join(absProject, candidate)
}