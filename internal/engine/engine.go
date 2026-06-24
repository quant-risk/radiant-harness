// Package engine implements the universal SDD harness engine.
// It calls LLM APIs directly — no external agent dependency.
// Works with any model via OpenRouter, OpenAI, Anthropic, or custom providers.
package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/quality"
	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/spec"
)

// MaxParallelTasks caps concurrent LLM calls during a parallel phase.
// Most OpenRouter/Anthropic accounts have low rate limits (5–20 req/min)
// and bursting more than ~4 in parallel produces 429s rather than speed.
const MaxParallelTasks = 4

// GateTimeout caps a single gate (test runner) execution. Hung gates usually
// indicate a flaky test or deadlock; 5 minutes is generous.
const GateTimeout = 5 * time.Minute

// Allowed path prefix for code blocks emitted by the LLM. Paths outside the
// project directory are rejected to prevent a misaligned response from
// writing into $HOME or /etc.
func pathIsSafe(projectDir, candidate string) bool {
	if candidate == "" {
		return false
	}
	absProj, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	full := filepath.Join(absProj, candidate)
	abs, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absProj, abs)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// Engine is the universal SDD harness engine.
type Engine struct {
	llmClient  *llm.Client
	projectDir string
	maxRetries int
	verbose    bool
	mu         sync.Mutex
}

// Config holds engine configuration.
type Config struct {
	Model      llm.Model
	ProjectDir string
	MaxRetries int
	Verbose    bool
}

// New creates a new engine.
func New(cfg Config) *Engine {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	return &Engine{
		llmClient:  llm.NewClient(cfg.Model),
		projectDir: cfg.ProjectDir,
		maxRetries: cfg.MaxRetries,
		verbose:    cfg.Verbose,
	}
}

// Run executes the full SDD pipeline for a feature.
func (e *Engine) Run(ctx context.Context, specDir string) (*Result, error) {
	// 1. Parse spec
	specFile := filepath.Join(specDir, "spec.md")
	s, err := spec.ParseSpec(specFile)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	// 2. Parse tasks
	taskFile := filepath.Join(specDir, "tasks.md")
	plan, err := spec.ParseTasks(taskFile)
	if err != nil {
		return nil, fmt.Errorf("parse tasks: %w", err)
	}

	result := &Result{
		Feature:   s.Name,
		ACs:       len(s.ACs),
		Tasks:     len(plan.Tasks),
		StartTime: time.Now(),
	}

	e.log("Feature: %s (%d ACs, %d tasks)", s.Name, len(s.ACs), len(plan.Tasks))

	// 3. Build system prompt from conventions
	systemPrompt := e.buildSystemPrompt(specDir)

	// 4. Execute each phase
	for _, phase := range plan.Phases {
		e.log("Phase: %s (%d tasks)", phase.Name, len(phase.Tasks))

		if len(phase.Tasks) == 1 {
			taskResult := e.executeTask(ctx, systemPrompt, phase.Tasks[0], specDir, s)
			result.merge(taskResult)
		} else {
			// Parallel execution with goroutines
			parallelResult := e.executeParallel(ctx, systemPrompt, phase.Tasks, specDir, s)
			result.merge(parallelResult)
		}
	}

	// 5. Final validation
	if result.Success {
		validation := quality.ValidateFeature(specDir)
		if !validation.Passed {
			result.Success = false
			result.Errors = append(result.Errors, validation.Errors...)
		}
	}

	result.EndTime = time.Now()
	return result, nil
}

