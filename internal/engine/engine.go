// Package engine implements the universal SDD harness engine.
// It calls LLM APIs directly — no external agent dependency.
// Works with any model via OpenRouter, OpenAI, Anthropic, or custom providers.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
	"github.com/quant-risk/radiant-harness/v3/internal/fsutil"
	"github.com/quant-risk/radiant-harness/v3/internal/gaterun"
	"github.com/quant-risk/radiant-harness/v3/internal/llm"
	"github.com/quant-risk/radiant-harness/v3/internal/policy"
	"github.com/quant-risk/radiant-harness/v3/internal/quality"
	"github.com/quant-risk/radiant-harness/v3/internal/spec"
	"github.com/quant-risk/radiant-harness/v3/internal/tools"
)

// gateAllowlist is the closed set of binaries the engine will allow
// as a gate command. Re-exported from internal/policy — add new
// binaries there. Kept as a package-level alias for backwards
// compatibility with internal callers and tests that referenced
// the variable directly.
var gateAllowlist = policy.GateBinaries

// validateGateCommand rejects any gate whose binary isn't in the
// allowlist. Delegated to internal/policy — the canonical
// implementation lives there so all three consumers (engine,
// harness, quality) agree on the closed set.
func validateGateCommand(gate string) error {
	return policy.ValidateGateCommand(gate)
}

// splitOnLogicalOps splits a string on `&&` and `||` only.
// Delegated to internal/policy.
func splitOnLogicalOps(s string) []string {
	return policy.SplitOnLogicalOps(s)
}

// splitShellTokens is a tiny shell tokenizer. Delegated to
// internal/policy.
func splitShellTokens(cmd string) []string {
	return policy.SplitShellTokens(cmd)
}

// isShellOp reports whether s is a shell metacharacter.
// Delegated to internal/policy.
func isShellOp(s string) bool {
	return policy.IsShellOp(s)
}

// MaxParallelTasks caps concurrent LLM calls during a parallel phase.
// Most OpenRouter/Anthropic accounts have low rate limits (5–20 req/min)
// and bursting more than ~4 in parallel produces 429s rather than speed.
const MaxParallelTasks = 4

// Allowed path prefix for code blocks emitted by the LLM. Paths outside the
// project directory are rejected to prevent a misaligned response from
// writing into $HOME or /etc.
//
// Symlinks are resolved before the boundary check, so a symlink inside the
// project that points outside is still rejected. The original implementation
// only checked the textual path, which let an attacker (or a confused LLM)
// create "../../etc/foo" via a symlink and bypass the gate.
//
// Strategy: resolve both the project root and the longest existing prefix
// of the candidate path. If the file doesn't exist yet (LLM proposing a
// new file), walk up the path until we find a directory that does, resolve
// that, and check the prefix. This catches the symlink-escape case where
// e.g. `evil/target.txt` passes the lexical check but `evil` is a symlink
// pointing outside the project.
// PathIsSafe is the public wrapper for the internal pathIsSafe check.
// It returns true iff `candidate` (a project-relative path) resolves
// to a location inside `projectDir` after symlink resolution.
//
// The actual implementation lives in internal/fsutil so it can be
// reused by internal/tools/fs (write_file tool) without creating an
// import cycle. This wrapper preserves the v2.37.0 public surface
// for any caller that depended on engine.PathIsSafe.
//
// Deprecated: new callers should import internal/fsutil directly.
func PathIsSafe(projectDir, candidate string) bool {
	return fsutil.PathIsSafe(projectDir, candidate)
}

func pathIsSafe(projectDir, candidate string) bool {
	return fsutil.PathIsSafe(projectDir, candidate)
}

