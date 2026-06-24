//go:build !windows

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// GateTimeout caps a single gate (test runner, type checker) execution.
// Hung gates usually indicate a flaky test or deadlock; 5 minutes is
// generous for any realistic test suite.
const GateTimeout = 5 * time.Minute

// DefaultGateMaxOutput is the per-gate byte cap on captured stdout+stderr
// when the caller doesn't override it. 10 MiB is well above any realistic
// test summary but well below the parent's RAM, so a chatty gate can't
// OOM us. Override via Engine.Config.GateMaxOutputBytes.
const DefaultGateMaxOutput = 10 << 20 // 10 MiB

// runShellGate executes the gate as `sh -c <gate>` with the projectDir
// as cwd, capping captured output at maxOutput bytes (use 0 for the
// package default). If the gate writes more than maxOutput bytes, we
// truncate the captured buffer and append a "[truncated]" marker so
// downstream consumers (log printers, validators) know the output was
// clipped rather than complete.
//
// Truncation kills the gate via SIGPIPE on the next write — the OS pipe
// buffer fills, we close our read end, and the gate's next stdout write
// returns EPIPE. The exit error is reported back to the caller; non-zero
// exit + truncation is still surfaced as a failed gate.
func runShellGate(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
	if maxOutput <= 0 {
		maxOutput = DefaultGateMaxOutput
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", gate)
	cmd.Dir = projectDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start gate: %w", err)
	}

	// Read both streams concurrently up to maxOutput total bytes.
	// io.LimitReader ensures we never read more than the cap; after
	// the cap, we stop reading and let the gate block on its next
	// write — at which point we'll close the pipe and the gate dies
	// with SIGPIPE (POSIX) / broken-pipe error (Windows).
	limited := io.LimitReader(io.MultiReader(stdout, stderr), int64(maxOutput))
	var buf bytes.Buffer
	n, _ := io.Copy(&buf, limited) // ignore read error — the gate may already be dying
	truncated := n >= int64(maxOutput)
	if truncated {
		buf.WriteString("\n[output truncated at ")
		buf.WriteString(fmt.Sprintf("%d", maxOutput))
		buf.WriteString(" bytes — gate wrote more than the configured cap]")
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return buf.String(), fmt.Errorf("gate timeout after %s", GateTimeout)
		}
		return buf.String(), fmt.Errorf("gate failed: %w", waitErr)
	}
	return buf.String(), nil
}
