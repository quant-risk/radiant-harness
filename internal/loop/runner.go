package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	radctx "github.com/quant-risk/radiant-harness/internal/context"
	"github.com/quant-risk/radiant-harness/internal/llm"
)

// Ensure os.Stdout satisfies StreamWriter at compile time.
var _ StreamWriter = os.Stdout

// RunConfig holds all runtime parameters for an autonomous loop run.
// It is built from CLI flags by the caller and passed to Run().
type RunConfig struct {
	// LLM clients — executor and verifier are intentionally separate.
	// The executor never grades its own work.
	ExecutorModel llm.Model
	VerifierModel llm.Model // if zero, falls back to ExecutorModel

	// Budget — token/iter/time/cost brakes from Sprint 44.
	Budget BudgetConfig

	// StallPatience — 0 disables the stall brake.
	StallPatience int

	// Verifier config — quorum, dimensions, min score.
	Verifier VerifierConfig

	// ReviewPanel — post-convergence slot from Sprint 45.
	Review ReviewPanel

	// Ground — inject commit-log grounding block each iteration.
	Ground bool

	// MaxGroundCommits — number of commits to include (0 → default 10).
	MaxGroundCommits int

	// ContextBudgetTokens caps the assembled CONTEXT.md injected into the
	// executor system prompt. 0 = no injection; >0 = detect + assemble + inject.
	// Recommended: 4000–8000 for most models.
	ContextBudgetTokens int

	// Trace — when non-nil, every LLM call is recorded as a TraceEvent.
	// If nil, Run() creates a Tracer automatically using projectDir + runID.
	// Pass an already-open Tracer to share it with an outer caller.
	Trace *Tracer

	// Stream — when true, executor output is streamed to stdout chunk by chunk.
	// Verifier and reviewer always use non-streaming (their output is parsed, not displayed).
	Stream bool

	// StreamOut is the writer for streamed executor chunks. Defaults to os.Stdout.
	// Set to a custom writer in tests to capture output without printing.
	StreamOut StreamWriter

	// Plan — when true, the Plan phase calls the LLM to decompose the goal
	// into a step-by-step plan before each executor call. The plan is injected
	// into the executor prompt as context. Disabled by default (lightweight mode).
	Plan bool

	// PlannerModel is the LLM used for planning. Zero value → falls back to
	// ExecutorModel. A cheaper/faster model (e.g. haiku) is often sufficient.
	PlannerModel llm.Model

	// AutoRoute — when true, derives per-phase models from ExecutorModel's
	// preset family using llm.AutoRoute:
	//   Research/Verify → top-tier sibling (e.g. opus when anchor is sonnet)
	//   Plan            → mid-tier (anchor or sibling)
	//   Execute         → anchor (unchanged)
	// When the family has no stronger sibling the anchor is used for all phases.
	// Overrides VerifierModel and PlannerModel when enabled.
	AutoRoute bool
}

// StreamWriter is the interface for streaming output — satisfied by *os.File,
// *bytes.Buffer, and any io.Writer. Kept minimal to avoid the io import cycle.
type StreamWriter interface {
	Write(p []byte) (n int, err error)
}

// RunResult is the outcome of a completed autonomous loop.
type RunResult struct {
	RunID      string
	Goal       string
	ExitReason ExitReason
	Iterations int
	FinalPhase Phase
	Elapsed    time.Duration
	TokensUsed int
	CostUSD    float64
}

