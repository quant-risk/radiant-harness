//go:build windows

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

// runShellGate executes the gate as `cmd /c <gate>` with the projectDir
// as cwd. On Windows there's no `sh` by default; cmd.exe is the closest
// equivalent. Quoting rules differ from POSIX — gate authors targeting
// Windows should write CMD-compatible commands.
func runShellGate(ctx context.Context, projectDir, gate string) (string, error) {
	cmd := exec.CommandContext(ctx, "cmd", "/c", gate)
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
