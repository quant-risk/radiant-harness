//go:build windows

package quality

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// gateTimeout is a per-gate timeout. Five minutes is generous for any
// realistic test suite.
const gateTimeout = 5 * 60 * 1_000_000_000 // 5 minutes in ns

const defaultGateMaxOutput = 10 << 20 // 10 MiB, mirrors engine.DefaultGateMaxOutput

// runShellGate is the Windows shell-execution path. Uses cmd /c since
// sh isn't available by default on Windows.
func runShellGate(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
	if maxOutput <= 0 {
		maxOutput = defaultGateMaxOutput
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
			return buf.String(), fmt.Errorf("gate timeout")
		}
		return buf.String(), fmt.Errorf("gate failed: %w", waitErr)
	}
	return buf.String(), nil
}