// Run executes the full autonomous Discover→Plan→Execute→Verify→Persist cycle
// for a free-form goal. It connects the Cycle state machine to real LLM calls,
// enforcing all configured brakes at each iteration boundary.
//
// This is the central integration point — the thing that makes
// `radiant loop start` actually call an LLM instead of just managing state.
func Run(ctx context.Context, projectDir, runID, goal string, cfg RunConfig) (*RunResult, error) {
	started := time.Now()

	// Build budget and cycle.
	b := NewBudget(cfg.Budget)
	c := NewCycle(projectDir, runID, goal, b)

	// Open tracer — use caller-supplied or create one automatically.
	tr := cfg.Trace
	if tr == nil {
		var err error
		tr, err = NewTracer(projectDir, runID)
		if err != nil {
			return nil, fmt.Errorf("init tracer: %w", err)
		}
		defer tr.Close()
	}

	// Resolve stream output writer.
	streamOut := cfg.StreamOut
	if streamOut == nil {
		streamOut = os.Stdout
	}

	// Assemble project context once per run (expensive — detect + write CONTEXT.md).
	// Injected into every executor system prompt. Empty string = disabled.
	projectCtxBlock := ""
	if cfg.ContextBudgetTokens > 0 {
		projectCtxBlock = assembleContextBlock(projectDir, cfg.ContextBudgetTokens)
	}

	// Build stall brake (disabled when patience == 0).
	var stall *StallBrake
	if cfg.StallPatience > 0 {
		stall = NewStallBrake(cfg.StallPatience)
	}

	// Build LLM clients.
	// When AutoRoute is enabled, derive per-phase models from the anchor's
	// preset family. Explicit VerifierModel / PlannerModel take precedence
	// only when AutoRoute is false.
	execModel := cfg.ExecutorModel
	verModel := cfg.VerifierModel
	planModel := cfg.PlannerModel

	if cfg.AutoRoute {
		anchor := cfg.ExecutorModel.Model
		if routed := llm.AutoRoute(anchor, llm.PhaseResearch); routed != anchor {
			verModel = llm.Model{
				Model:   routed,
				APIKey:  cfg.ExecutorModel.APIKey,
				BaseURL: cfg.ExecutorModel.BaseURL,
			}
		}
		if routed := llm.AutoRoute(anchor, llm.PhasePlan); routed != anchor {
			planModel = llm.Model{
				Model:   routed,
				APIKey:  cfg.ExecutorModel.APIKey,
				BaseURL: cfg.ExecutorModel.BaseURL,
			}
		}
		// Execute always stays on the anchor model.
	}

	if verModel.Model == "" {
		verModel = execModel
	}
	if planModel.Model == "" {
		planModel = execModel
	}

	execClient := llm.NewClient(execModel)
	verClient := llm.NewClient(verModel)
	planClient := llm.NewClient(planModel)

	// Verifier config defaults.
	verCfg := cfg.Verifier
	if verCfg.MinScore == 0 {
		verCfg = DefaultVerifierConfig()
		verCfg.Quorum = cfg.Verifier.Quorum
	}

	// Transition to discover immediately.
	if err := c.Transition(PhaseDiscover, "run started"); err != nil {
		return nil, fmt.Errorf("initial transition: %w", err)
	}

	var lastVerifyOutput string
	var reviewRestarts int
	reviewPanel := cfg.Review
	if reviewPanel.MaxRestarts == 0 {
		reviewPanel = DefaultReviewPanel()
	}
	var lastReviewFindings []string

	for {
		// ── Iteration boundary checks ────────────────────────────────────
		if ok, reason := c.ShouldContinue(b); !ok {
			_ = c.SetExit(reason, "limit reached")
			return buildResult(runID, goal, reason, c, b, started), nil
		}

		now := time.Now()
		if exceeded, _ := b.CheckTime(now); exceeded {
			_ = c.SetExit(ExitTimeLimitReached, "wall-clock limit exceeded")
			return buildResult(runID, goal, ExitTimeLimitReached, c, b, started), nil
		}
		if b.CheckCost() {
			_ = c.SetExit(ExitCostLimitReached, "dollar cost limit exceeded")
			return buildResult(runID, goal, ExitCostLimitReached, c, b, started), nil
		}

		if err := ctx.Err(); err != nil {
			_ = c.SetExit(ExitCanceled, "context canceled")
			return buildResult(runID, goal, ExitCanceled, c, b, started), nil
		}

		// ── Discover / Plan ──────────────────────────────────────────────
		// Skip discover transition if already in discover (first iteration after startup).
		if c.State().Phase != PhaseDiscover {
			if err := c.Transition(PhaseDiscover, fmt.Sprintf("iter %d", c.State().Iteration+1)); err != nil {
				return nil, err
			}
		}
		if err := c.Transition(PhasePlan, "planning"); err != nil {
			return nil, err
		}

		// Optional LLM-based planning: decompose the goal before execution.
		// Only runs on iteration 0 (fresh start) or when cfg.Plan is enabled
		// and there is no prior verifier feedback to act on directly.
		var planOutput string
		if cfg.Plan && len(lastReviewFindings) == 0 {
			planPrompt := BuildPlannerPrompt(goal, c.State().Iteration)
			planResp, planErr := planClient.SimpleChat(ctx, plannerSystemPrompt(), planPrompt)
			planToks := estimateTokens(planPrompt, planResp)
			traceCall(tr, runID, PhasePlan, "planner", planModel.Model, planPrompt, planResp, planToks, planErr)
			if planErr == nil {
				b.Consume(planToks, PhasePlan)
				planOutput = planResp
			}
			// Fail-open: if planner errors, continue without a plan.
		}

		// ── Execute ──────────────────────────────────────────────────────
		if err := c.Transition(PhaseExecute, "calling executor"); err != nil {
			return nil, err
		}

		groundBlock := ""
		if cfg.Ground {
			if gb, err := GroundingBlock(projectDir, cfg.MaxGroundCommits); err == nil {
				groundBlock = gb
			}
		}

		execPrompt := buildExecutorPrompt(goal, groundBlock, planOutput, lastReviewFindings)
		var execOutput string
		var execErr error
		if cfg.Stream {
			iter := c.State().Iteration + 1
			fmt.Fprintf(streamOut, "\n── executor (iter %d) ──────────────────────────────\n", iter)
			execOutput, execErr = simpleChatStream(ctx, execClient, executorSystemPrompt(projectCtxBlock), execPrompt, streamOut)
			fmt.Fprintf(streamOut, "\n────────────────────────────────────────────────────\n")
		} else {
			execOutput, execErr = execClient.SimpleChat(ctx, executorSystemPrompt(projectCtxBlock), execPrompt)
		}
		toks := estimateTokens(execPrompt, execOutput)
		traceCall(tr, runID, PhaseExecute, "executor", cfg.ExecutorModel.Model, execPrompt, execOutput, toks, execErr)
		if execErr != nil {
			_ = c.Transition(PhaseFailed, fmt.Sprintf("executor error: %v", execErr))
			_ = c.IncrIteration()
			b.Consume(500, PhaseExecute)
			continue
		}
		b.Consume(toks, PhaseExecute)
		lastVerifyOutput = execOutput

		// Stall check — hash the executor output.
		if stall != nil && stall.Record(execOutput) {
			_ = c.SetExit(ExitStalled, fmt.Sprintf("no progress after %d identical outputs", cfg.StallPatience))
			return buildResult(runID, goal, ExitStalled, c, b, started), nil
		}

		// ── Verify ───────────────────────────────────────────────────────
		if err := c.Transition(PhaseVerify, "calling verifier"); err != nil {
			return nil, err
		}

		verPrompt := BuildVerifierPrompt(goal, lastVerifyOutput, verCfg)
		verResponse, verErr := verClient.SimpleChat(ctx, verifierSystemPrompt(), verPrompt)
		verToks := estimateTokens(verPrompt, verResponse)
		traceCall(tr, runID, PhaseVerify, "verifier", verModel.Model, verPrompt, verResponse, verToks, verErr)
		if verErr != nil {
			_ = c.Transition(PhaseFailed, fmt.Sprintf("verifier error: %v", verErr))
			_ = c.IncrIteration()
			b.Consume(500, PhaseVerify)
			continue
		}
		b.Consume(verToks, PhaseVerify)

		result := ParseVerifyResponse(verResponse, verCfg)

		// Escalation check.
		if result.Escalate {
			id, _ := c.WriteInboxItem(result)
			_ = c.Transition(PhaseAwaitingHuman, fmt.Sprintf("verifier escalated: %s", id))
			_ = c.SetExit(ExitNeedsHuman, fmt.Sprintf("inbox item %s", id))
			return buildResult(runID, goal, ExitNeedsHuman, c, b, started), nil
		}

		if !result.Approved {
			_ = c.Transition(PhaseExecute, "verifier rejected — retrying")
			_ = c.Transition(PhaseFailed, fmt.Sprintf("rejected: %s", strings.Join(result.Issues, "; ")))
			_ = c.IncrIteration()
			b.Consume(0, PhaseVerify)
			continue
		}

		// ── Post-convergence review panel ────────────────────────────────
		reviewPrompt := BuildReviewPrompt(goal, lastVerifyOutput, lastReviewFindings)
		reviewResponse, reviewErr := verClient.SimpleChat(ctx, reviewerSystemPrompt(), reviewPrompt)
		revToks := estimateTokens(reviewPrompt, reviewResponse)
		traceCall(tr, runID, PhaseVerify, "reviewer", verModel.Model, reviewPrompt, reviewResponse, revToks, reviewErr)
		if reviewErr == nil {
			b.Consume(revToks, PhaseVerify)
			reviewResult := ParseReviewResponse(reviewResponse)
			if !reviewResult.Pass {
				reviewRestarts++
				lastReviewFindings = reviewResult.Findings
				if reviewRestarts >= reviewPanel.maxRestarts() {
					_ = c.SetExit(ExitCritical, fmt.Sprintf("review panel rejected %d times", reviewRestarts))
					return buildResult(runID, goal, ExitCritical, c, b, started), nil
				}
				_ = c.IncrIteration()
				stall.reset()
				continue
			}
		}
		// Review passed (or errored — fail open for review).
		lastReviewFindings = nil
		reviewRestarts = 0

		// ── Persist ──────────────────────────────────────────────────────
		if err := c.Transition(PhasePersist, "checkpointing"); err != nil {
			return nil, err
		}
		_ = c.UpdateBudget(b)
		stall.reset()

		// ── Done ─────────────────────────────────────────────────────────
		_ = c.SetExit(ExitSuccess, "goal met and review passed")
		return buildResult(runID, goal, ExitSuccess, c, b, started), nil
	}
}

