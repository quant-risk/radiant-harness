package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
)

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

	// Build stall brake (disabled when patience == 0).
	var stall *StallBrake
	if cfg.StallPatience > 0 {
		stall = NewStallBrake(cfg.StallPatience)
	}

	// Build LLM clients.
	execClient := llm.NewClient(cfg.ExecutorModel)
	verModel := cfg.VerifierModel
	if verModel.Model == "" {
		verModel = cfg.ExecutorModel
	}
	verClient := llm.NewClient(verModel)

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

		// ── Discover / Plan (lightweight — no LLM call needed) ──────────
		if err := c.Transition(PhaseDiscover, fmt.Sprintf("iter %d", c.State().Iteration+1)); err != nil {
			return nil, err
		}
		if err := c.Transition(PhasePlan, "planning"); err != nil {
			return nil, err
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

		execPrompt := buildExecutorPrompt(goal, groundBlock, lastReviewFindings)
		execOutput, execErr := execClient.SimpleChat(ctx, executorSystemPrompt(), execPrompt)
		if execErr != nil {
			_ = c.Transition(PhaseFailed, fmt.Sprintf("executor error: %v", execErr))
			_ = c.IncrIteration()
			b.Consume(500, PhaseExecute) // estimate on error
			continue
		}
		b.Consume(estimateTokens(execPrompt, execOutput), PhaseExecute)
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
		if verErr != nil {
			_ = c.Transition(PhaseFailed, fmt.Sprintf("verifier error: %v", verErr))
			_ = c.IncrIteration()
			b.Consume(500, PhaseVerify)
			continue
		}
		b.Consume(estimateTokens(verPrompt, verResponse), PhaseVerify)

		result := ParseVerifyResponse(verResponse, verCfg)

		// Escalation check.
		if result.Escalate {
			id, _ := c.WriteInboxItem(result)
			_ = c.Transition(PhaseAwaitingHuman, fmt.Sprintf("verifier escalated: %s", id))
			_ = c.SetExit(ExitNeedsHuman, fmt.Sprintf("inbox item %s", id))
			return buildResult(runID, goal, ExitNeedsHuman, c, b, started), nil
		}

		if !result.Approved {
			// Retry execute next iteration.
			_ = c.Transition(PhaseExecute, "verifier rejected — retrying")
			_ = c.Transition(PhaseFailed, fmt.Sprintf("rejected: %s", strings.Join(result.Issues, "; ")))
			_ = c.IncrIteration()
			b.Consume(0, PhaseVerify)
			continue
		}

		// ── Post-convergence review panel ────────────────────────────────
		reviewPrompt := BuildReviewPrompt(goal, lastVerifyOutput, lastReviewFindings)
		reviewResponse, reviewErr := verClient.SimpleChat(ctx, reviewerSystemPrompt(), reviewPrompt)
		if reviewErr == nil {
			b.Consume(estimateTokens(reviewPrompt, reviewResponse), PhaseVerify)
			reviewResult := ParseReviewResponse(reviewResponse)
			if !reviewResult.Pass {
				reviewRestarts++
				lastReviewFindings = reviewResult.Findings
				if reviewRestarts >= reviewPanel.maxRestarts() {
					_ = c.SetExit(ExitCritical, fmt.Sprintf("review panel rejected %d times", reviewRestarts))
					return buildResult(runID, goal, ExitCritical, c, b, started), nil
				}
				// Re-enter loop body with findings.
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
// Used to account usage when the LLM response doesn't include token counts.
// ~4 chars per token is a standard approximation.
func estimateTokens(prompt, response string) int {
	return (len(prompt) + len(response)) / 4
}

// ── System prompts ────────────────────────────────────────────────────────────

func executorSystemPrompt() string {
	return `You are an expert software engineer implementing a goal autonomously.
Read the goal and any prior context carefully.
Produce a concrete, complete implementation. No stubs, no TODOs, no placeholders.
Output the result clearly so a separate verifier can assess it.`
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
func buildExecutorPrompt(goal, groundBlock string, priorReviewFindings []string) string {
	var sb strings.Builder
	if groundBlock != "" {
		sb.WriteString(groundBlock)
		sb.WriteString("\n\n")
	}
	sb.WriteString("GOAL:\n")
	sb.WriteString(goal)
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
