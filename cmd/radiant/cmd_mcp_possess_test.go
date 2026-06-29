package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/internal/llm"
)

// unsupportedSamplingBackend returns JSON-RPC -32601 for every sampling
// call. Used by tests that need to exercise the self-driven fallback
// path introduced in v3.6.0.
type unsupportedSamplingBackend struct{}

func (unsupportedSamplingBackend) Chat(context.Context, []llm.Message) (*llm.ChatResponse, error) {
	return nil, fmt.Errorf("%w (method=method not found: sampling/createMessage)", llm.ErrSamplingUnsupported)
}

func (unsupportedSamplingBackend) ChatStream(ctx context.Context, messages []llm.Message, cb llm.StreamCallback) (*llm.ChatResponse, error) {
	return unsupportedSamplingBackend{}.Chat(ctx, messages)
}

func (unsupportedSamplingBackend) ModelID() string { return "unsupported-sampling-test" }

// TestRunPossessWithBackendFallsBackToSelfDriven is the regression
// test for the v3.6.0 fix. Previously, an unsupported sampling
// backend caused `runPossessWithBackend` to fail outright and leave the
// project scaffold empty (docs/, specs/, scripts/ existed as bare
// directories). The harness now hands off to the self-driven pipeline,
// which produces real templated artefacts the host agent can fill in.
func TestRunPossessWithBackendFallsBackToSelfDriven(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	st, err := runPossessWithBackend(context.Background(), dir, "ship the feature", "standard", unsupportedSamplingBackend{}, &out)
	if err != nil {
		t.Fatalf("expected self-driven fallback to succeed, got error: %v", err)
	}
	if st == nil {
		t.Fatal("expected persisted state")
	}
	if st.CurrentPhase != "done" {
		t.Fatalf("CurrentPhase = %q, want done (after self-driven handoff)", st.CurrentPhase)
	}
	for _, phase := range []string{"discover", "plan", "execute", "verify"} {
		got := st.Phases[phase].Status
		if got != "done" {
			t.Errorf("phase %q status = %q, want done", phase, got)
		}
		if st.Phases[phase].Error != "" {
			t.Errorf("phase %q error = %q, want empty", phase, st.Phases[phase].Error)
		}
	}

	statePath := possessStatePath(dir, taskID(dir, "ship the feature"))
	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Fatalf("state file not written at %s: %v", statePath, statErr)
	}

	// Self-driven scaffold must produce the conventional layout AND at
	// least one templated artefact under specs/.
	for _, want := range []string{
		filepath.Join(dir, ".radiant-harness", "manifest.json"),
		filepath.Join(dir, ".radiant-harness", "skills"),
		filepath.Join(dir, "AGENTS.md"),
		filepath.Join(dir, "docs"),
		filepath.Join(dir, "specs"),
		filepath.Join(dir, "scripts"),
		filepath.Join(dir, ".radiant-harness", "handoff.md"),
		filepath.Join(dir, ".radiant-harness", "verify.md"),
	} {
		if _, statErr := os.Stat(want); statErr != nil {
			t.Fatalf("expected scaffold path %s: %v", want, statErr)
		}
	}

	// Find the spec.md the self-driven pipeline emitted and check it
	// really contains the host-agent fill-in marker.
	spec, err := filepath.Glob(filepath.Join(dir, "specs", "0001-*", "spec.md"))
	if err != nil || len(spec) == 0 {
		t.Fatalf("expected specs/0001-*/spec.md under %s, got %v", dir, spec)
	}
	body, readErr := os.ReadFile(spec[0])
	if readErr != nil {
		t.Fatalf("read spec.md: %v", readErr)
	}
	if !strings.Contains(string(body), "[host-agent: fill in") {
		t.Errorf("spec.md missing host-agent marker; got:\n%s", string(body))
	}
}
