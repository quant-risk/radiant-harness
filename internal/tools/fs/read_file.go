package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/fsutil"
	"github.com/quant-risk/radiant-harness/v3/internal/tools"
)

// MaxReadBytes caps the file size accepted by read_file. Symmetric
// with MaxWriteBytes (4 MiB) — the LLM can read back what it just
// wrote. Raise this in fs.go if a real workload needs more.
const MaxReadBytes = 4 * 1024 * 1024

// ReadArgs is the typed shape of the LLM-emitted read_file args.
type ReadArgs struct {
	Path string `json:"path"`
}

// ReadResult is what the tool returns to the executor. Carries the
// file content plus metadata for the verifier trace (bytes, lines,
// path) so the verifier sees the actual data the LLM observed.
type ReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Bytes   int    `json:"bytes"`
	Lines   int    `json:"lines"`
}

// Annotate implements the engine.annotator duck-typed interface so
// the executor surfaces read_file results in the verifier trace.
// Verifier prompt sees path + size without re-parsing the content.
func (r ReadResult) Annotate() map[string]any {
	return map[string]any{
		"path":  r.Path,
		"bytes": r.Bytes,
		"lines": r.Lines,
	}
}

// ReadFileTool returns the read_file tool bound to the given project
// directory. Path safety is re-checked on every invocation; reads
// resolve symlinks before the boundary check (reusing fsutil.PathIsSafe
// shared with write_file).
//
// Returns the file content plus metadata. Binary files are allowed
// but the LLM will see noise — the read tool doesn't try to detect
// content type. If a use case emerges that requires stricter binary
// handling, add it here without changing the wire format.
func ReadFileTool(projectDir string) *tools.Tool {
	return &tools.Tool{
		Name: "read_file",
		Description: "Read the contents of a file at the given path (project-relative). " +
			"Returns content, byte count, and line count. Path must resolve inside the " +
			"project directory after symlink resolution. Max 4 MiB.",
		Params: []tools.Param{
			{Name: "path", Type: "string", Required: true,
				Description: "Project-relative path to read."},
		},
		Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
			return invokeReadFile(ctx, projectDir, raw)
		},
	}
}

func invokeReadFile(ctx context.Context, projectDir string, raw json.RawMessage) (ReadResult, error) {
	var args ReadArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return ReadResult{}, fmt.Errorf("read_file: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return ReadResult{}, fmt.Errorf("read_file: path is required")
	}

	// Boundary check — same as write_file. A symlinked project
	// subdir pointing outside is caught here.
	if !fsutil.PathIsSafe(projectDir, args.Path) {
		return ReadResult{}, fmt.Errorf("read_file: refusing path %q — resolves outside project", args.Path)
	}

	absProject, err := absProjectDir(projectDir)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read_file: abs project: %w", err)
	}
	full := joinPath(absProject, args.Path)

	// Stat first to give a clearer error than ReadFile's "is a directory".
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadResult{}, fmt.Errorf("read_file: file not found: %s", args.Path)
		}
		return ReadResult{}, fmt.Errorf("read_file: stat %s: %w", args.Path, err)
	}
	if info.IsDir() {
		return ReadResult{}, fmt.Errorf("read_file: %s is a directory, not a file", args.Path)
	}
	if info.Size() > MaxReadBytes {
		return ReadResult{}, fmt.Errorf("read_file: file too large (%d bytes; max %d)",
			info.Size(), MaxReadBytes)
	}

	// Honour cancellation before the read.
	if err := ctx.Err(); err != nil {
		return ReadResult{}, fmt.Errorf("read_file: context cancelled: %w", err)
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read_file: read %s: %w", args.Path, err)
	}

	content := string(data)
	lines := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && len(content) > 0 {
		// Trailing partial line counts as one.
		lines++
	}

	return ReadResult{
		Path:    args.Path,
		Content: content,
		Bytes:   len(data),
		Lines:   lines,
	}, nil
}