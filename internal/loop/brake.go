package loop

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

// StallBrake detects when the loop is making no forward progress.
// It tracks a ring buffer of action hashes; if the last Patience
// consecutive actions all hash identically, the loop is stalled.
//
// Design: pure. Time and external state never enter this struct.
// The caller provides the action string; StallBrake only counts.
type StallBrake struct {
	mu       sync.Mutex
	patience int      // consecutive identical hashes before stall is declared
	ring     []string // ring buffer of recent action hashes
	pos      int      // next write position
	size     int      // number of entries recorded so far
}

// NewStallBrake creates a StallBrake with the given patience window.
// patience is the number of consecutive no-change turns before stall.
// A patience of 0 or negative defaults to 3.
func NewStallBrake(patience int) *StallBrake {
	if patience <= 0 {
		patience = 3
	}
	return &StallBrake{
		patience: patience,
		ring:     make([]string, patience),
	}
}

// Record hashes action and appends it to the ring buffer.
// Returns true if the last patience entries are all identical (stalled).
// action should encode the full intent of the iteration — tool name,
// args, or a diff fingerprint — so two genuinely different failed
// attempts don't count as the same stall.
func (s *StallBrake) Record(action string) bool {
	h := actionHash(action)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ring[s.pos] = h
	s.pos = (s.pos + 1) % s.patience
	if s.size < s.patience {
		s.size++
	}

	// Need at least patience entries before declaring stall
	if s.size < s.patience {
		return false
	}

	// All entries in the ring must match the first
	first := s.ring[0]
	for _, v := range s.ring[1:] {
		if v != first {
			return false
		}
	}
	return true
}

// Reset clears the ring buffer. Call after any successful phase
// (e.g. after Persist) so a new iteration starts with a clean slate.
func (s *StallBrake) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ring {
		s.ring[i] = ""
	}
	s.pos = 0
	s.size = 0
}

// Patience returns the configured patience window.
func (s *StallBrake) Patience() int {
	return s.patience
}

// actionHash returns a short hex fingerprint of an action string.
func actionHash(action string) string {
	h := sha256.Sum256([]byte(action))
	return fmt.Sprintf("%x", h[:8])
}
