package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/quality"
	"github.com/quant-risk/radiant-harness/internal/spec"
)

// Orchestrator manages the SDD harness pipeline.
type Orchestrator struct {
	ProjectDir string
	MaxRetries int
	AgentCmd   string
	AgentID    AgentID
	State      *State
	Budget     *RPIBudget
	Runner     *AgentRunner
	mu         sync.Mutex
}

// New creates a new Orchestrator with auto-detected agent. If agentCmd is
// empty, it scans $PATH for the first allowed agent. If agent construction
// fails (no agent, not in allowlist), the orchestrator runs in dry-run mode
// — useful for `radiant validate` and CI smoke tests where no LLM should
// fire.
func New(projectDir, agentCmd string, maxRetries int) *Orchestrator {
	agentID := AgentID("")
	if agentCmd == "" {
		agentID, agentCmd = DetectAgent()
	}

	var runner *AgentRunner
	if agentCmd != "" {
		cfg := AgentConfig{
			ID:         agentID,
			Command:    agentCmd,
			WorkingDir: projectDir,
			MaxTokens:  200000,
		}
		// NewAgentRunner enforces the allowlist; if the binary isn't
		// permitted, fall back to dry-run rather than crash so CI can still
		// run validation. The error is surfaced via Runner==nil checks.
		r, err := NewAgentRunner(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: %v — running in dry-run mode\n", err)
			agentCmd = ""
			agentID = ""
			runner = nil
		} else {
			runner = r
		}
	}

	return &Orchestrator{
		ProjectDir: projectDir,
		MaxRetries: maxRetries,
		AgentCmd:   agentCmd,
		AgentID:    agentID,
		State:      NewState(projectDir),
		Budget:     NewRPIBudget(200000),
		Runner:     runner,
	}
}

// Run executes the full SDD pipeline for a feature.
func (o *Orchestrator) Run(ctx context.Context, specDir string) (*radiant.FeedbackResult, error) {
	// Acquire the advisory flock so concurrent `radiant run` invocations on
	// the same project serialize instead of racing on progress.json.
	if err := o.State.Lock(); err != nil {
		return nil, fmt.Errorf("acquire harness lock: %w", err)
	}
	defer o.State.Release()

	specFile := filepath.Join(specDir, "spec.md")
	s, err := spec.ParseSpec(specFile)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	taskFile := filepath.Join(specDir, "tasks.md")
	plan, err := spec.ParseTasks(taskFile)
	if err != nil {
		return nil, fmt.Errorf("parse tasks: %w", err)
	}

	o.State.MustTransition(radiant.StateImplement)
	o.State.SetFeature(filepath.Base(specDir))
	o.State.SetTotalTasks(len(plan.Tasks))

	fmt.Printf("  Feature: %s (%d ACs, %d tasks)\n", s.Name, len(s.ACs), len(plan.Tasks))
	fmt.Printf("  Agent: %s (max retries: %d)\n", o.agentDisplay(), o.MaxRetries)

	result := &radiant.FeedbackResult{Succeeded: true}

	for _, phase := range plan.Phases {
		fmt.Printf("  Phase: %s (%d tasks)\n", phase.Name, len(phase.Tasks))

		if len(phase.Tasks) == 1 {
			taskResult := o.executeTask(ctx, phase.Tasks[0], specDir)
			o.mergeResult(result, taskResult)
		} else {
			parallelResult := o.executeParallel(ctx, phase.Tasks, specDir)
			o.mergeResult(result, parallelResult)
		}
	}

	if result.Succeeded {
		fullValidation := quality.ValidateFeature(specDir)
		if !fullValidation.Passed {
			result.Succeeded = false
			result.FinalResult = fullValidation
			o.State.MustTransition(radiant.StateFailed)
		} else {
			o.State.MustTransition(radiant.StateDone)
		}
	} else {
		o.State.MustTransition(radiant.StateFailed)
	}

	if err := o.State.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: persist state: %v\n", err)
	}
	return result, nil
}

// executeTask runs a single task with implement, validate, auto-correct loop.
func (o *Orchestrator) executeTask(ctx context.Context, task radiant.Task, specDir string) *radiant.FeedbackResult {
	result := &radiant.FeedbackResult{Succeeded: true}
	o.State.StartTask(task.ID)

	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("    Retry %d/%d for task %d\n", attempt, o.MaxRetries, task.ID)
			o.State.MustTransition(radiant.StateCorrecting)
		}

		if o.Budget.Implement.IsDumbZone() {
			fmt.Println("    Warning: context dumb zone - open new window!")
		}

		implResult, implErr := o.implement(ctx, task, specDir)
		result.Attempts++

		if implErr != nil {
			result.Corrections = append(result.Corrections, radiant.Correction{
				Attempt: attempt + 1,
				Errors:  []string{implErr.Error()},
			})
			continue
		}

		if implResult != nil {
			o.Budget.Implement.AddTokens(implResult.TokensUsed)
			o.Budget.Total.AddTokens(implResult.TokensUsed)
		}

		validation := o.validate(ctx, task, specDir)

		if validation.Passed {
			o.State.CompleteTask(task.ID)
			result.FinalResult = validation
			fmt.Printf("    Task %d passed\n", task.ID)
			return result
		}

		result.Corrections = append(result.Corrections, radiant.Correction{
			Attempt: attempt + 1,
			Errors:  validation.Errors,
		})

		if attempt >= o.MaxRetries {
			result.Succeeded = false
			result.FinalResult = validation
			o.State.FailTask(task.ID, validation.Errors)
			fmt.Printf("    Task %d failed after %d attempts\n", task.ID, attempt+1)
			return result
		}
	}

	result.Succeeded = false
	return result
}