// executeTask runs a single task: implement → validate → auto-correct.
func (e *Engine) executeTask(ctx context.Context, systemPrompt string, task radiant.Task, specDir string, s *radiant.Spec) *TaskResult {
	result := &TaskResult{TaskID: task.ID, TaskName: task.Name}

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			e.log("  Retry %d/%d for task %d", attempt, e.maxRetries, task.ID)
		}

		// IMPLEMENT: call LLM to generate code
		implPrompt := e.buildImplementPrompt(task, specDir, s)
		response, err := e.llmClient.SimpleChat(ctx, systemPrompt, implPrompt)
		if err != nil {
			result.Attempts++
			result.Errors = append(result.Errors, fmt.Sprintf("LLM error: %v", err))
			continue
		}

		// Parse LLM response and apply changes
		if err := e.applyLLMResponse(response, specDir); err != nil {
			result.Attempts++
			result.Errors = append(result.Errors, fmt.Sprintf("apply error: %v", err))
			continue
		}

		result.Attempts++

		// VALIDATE: run gate command
		if task.Gate != "" {
			gateErr := e.runGate(ctx, task.Gate)
			if gateErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("gate failed: %v", gateErr))

				// Auto-correct: ask LLM to fix the error
				if attempt < e.maxRetries {
					correctPrompt := e.buildCorrectPrompt(task, gateErr.Error(), response)
					correctResponse, correctErr := e.llmClient.SimpleChat(ctx, systemPrompt, correctPrompt)
					if correctErr == nil {
						e.applyLLMResponse(correctResponse, specDir)
					}
				}
				continue
			}
		}

		// SUCCESS
		result.Success = true
		e.log("  Task %d passed", task.ID)
		return result
	}

	e.log("  Task %d failed after %d attempts", task.ID, result.Attempts)
	return result
}

// executeParallel runs multiple tasks concurrently, capped by a semaphore
// so we don't burst the LLM provider and trigger 429 rate-limit responses.
func (e *Engine) executeParallel(ctx context.Context, systemPrompt string, tasks []radiant.Task, specDir string, s *radiant.Spec) *TaskResult {
	result := &TaskResult{TaskName: "parallel"}
	var mu sync.Mutex
	var wg sync.WaitGroup

	e.log("  Running %d tasks in parallel (max %d concurrent)...", len(tasks), MaxParallelTasks)

	// Semaphore: only MaxParallelTasks goroutines actually call the LLM at once.
	sem := make(chan struct{}, MaxParallelTasks)

	for _, task := range tasks {
		wg.Add(1)
		go func(t radiant.Task) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			r := e.executeTask(ctx, systemPrompt, t, specDir, s)
			mu.Lock()
			defer mu.Unlock()
			result.mergeTask(r)
		}(task)
	}

	wg.Wait()
	return result
}

// buildSystemPrompt creates the system prompt from project conventions.
func (e *Engine) buildSystemPrompt(specDir string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert software engineer following Spec-Driven Development (SDD).\n\n")
	prompt.WriteString("## Rules\n")
	prompt.WriteString("- Implement EXACTLY what the spec says — nothing more, nothing less\n")
	prompt.WriteString("- Each AC (Acceptance Criterion) is a contract — implement all Given/When/Then\n")
	prompt.WriteString("- Never implement beyond scope ('Out of scope' is binding)\n")
	prompt.WriteString("- If spec is ambiguous, ask — don't guess\n")
	prompt.WriteString("- Use the project's existing patterns and conventions\n")
	prompt.WriteString("- Write tests that exercise the Given/When/Then of each AC\n")
	prompt.WriteString("- Run gate commands after implementation\n\n")

	// Load project conventions if they exist
	conventions := []string{"CONVENTIONS.md", "CLAUDE.md", "AGENTS.md", "README.md"}
	for _, conv := range conventions {
		path := filepath.Join(e.projectDir, conv)
		if data, err := os.ReadFile(path); err == nil {
			prompt.WriteString(fmt.Sprintf("## %s\n%s\n\n", conv, string(data)))
			break // Only load the first one found
		}
	}

	// Load glossary if it exists
	glossaryPath := filepath.Join(e.projectDir, "docs", "glossary.md")
	if data, err := os.ReadFile(glossaryPath); err == nil {
		prompt.WriteString(fmt.Sprintf("## Glossary\n%s\n\n", string(data)))
	}

	return prompt.String()
}

