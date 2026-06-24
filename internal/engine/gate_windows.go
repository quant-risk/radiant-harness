//go:build windows

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
// when the caller doesn't override it. Mirrors the POSIX constant in
// gate_unix.go — keep them in sync.
const DefaultGateMaxOutput = 10 << 20 // 10 MiB

// runShellGate executes the gate as `cmd /c <gate>` with the projectDir
// as cwd. On Windows there's no `sh` by default; cmd.exe is the closest
// equivalent. Quoting rules differ from POSIX — gate authors targeting
// Windows should write CMD-compatible commands.
//
// Output capping uses the same io.LimitReader + SIGPIPE strategy as the
// POSIX version. Windows doesn't deliver SIGPIPE; instead, closing our
// read end while the gate is still writing causes the next write to
// fail with ERROR_BROKEN_PIPE, which cmd surfaces as a non-zero exit.
func runShellGate(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
	if maxOutput <= 0 {
		maxOutput = DefaultGateMaxOutput
	}
	cmd := exec.CommandContext(ctx, "cmd", "/c", gate)
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

	limited := io.LimitReader(io.MultiReader(stdout, stderr), int64(maxOutput))
	var buf bytes.Buffer
	n, _ := io.Copy(&buf, limited)
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
