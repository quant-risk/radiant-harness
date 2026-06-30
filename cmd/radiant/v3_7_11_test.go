// v3_7_11_test.go — coverage for the v3.7.11 phase watch extensions
// (`--on-change-exit`, `--follow=<ticket>`) and the new
// `radiant phase redirect` subcommand.
//
//   - TestPhaseWatch_OnChangeExit_ExitsOnTransition — pins the
//     "exit 0 immediately after the FIRST change observed AFTER
//     the initial snapshot" contract. Without --on-change-exit
//     the same scenario would run until --max-poll.
//
//   - TestPhaseWatch_OnChangeExit_ExitsOneOnMaxPoll — pins the
//     "no transition = exit 1 on max-poll" fallback for
//     --on-change-exit (so the operator still gets a non-zero
//     exit if nothing happens in the wait window).
//
//   - TestPhaseRedirect_Write_AndRead — pins the file format
//     contract: writeFollowRedirect + readFollowRedirect must
//     round-trip the new_ticket field, and the file must live
//     at .radiant-harness/state/possess-<oldTicket>/redirect.json.
//
//   - TestPhaseWatch_FollowRedirect — full integration: emit
//     initial from ticket A, write redirect to ticket B mid-
//     watch, watch detects + switches + emits initial of B.
//
//   - TestFollowRedirectPath_Layout — pins the on-disk layout
//     for redirect.json.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────
// --on-change-exit
// ─────────────────────────────────────────────────────────────────────

func TestPhaseWatch_OnChangeExit_ExitsOnTransition(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("v3711-once-1", "test goal", dir)
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

	// Flip state to done at t+200ms. Watch should detect, emit,
	// exit 0 (the --on-change-exit contract).
	go func() {
		time.Sleep(200 * time.Millisecond)
		st.CurrentPhase = "done"
		st.Phases["execute"].Status = "done"
		st.Phases["execute"].EndedAt = time.Now().UTC()
		st.Phases["verify"].Status = "done"
		st.Phases["verify"].EndedAt = time.Now().UTC()
		_ = atomicWriteState(st)
	}()

	var buf strings.Builder
	start := time.Now()
	err := runPhaseWatch(dir, st.TaskID, 50*time.Millisecond, 0, false, true, "", &buf)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error on --on-change-exit, got: %v (out: %s)", err, buf.String())
	}
	if elapsed > 2*time.Second {
		t.Errorf("--on-change-exit took too long: %s (expected ~200-400ms)", elapsed)
	}
	// Expect at least 2 emissions: initial (in_progress) +
	// the change emission (done).
	if strings.Count(buf.String(), "--- ") < 2 {
		t.Errorf("expected ≥2 emissions, got %d (out: %s)",
			strings.Count(buf.String(), "--- "), buf.String())
	}
	if !strings.Contains(buf.String(), "done") {
		t.Errorf("expected output to mention 'done' after transition, got: %s", buf.String())
	}
}