// buildImplementPrompt creates the implementation prompt for a task.
func (e *Engine) buildImplementPrompt(task radiant.Task, specDir string, s *radiant.Spec) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("## Task %d: %s\n\n", task.ID, task.Name))
	prompt.WriteString(fmt.Sprintf("**Covers ACs:** %s\n", strings.Join(task.CoversACs, ", ")))
	prompt.WriteString(fmt.Sprintf("**Gate command:** %s\n\n", task.Gate))

	// Include the spec
	specData, _ := os.ReadFile(filepath.Join(specDir, "spec.md"))
	prompt.WriteString(fmt.Sprintf("## Spec\n%s\n\n", string(specData)))

	// Include relevant ACs
	prompt.WriteString("## Relevant ACs\n")
	for _, ac := range s.ACs {
		for _, taskAC := range task.CoversACs {
			if ac.ID == taskAC {
				prompt.WriteString(fmt.Sprintf("### %s: %s\n", ac.ID, ac.Title))
				prompt.WriteString(fmt.Sprintf("- Given: %s\n", ac.Given))
				prompt.WriteString(fmt.Sprintf("- When: %s\n", ac.When))
				prompt.WriteString(fmt.Sprintf("- Then: %s\n\n", ac.Then))
			}
		}
	}

	prompt.WriteString("## Instructions\n")
	prompt.WriteString("1. Implement this task following the spec exactly\n")
	prompt.WriteString("2. Write the code in the appropriate files\n")
	prompt.WriteString("3. Write tests that cover the ACs\n")
	prompt.WriteString(fmt.Sprintf("4. Run the gate command: `%s`\n", task.Gate))
	prompt.WriteString("5. If gate fails, fix and re-run\n\n")
	prompt.WriteString("Respond with the implementation code and file paths.")

	return prompt.String()
}

// buildCorrectPrompt creates a correction prompt when a gate fails.
func (e *Engine) buildCorrectPrompt(task radiant.Task, gateError string, previousResponse string) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("## Task %d failed: %s\n\n", task.ID, task.Name))
	prompt.WriteString(fmt.Sprintf("**Gate error:**\n```\n%s\n```\n\n", gateError))
	prompt.WriteString(fmt.Sprintf("**Previous implementation:**\n%s\n\n", previousResponse))
	prompt.WriteString("## Instructions\n")
	prompt.WriteString("Fix the implementation so the gate passes. Only change what's needed.")

	return prompt.String()
}

// applyLLMResponse parses the LLM response and applies code changes. Each
// emitted path is checked against the project boundary so a misaligned
// response can't escape the project directory.
func (e *Engine) applyLLMResponse(response string, specDir string) error {
	// Extract code blocks from the response
	blocks := extractCodeBlocks(response)

	for _, block := range blocks {
		if block.Path == "" {
			continue
		}

		if !pathIsSafe(e.projectDir, block.Path) {
			return fmt.Errorf("refusing to write outside project: %s", block.Path)
		}

		// Resolve path relative to project dir
		fullPath := filepath.Join(e.projectDir, block.Path)
		dir := filepath.Dir(fullPath)

		// Create directory if needed
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}

		// Write the file
		if err := os.WriteFile(fullPath, []byte(block.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", fullPath, err)
		}

		e.log("  Wrote %s", block.Path)
	}

	return nil
}

// runGate executes a gate command (test runner, type checker). The command
// is validated against the same allowlist the orchestrator uses (see
// internal/quality/validate.go) so a malicious or naive spec can't turn a
// `radiant run` into a shell-out to arbitrary code. The project dir is the
// working directory; cancellation propagates from ctx.
func (e *Engine) runGate(ctx context.Context, gate string) error {
	if err := validateGateCommand(gate); err != nil {
		return fmt.Errorf("gate refused by allowlist: %w", err)
	}
	gateCtx, cancel := context.WithTimeout(ctx, GateTimeout)
	defer cancel()

	e.log("  Gate: %s", gate)
	cmd := exec.CommandContext(gateCtx, "sh", "-c", gate)
	cmd.Dir = e.projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(gateCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("gate timeout after %s\n%s", GateTimeout, string(out))
		}
		return fmt.Errorf("gate failed: %w\n%s", err, string(out))
	}
	return nil
}