// buildResult assembles a RunResult from cycle + budget state.
func buildResult(runID, goal string, reason ExitReason, c *Cycle, b *Budget, started time.Time) *RunResult {
	state := c.State()
	return &RunResult{
		RunID:      runID,
		Goal:       goal,
		ExitReason: reason,
		Iterations: state.Iteration,
		FinalPhase: state.Phase,
		Elapsed:    time.Since(started),
		TokensUsed: b.UsedTokens(),
		CostUSD:    b.EstimatedCostUSD(),
	}
}

// reset is a nil-safe wrapper so stall.reset() is safe when stall brake is disabled.
func (s *StallBrake) reset() {
	if s != nil {
		s.Reset()
	}
}

// estimateTokens returns a rough token estimate for a prompt+response pair.
// Counts Unicode code points (runes) rather than bytes so CJK and accented
// text don't get systematically underestimated. ~3.5 chars/token is more
// conservative than the ASCII-only 4-byte rule and works better for mixed
// Portuguese/code content. Integer division truncates toward zero.
func estimateTokens(prompt, response string) int {
	runes := utf8.RuneCountInString(prompt) + utf8.RuneCountInString(response)
	return (runes*10 + 34) / 35 // ≈ runes / 3.5, integer-only
}

// ── System prompts ────────────────────────────────────────────────────────────