// executeParallel runs multiple tasks concurrently with goroutines.
func (o *Orchestrator) executeParallel(ctx context.Context, tasks []radiant.Task, specDir string) *radiant.FeedbackResult {
	result := &radiant.FeedbackResult{Succeeded: true}
	var mu sync.Mutex
	var wg sync.WaitGroup

	fmt.Printf("    Running %d tasks in parallel...\n", len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t radiant.Task) {
			defer wg.Done()
			r := o.executeTask(ctx, t, specDir)
			mu.Lock()
			defer mu.Unlock()
			o.mergeResult(result, r)
		}(task)
	}

	wg.Wait()
	return result
}

// implement runs the implementation via agent.
func (o *Orchestrator) implement(ctx context.Context, task radiant.Task, specDir string) (*AgentResult, error) {
	if o.Runner == nil || o.AgentCmd == "" {
		fmt.Printf("    [dry-run] implement task %d: %s\n", task.ID, task.Name)
		return nil, nil
	}

	prompt := fmt.Sprintf(
		"Implement task %d: %s\n\nSpec directory: %s\nGate command: %s\nACs to cover: %v\n\n"+
			"Instructions:\n1. Read the spec.md and tasks.md\n"+
			"2. Implement the task following the spec exactly\n"+
			"3. Run the gate command after implementation\n"+
			"4. Mark as done only if gate passes\n"+
			"5. If gate fails, fix and re-run",
		task.ID, task.Name, specDir, task.Gate, task.CoversACs,
	)

	return o.Runner.Run(ctx, prompt)
}

// validate runs validation: gate command + AC verification.
func (o *Orchestrator) validate(ctx context.Context, task radiant.Task, specDir string) radiant.ValidationResult {
	if task.Gate != "" {
		gateErr := o.runGate(ctx, task.Gate)
		if gateErr != nil {
			return radiant.ValidationResult{
				Passed: false,
				Errors: []string{"gate failed: " + gateErr.Error()},
			}
		}
	}

	validation := quality.ValidateFeature(specDir)

	if o.Runner != nil && o.AgentCmd != "" {
		agentValidation := o.agentValidate(ctx, task, specDir)
		if agentValidation != nil && !agentValidation.Success {
			validation.Passed = false
			validation.Errors = append(validation.Errors, "agent validation failed: "+agentValidation.Error)
		}
	}

	return validation
}

// agentValidate runs validation via the AI agent.
func (o *Orchestrator) agentValidate(ctx context.Context, task radiant.Task, specDir string) *AgentResult {
	prompt := fmt.Sprintf(
		"Validate task %d: %s\n\n1. Run gate command: %s\n2. Verify ACs: %v\n"+
			"3. Check that tests exercise the Given/When/Then\n"+
			"4. Check no SPEC_DEVIATION is open\n5. Report: PASS or FAIL with evidence",
		task.ID, task.Name, task.Gate, task.CoversACs,
	)

	result, err := o.Runner.Run(ctx, prompt)
	if err != nil {
		return &AgentResult{Success: false, Error: err.Error()}
	}
	return result
}

// runGate executes a gate command directly. The command is validated against
// the gate-binary allowlist (see agent.go) so a malicious or naive spec
// can't turn `radiant run` into a shell-out to arbitrary code. A timeout is
// enforced via context to keep one stuck gate from stalling the whole
// pipeline.
func (o *Orchestrator) runGate(ctx context.Context, gate string) error {
	if err := validateGateCommand(gate); err != nil {
		return fmt.Errorf("gate refused by allowlist: %w", err)
	}

	gateCtx, cancel := context.WithTimeout(ctx, DefaultGateTimeout)
	defer cancel()

	cmd := exec.CommandContext(gateCtx, "sh", "-c", gate)
	cmd.Dir = o.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(gateCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("gate timeout after %s\n%s", DefaultGateTimeout, string(out))
		}
		return fmt.Errorf("gate failed: %s\n%s", err, string(out))
	}
	return nil
}

// mergeResult merges a task result into the overall result.
func (o *Orchestrator) mergeResult(dest, src *radiant.FeedbackResult) {
	if !src.Succeeded {
		dest.Succeeded = false
	}
	dest.Attempts += src.Attempts
	dest.Corrections = append(dest.Corrections, src.Corrections...)
	if src.FinalResult.Passed {
		dest.FinalResult = src.FinalResult
	}
}

// agentDisplay returns a human-readable agent name.
func (o *Orchestrator) agentDisplay() string {
	if o.AgentCmd == "" {
		return "none (dry-run)"
	}
	return string(o.AgentID) + " (" + o.AgentCmd + ")"
}

// NewWithNoDetect creates an Orchestrator without auto-detecting an agent (for testing).
func NewWithNoDetect(projectDir string, maxRetries int) *Orchestrator {
	return &Orchestrator{
		ProjectDir: projectDir,
		MaxRetries: maxRetries,
		AgentCmd:   "",
		AgentID:    "",
		State:      NewState(projectDir),
		Budget:     NewRPIBudget(200000),
		Runner:     nil,
	}
}