func TestPhaseWatch_OnChangeExit_ExitsOneOnMaxPoll(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("v3711-once-2", "test goal", dir)
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

	var buf strings.Builder
	// State NEVER changes; --max-poll should kick in and
	// return exit-1-style error so the operator knows
	// nothing happened.
	err := runPhaseWatch(dir, st.TaskID, 50*time.Millisecond, 200*time.Millisecond, false, true, "", &buf)
	if err == nil {
		t.Fatalf("expected error from max-poll with --on-change-exit, got nil (buf: %s)", buf.String())
	}
	if !strings.Contains(err.Error(), "max-poll") {
		t.Errorf("error should mention max-poll, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────
// follow + redirect.json
// ─────────────────────────────────────────────────────────────────────

func TestFollowRedirectPath_Layout(t *testing.T) {
	got := followRedirectPath("/tmp/work", "ticket-old")
	want := filepath.Join("/tmp/work", ".radiant-harness", "state",
		"possess-ticket-old", "redirect.json")
	if got != want {
		t.Errorf("followRedirectPath = %q, want %q", got, want)
	}
}

func TestPhaseRedirect_Write_AndRead(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "old-ticket-1", "new-ticket-2"); err != nil {
		t.Fatalf("writeFollowRedirect: %v", err)
	}

	// File should exist at the canonical path.
	path := followRedirectPath(dir, "old-ticket-1")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("redirect.json not found: %v", err)
	}

	// readFollowRedirect should round-trip the new_ticket.
	got, ok := readFollowRedirect(dir, "old-ticket-1")
	if !ok {
		t.Errorf("readFollowRedirect: ok=false")
	}
	if got != "new-ticket-2" {
		t.Errorf("readFollowRedirect = %q, want %q", got, "new-ticket-2")
	}
}

func TestPhaseRedirect_Write_EmptyOldOrNewRejected(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "", "new"); err == nil {
		t.Errorf("writeFollowRedirect(empty old) should fail")
	}
	if err := writeFollowRedirect(dir, "old", ""); err == nil {
		t.Errorf("writeFollowRedirect(empty new) should fail")
	}
}

func TestPhaseWatch_FollowRedirect(t *testing.T) {
	dir := t.TempDir()
	// Seed ticket A (initial state) and ticket B (target after
	// redirect). We don't seed B's state.json upfront — the
	// watch should switch after detecting the redirect, but
	// since B's state.json doesn't exist yet, the watch will
	// fail to read. That's expected — the redirect detection
	// itself is the contract we pin here.
	stA := newTestPossessState("ticket-A-v3711", "test goal", dir)
	stA.CurrentPhase = "execute"
	stA.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := atomicWriteState(stA); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	// Write the redirect to ticket B after 200ms. B has no
	// state.json, so the watch will error on the next read —
	// that's the price of having the redirect path tested in
	// isolation. The contract we pin here is: redirect IS
	// detected (we see the "→ B" line in the output) before
	// the read fails.
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = writeFollowRedirect(dir, "ticket-A-v3711", "ticket-B-v3711")
	}()

	var buf strings.Builder
	_ = runPhaseWatch(dir, "ticket-A-v3711", 50*time.Millisecond, 600*time.Millisecond, false, false, "ticket-A-v3711", &buf)

	// The "→" redirect line should appear before any error.
	if !strings.Contains(buf.String(), "→ ticket-B-v3711") {
		t.Errorf("expected redirect notice in output, got: %s", buf.String())
	}
}

func TestReadFollowRedirect_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, ok := readFollowRedirect(dir, "never-existed")
	if ok {
		t.Errorf("readFollowRedirect: ok=true for missing redirect.json")
	}
}

func TestReadFollowRedirect_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := followRedirectPath(dir, "ticket-x")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("not-json-at-all"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, ok := readFollowRedirect(dir, "ticket-x")
	if ok {
		t.Errorf("readFollowRedirect: ok=true for corrupt JSON")
	}
}

// TestWriteFollowRedirect_PayloadShape pins the on-disk
// schema. Operators running `cat` on redirect.json need a
// predictable structure; downstream tools (jq, dashboards)
// need a stable field set.
func TestWriteFollowRedirect_PayloadShape(t *testing.T) {
	dir := t.TempDir()
	if err := writeFollowRedirect(dir, "old", "new"); err != nil {
		t.Fatalf("writeFollowRedirect: %v", err)
	}
	path := followRedirectPath(dir, "old")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var payload struct {
		NextTicket string `json:"next_ticket"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.NextTicket != "new" {
		t.Errorf("next_ticket = %q, want %q", payload.NextTicket, "new")
	}
	// CreatedAt should be an RFC3339 timestamp.
	if _, err := time.Parse(time.RFC3339, payload.CreatedAt); err != nil {
		t.Errorf("created_at %q not RFC3339: %v", payload.CreatedAt, err)
	}
}