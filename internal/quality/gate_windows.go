//go:build windows

package quality

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// gateTimeout is a per-gate timeout. Five minutes is generous for any
// realistic test suite.
const gateTimeout = 5 * 60 * 1_000_000_000 // 5 minutes in ns

// runShellGate is the Windows shell-execution path. Uses cmd /c since
// sh isn't available by default on Windows.
func runShellGate(ctx context.Context, projectDir, gate string) (string, error) {
	cmd := exec.CommandContext(ctx, "cmd", "/c", gate)
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
