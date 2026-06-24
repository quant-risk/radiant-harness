// Package harness — cross-platform advisory lock.
//
// The lock is implemented via atomic file rename, which is supported on
// every OS we target (POSIX rename(2) is atomic; Windows NTFS has atomic
// rename since Vista). The current holder owns a file at
// `<dir>/lock.held`; contenders try to rename `<dir>/lock` to
// `<dir>/lock.held`. Only the contender whose rename succeeds owns the
// lock. The release path deletes `lock.held`.
//
// This is advisory (same as flock(2)) and per-directory. It is NOT safe
// across network filesystems (NFS, SMB) — atomic rename isn't guaranteed
// there — but it's fine for the typical use case: two `radiant run`
// invocations on the same laptop.
//
// We do NOT use `syscall.Flock` because it is Unix-only (Linux + macOS
// + BSD). Same for `LockFileEx` on Windows. The rename trick is the
// only mechanism that compiles and works identically on all three.
package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock represents an acquired advisory lock on a directory. Held via
// an open file handle to `lock.held` so the OS keeps the inode alive
// even if the directory is otherwise empty.
type Lock struct {
	held string // path to the lock.held file
	f    *os.File
}

// ErrLockBusy means another process currently holds the lock and the
// caller didn't ask to wait.
var ErrLockBusy = errors.New("lock held by another process")

// TryLock attempts to acquire the lock without blocking. Returns
// ErrLockBusy if another process holds it.
func TryLock(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	candidate := filepath.Join(dir, "lock")
	held := filepath.Join(dir, "lock.held")

	// Fast path: if someone is holding it, bail immediately.
	if _, err := os.Stat(held); err == nil {
		return nil, ErrLockBusy
	}

	// Ensure the candidate exists (it gets removed and recreated as
	// lock holders come and go).
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return nil, fmt.Errorf("create candidate: %w", err)
		}
		_ = f.Close()
	}

	// Try to atomically rename candidate → held. If `held` appeared
	// between our Stat check and Rename (a race), the rename fails on
	// POSIX with EEXIST or on Windows with ERROR_ALREADY_EXISTS; we
	// treat that as ErrLockBusy.
	if err := os.Rename(candidate, held); err != nil {
		return nil, ErrLockBusy
	}

	// Double-check: stat held again. If it's not there, somebody deleted
	// it under us (very unusual but possible on NFS) — fail conservatively.
	if _, err := os.Stat(held); err != nil {
		return nil, fmt.Errorf("lock acquired but held file vanished: %w", err)
	}

	f, err := os.OpenFile(held, os.O_RDWR, 0o644)
	if err != nil {
		// We renamed but can't open — release and fail.
		_ = os.Remove(held)
		return nil, fmt.Errorf("open held: %w", err)
	}
	return &Lock{held: held, f: f}, nil
}

// LockWithRetry blocks until TryLock succeeds or the timeout elapses.
// Polls every 50ms, doubling up to 500ms — fast enough to feel
// immediate, slow enough not to burn CPU under contention.
func LockWithRetry(dir string, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	delay := 50 * time.Millisecond
	for {
		l, err := TryLock(dir)
		if err == nil {
			return l, nil
		}
		if !errors.Is(err, ErrLockBusy) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock busy after %s: %w", timeout, ErrLockBusy)
		}
		time.Sleep(delay)
		if delay < 500*time.Millisecond {
			delay *= 2
		}
	}
}

// Release drops the lock and recreates the candidate file so the next
// contender can race for it. Idempotent: calling Release twice is safe.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
	// Recreate the candidate so the next contender has something to
	// rename. If someone else is already holding it we just proceed —
	// they'll release and the next TryLock will create the candidate.
	if err := os.Remove(l.held); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release held: %w", err)
	}
	dir := filepath.Dir(l.held)
	candidate := filepath.Join(dir, "lock")
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_RDWR, 0o644)
		if err == nil {
			_ = f.Close()
		}
	}
	return nil
}
