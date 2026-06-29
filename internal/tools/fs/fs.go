// Package fs provides concrete filesystem tools for the radiant-harness
// tool registry. Each tool's Invoke function enforces the project
// boundary via engine.PathIsSafe (symlink-aware) and writes atomically
// (temp file + fsync + rename) so a crash mid-write doesn't leave a
// half-written file in the project tree.
//
// Status: Sprint 69 (v2.38.0) — only WriteFile is concrete.
// ReadFile, Search, RunGate land in Sprints 70-71.
package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quant-risk/radiant-harness/internal/fsutil"
	"github.com/quant-risk/radiant-harness/internal/tools"
)

// MaxWriteBytes caps the content size accepted by write_file. The LLM
// can emit large blobs (10k+ tokens), but a runaway model emitting a
// 50MB file would DoS the project. 4 MiB is generous for source code
// and configuration; raise this if a real workload needs more.
const MaxWriteBytes = 4 * 1024 * 1024

// WriteArgs is the typed shape of the LLM-emitted write_file args.
// Marshalled/Unmarshalled at the boundary so a malformed payload is
// rejected before any filesystem side effect.
type WriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteResult is what the tool returns to the executor. The tracer
// picks these fields up for the verifier prompt.
type WriteResult struct {
	Written   string `json:"written"`
	Bytes     int    `json:"bytes"`
	Created   bool   `json:"created"`
	Existed   bool   `json:"existed"`
	ProjectOK bool   `json:"project_ok"` // mirrors PathIsSafe — surfaces "safe" in trace
}

// Annotate satisfies the duck-typed annotator interface declared in
// internal/engine. The executor's type-switch picks this up to surface
// trace-friendly metadata (bytes written, paths, project-boundary OK
// flag) in the verifier prompt.
//
// Keys are stable and consumed by:
//   - engine.applyToolCalls (records bytes/written/created/project_ok)
//   - loop.BuildVerifierPrompt (renders the TOOL CALLS OBSERVED section)
// Keep these names in sync if you ever add new ones.
func (r WriteResult) Annotate() map[string]any {
	return map[string]any{
		"written":    r.Written,
		"bytes":      r.Bytes,
		"created":    r.Created,
		"existed":    r.Existed,
		"project_ok": r.ProjectOK,
	}
}

// WriteFileTool returns the write_file tool bound to the given project
// directory. Pass the result to a tools.Registry via Register.
//
// The project dir is captured at construction time so the tool stays
// useful across iterations (the executor's Run can construct it once
// and reuse the same registry). Path safety is re-checked on every
// invocation so a hostile LLM can't bypass via the trace.
func WriteFileTool(projectDir string) *tools.Tool {
	return &tools.Tool{
		Name: "write_file",
		Description: "Write content to a file at the given path (project-relative). " +
			"Creates parent directories as needed. Path must resolve inside the project " +
			"directory after symlink resolution. Atomic: a crash mid-write leaves the " +
			"previous file intact (or no file if it didn't exist).",
		Params: []tools.Param{
			{Name: "path", Type: "string", Required: true,
				Description: "Project-relative path (e.g. \"internal/foo/bar.go\")."},
			{Name: "content", Type: "string", Required: true,
				Description: "complete file contents (UTF-8). Max 4 MiB."},
		},
		Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
			return invokeWriteFile(ctx, projectDir, raw)
		},
	}
}

// invokeWriteFile is the actual implementation. Exposed unexported
// so the test file can call it directly without going through JSON.
func invokeWriteFile(ctx context.Context, projectDir string, raw json.RawMessage) (WriteResult, error) {
	var args WriteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return WriteResult{}, fmt.Errorf("write_file: invalid args: %w", err)
	}
	if args.Path == "" {
		return WriteResult{}, fmt.Errorf("write_file: path is required")
	}
	if strings.TrimSpace(args.Path) == "" {
		return WriteResult{}, fmt.Errorf("write_file: path is whitespace-only")
	}
	if len(args.Content) > MaxWriteBytes {
		return WriteResult{}, fmt.Errorf("write_file: content too large (%d bytes; max %d)",
			len(args.Content), MaxWriteBytes)
	}

	// Boundary check — fsutil.PathIsSafe resolves symlinks before
	// checking. A symlinked project subdir pointing outside is caught
	// here, not at the final write.
	if !fsutil.PathIsSafe(projectDir, args.Path) {
		return WriteResult{}, fmt.Errorf("write_file: refusing path %q — resolves outside project", args.Path)
	}

	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return WriteResult{}, fmt.Errorf("write_file: abs project: %w", err)
	}
	full := filepath.Join(absProject, args.Path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("write_file: mkdir %s: %w", dir, err)
	}

	// Did the file exist before? (For the trace.)
	existed := false
	if _, err := os.Stat(full); err == nil {
		existed = true
	}

	// Atomic write: temp file in the same dir (so rename is on the
	// same filesystem), write, fsync, close, rename. A SIGKILL between
	// the temp write and the rename leaves the temp file — cleaned up
	// by the next MkdirAll or by an OS-level sweep — and the original
	// file untouched.
	tmp, err := os.CreateTemp(dir, ".write-*.tmp")
	if err != nil {
		return WriteResult{}, fmt.Errorf("write_file: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		// Best-effort cleanup on any failure path.
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write([]byte(args.Content)); err != nil {
		tmp.Close()
		cleanup()
		return WriteResult{}, fmt.Errorf("write_file: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return WriteResult{}, fmt.Errorf("write_file: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return WriteResult{}, fmt.Errorf("write_file: close temp: %w", err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		cleanup()
		return WriteResult{}, fmt.Errorf("write_file: rename: %w", err)
	}

	// Honour cancellation between the rename and the trace return.
	if err := ctx.Err(); err != nil {
		return WriteResult{}, fmt.Errorf("write_file: context cancelled: %w", err)
	}

	return WriteResult{
		Written:   args.Path,
		Bytes:     len(args.Content),
		Created:   !existed,
		Existed:   existed,
		ProjectOK: true,
	}, nil
}