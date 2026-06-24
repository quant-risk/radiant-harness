//go:build windows

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

// runGateShell executes the gate as `cmd /c <gate>` with the projectDir
// as cwd. On Windows there's no `sh` by default; `cmd /c` is the closest
// equivalent available out-of-the-box. PowerShell users can call
// `powershell -Command <gate>` directly via the gate command itself.
//
// Note: cmd.exe quoting rules differ from POSIX. We pass the gate through
// directly — gate authors targeting Windows should write CMD-compatible
// commands, not shell-script-style ones. Use && for chaining, not the
// POSIX &&.
func runGateShell(ctx context.Context, projectDir, gate string) (string, error) {
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
