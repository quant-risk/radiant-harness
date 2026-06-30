package fs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/fsutil"
	"github.com/quant-risk/radiant-harness/v3/internal/tools"
)

// DefaultSearchMaxResults caps the number of matches returned per
// invocation. A typical project has <10k source files; 1000 matches
// is roughly 10 files × 100 lines, plenty for the LLM to identify
// the right spot. Raise this in fs.go if a workload needs more.
const DefaultSearchMaxResults = 1000

// MaxSearchPatternBytes caps the regex pattern size. A 4 KiB pattern
// is already absurdly large; this is a sanity bound against
// runaway emissions.
const MaxSearchPatternBytes = 4 * 1024

// Hidden directory prefixes skipped during traversal. `.git`,
// `.radiant-harness`, etc. would explode search time without useful
// matches. Hidden *files* are not skipped — only directories.
var hiddenDirPrefixes = []string{
	".git",
	".radiant-harness",
	"node_modules",
	"vendor",
	".idea",
	".vscode",
}

// SearchArgs is the typed shape of the LLM-emitted search_code args.
type SearchArgs struct {
	Pattern    string `json:"pattern"`              // Go regexp
	Path       string `json:"path,omitempty"`       // search root; default project root
	MaxResults int    `json:"max_results,omitempty"` // default 1000
	Include    string `json:"include,omitempty"`    // glob filter (e.g. "*.go")
}

// SearchMatch is one regex hit. Carries the file (project-relative),
// 1-based line and column, and the matched line content (verbatim).
type SearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Content string `json:"content"`
}

// SearchResult is the tool's return value.
type SearchResult struct {
	Pattern    string        `json:"pattern"`
	Root       string        `json:"root"`
	MatchCount int           `json:"match_count"`
	Truncated  bool          `json:"truncated"`
	Matches    []SearchMatch `json:"matches"`
}

// Annotate implements the engine.annotator duck-typed interface so
// the executor surfaces search_code results in the verifier trace.
func (r SearchResult) Annotate() map[string]any {
	return map[string]any{
		"pattern":     r.Pattern,
		"root":        r.Root,
		"match_count": r.MatchCount,
		"truncated":   r.Truncated,
	}
}

// SearchCodeTool returns the search_code tool bound to the given
// project directory. The search root defaults to the project root
// when the LLM omits the `path` argument. Traversal skips hidden
// directories and binary files. Results are capped at MaxResults.
func SearchCodeTool(projectDir string) *tools.Tool {
	return &tools.Tool{
		Name: "search_code",
		Description: "Search the project for a regex pattern. Returns matching lines with " +
			"{file, line, column, content}. Path defaults to project root. Skips hidden " +
			"directories (.git, .radiant-harness, node_modules, vendor, .idea, .vscode) " +
			"and binary files. Results capped at 1000 matches by default; Truncated=true " +
			"indicates the cap was hit (narrow the search with `path` or `include`).",
		Params: []tools.Param{
			{Name: "pattern", Type: "string", Required: true,
				Description: "Go regexp syntax (https://pkg.go.dev/regexp/syntax)."},
			{Name: "path", Type: "string",
				Description: "Project-relative directory to search in. Defaults to project root."},
			{Name: "max_results", Type: "integer",
				Description: "Maximum matches to return. Defaults to 1000."},
			{Name: "include", Type: "string",
				Description: "Glob filter on filenames (e.g. \"*.go\", \"*_test.go\")."},
		},
		Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
			return invokeSearchCode(ctx, projectDir, raw)
		},
	}
}

