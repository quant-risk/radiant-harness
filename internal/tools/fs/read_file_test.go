package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── read_file tests ──────────────────────────────────────────────────────────

func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hello.txt")
	want := "line one\nline two\nline three"
	if err := os.WriteFile(target, []byte(want), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	res, err := invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Path != "hello.txt" {
		t.Errorf("Path: got %q want hello.txt", res.Path)
	}
	if res.Content != want {
		t.Errorf("Content mismatch: got %q want %q", res.Content, want)
	}
	if res.Bytes != len(want) {
		t.Errorf("Bytes: got %d want %d", res.Bytes, len(want))
	}
	if res.Lines != 3 {
		t.Errorf("Lines: got %d want 3", res.Lines)
	}
}

func TestReadFile_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "no_trail.txt")
	want := "one\ntwo" // 2 lines, no trailing newline
	if err := os.WriteFile(target, []byte(want), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	res, err := invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"no_trail.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Lines != 2 {
		t.Errorf("Lines (no trailing newline): got %d want 2", res.Lines)
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	res, err := invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"empty.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Bytes != 0 || res.Lines != 0 {
		t.Errorf("empty file: bytes=%d lines=%d want 0/0", res.Bytes, res.Lines)
	}
}

func TestReadFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"does_not_exist.txt"}`))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestReadFile_Directory(t *testing.T) {
	dir := t.TempDir()
	_, err := invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"."}`))
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error should mention directory: %v", err)
	}
}

func TestReadFile_RejectsUnsafePath(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		`{"path":"../escape.txt"}`,
		`{"path":"../../etc/passwd"}`,
	}
	for _, raw := range cases {
		_, err := invokeReadFile(context.Background(), dir, json.RawMessage(raw))
		if err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestReadFile_RejectsSymlinkedProjectSubdir(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	link := filepath.Join(project, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	_, err := invokeReadFile(context.Background(), project,
		json.RawMessage(`{"path":"evil/secret.txt"}`))
	if err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
}

func TestReadFile_RejectsOversizeFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "huge.txt")
	// Create a sparse file bigger than MaxReadBytes via truncate.
	f, err := os.Create(target)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := f.Truncate(MaxReadBytes + 1); err != nil {
		f.Close()
		t.Fatalf("truncate: %v", err)
	}
	f.Close()

	_, err = invokeReadFile(context.Background(), dir,
		json.RawMessage(`{"path":"huge.txt"}`))
	if err == nil {
		t.Fatal("expected error for oversize file, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention size: %v", err)
	}
}

func TestReadFile_RejectsEmptyPath(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		`{"path":""}`,
		`{"path":"   "}`,
		`{}`,
	}
	for _, raw := range cases {
		_, err := invokeReadFile(context.Background(), dir, json.RawMessage(raw))
		if err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestReadFile_RejectsMalformedArgs(t *testing.T) {
	dir := t.TempDir()
	_, err := invokeReadFile(context.Background(), dir, json.RawMessage(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestReadFile_Annotate(t *testing.T) {
	r := ReadResult{Path: "x.go", Content: "...", Bytes: 3, Lines: 1}
	m := r.Annotate()
	if m["path"] != "x.go" || m["bytes"] != 3 || m["lines"] != 1 {
		t.Errorf("Annotate: got %+v", m)
	}
}

func TestReadFile_ViaRegistry(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "via_registry.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	tool := ReadFileTool(dir)
	got, err := tool.Invoke(context.Background(),
		json.RawMessage(`{"path":"via_registry.txt"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	res, ok := got.(ReadResult)
	if !ok {
		t.Fatalf("result type: got %T want ReadResult", got)
	}
	if res.Content != "hello" {
		t.Errorf("Content: got %q want hello", res.Content)
	}
}