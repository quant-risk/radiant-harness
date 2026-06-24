//go:build !windows

package engine

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// GateTimeout caps a single gate (test runner, type checker) execution.
// Hung gates usually indicate a flaky test or deadlock; 5 minutes is
// generous for any realistic test suite.
const GateTimeout = 5 * time.Minute

// runShellGate executes the gate as `sh -c <gate>` with the projectDir
// as cwd. Mirrors the implementation in internal/harness so both code
// paths produce identical behavior on POSIX systems.
func runShellGate(ctx context.Context, projectDir, gate string) (string, error) {
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