// Engine is the universal SDD harness engine.
type Engine struct {
	llmClient         *llm.Client
	plannerClient     *llm.Client
	implementerClient *llm.Client
	validatorClient   *llm.Client // optional — reviews each task against ACs
	plannerModelName  string
	projectDir        string
	maxRetries        int
	verbose           bool
	gateMaxOutput     int // per-gate stdout+stderr cap in bytes; 0 = DefaultGateMaxOutput
	mu                sync.Mutex

	// ToolRegistry enables structured tool-use (Sprint 69). When
	// non-nil, applyLLMResponse routes tool calls emitted by the LLM
	// through this registry instead of (or before) the legacy code-block
	// emission path. nil = legacy code-block path only (v2.37.0 default).
	//
	// The Engine does not own the registry — callers (CLI, MCP server,
	// tests) construct it once and inject it. This keeps Engine free of
	// cyclic dependencies on internal/tools/fs and lets multiple
	// harnesses share one registry if needed.
	ToolRegistry *tools.Registry

	// toolTrace is the accumulated trace from the most recent
	// applyLLMResponse call. Reset on every applyLLMResponse; read by
	// the verifier via LastToolTrace. Guarded by e.mu (the existing
	// Engine mutex).
	toolTrace []ToolCallRecord

	// runUsage accumulates token counts across every LLM call in a
	// single Run. Populated by executeTask via accountUsage, copied
	// into Result at the end of Run.
	runUsage chatUsage

	// trace records one TraceEvent per LLM call, gate run, and code
	// block write. Always populated (cheap, in-memory), but only
	// printed when --verbose is on. Drained via DumpTrace() so the
	// final summary can show the per-call latency / cost breakdown.
	trace []TraceEvent

	// currentTaskID is set by executeTask so chatWith can tag each
	// chat event with the task that produced it. 0 means "not in a
	// task" (e.g. a top-level planner call).
	currentTaskID int
}

// TraceEvent is a single record of an engine activity: LLM chat call,
// gate run, code-block write. Capturing per-call latency + token cost
// makes it trivial to identify which phase of a multi-task run is the
// expensive one, without dragging in OpenTelemetry.
//
// Phase is one of: "planner", "implement", "correct". Type is the
// broader activity: "chat", "gate", "write".
type TraceEvent struct {
	Type         string `json:"type"`    // "chat" | "gate" | "write"
	Phase        string `json:"phase"`   // "planner" | "implement" | "correct" | ""
	TaskID       int    `json:"task_id"` // radiant.Task.ID (0 if not in a task)
	Model        string `json:"model"`   // model name for "chat" events; empty otherwise
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	OK           bool   `json:"ok"`               // false on error / failed gate
	Detail       string `json:"detail,omitempty"` // free-form: command, path, error message
}

// Config holds engine configuration.
type Config struct {
	// Model is the default LLM used when neither Planner nor
	// Implementer is set. Optional if both are set explicitly.
	Model llm.Model

	// PlannerModel is the LLM used for research and planning (system
	// prompt construction). When unset, falls back to Model.
	PlannerModel llm.Model

	// ImplementerModel is the LLM used for per-task execution (the
	// implement + correct loop). When unset, falls back to Model.
	ImplementerModel llm.Model

	// ValidatorModel is an optional separate LLM that reviews each
	// task's implementation against its ACs after the gate passes.
	// Per video research #4: separate agents by role. The
	// implementer produces code; the validator (typically a more
	// capable model like Opus) checks it. Empty means no separate
	// validator — the gate command alone decides pass/fail.
	ValidatorModel llm.Model

	ProjectDir string
	MaxRetries int
	Verbose    bool

	// GateMaxOutputBytes caps the stdout+stderr captured from each
	// gate command. Gates that write more than this are truncated
	// (the captured buffer is clipped, a marker is appended, and the
	// gate is killed via broken-pipe on its next write). 0 means
	// use DefaultGateMaxOutput (10 MiB). Without a cap, a chatty
	// gate can OOM the harness process.
	GateMaxOutputBytes int
}

