// v3_7_13_test.go — CLI surface tests for the v3.7.13 backlog items
// (A/B/C of the current sprint). Lives in cmd/radiant/ because it
// exercises cobra subcommands (phase redirect --purge, fleet status
// --html) and runPhaseWatch plumbing.
//
// pid-tree-great-grandchildren tests live in internal/fleet/
// because the pidfile helpers (writePidFile, writeChildPids,
// writeGrandchildrenPids, writeGreatGrandchildrenPids, etc.) are
// unexported. See internal/fleet/pidtree_great_grandchildren_test.go.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

// ─────────────────────────────────────────────────────────────────────
// B. phase redirect --purge=<ticket>
// ─────────────────────────────────────────────────────────────────────

// TestPhaseRedirect_Purge_RemovesFile — happy path: existing redirect
// is removed, exit 0, file gone.
func TestPhaseRedirect_Purge_RemovesFile(t *testing.T) {
	dir := t.TempDir()

	// Seed a redirect first via the writer helper.
	if err := writeFollowRedirect(dir, "old-ticket", "new-ticket"); err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	// Sanity: file exists.
	path := followRedirectPath(dir, "old-ticket")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected redirect file at %s: %v", path, err)
	}

	// Run purge.
	var buf strings.Builder
	var errBuf strings.Builder
	err := purgeFollowRedirectW(dir, "old-ticket", &buf, &errBuf)
	if err != nil {
		t.Fatalf("purgeFollowRedirect: %v", err)
	}

	// File should be gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected redirect file gone, got stat err = %v", err)
	}

	// Output should mention purge success.
	if !strings.Contains(buf.String(), "old-ticket") {
		t.Errorf("output should reference ticket name, got %q", buf.String())
	}
	if !strings.Contains(strings.ToLower(buf.String()), "removed") {
		t.Errorf("output should describe the purge action, got %q", buf.String())
	}
}

