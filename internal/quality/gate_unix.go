//go:build !windows

package quality

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// gateTimeout is a per-gate timeout. Five minutes is generous for any
// realistic test suite.
const gateTimeout = 5 * 60 * 1_000_000_000 // 5 minutes in ns (kept as var so it can be tuned)

// runShellGate is the POSIX shell-execution path. On Windows we use
// cmd /c instead (see gate_windows.go).
func runShellGate(ctx context.Context, projectDir, gate string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", gate)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return string(out), fmt.Errorf("gate timeout")
		}
		return string(out), fmt.Errorf("gate failed: %w", err)
	}
	return string(out), nil
}