// New creates a new engine.
func New(cfg Config) *Engine {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	planner := cfg.PlannerModel
	if planner.Model == "" {
		planner = cfg.Model
	}
	implementer := cfg.ImplementerModel
	if implementer.Model == "" {
		implementer = cfg.Model
	}

	return &Engine{
		llmClient:         llm.NewClient(cfg.Model),
		plannerClient:     llm.NewClient(planner),
		implementerClient: llm.NewClient(implementer),
		validatorClient:   llm.NewClient(cfg.ValidatorModel), // empty Model → falls back to Model
		plannerModelName:  planner.Model,
		projectDir:        cfg.ProjectDir,
		maxRetries:        cfg.MaxRetries,
		verbose:           cfg.Verbose,
		gateMaxOutput:     cfg.GateMaxOutputBytes, // 0 = use package default
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

	// 3a. Planner advisory step. When the user passed --planner (or a
	// non-default planner is configured), call it once with the spec +
	// tasks so it can flag ambiguities, missing acceptance criteria,
	// or tasks that look unprovable. The planner's response is logged
	// and surfaced in --verbose output, but it never blocks execution
	// — the user is the source of truth for the plan.
	if e.plannerModelName != "" {
		if warnings := e.runPlannerAdvisory(ctx, specDir, s, plan); len(warnings) > 0 {
			result.Warnings = warnings
		}
	}

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
	result.InputTokens = e.runUsage.InputTokens
	result.OutputTokens = e.runUsage.OutputTokens

	// Print the per-call trace summary only if --verbose is on. The
	// summary is grouped by phase so a multi-agent run (planner ≠
	// implementer) makes the cost split obvious.
	if e.verbose {
		e.printTraceSummary()
	}
	return result, nil
}

// printTraceSummary groups the accumulated trace events by phase and
// prints per-phase totals (calls, tokens, latency). Helps users see at
// a glance which model burned the most budget on a run.
func (e *Engine) printTraceSummary() {
	events := e.DumpTrace()
	if len(events) == 0 {
		return
	}
	phaseStats := make(map[string]struct {
		calls        int
		inputTokens  int
		outputTokens int
		totalMS      int64
	})
	for _, ev := range events {
		if ev.Type != "chat" {
			continue
		}
		s := phaseStats[ev.Phase]
		s.calls++
		s.inputTokens += ev.InputTokens
		s.outputTokens += ev.OutputTokens
		s.totalMS += ev.LatencyMS
		phaseStats[ev.Phase] = s
	}
	if len(phaseStats) == 0 {
		return
	}
	fmt.Println("  Trace summary (per phase):")
	for _, phase := range []string{"planner", "implement", "correct", "default"} {
		s, ok := phaseStats[phase]
		if !ok {
			continue
		}
		fmt.Printf("    %-10s %d calls, in=%d out=%d tokens, total %dms\n",
			phase, s.calls, s.inputTokens, s.outputTokens, s.totalMS)
	}
}

// chatUsage captures the token counts returned by an LLM call. Vendors
// expose these differently (OpenAI: prompt + completion; Anthropic via
// proxy: input + output; some have cached read tokens). We track the two
// halves separately so CostUSD can apply vendor-specific pricing later
// without re-parsing the response.
type chatUsage struct {
	InputTokens  int
	OutputTokens int
}

// chatTracked is a wrapper around llmClient.Chat that returns the assistant
// text AND the token usage. Used by executeTask so the run can surface a
// cost estimate. If the call fails, both the text and usage are zero.
func (e *Engine) chatTracked(ctx context.Context, systemPrompt, userPrompt string) (string, chatUsage, error) {
	return e.chatWith(ctx, e.llmClient, "default", systemPrompt, userPrompt)
}

// chatImplementer routes a call to the implementer client (the model
// that actually generates code per task). When the engine is configured
// with a separate ImplementerModel, this is the client for that model;
// otherwise it falls back to the default client.
func (e *Engine) chatImplementer(ctx context.Context, systemPrompt, userPrompt string) (string, chatUsage, error) {
	return e.chatWith(ctx, e.implementerClient, "implement", systemPrompt, userPrompt)
}

// chatImplementerCorrect routes the auto-correct retry through the
// implementer client but tags the trace event with phase="correct" so
// the summary can separate first-attempt calls from self-correction
// retries.
func (e *Engine) chatImplementerCorrect(ctx context.Context, systemPrompt, userPrompt string) (string, chatUsage, error) {
	return e.chatWith(ctx, e.implementerClient, "correct", systemPrompt, userPrompt)
}

// chatPlanner routes a call to the planner client (the model used for
// the system prompt / planning step). Same fallback semantics as
// chatImplementer.
func (e *Engine) chatPlanner(ctx context.Context, systemPrompt, userPrompt string) (string, chatUsage, error) {
	return e.chatWith(ctx, e.plannerClient, "planner", systemPrompt, userPrompt)
}

// chatValidator routes a call to the validator client. Returns
// ("", usage, nil) if no validator is configured (empty model),
// so callers can use this unconditionally without nil checks.
// Per video research #4: separate agents by role — implementer
// writes code, validator (typically Opus or a stronger model)
// reviews it against the spec.
func (e *Engine) chatValidator(ctx context.Context, systemPrompt, userPrompt string) (string, chatUsage, error) {
	if e.validatorClient == nil || e.validatorClient.Model().Model == "" {
		return "", chatUsage{}, nil
	}
	return e.chatWith(ctx, e.validatorClient, "validator", systemPrompt, userPrompt)
}

// chatWith is the underlying call — extract so all three entry points
// (default, planner, implementer) share the same response parsing.
// It also records a TraceEvent so the per-call latency and token cost
// are available for the final summary. The phaseTag parameter lets
// callers override the default phase mapping (e.g. "correct" for
// auto-correction retries) without re-implementing the call body.
func (e *Engine) chatWith(ctx context.Context, client *llm.Client, phaseTag, systemPrompt, userPrompt string) (string, chatUsage, error) {
	if phaseTag == "" {
		if client == e.plannerClient {
			phaseTag = "planner"
		} else if client == e.implementerClient {
			phaseTag = "implement"
		} else {
			phaseTag = "default"
		}
	}
	start := time.Now()
	resp, err := client.Chat(ctx, []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
	latencyMS := time.Since(start).Milliseconds()
	modelName := client.Model().Model
	// Lock while reading currentTaskID: executeParallel spawns N
	// goroutines that all call executeTask, which sets/clears the
	// field. Without the lock, the race detector flags this read.
	e.mu.Lock()
	taskID := e.currentTaskID
	e.mu.Unlock()
	if err != nil {
		e.recordTrace(TraceEvent{
			Type:      "chat",
			Phase:     phaseTag,
			TaskID:    taskID,
			Model:     modelName,
			LatencyMS: latencyMS,
			OK:        false,
			Detail:    err.Error(),
		})
		return "", chatUsage{}, err
	}
	text := ""
	if len(resp.Choices) > 0 {
		text = resp.Choices[0].Message.Content
	}
	usage := chatUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}
	e.recordTrace(TraceEvent{
		Type:         "chat",
		Phase:        phaseTag,
		TaskID:       taskID,
		Model:        modelName,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		LatencyMS:    latencyMS,
		OK:           true,
	})
	return text, usage, nil
}

// accountUsage adds token counts from a chatUsage into the engine's
// running totals. Called from executeTask so the totals span every
// implementation + correction call across every task.
func (e *Engine) accountUsage(u *chatUsage) {
	e.mu.Lock()
	e.runUsage.InputTokens += u.InputTokens
	e.runUsage.OutputTokens += u.OutputTokens
	e.mu.Unlock()
}

// recordTrace appends a TraceEvent to the engine's in-memory trace log.
// Cheap (append + no I/O), so we call it on every LLM call. The trace
// is only printed if --verbose is set, so non-verbose runs don't pay
// any user-facing cost.
func (e *Engine) recordTrace(ev TraceEvent) {
	e.mu.Lock()
	e.trace = append(e.trace, ev)
	e.mu.Unlock()
}

// DumpTrace returns a snapshot of the accumulated trace events. Called
// once at the end of Run to print the per-call latency / cost summary.
func (e *Engine) DumpTrace() []TraceEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]TraceEvent, len(e.trace))
	copy(out, e.trace)
	return out
}

