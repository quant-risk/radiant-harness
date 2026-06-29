package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupSearchTree creates a project tree:
//   .
//   ├── main.go         (contains "TODO" and "FIXME")
//   ├── README.md       (contains "TODO")
//   ├── .git/HEAD       (should be skipped)
//   ├── .radiant-harness/state.json (should be skipped)
//   ├── node_modules/lib/index.js (should be skipped)
//   ├── src/
//   │   ├── app.go      (contains "FIXME")
//   │   └── app_test.go (contains "TODO")
//   └── binary.bin      (binary content, should be skipped)
// Returns the project dir.
func setupSearchTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	must := func(rel, content string, mode os.FileMode) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), mode); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	must("main.go", "package main\n// TODO: real impl\nfunc main() {}\n", 0o644)
	must("README.md", "# Project\n\nTODO list\n", 0o644)
	must(".git/HEAD", "ref: refs/heads/main\nTODO inside git\n", 0o644)
	must(".radiant-harness/state.json", `{"phase":"execute","todo":"hidden"}`, 0o644)
	must("node_modules/lib/index.js", "// TODO: hidden in deps\n", 0o644)
	must("src/app.go", "package app\n// FIXME: optimise this\n", 0o644)
	must("src/app_test.go", "package app\n// TODO: write the test\n", 0o644)
	// Binary content (PNG header is 8 bytes; pad with zeros).
	must("binary.bin", "\x89PNG\r\n\x1a\n"+strings.Repeat("\x00", 100), 0o644)

	return dir
}

// ── search_code tests ────────────────────────────────────────────────────────

func TestSearchCode_FindsMatches(t *testing.T) {
	dir := setupSearchTree(t)
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"TODO"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.MatchCount == 0 {
		t.Fatal("expected matches, got 0")
	}
	// Should find TODO in main.go, README.md, src/app_test.go.
	// Should NOT find TODO in .git/HEAD, .radiant-harness, node_modules.
	found := map[string]bool{}
	for _, m := range res.Matches {
		found[m.File] = true
	}
	for _, want := range []string{"main.go", "README.md", "src/app_test.go"} {
		if !found[want] {
			t.Errorf("expected match in %s, not found", want)
		}
	}
	for _, banned := range []string{".git/HEAD", ".radiant-harness/state.json", "node_modules/lib/index.js"} {
		if found[banned] {
			t.Errorf("did not expect match in %s (hidden dir)", banned)
		}
	}
}

func TestSearchCode_FindsMultipleMatchesInSameLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("TODO one TODO two TODO three"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"TODO"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 3 {
		t.Errorf("got %d matches, want 3 (one per occurrence on same line)", len(res.Matches))
	}
	if res.Matches[0].Column != 1 || res.Matches[1].Column != 10 || res.Matches[2].Column != 19 {
		t.Errorf("columns: got %d %d %d want 1 10 19",
			res.Matches[0].Column, res.Matches[1].Column, res.Matches[2].Column)
	}
}

func TestSearchCode_NoMatches(t *testing.T) {
	dir := setupSearchTree(t)
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"NONEXISTENT_PATTERN_XYZ_123"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.MatchCount != 0 {
		t.Errorf("MatchCount: got %d want 0", res.MatchCount)
	}
	if len(res.Matches) != 0 {
		t.Errorf("Matches: got %d want 0", len(res.Matches))
	}
}

func TestSearchCode_InvalidRegex(t *testing.T) {
	dir := setupSearchTree(t)
	_, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"[invalid("}`))
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
	if !strings.Contains(err.Error(), "invalid regex") {
		t.Errorf("error should mention invalid regex: %v", err)
	}
}

func TestSearchCode_EmptyPattern(t *testing.T) {
	dir := setupSearchTree(t)
	cases := []string{`{"pattern":""}`, `{"pattern":"   "}`, `{}`}
	for _, raw := range cases {
		_, err := invokeSearchCode(context.Background(), dir, json.RawMessage(raw))
		if err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestSearchCode_RespectsScope(t *testing.T) {
	dir := setupSearchTree(t)
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"FIXME","path":"src"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.MatchCount != 1 {
		t.Errorf("MatchCount: got %d want 1 (only src/app.go)", res.MatchCount)
	}
	for _, m := range res.Matches {
		if !strings.HasPrefix(m.File, "src/") {
			t.Errorf("match outside src/: %s", m.File)
		}
	}
}

func TestSearchCode_RespectsIncludeGlob(t *testing.T) {
	dir := setupSearchTree(t)
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"TODO","include":"*.md"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range res.Matches {
		if !strings.HasSuffix(m.File, ".md") {
			t.Errorf("non-md match: %s", m.File)
		}
	}
}

func TestSearchCode_RejectsUnsafeScope(t *testing.T) {
	dir := setupSearchTree(t)
	_, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"x","path":"../outside"}`))
	if err == nil {
		t.Fatal("expected error for unsafe scope, got nil")
	}
	if !strings.Contains(err.Error(), "outside project") {
		t.Errorf("error should mention boundary: %v", err)
	}
}

func TestSearchCode_RespectsMaxResults(t *testing.T) {
	dir := t.TempDir()
	// Create 50 files with the same pattern.
	for i := 0; i < 50; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+".go")
		if err := os.WriteFile(name, []byte("package x\n// TODO\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := invokeSearchCode(context.Background(), dir,
		json.RawMessage(`{"pattern":"TODO","max_results":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) > 5 {
		t.Errorf("got %d matches, max_results should cap at 5", len(res.Matches))
	}
	if !res.Truncated {
		t.Errorf("Truncated should be true when cap is hit")
	}
}

func TestSearchCode_Annotate(t *testing.T) {
	r := SearchResult{Pattern: "TODO", Root: ".", MatchCount: 3, Truncated: false}
	m := r.Annotate()
	if m["pattern"] != "TODO" || m["match_count"] != 3 {
		t.Errorf("Annotate: got %+v", m)
	}
}

func TestSearchCode_ViaRegistry(t *testing.T) {
	dir := setupSearchTree(t)
	tool := SearchCodeTool(dir)
	got, err := tool.Invoke(context.Background(),
		json.RawMessage(`{"pattern":"TODO","max_results":2}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	res, ok := got.(SearchResult)
	if !ok {
		t.Fatalf("result type: got %T want SearchResult", got)
	}
	if res.MatchCount == 0 {
		t.Error("expected at least 1 match via registry")
	}
}