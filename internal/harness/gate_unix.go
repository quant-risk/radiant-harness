//go:build !windows

package harness

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// GateTimeout caps any single gate (test runner, type-checker) execution.
// Hung gates usually indicate a flaky test or deadlock; 5 minutes is
// generous for any realistic test suite.
const GateTimeout = 5 * time.Minute

// runGateShell executes the gate as `sh -c <gate>` with the projectDir
// as cwd. Cancellation propagates from ctx; the timeout is enforced by
// the caller (the orchestrator / quality validator / engine).
func runGateShell(ctx context.Context, projectDir, gate string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", gate)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return string(out), fmt.Errorf("gate timeout after %s", GateTimeout)
		}
		return string(out), fmt.Errorf("gate failed: %w", err)
	}
	return string(out), nil
}