func executorSystemPrompt(contextBlock string) string {
	base := `You are an expert software engineer implementing a goal autonomously.
Read the goal and any prior context carefully.
Produce a concrete, complete implementation. No stubs, no TODOs, no placeholders.
Output the result clearly so a separate verifier can assess it.`
	if contextBlock == "" {
		return base
	}
	return base + "\n\n" + contextBlock
}

// assembleContextBlock runs project detection and assembles CONTEXT.md,
// returning its content trimmed to contextBudgetTokens.
// Returns "" cleanly on any error (fail-open: missing context ≠ broken run).
func assembleContextBlock(projectDir string, contextBudgetTokens int) string {
	det, err := radctx.Detect(projectDir)
	if err != nil {
		return ""
	}
	path, _, err := radctx.Assemble(projectDir, det, radctx.AssembleOptions{
		BudgetTokens: contextBudgetTokens,
	})
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}
	return "## PROJECT CONTEXT\n\n" + content
}

func verifierSystemPrompt() string {
	return `You are an adversarial verifier. Your default stance is REJECTED.
You do not grade your own work — you received output from a separate executor.
Be skeptical. Cite specific evidence. Follow the format exactly.`
}

func reviewerSystemPrompt() string {
	return `You are a senior technical reviewer performing a final quality gate.
The per-iteration verifier already approved this output.
Your job is a deeper review across multiple dimensions.
Score honestly. Flag regressions. Follow the format exactly.`
}