// gateAllowlist mirrors the closed set in internal/quality/validate.go and
// internal/harness/agent.go. Kept in sync deliberately: opening a binary
// here must also open it in the harness allowlist, so a typo in one place
// shows up immediately as an "agent not allowed" error rather than silently
// running the wrong command.
var gateAllowlist = map[string]struct{}{
	"node": {}, "npm": {}, "pnpm": {}, "yarn": {}, "bun": {}, "deno": {},
	"go": {}, "make": {},
	"pytest": {}, "python": {}, "python3": {}, "pip": {},
	"cargo": {}, "rustc": {},
	"jest": {}, "vitest": {},
	"tsc": {}, "eslint": {},
	"shellcheck": {},
}

// validateGateCommand rejects any gate whose executable token isn't in the
// allowlist. Splits on shell metacharacters so `npm test && go test` is
// fully validated, both sides.
func validateGateCommand(gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" {
		return nil
	}
	repl := strings.NewReplacer(
		"&&", " ", "||", " ", "|", " ",
		";", " ", ">", " ", "<", " ",
		"(", " ", ")", " ",
	)
	parts := strings.Fields(repl.Replace(gate))
	for _, part := range parts {
		switch {
		case part == "":
			continue
		case isShellOp(part), strings.HasPrefix(part, "-"), strings.Contains(part, "="):
			continue
		}
		base := part
		if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
			base = base[idx+1:]
		}
		if _, ok := gateAllowlist[base]; !ok {
			return fmt.Errorf("binary %q is not in the gate allowlist", base)
		}
	}
	return nil
}

func isShellOp(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
}

// log prints a message if verbose mode is enabled.
func (e *Engine) log(format string, args ...interface{}) {
	if e.verbose {
		fmt.Printf("  "+format+"\n", args...)
	}
}

// ── Code Block Extraction ──

// CodeBlock represents an extracted code block with its file path.
type CodeBlock struct {
	Path    string
	Content string
	Lang    string
}

// extractCodeBlocks extracts code blocks with file paths from LLM response.
func extractCodeBlocks(response string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(response, "\n")

	var current *CodeBlock
	inBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") && !inBlock {
			// Start of code block
			lang := strings.TrimPrefix(line, "```")
			lang = strings.TrimSpace(lang)

			// Check if next line or this line has a file path
			inBlock = true
			current = &CodeBlock{Lang: lang}
			continue
		}

		if strings.HasPrefix(line, "```") && inBlock {
			// End of code block
			if current != nil && current.Path != "" {
				blocks = append(blocks, *current)
			}
			current = nil
			inBlock = false
			continue
		}

		if inBlock && current != nil {
			// Check for file path comment
			if current.Path == "" {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "// File:") || strings.HasPrefix(trimmed, "# File:") || strings.HasPrefix(trimmed, "-- File:") {
					current.Path = strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1])
					continue
				}
				if strings.HasPrefix(trimmed, "// ") && (strings.HasSuffix(trimmed, ".go") || strings.HasSuffix(trimmed, ".py") || strings.HasSuffix(trimmed, ".js") || strings.HasSuffix(trimmed, ".ts")) {
					current.Path = strings.TrimPrefix(trimmed, "// ")
					continue
				}
			}
			current.Content += line + "\n"
		}
	}

	return blocks
}

// ── Result Types ──

// Result is the overall result of a harness run.
type Result struct {
	Feature   string
	ACs       int
	Tasks     int
	Success   bool
	Attempts  int
	Errors    []string
	StartTime time.Time
	EndTime   time.Time
}

// TaskResult is the result of a single task.
type TaskResult struct {
	TaskID   int
	TaskName string
	Success  bool
	Attempts int
	Errors   []string
}

func (r *Result) merge(tr *TaskResult) {
	r.Attempts += tr.Attempts
	if !tr.Success {
		r.Success = false
		r.Errors = append(r.Errors, tr.Errors...)
	}
}

func (r *Result) mergeTask(tr *TaskResult) {
	r.Attempts += tr.Attempts
	if !tr.Success {
		r.Success = false
		r.Errors = append(r.Errors, tr.Errors...)
	}
}

// Duration returns the total duration of the run.
func (r *Result) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

func (r *TaskResult) mergeTask(tr *TaskResult) {
	r.Attempts += tr.Attempts
	if !tr.Success {
		r.Success = false
		r.Errors = append(r.Errors, tr.Errors...)
	}
}
