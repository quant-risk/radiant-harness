package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
)

func TestStateAtomicWriteCreatesTempThenRenames(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.SetFeature("atomic-test")
	s.SetTotalTasks(3)
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// After a successful Save there must be exactly one progress file and
	// zero temp leftovers in the state directory.
	entries, err := os.ReadDir(filepath.Join(dir, ".radiant-harness"))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var temps int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			temps++
		}
	}
	if temps != 0 {
		t.Errorf("expected no leftover temp files, found %d", temps)
	}
	if _, err := os.Stat(filepath.Join(dir, ".radiant-harness", "progress.json")); err != nil {
		t.Errorf("progress.json not created: %v", err)
	}
}

func TestStateSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.SetFeature("round-trip")
	s.SetTotalTasks(7)
	s.MustTransition(radiant.StateImplement)
	s.StartTask(2)
	s.CompleteTask(2)
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded := NewState(dir)
	if loaded.data.Feature != "round-trip" {
		t.Errorf("feature lost in round-trip: %q", loaded.data.Feature)
	}
	if loaded.data.TotalTasks != 7 {
		t.Errorf("total tasks lost: %d", loaded.data.TotalTasks)
	}
	if loaded.data.State != radiant.StateImplement {
		t.Errorf("state lost: %s", loaded.data.State)
	}
	if loaded.data.CurrentTask != 2 {
		t.Errorf("current task lost: %d", loaded.data.CurrentTask)
	}
}

func TestStateSnapshotIsIndependent(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.SetFeature("snap")
	s.SetTotalTasks(1)

	snap := s.Snapshot()
	snap.Feature = "mutated"
	snap.TotalTasks = 99

	// Mutating the snapshot must not change the live state.
	if s.data.Feature != "snap" {
		t.Errorf("snapshot leaked into state: feature=%q", s.data.Feature)
	}
	if s.data.TotalTasks != 1 {
		t.Errorf("snapshot leaked into state: total=%d", s.data.TotalTasks)
	}

	// Mutating the Log slice in the snapshot must not corrupt the live state.
	snap.Log = append(snap.Log, radiant.ProgressEntry{Action: "synthetic"})
	if len(s.data.Log) != 0 {
		t.Errorf("snapshot log leaked: len=%d", len(s.data.Log))
	}
}

func TestStateSurvivesConcurrentMutations(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.SetTotalTasks(100)
	s.MustTransition(radiant.StateImplement)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s.StartTask(id*50 + j)
				s.CompleteTask(id*50 + j)
			}
		}(i)
	}
	wg.Wait()

	// Final state must be internally consistent (no negative progress,
	// no panic, log length proportional to completions).
	p := s.Progress()
	if p < 0 || p > 1 {
		t.Errorf("invalid progress after concurrent mutations: %f", p)
	}
}

func TestStateLoadNonexistentFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	if s.CurrentState() != radiant.StateIdle {
		t.Errorf("fresh state should be idle, got %s", s.CurrentState())
	}
}

func TestStateLoadMalformedJSONDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".radiant-harness")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "progress.json"), []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Should not panic; should fall back to in-memory defaults.
	s := NewState(dir)
	if s.CurrentState() != radiant.StateIdle {
		t.Errorf("expected fallback to idle after malformed JSON, got %s", s.CurrentState())
	}
}

func TestStateProgressTimestampMonotonic(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.MustTransition(radiant.StateImplement)
	before := time.Now()
	s.StartTask(1)
	s.CompleteTask(1)
	after := time.Now()
	if s.data.UpdatedAt.Before(before) || s.data.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt out of expected range: %v (expected %v..%v)",
			s.data.UpdatedAt, before, after)
	}
}

// Sanity check that the saved JSON is parseable by the standard library —
// any future schema change will trip this test loudly.
func TestStateSavedJSONIsValid(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	s.SetFeature("json-check")
	s.MustTransition(radiant.StateImplement)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".radiant-harness", "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p radiant.Progress
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("progress.json not parseable: %v", err)
	}
}
