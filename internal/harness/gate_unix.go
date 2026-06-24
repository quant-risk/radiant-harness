//go:build !windows

package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// GateTimeout caps any single gate (test runner, type-checker) execution.
// Hung gates usually indicate a flaky test or deadlock; 5 minutes is
// generous for any realistic test suite.
const GateTimeout = 5 * time.Minute

// DefaultGateMaxOutput is the per-gate byte cap on captured stdout+stderr
// when the caller doesn't override it. Mirrors engine.DefaultGateMaxOutput —
// keep in sync.
const DefaultGateMaxOutput = 10 << 20 // 10 MiB

// runGateShell executes the gate as `sh -c <gate>` with the projectDir
// as cwd. Cancellation propagates from ctx; the timeout is enforced by
// the caller (the orchestrator / quality validator / engine). Output
// is capped at maxOutput bytes — gates that write more are truncated
// and killed via broken-pipe on their next write.
func runGateShell(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
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
			return buf.String(), fmt.Errorf("gate timeout after %s", GateTimeout)
		}
		return buf.String(), fmt.Errorf("gate failed: %w", waitErr)
	}
	return buf.String(), nil
}
