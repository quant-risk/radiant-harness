//go:build windows

// Package gaterun is the single implementation of shell-gate execution,
// shared by internal/engine, internal/harness, and internal/quality.
// Previously each package had its own copy of this logic (6 files total);
// this package is the canonical source.
package gaterun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Timeout caps any single gate (test runner, type-checker) execution.
const Timeout = 5 * time.Minute

// DefaultMaxOutput is the per-gate byte cap on captured stdout+stderr
// when the caller passes 0 for maxOutput.
const DefaultMaxOutput = 10 << 20 // 10 MiB

// RunShellGate executes gate as `cmd /c <gate>` with projectDir as cwd.
// On Windows there is no sh by default; cmd /c is the closest equivalent.
// Quoting rules differ from POSIX — gate authors targeting Windows should
// write CMD-compatible commands.
// Output is capped at maxOutput bytes (0 = DefaultMaxOutput). Windows does
// not deliver SIGPIPE; the gate dies with ERROR_BROKEN_PIPE instead.
func RunShellGate(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutput
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
	if n >= int64(maxOutput) {
		buf.WriteString("\n[output truncated at ")
		buf.WriteString(fmt.Sprintf("%d", maxOutput))
		buf.WriteString(" bytes — gate wrote more than the configured cap]")
	}
	waitErr := cmd.Wait()
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return buf.String(), fmt.Errorf("gate timeout after %s", Timeout)
		}
		return buf.String(), fmt.Errorf("gate failed: %w", waitErr)
	}
	return buf.String(), nil
}