// WriteTraceJSONL dumps every accumulated trace event to w as one JSON
// object per line. This is the format `jq` and most observability
// tools ingest natively. Caller is responsible for closing w (we use
// io.Writer so tests can pass bytes.Buffer; production passes *os.File).
//
// We take the lock once and copy the slice under it, then iterate
// outside the lock — serialising the slice is cheap; calling json.Marshal
// on each event would block concurrent appenders if we held the lock.
func (e *Engine) WriteTraceJSONL(w io.Writer) error {
	e.mu.Lock()
	events := make([]TraceEvent, len(e.trace))
	copy(events, e.trace)
	e.mu.Unlock()

	enc := json.NewEncoder(w)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return fmt.Errorf("encode trace event: %w", err)
		}
	}
	return nil
}

// executeTask runs a single task: implement → validate → auto-correct.
func (e *Engine) executeTask(ctx context.Context, systemPrompt string, task radiant.Task, specDir string, s *radiant.Spec) *TaskResult {
	result := &TaskResult{TaskID: task.ID, TaskName: task.Name}

	// Tag every chat call from this task with the task ID so the
	// trace summary can group per-task costs.
	e.mu.Lock()
	e.currentTaskID = task.ID
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.currentTaskID = 0
		e.mu.Unlock()
	}()

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			e.log("  Retry %d/%d for task %d", attempt, e.maxRetries, task.ID)
		}

		// IMPLEMENT: call LLM to generate code
		implPrompt := e.buildImplementPrompt(task, specDir, s)
		response, usage, err := e.chatImplementer(ctx, systemPrompt, implPrompt)
		if err != nil {
			result.Attempts++
			result.Errors = append(result.Errors, fmt.Sprintf("LLM error: %v", err))
			continue
		}
		e.accountUsage(&usage)

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
					correctResponse, correctUsage, correctErr := e.chatImplementerCorrect(ctx, systemPrompt, correctPrompt)
					if correctErr == nil {
						e.accountUsage(&correctUsage)
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

// runPlannerAdvisory asks the planner LLM to scan the spec + tasks for
// ambiguities, missing acceptance criteria, or tasks that look
// unprovable. The planner returns a short, machine-greppable list:
// each warning must start with "- " on its own line. We extract them,
// log them under --verbose, and surface them on Result.Warnings.
//
// The planner call is best-effort: if it fails (timeout, network,
// rate-limit), we log and continue — the planner is advisory, not a
// gate. The call goes through chatPlanner, so the trace summary shows
// it under phase="planner".
func (e *Engine) runPlannerAdvisory(ctx context.Context, specDir string, s *radiant.Spec, plan *radiant.TaskPlan) []string {
	specFile := filepath.Join(specDir, "spec.md")
	taskFile := filepath.Join(specDir, "tasks.md")
	specBody, err1 := os.ReadFile(specFile)
	taskBody, err2 := os.ReadFile(taskFile)
	if err1 != nil || err2 != nil {
		// Missing files were already caught by the parsers; bail.
		return nil
	}

	systemPrompt := "You are a senior staff engineer reviewing a Spec-Driven " +
		"Development plan before implementation. Your job is to surface " +
		"risks early — never to block. Be terse."

	userPrompt := fmt.Sprintf(
		"Review the spec and tasks below. Output a markdown bullet list "+
			"(each line starts with \"- \") of any concerns: missing "+
			"acceptance criteria, ambiguous Given/When/Then, tasks "+
			"without an obvious test, or ACs that no task covers. If "+
			"nothing is wrong, output exactly: - OK\n\n"+
			"## SPEC\n%s\n\n## TASKS\n%s",
		string(specBody), string(taskBody),
	)

	e.log("  Planner advisory: asking %s to review the plan...", e.plannerModelName)
	text, _, err := e.chatPlanner(ctx, systemPrompt, userPrompt)
	if err != nil {
		e.log("  Planner advisory failed (continuing without): %v", err)
		// Even on failure, record a placeholder trace event so the
		// user can see the planner was attempted.
		return nil
	}

	var warnings []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		item := strings.TrimPrefix(line, "- ")
		item = strings.TrimSpace(item)
		if item == "" || item == "OK" {
			continue
		}
		warnings = append(warnings, item)
	}
	if len(warnings) == 0 {
		e.log("  Planner: no concerns raised.")
		return nil
	}
	e.log("  Planner raised %d concern(s):", len(warnings))
	for _, w := range warnings {
		e.log("    • %s", w)
	}
	return warnings
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
//
// In v2.38.0 (Sprint 69), applyLLMResponse prefers tool calls over
// code blocks when both are present. The contract is:
//
//   - If the response contains any ```tool_call``` fences, they are
//     dispatched via e.ToolRegistry. Code blocks in the same response
//     are ignored (logged at verbose=2).
//   - If the response contains only code blocks, the legacy path
//     applies (back-compat with v2.37.0 and all earlier releases).
//   - If e.ToolRegistry is nil, only the legacy path is available.
//
// This keeps the contract unambiguous: tool calls are an alternative,
// not an addition. A confused LLM that emits both will still produce
// a deterministic outcome.
func (e *Engine) applyLLMResponse(response string, specDir string) error {
	// Prefer tool calls when the response contains them AND a registry
	// is wired. Without a registry, tool calls can't be dispatched and
	// we silently fall back to the code-block path — this preserves
	// behavior for tests / callers that never set ToolRegistry.
	if e.ToolRegistry != nil {
		calls := extractToolCalls(response)
		if len(calls) > 0 {
			return e.applyToolCalls(response, calls)
		}
	}
	return e.applyCodeBlocks(response)
}

// applyCodeBlocks is the legacy path, factored out of applyLLMResponse
// so the tool-call branch can be tested in isolation.
func (e *Engine) applyCodeBlocks(response string) error {
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
	gateCtx, cancel := context.WithTimeout(ctx, gaterun.Timeout)
	defer cancel()

	e.log("  Gate: %s", gate)
	out, err := gaterun.RunShellGate(gateCtx, e.projectDir, gate, e.gateMaxOutput)
	if err != nil {
		if errors.Is(gateCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("gate timeout after %s\n%s", gaterun.Timeout, out)
		}
		return fmt.Errorf("gate failed: %w\n%s", err, out)
	}
	return nil
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

// ── Tool Call Extraction (Sprint 69 / v2.38.0) ──

// ToolCall is one structured tool invocation emitted by the LLM inside
// a ```tool_call``` fenced block. The wire format mirrors what the
// standard SDKs (Anthropic, OpenAI, Gemini) call a "function call",
// but expressed as a JSON object in a markdown fence so we don't need
// any SDK to parse it.
type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ToolCallRecord is what the executor passes to the verifier after a
// tool call has been dispatched. Carries enough information for the
// verifier to audit the action without re-running it.
type ToolCallRecord struct {
	Name      string          `json:"name"`
	Args      json.RawMessage `json:"args"`
	Result    json.RawMessage `json:"result,omitempty"`
	Err       string          `json:"err,omitempty"`
	Bytes     int             `json:"bytes,omitempty"`     // populated by write_file
	Written   string          `json:"written,omitempty"`   // populated by write_file
	Created   bool            `json:"created,omitempty"`   // populated by write_file
	ProjectOK bool            `json:"project_ok,omitempty"`
}

// LastToolTrace returns the trace from the most recent applyLLMResponse
// call. Used by the verifier prompt builder (internal/loop/verifier.go)
// to inject the "TOOL CALLS OBSERVED" section.
//
// Returns nil if no tool calls were dispatched in the most recent call
// (i.e. legacy code-block path was used) or if ToolRegistry was nil.
func (e *Engine) LastToolTrace() []ToolCallRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.toolTrace == nil {
		return nil
	}
	// Return a copy so callers can't mutate internal state.
	out := make([]ToolCallRecord, len(e.toolTrace))
	copy(out, e.toolTrace)
	return out
}

// extractToolCalls parses ```tool_call``` fenced blocks from an LLM
// response. Each block must contain a single JSON object with "name"
// and "args" fields. Malformed blocks are skipped (the executor
// surfaces the count of skipped calls in its log so the verifier can
// catch a pattern of malformed emissions).
//
// Symmetric with extractCodeBlocks — both share the markdown-fence
// structure. This keeps the parser simple and the wire format human-
// readable (a developer can paste the same payload into a markdown
// file and see what the LLM emitted).
func extractToolCalls(response string) []ToolCall {
	var calls []ToolCall
	lines := strings.Split(response, "\n")

	var body []byte
	inBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") && !inBlock {
			lang := strings.TrimPrefix(line, "```")
			lang = strings.TrimSpace(lang)
			if lang != "tool_call" {
				// Not a tool-call fence; let extractCodeBlocks handle it.
				body = nil
				continue
			}
			inBlock = true
			body = body[:0]
			continue
		}

		if strings.HasPrefix(line, "```") && inBlock {
			// End of block. Parse the accumulated body as JSON.
			if len(body) > 0 {
				var parsed struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				}
				if err := json.Unmarshal(body, &parsed); err == nil && parsed.Name != "" {
					calls = append(calls, ToolCall{Name: parsed.Name, Args: parsed.Args})
				}
				// Malformed blocks silently skip — the dispatcher will
				// surface the count of skipped calls when needed.
			}
			body = nil
			inBlock = false
			continue
		}

		if inBlock {
			body = append(body, []byte(line)...)
			body = append(body, '\n')
		}
	}

	return calls
}