// buildExecutorPrompt assembles the executor's user prompt for this iteration.
func buildExecutorPrompt(goal, groundBlock, planOutput string, priorReviewFindings []string) string {
	var sb strings.Builder
	if groundBlock != "" {
		sb.WriteString(groundBlock)
		sb.WriteString("\n\n")
	}
	sb.WriteString("GOAL:\n")
	sb.WriteString(goal)
	if planOutput != "" {
		sb.WriteString("\n\nPLAN:\n")
		sb.WriteString(planOutput)
	}
	if len(priorReviewFindings) > 0 {
		sb.WriteString("\n\nPRIOR REVIEW FINDINGS TO ADDRESS:\n")
		for _, f := range priorReviewFindings {
			sb.WriteString("- ")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// BuildPlannerPrompt assembles the planner's user prompt.
// Exported so tests can inspect its structure without calling the LLM.
func BuildPlannerPrompt(goal string, iteration int) string {
	var sb strings.Builder
	sb.WriteString("GOAL:\n")
	sb.WriteString(goal)
	if iteration > 0 {
		fmt.Fprintf(&sb, "\n\nThis is iteration %d. Prior attempts did not satisfy the verifier.", iteration)
		sb.WriteString("\nDecompose the goal into concrete steps that address likely gaps.")
	}
	return sb.String()
}

func plannerSystemPrompt() string {
	return `You are a software planning assistant. Your job is to decompose a goal
into a numbered list of concrete implementation steps. Be specific and actionable.
Do not implement — only plan. Output a numbered list, one step per line.
Keep it under 10 steps. Focus on what the executor needs to do next.`
}

// simpleChatStream calls ChatStream and writes each chunk to w, returning the
// full accumulated response. w may be nil (output discarded).
func simpleChatStream(ctx context.Context, client *llm.Client, systemPrompt, userPrompt string, w StreamWriter) (string, error) {
	var sb strings.Builder
	callback := func(chunk string) {
		sb.WriteString(chunk)
		if w != nil {
			_, _ = fmt.Fprint(w, chunk)
		}
	}
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	_, err := client.ChatStream(ctx, messages, callback)
	return sb.String(), err
}

// traceCall records a single LLM call to the Tracer.
// It is nil-safe: if tr is nil, it does nothing.
// errVal drives the result field: nil → "ok", non-nil → "failed".
func traceCall(tr *Tracer, runID string, phase Phase, agent, model, prompt, response string, tokens int, errVal error) {
	if tr == nil {
		return
	}
	result := "ok"
	evidence := ""
	if errVal != nil {
		result = "failed"
		evidence = errVal.Error()
	}
	h := sha256.Sum256([]byte(prompt))
	_ = tr.Record(TraceEvent{
		Timestamp:  time.Now().UTC(),
		RunID:      runID,
		Phase:      phase,
		Action:     "llm_call",
		Agent:      agent,
		PromptHash: hex.EncodeToString(h[:4]),
		TokensIn:   tokens / 2, // rough split: half in, half out
		TokensOut:  tokens / 2,
		Result:     result,
		Evidence:   evidence,
		Meta:       map[string]string{"model": model},
	})
}
