//go:build windows

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

const DefaultGateMaxOutput = 10 << 20 // 10 MiB, mirrors engine.DefaultGateMaxOutput

// runGateShell executes the gate as `cmd /c <gate>` with the projectDir
// as cwd. On Windows there's no `sh` by default; `cmd /c` is the closest
// equivalent available out-of-the-box. PowerShell users can call
// `powershell -Command <gate>` directly via the gate command itself.
//
// Note: cmd.exe quoting rules differ from POSIX. We pass the gate through
// directly — gate authors targeting Windows should write CMD-compatible
// commands, not shell-script-style ones. Use && for chaining, not the
// POSIX &&.
func runGateShell(ctx context.Context, projectDir, gate string, maxOutput int) (string, error) {
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
