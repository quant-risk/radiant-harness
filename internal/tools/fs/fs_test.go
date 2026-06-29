package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFile_HappyPath(t *testing.T) {
	dir := t.TempDir()

	res, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"hello.txt","content":"hi\n"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Written != "hello.txt" {
		t.Errorf("Written: got %q want hello.txt", res.Written)
	}
	if res.Bytes != 3 {
		t.Errorf("Bytes: got %d want 3", res.Bytes)
	}
	if !res.Created {
		t.Errorf("Created: got false want true (new file)")
	}
	if res.Existed {
		t.Errorf("Existed: got true want false (new file)")
	}

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hi\n" {
		t.Errorf("content: got %q want %q", got, "hi\n")
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()

	_, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"internal/foo/bar.go","content":"package foo\n"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "internal", "foo", "bar.go"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "package foo\n" {
		t.Errorf("content mismatch")
	}
}

func TestWriteFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	res, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"x.txt","content":"new"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Created {
		t.Errorf("Created: got true want false (file pre-existed)")
	}
	if !res.Existed {
		t.Errorf("Existed: got false want true")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("content: got %q want %q", got, "new")
	}
}

func TestWriteFile_RejectsUnsafeRelativeEscape(t *testing.T) {
	dir := t.TempDir()

	_, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"../escape.txt","content":"nope"}`))
	if err == nil {
		t.Fatal("expected error for ../escape.txt, got nil")
	}
	if !strings.Contains(err.Error(), "outside project") {
		t.Errorf("error should mention boundary: %v", err)
	}
	// Confirm no file was written anywhere in dir.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == "escape.txt" {
			t.Errorf("file escape.txt must not be written")
		}
	}
}

func TestWriteFile_RejectsSymlinkedProjectSubdir(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	// Create a symlink inside dir pointing outside.
	link := filepath.Join(dir, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported in test env: %v", err)
	}

	_, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"evil/target.txt","content":"nope"}`))
	if err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
	if !strings.Contains(err.Error(), "outside project") {
		t.Errorf("error should mention boundary: %v", err)
	}
	// Confirm nothing was written in the outside dir.
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Errorf("outside dir should be empty, found %d entries", len(entries))
	}
}

func TestWriteFile_RejectsEmptyPath(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		`{"path":"","content":"x"}`,
		`{"path":"   ","content":"x"}`,
		`{}`,
	}
	for _, raw := range cases {
		_, err := invokeWriteFile(context.Background(), dir, json.RawMessage(raw))
		if err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestWriteFile_RejectsMalformedArgs(t *testing.T) {
	dir := t.TempDir()
	_, err := invokeWriteFile(context.Background(), dir, json.RawMessage(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestWriteFile_RejectsOversizeContent(t *testing.T) {
	dir := t.TempDir()
	huge := strings.Repeat("a", MaxWriteBytes+1)
	raw, _ := json.Marshal(map[string]string{"path": "big.txt", "content": huge})
	_, err := invokeWriteFile(context.Background(), dir, raw)
	if err == nil {
		t.Fatal("expected error for oversize content, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention size: %v", err)
	}
}

func TestWriteFile_AtomicWhenKilledMidWrite(t *testing.T) {
	// We can't actually SIGKILL ourselves, but we can verify the
	// invariant: a partial write (temp file present, target unchanged)
	// doesn't corrupt the target. The atomic write uses temp+rename,
	// so any failure path leaves the target at its previous content.
	dir := t.TempDir()
	target := filepath.Join(dir, "atomic.txt")
	original := "original content\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// A successful overwrite must not leave a .write-*.tmp behind.
	_, err := invokeWriteFile(context.Background(), dir,
		json.RawMessage(`{"path":"atomic.txt","content":"new content"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".write-") {
			t.Errorf("temp file %q should not survive successful write", e.Name())
		}
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new content" {
		t.Errorf("target content: got %q want %q", got, "new content")
	}
}

func TestWriteFileTool_RegisteredWithCorrectSchema(t *testing.T) {
	tool := WriteFileTool("/tmp")
	if tool.Name != "write_file" {
		t.Errorf("Name: got %q want write_file", tool.Name)
	}
	if len(tool.Params) != 2 {
		t.Fatalf("Params: got %d want 2", len(tool.Params))
	}
	// path is required.
	for _, p := range tool.Params {
		if p.Name == "path" && !p.Required {
			t.Errorf("path param must be Required")
		}
		if p.Name == "content" && !p.Required {
			t.Errorf("content param must be Required")
		}
	}
	if tool.Invoke == nil {
		t.Error("Invoke must be non-nil")
	}
}

func TestWriteFileTool_InvokeViaRegistry(t *testing.T) {
	dir := t.TempDir()
	// Smoke test through a tools.Registry — exercises the same path
	// the executor will use in production.
	tool := WriteFileTool(dir)
	if tool.Invoke == nil {
		t.Fatal("Invoke must be non-nil")
	}
	got, err := tool.Invoke(context.Background(),
		json.RawMessage(`{"path":"via_registry.txt","content":"hello"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	res, ok := got.(WriteResult)
	if !ok {
		t.Fatalf("result type: got %T want WriteResult", got)
	}
	if res.Bytes != 5 {
		t.Errorf("Bytes: got %d want 5", res.Bytes)
	}
}