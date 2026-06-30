package possess

import (
	"errors"
	"testing"
)

// v3.7.2-prep: StubAsyncGate should return ErrAsyncInDevelopment on every
// call. Real implementation lands in v3.7.2 PR-B.

func TestStubAsyncGate_AllMethodsReturnErrAsyncInDevelopment(t *testing.T) {
	g := StubAsyncGate{}

	if _, err := g.Spawn(PhaseDiscover, "task", "/tmp"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Spawn: want ErrAsyncInDevelopment, got %v", err)
	}
	if _, err := g.Status(NewTicket(), "/tmp"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Status: want ErrAsyncInDevelopment, got %v", err)
	}
	if err := g.Cancel(NewTicket(), "/tmp"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Cancel: want ErrAsyncInDevelopment, got %v", err)
	}
}

func TestStubPossessAsync_AllMethodsReturnErrAsyncInDevelopment(t *testing.T) {
	p := StubPossessAsync{}

	if _, err := p.Spawn("task", "/tmp", "standard"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Spawn: want ErrAsyncInDevelopment, got %v", err)
	}
	if _, err := p.Status(NewTicket(), "/tmp"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Status: want ErrAsyncInDevelopment, got %v", err)
	}
	if err := p.Cancel(NewTicket(), "/tmp"); !errors.Is(err, ErrAsyncInDevelopment) {
		t.Errorf("Cancel: want ErrAsyncInDevelopment, got %v", err)
	}
}

func TestValidPhase(t *testing.T) {
	cases := []struct {
		p    Phase
		want bool
	}{
		{PhaseDiscover, true},
		{PhasePlan, true},
		{PhaseExecute, true},
		{PhaseVerify, true},
		{"", false},
		{"unknown", false},
		{"Discover", false}, // case-sensitive
	}
	for _, tc := range cases {
		if got := ValidPhase(tc.p); got != tc.want {
			t.Errorf("ValidPhase(%q) = %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestNewTicket_FormatAndUniqueness(t *testing.T) {
	t1 := NewTicket()
	t2 := NewTicket()
	if len(string(t1)) != 16 {
		t.Errorf("NewTicket: want 16 chars, got %d (%q)", len(t1), t1)
	}
	if t1 == t2 {
		t.Errorf("NewTicket: two consecutive tickets must differ, both = %q", t1)
	}
}

func TestStatePathFor(t *testing.T) {
	got := StatePathFor("/home/user/proj", "abcdef0123456789")
	want := "/home/user/proj/.radiant-harness/state/abcdef0123456789/state.json"
	if got != want {
		t.Errorf("StatePathFor = %q, want %q", got, want)
	}
}