// annotator is the duck-typed interface a tool's return value can
// satisfy to surface structured trace metadata (bytes written, paths,
// etc.) without taking a direct dependency on the tool's package.
//
// Defined here (not in internal/tools) so the engine doesn't import
// any concrete tool package — which would cycle when fs.WriteFileTool
// (in internal/tools/fs) wants to import internal/tools for its
// tool.Tool wrapper.
//
// A concrete tool's result type simply implements:
//
//	func (r MyResult) Annotate() map[string]any { return ... }
//
// and the executor's type-switch picks it up automatically.
type annotator interface {
	Annotate() map[string]any
}

// applyToolCalls dispatches parsed tool calls through the registry and
// accumulates the trace. Returns an error if any call fails (the loop
// surfaces this to the verifier, same as a write failure in the legacy
// path).
func (e *Engine) applyToolCalls(response string, calls []ToolCall) error {
	e.mu.Lock()
	e.toolTrace = e.toolTrace[:0] // reset for this iteration
	e.mu.Unlock()

	for i, call := range calls {
		record := ToolCallRecord{Name: call.Name, Args: call.Args}
		ctx := context.Background() // TODO: thread Run context through here
		result, err := e.ToolRegistry.Call(ctx, call.Name, call.Args)
		if err != nil {
			record.Err = err.Error()
			e.mu.Lock()
			e.toolTrace = append(e.toolTrace, record)
			e.mu.Unlock()
			e.log("  Tool call %d (%s) failed: %v", i+1, call.Name, err)
			return fmt.Errorf("tool call %s failed: %w", call.Name, err)
		}
		// Type-switch against the local annotator interface (duck-typed).
		// Tools that implement Annotate() map[string]any surface their
		// metadata in the trace; tools that don't just get their result
		// JSON-marshalled verbatim.
		if a, ok := result.(annotator); ok {
			meta := a.Annotate()
			record.Result, _ = json.Marshal(meta)
			if v, ok := meta["bytes"].(int); ok {
				record.Bytes = v
			}
			if v, ok := meta["written"].(string); ok {
				record.Written = v
			}
			if v, ok := meta["created"].(bool); ok {
				record.Created = v
			}
			if v, ok := meta["project_ok"].(bool); ok {
				record.ProjectOK = v
			}
		} else {
			record.Result, _ = json.Marshal(result)
		}
		e.mu.Lock()
		e.toolTrace = append(e.toolTrace, record)
		e.mu.Unlock()
		e.log("  Tool call %d (%s) ok: %s", i+1, call.Name, record.Written)
	}
	return nil
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

	// Token accounting. Accumulated across every Chat call in the run.
	// Used to surface a cost estimate to the operator (see internal/llm
	// CostUSD for the model-aware pricing table).
	InputTokens  int
	OutputTokens int

	// Warnings are advisory notes from the planner LLM (when one is
	// configured). They never block execution — the spec is the source
	// of truth — but they're surfaced in --verbose output and the
	// post-run summary so the operator can revisit the spec before
	// shipping.
	Warnings []string
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
