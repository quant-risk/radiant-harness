package harness

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTryLockAcquiresAndReleases(t *testing.T) {
	dir := t.TempDir()
	lock, err := TryLock(dir)
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if lock == nil {
		t.Fatal("lock is nil")
	}
	// Held file should exist
	if _, err := os.Stat(filepath.Join(dir, "lock.held")); err != nil {
		t.Errorf("lock.held not created: %v", err)
	}
	// Candidate should be gone (renamed)
	if _, err := os.Stat(filepath.Join(dir, "lock")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lock candidate should be gone, got err=%v", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release: %v", err)
	}
	// After release, candidate should be back
	if _, err := os.Stat(filepath.Join(dir, "lock")); err != nil {
		t.Errorf("lock candidate should be recreated after release: %v", err)
	}
}

func TestTryLockFailsWhenBusy(t *testing.T) {
	dir := t.TempDir()
	lock, err := TryLock(dir)
	if err != nil {
		t.Fatalf("first TryLock: %v", err)
	}
	defer lock.Release()

	_, err = TryLock(dir)
	if !errors.Is(err, ErrLockBusy) {
		t.Errorf("second TryLock should return ErrLockBusy, got: %v", err)
	}
}

func TestLockWithRetryWaitsAndAcquires(t *testing.T) {
	dir := t.TempDir()
	lock1, err := TryLock(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Schedule a release after 200ms.
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = lock1.Release()
	}()

	start := time.Now()
	lock2, err := LockWithRetry(dir, 2*time.Second)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("LockWithRetry: %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("returned too fast (%v); should have waited for release", elapsed)
	}
	defer lock2.Release()
}

func TestLockWithRetryTimesOut(t *testing.T) {
	dir := t.TempDir()
	lock, err := TryLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	start := time.Now()
	_, err = LockWithRetry(dir, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, ErrLockBusy) {
		t.Errorf("expected ErrLockBusy, got: %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("returned too fast (%v)", elapsed)
	}
	if elapsed > 1*time.Second {
		t.Errorf("waited too long (%v); expected ~200ms", elapsed)
	}
}

func TestLockReleaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	lock, err := TryLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	// Second release should not error.
	if err := lock.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
	// Nil receiver should be safe.
	var nilLock *Lock
	if err := nilLock.Release(); err != nil {
		t.Errorf("nil Release: %v", err)
	}
}

func TestLockSerializesConcurrentProcesses(t *testing.T) {
	dir := t.TempDir()

	var wg sync.WaitGroup
	acquired := make(chan struct{}, 10)
	released := make(chan struct{}, 10)

	// 10 goroutines all try to acquire the same lock. Only one should
	// succeed at any moment; the others should spin until the holder
	// releases. We use a longer timeout so the test has time to settle.
	const goroutines = 10
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := LockWithRetry(dir, 10*time.Second)
			if err != nil {
				t.Errorf("LockWithRetry: %v", err)
				return
			}
			acquired <- struct{}{}
			// Hold for a moment so other goroutines actually contend.
			time.Sleep(10 * time.Millisecond)
			_ = lock.Release()
			released <- struct{}{}
		}()
	}

	// Wait for all to complete.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("goroutines did not finish in time")
	}

	// We expect 10 acquisitions and 10 releases.
	if len(acquired) != goroutines || len(released) != goroutines {
		t.Errorf("expected %d acquired/released, got %d/%d",
			goroutines, len(acquired), len(released))
	}
}

func TestStateLockReleaseIdempotency(t *testing.T) {
	dir := t.TempDir()
	s := NewState(dir)
	if err := s.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	s.Release()
	// Second release should not panic.
	s.Release()
}