func invokeSearchCode(ctx context.Context, projectDir string, raw json.RawMessage) (SearchResult, error) {
	var args SearchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return SearchResult{}, fmt.Errorf("search_code: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return SearchResult{}, fmt.Errorf("search_code: pattern is required")
	}
	if len(args.Pattern) > MaxSearchPatternBytes {
		return SearchResult{}, fmt.Errorf("search_code: pattern too long (%d bytes; max %d)",
			len(args.Pattern), MaxSearchPatternBytes)
	}

	// Compile pattern first — surface the error before any disk I/O.
	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search_code: invalid regex: %w", err)
	}

	// Resolve search root. Default to project root; honour explicit
	// `path` arg when provided, with the same boundary check as
	// read_file and write_file.
	absProject, err := absProjectDir(projectDir)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search_code: abs project: %w", err)
	}

	root := absProject
	rootDisplay := "."
	if args.Path != "" {
		if !fsutil.PathIsSafe(projectDir, args.Path) {
			return SearchResult{}, fmt.Errorf("search_code: refusing search root %q — resolves outside project", args.Path)
		}
		root = joinPath(absProject, args.Path)
		rootDisplay = args.Path
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = DefaultSearchMaxResults
	}

	includeGlob := args.Include // empty = no filter

	result := SearchResult{
		Pattern: args.Pattern,
		Root:    rootDisplay,
		Matches: []SearchMatch{},
	}

	// Honour cancellation before traversal starts.
	if err := ctx.Err(); err != nil {
		return SearchResult{}, fmt.Errorf("search_code: context cancelled: %w", err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Permission errors etc. — skip but continue traversal.
			return nil
		}
		// Check cancellation periodically.
		if err := ctx.Err(); err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			for _, prefix := range hiddenDirPrefixes {
				if strings.HasPrefix(name, prefix) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip files outside the include glob (if set).
		if includeGlob != "" {
			matched, err := filepath.Match(includeGlob, d.Name())
			if err != nil {
				return nil // invalid glob, just skip the filter
			}
			if !matched {
				return nil
			}
		}

		// Skip obviously large files (don't regex-match a 100 MB log).
		info, err := d.Info()
		if err != nil || info.Size() > MaxReadBytes {
			return nil
		}

		// Skip binary files (best-effort detection).
		if isBinary(path) {
			return nil
		}

		matches, err := scanFile(path, absProject, re, maxResults-len(result.Matches))
		if err != nil {
			return nil // unreadable file — skip and continue
		}
		for _, m := range matches {
			result.Matches = append(result.Matches, m)
			if len(result.Matches) >= maxResults {
				result.Truncated = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return SearchResult{}, fmt.Errorf("search_code: walk %s: %w", rootDisplay, err)
	}

	result.MatchCount = len(result.Matches)
	return result, nil
}

// scanFile reads a single file line-by-line and returns matches up to
// `remaining` cap. Stops reading as soon as the cap is hit.
func scanFile(path, projectRoot string, re *regexp.Regexp, remaining int) ([]SearchMatch, error) {
	if remaining <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		rel = path
	}

	var matches []SearchMatch
	scanner := bufio.NewScanner(f)
	// 1 MiB line cap — generous for source code, blocks pathological inputs.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// Find all matches in the line (not just the first).
		locs := re.FindAllStringIndex(line, -1)
		for _, loc := range locs {
			matches = append(matches, SearchMatch{
				File:    rel,
				Line:    lineNum,
				Column:  loc[0] + 1, // 1-based
				Content: line,
			})
			if len(matches) >= remaining {
				return matches, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return matches, err
	}
	return matches, nil
}

// isBinary does a best-effort detection: read first 512 bytes and
// apply http.DetectContentType. Anything not text/* or application/json
// is treated as binary. Fast and reliable for source code projects.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false // unreadable — let scanFile handle the error
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	ct := contentType(buf[:n])
	// Whitelist text-ish types. Everything else (image, audio, archive,
	// compiled object) is treated as binary.
	switch {
	case strings.HasPrefix(ct, "text/"):
		return false
	case strings.HasPrefix(ct, "application/json"):
		return false
	case strings.HasPrefix(ct, "application/xml"):
		return false
	case strings.HasPrefix(ct, "application/x-sh"):
		return false
	case strings.HasPrefix(ct, "application/javascript"):
		return false
	}
	return true
}

// contentType is a small wrapper around http.DetectContentType that
// returns "application/octet-stream" for empty input (which is what
// the stdlib returns, but we treat empty as text — it's usually a
// brand-new empty file the LLM just created).
func contentType(b []byte) string {
	if len(b) == 0 {
		return "text/plain"
	}
	return http.DetectContentType(b)
}