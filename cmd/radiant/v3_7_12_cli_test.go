// v3_7_12_cli_test.go — CLI surface tests for the v3.7.12 backlog
// items. Lives in cmd/radiant/ because it exercises the cobra
// subcommands + runPhaseWatch plumbing (which are exported).
//
// pid-tree-grandchildren tests live in internal/fleet/ because
// the pidfile helpers (writePidFile, readChildPids, etc.) are
// unexported. See internal/fleet/pidtree_grandchildren_test.go.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// ─────────────────────────────────────────────────────────────────────
// A. phase redirect --list
// ─────────────────────────────────────────────────────────────────────

func TestPhaseRedirect_List_Empty(t *testing.T) {
	dir := t.TempDir()
	// No state dir at all → empty result.
	var buf strings.Builder
	if err := listFollowRedirects(dir, false, &buf); err != nil {
		t.Fatalf("listFollowRedirects: %v", err)
	}
	if !strings.Contains(buf.String(), "(no redirects found)") {
		t.Errorf("expected empty-list marker, got: %s", buf.String())
	}
}

func TestPhaseRedirect_List_WithEntries(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "ticket-A-1", "ticket-B-1"); err != nil {
		t.Fatalf("write A: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := writeFollowRedirect(dir, "ticket-A-2", "ticket-B-2"); err != nil {
		t.Fatalf("write B: %v", err)
	}

	var buf strings.Builder
	if err := listFollowRedirects(dir, false, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ticket-A-1") {
		t.Errorf("output should mention ticket-A-1: %s", out)
	}
	if !strings.Contains(out, "ticket-B-2") {
		t.Errorf("output should mention ticket-B-2: %s", out)
	}
	if !strings.Contains(out, "2 redirect(s)") {
		t.Errorf("output should report count: %s", out)
	}
}

func TestPhaseRedirect_List_JSON(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "old-x", "new-x"); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf strings.Builder
	if err := listFollowRedirects(dir, true, &buf); err != nil {
		t.Fatalf("list json: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatalf("invalid JSON: %v (out: %s)", err, out)
	}
	if entry["old_ticket"] != "old-x" {
		t.Errorf("old_ticket = %v, want old-x", entry["old_ticket"])
	}
	if entry["next_ticket"] != "new-x" {
		t.Errorf("next_ticket = %v, want new-x", entry["next_ticket"])
	}
	if _, ok := entry["created_at"]; !ok {
		t.Errorf("created_at missing: %v", entry)
	}
}

func TestPhaseRedirect_List_IgnoresCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "valid-anchor", "valid-next"); err != nil {
		t.Fatalf("write valid: %v", err)
	}
	corruptPath := followRedirectPath(dir, "corrupt-anchor")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(corruptPath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	var buf strings.Builder
	if err := listFollowRedirects(dir, false, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "valid-anchor") {
		t.Errorf("valid anchor should appear: %s", out)
	}
	if strings.Contains(out, "corrupt-anchor") {
		t.Errorf("corrupt anchor should be skipped: %s", out)
	}
}

func TestPhaseRedirect_List_IgnoresNonPossessDirs(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "good-ticket", "good-next"); err != nil {
		t.Fatalf("write good: %v", err)
	}
	wrongPath := filepath.Join(dir, ".radiant-harness", "state", "other-thing", "redirect.json")
	if err := os.MkdirAll(filepath.Dir(wrongPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(wrongPath, []byte(`{"next_ticket":"foo","created_at":"2026"}`), 0o644); err != nil {
		t.Fatalf("write wrong: %v", err)
	}

	var buf strings.Builder
	if err := listFollowRedirects(dir, false, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "good-ticket") {
		t.Errorf("good-ticket should appear: %s", out)
	}
	if strings.Contains(out, "other-thing") {
		t.Errorf("non-possess dir should be skipped: %s", out)
	}
}

// ─────────────────────────────────────────────────────────────────────
// B. phase follow alias
// ─────────────────────────────────────────────────────────────────────

func TestPhaseFollow_HelpReachable(t *testing.T) {
	root := &cobra.Command{Use: "radiant"}
	registerPhaseWatchCmd(root)
	root.SetArgs([]string{"phase", "follow", "--help"})
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	// Cobra lowercases the first character of Long. Grep on the
	// substring that survives lower-casing.
	if !strings.Contains(out, "for 'phase watch --follow=") {
		t.Errorf("help should describe the alias: got %q", out)
	}
	// Use line should show the positional anchor arg. cobra renders
	// positional args as "<name>" in the Use line.
	if !strings.Contains(out, "anchor-ticket-id") {
		t.Errorf("help should show the anchor arg in Use line: got %q", out)
	}
	// The follow subcommand should be listed under phase.
	if !strings.Contains(out, "follow") {
		t.Errorf("help should list follow subcommand: got %q", out)
	}
}

func TestPhaseFollow_PassesAnchorToRunPhaseWatch(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("v3712-follow-anchor", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := atomicWriteState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The watch will run with --follow=anchor AND read from
	// the anchor's state.json. We can't easily check that the
	// anchor was passed without inspecting the resume_command
	// in the output, which DOES include the task prompt +
	// workdir (not the ticket id directly). So we assert on
	// the max-poll error (proves the watch ran) + the formatted
	// summary (proves state was read successfully).
	var buf strings.Builder
	err := runPhaseWatch(dir, "v3712-follow-anchor", 50*time.Millisecond, 250*time.Millisecond, false, false, "v3712-follow-anchor", &buf)
	if err == nil {
		t.Fatalf("expected max-poll error, got nil")
	}
	if !strings.Contains(err.Error(), "max-poll") {
		t.Errorf("error should mention max-poll: %v", err)
	}
	// Confirm the watch actually read state — the formatted
	// summary mentions the task prompt from the seed.
	if !strings.Contains(buf.String(), "test goal") {
		t.Errorf("output should mention the task prompt (proves state was read): %s", buf.String())
	}
	if !strings.Contains(buf.String(), "in_progress") {
		t.Errorf("output should mention in_progress (proves state read correctly): %s", buf.String())
	}
}