// TestPhaseRedirect_Purge_MissingFile — purge on a ticket that has
// no redirect file exits with error (so CI / lints detect "you asked
// me to purge something that wasn't there").
func TestPhaseRedirect_Purge_MissingFile(t *testing.T) {
	dir := t.TempDir()

	var buf strings.Builder
	var errBuf strings.Builder
	err := purgeFollowRedirectW(dir, "never-existed", &buf, &errBuf)
	if err == nil {
		t.Fatal("expected error when purging non-existent redirect, got nil")
	}
	if !strings.Contains(err.Error(), "never-existed") {
		t.Errorf("error should reference the ticket name, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no redirect") {
		t.Errorf("error should explain the failure, got %v", err)
	}
}

// TestPhaseRedirect_Purge_DoesNotNukeStateDir — purging one redirect
// must not affect other redirects in the same state dir. Defense-
// in-depth against the v3.7.13 design goal of "explicit cleanup
// without nuking state dir".
func TestPhaseRedirect_Purge_DoesNotNukeStateDir(t *testing.T) {
	dir := t.TempDir()

	// Seed two redirects in the same state dir.
	if err := writeFollowRedirect(dir, "ticket-a", "new-a"); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if err := writeFollowRedirect(dir, "ticket-b", "new-b"); err != nil {
		t.Fatalf("seed b: %v", err)
	}

	// Purge only A.
	var buf strings.Builder
	var errBuf strings.Builder
	if err := purgeFollowRedirectW(dir, "ticket-a", &buf, &errBuf); err != nil {
		t.Fatalf("purge a: %v", err)
	}

	// A gone, B intact.
	if _, err := os.Stat(followRedirectPath(dir, "ticket-a")); !os.IsNotExist(err) {
		t.Errorf("ticket-a should be gone, got stat err = %v", err)
	}
	if _, err := os.Stat(followRedirectPath(dir, "ticket-b")); err != nil {
		t.Errorf("ticket-b should still exist, got stat err = %v", err)
	}

	// Re-read B's content — should be untouched.
	data, err := os.ReadFile(followRedirectPath(dir, "ticket-b"))
	if err != nil {
		t.Fatalf("read b: %v", err)
	}
	if !strings.Contains(string(data), "new-b") {
		t.Errorf("ticket-b content corrupted, got %q", string(data))
	}
}

// TestPhaseRedirect_Purge_PreservesSpec — purging a redirect must
// NOT delete the spec.md or tasks.md the redirect points away from.
// The redirect file lives in .radiant-harness/state/possess-<old>/,
// while the spec lives in specs/<n>-<slug>/. Different trees.
func TestPhaseRedirect_Purge_PreservesSpec(t *testing.T) {
	dir := t.TempDir()

	// Seed spec dir + redirect.
	specDir := filepath.Join(dir, "specs", "0001-foo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	specFile := filepath.Join(specDir, "spec.md")
	if err := os.WriteFile(specFile, []byte("# original spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if err := writeFollowRedirect(dir, "old", "new"); err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	// Purge.
	var buf strings.Builder
	var errBuf strings.Builder
	if err := purgeFollowRedirectW(dir, "old", &buf, &errBuf); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// Spec still on disk.
	if _, err := os.Stat(specFile); err != nil {
		t.Errorf("spec.md should survive purge, got stat err = %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────
// A. radiant fleet status <run-id> --html
// ─────────────────────────────────────────────────────────────────────

// TestFleetStatus_HTML_RendersBasicStructure — verify the HTML report
// has the expected scaffolding (DOCTYPE, title, run-id in <h1>, at
// least one task pill, footer with version marker).
func TestFleetStatus_HTML_RendersBasicStructure(t *testing.T) {
	status := fleet.FleetStatus{
		RunID:      "run-html-test",
		Goal:       "verify html structure",
		AgentCount: 1,
		Tasks: []fleet.Task{
			{ID: "t1", Title: "first", Status: fleet.TaskDone, AgentID: "agent-1", Evidence: "ok"},
			{ID: "t2", Title: "second", Status: fleet.TaskPending},
		},
	}

	html := fleet.FormatStatusHTML(status)

	// DOCTYPE + html + head + body present.
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		t.Errorf("expected DOCTYPE prefix, got first 40 chars: %q", html[:min(40, len(html))])
	}
	if !strings.Contains(html, "<title>Fleet run-html-test</title>") {
		t.Errorf("expected <title> with run id, got fragment: %q", trimToSubstr(html, "title"))
	}

	// Header
	if !strings.Contains(html, "<h1>Fleet run-html-test</h1>") {
		t.Errorf("expected <h1> with run id")
	}
	if !strings.Contains(html, "Goal:") || !strings.Contains(html, "verify html structure") {
		t.Errorf("expected goal in meta section")
	}
	if !strings.Contains(html, "Total:") || !strings.Contains(html, "<strong>2</strong>") {
		t.Errorf("expected task total count")
	}

	// Task pills
	if !strings.Contains(html, "status-done") {
		t.Errorf("expected done status class")
	}
	if !strings.Contains(html, "status-pending") {
		t.Errorf("expected pending status class")
	}

	// Evidence row for done task
	if !strings.Contains(html, "Evidence") {
		t.Errorf("expected Evidence dt")
	}
	if !strings.Contains(html, "ok") {
		t.Errorf("expected evidence content 'ok'")
	}

	// Footer
	if !strings.Contains(html, "radiant fleet status --html") {
		t.Errorf("expected footer marker")
	}
}

// TestFleetStatus_HTML_RendersPidTree — when TaskLiveness has a
// populated PidTree, the HTML report must render the tree visually
// (parent, children, grandchildren, great-grandchildren as nested
// box-drawing glyphs with alive/dead classes).
func TestFleetStatus_HTML_RendersPidTree(t *testing.T) {
	status := fleet.FleetStatus{
		RunID:      "run-pid-tree",
		Goal:       "test pid tree render",
		AgentCount: 1,
		Tasks: []fleet.Task{
			{ID: "t1", Title: "pid task", Status: fleet.TaskAssigned, AgentID: "agent-1"},
		},
		TaskLiveness: map[string]fleet.TaskLive{
			"t1": {
				Alive: true,
				Pid:   12345,
				Tree: fleet.PidTree{
					ParentPid:              12345,
					ParentAlive:            true,
					ChildrenPids:           []int{12346, 12347},
					ChildrenAlive:          true,
					ChildCount:             2,
					GrandchildrenPids:      []int{12348},
					GrandchildrenAlive:     true,
					GrandchildrenCount:     1,
					GreatGrandchildrenPids: []int{12349},
					GreatGrandchildrenAlive: false,
					GreatGrandchildrenCount: 0,
				},
			},
		},
	}

	html := fleet.FormatStatusHTML(status)

	// pid-tree pre block
	if !strings.Contains(html, `<pre class="pid-tree">`) {
		t.Errorf("expected pid-tree pre block, got: %q", trimToSubstr(html, "PID tree"))
	}

	// Parent
	if !strings.Contains(html, "parent 12345") {
		t.Errorf("expected parent label with pid 12345")
	}

	// Children
	if !strings.Contains(html, "child 12346") || !strings.Contains(html, "child 12347") {
		t.Errorf("expected both children pids")
	}

	// Grandchildren
	if !strings.Contains(html, "grandchild 12348") {
		t.Errorf("expected grandchild pid")
	}

	// Great-grandchildren (v3.7.13)
	if !strings.Contains(html, "great-grandchild 12349") {
		t.Errorf("expected great-grandchild pid")
	}

	// Alive class on parent (green)
	if !strings.Contains(html, `<span class="pid-alive">● parent 12345</span>`) {
		t.Errorf("expected alive parent span with bullet, got fragment: %q",
			trimToSubstr(html, "parent 12345"))
	}
}

// TestFleetStatus_HTML_EmptyRun — a freshly-created fleet run with
// no tasks must render the empty placeholder (operator needs to know
// the report was generated, not crash on missing data).
func TestFleetStatus_HTML_EmptyRun(t *testing.T) {
	status := fleet.FleetStatus{
		RunID:      "run-empty",
		Goal:       "no tasks yet",
		AgentCount: 0,
	}

	html := fleet.FormatStatusHTML(status)

	// Empty placeholder
	if !strings.Contains(html, "(no tasks") {
		t.Errorf("expected empty-state placeholder")
	}
	if !strings.Contains(html, "fleet plan") {
		t.Errorf("expected hint to run fleet plan")
	}

	// Header still present
	if !strings.Contains(html, "<h1>Fleet run-empty</h1>") {
		t.Errorf("expected run-id in h1")
	}

	// Total count
	if !strings.Contains(html, "Total: <strong>0</strong>") {
		t.Errorf("expected total count of 0")
	}
}

// TestFleetStatus_HTML_EscapesDangerousChars — goal/titles with
// HTML metacharacters must be escaped, not rendered as raw HTML.
// Defense against XSS via injected fleet metadata.
func TestFleetStatus_HTML_EscapesDangerousChars(t *testing.T) {
	status := fleet.FleetStatus{
		RunID: "run-xss",
		Goal:  `<script>alert("xss")</script>`,
		Tasks: []fleet.Task{
			{ID: "t1", Title: `"><img src=x onerror=alert(1)>`, Status: fleet.TaskDone},
		},
	}

	html := fleet.FormatStatusHTML(status)

	// Raw script tag must NOT appear in output (escaped form is OK).
	if strings.Contains(html, "<script>alert") {
		t.Errorf("raw script tag leaked into HTML — XSS vulnerability")
	}
	if strings.Contains(html, `<img src=x onerror`) {
		t.Errorf("raw img tag with onerror leaked into HTML — XSS vulnerability")
	}

	// Escaped form should appear.
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped <script> in output")
	}
}

// TestFleetStatus_HTML_SelfContained — verify no remote deps: no
// CDN URLs, no <script src=> tags, no <link href=> to external
// stylesheets. Designed to work offline.
func TestFleetStatus_HTML_SelfContained(t *testing.T) {
	status := fleet.FleetStatus{
		RunID: "run-offline",
		Goal:  "test offline",
		Tasks: []fleet.Task{
			{ID: "t1", Title: "a task", Status: fleet.TaskDone},
		},
	}

	html := fleet.FormatStatusHTML(status)

	// No http(s):// URLs (other than possible w3.org namespaces which
	// would only appear if we were using <html xmlns> — we don't).
	if strings.Contains(html, `src="http`) {
		t.Errorf("found external src=http URL")
	}
	if strings.Contains(html, `href="http`) {
		t.Errorf("found external href=http URL")
	}
	if strings.Contains(html, "@import") {
		t.Errorf("found @import statement (external CSS)")
	}
	if strings.Contains(html, "<script src=") {
		t.Errorf("found <script src=> — should be JS-free")
	}
}

// TestFleetStatus_HTML_HTMLOut_WritesFile — the `--html-out=<path>`
// flag must write to disk and print the success line. We exercise
// this end-to-end via cobra to confirm wiring.
func TestFleetStatus_HTML_HTMLOut_WritesFile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available for end-to-end binary build")
	}

	dir := t.TempDir()
	outFile := filepath.Join(dir, "report.html")

	// Seed a fleet store directly via the fleet package so we can
	// invoke the CLI against it.
	storePath := filepath.Join(dir, ".radiant-harness", "fleet", "run-out.json")
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatalf("mkdir fleet: %v", err)
	}
	// Minimal store JSON: RunID + Goal + empty Tasks array.
	seed := `{"run_id":"run-out","goal":"html-out test","tasks":[]}` + "\n"
	if err := os.WriteFile(storePath, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	// Build a temporary binary to exercise the CLI. Walk up from
	// the test cwd to find the project root (the directory
	// containing go.mod).
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("locate project root: %v", err)
	}

	binDir := t.TempDir()
	bin := filepath.Join(binDir, "radiant")
	build := exec.Command("go", "build", "-o", bin, "./cmd/radiant")
	build.Dir = projectRoot
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Run with --html-out. cwd=dir so the harness finds the seeded store.
	run := exec.Command(bin, "fleet", "status", "run-out", "--html", "--html-out", outFile)
	run.Dir = dir
	out, err = run.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet status --html failed: %v\n%s", err, out)
	}

	// File should exist with non-zero content.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read html-out: %v", err)
	}
	if len(data) < 1000 {
		t.Errorf("html-out file is suspiciously small: %d bytes", len(data))
	}
	if !strings.Contains(string(data), "run-out") {
		t.Errorf("html-out should contain run id 'run-out'")
	}
}

// findProjectRoot walks up from cwd until it finds go.mod. Returns
// the absolute path to that directory. Used by the end-to-end
// binary-build tests so we can invoke `go build ./cmd/radiant`
// from the right cwd regardless of where `go test` was invoked.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found within 16 levels above %s", dir)
}

// trimToSubstr returns a substring of s centered around the first
// occurrence of marker, with up to 120 chars of context. Returns
// "no marker" if marker is absent.
func trimToSubstr(s, marker string) string {
	idx := strings.Index(s, marker)
	if idx < 0 {
		return "no marker"
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + 80
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}