// Package fleet implements multi-agent coordination.
// Multiple specialized agents (Planner, Implementer, Verifier, Summarizer)
// work in isolated worktrees, share a common context store, and converge
// through adversarial verification before results are merged.
package fleet

import "fmt"

// AgentRole identifies what a fleet agent does.
type AgentRole string

const (
	RolePlanner     AgentRole = "planner"
	RoleImplementer AgentRole = "implementer"
	RoleVerifier    AgentRole = "verifier"
	RoleSummarizer  AgentRole = "summarizer"
)

// RoleConfig defines the prompt template and budget for an agent role.
type RoleConfig struct {
	Role          AgentRole
	SystemPrompt  string
	TokenBudget   int // per-agent token budget
	MaxIterations int
}

// DefaultRoleConfigs returns the standard role configurations.
func DefaultRoleConfigs() map[AgentRole]RoleConfig {
	return map[AgentRole]RoleConfig{
		RolePlanner: {
			Role:          RolePlanner,
			TokenBudget:   8_000,
			MaxIterations: 3,
			SystemPrompt: `You are the Planner agent.
Your job: decompose the goal into discrete, non-overlapping tasks.
Each task must be:
- Self-contained (an Implementer can work on it without asking questions)
- Verifiable (has a clear done-condition)
- Scoped (touches ≤3 files)

Output format:
TASK 1: <title>
  Files: <list>
  Done when: <criterion>

TASK 2: ...`,
		},
		RoleImplementer: {
			Role:          RoleImplementer,
			TokenBudget:   25_000,
			MaxIterations: 5,
			SystemPrompt: `You are an Implementer agent.
Your job: execute ONE task from the plan — nothing more.
Rules:
- Read your task specification carefully before touching any file
- Stay within the specified files; touching others requires explicit approval
- Produce working code with tests
- Record EVIDENCE of completion (test output, diff summary)

Do not self-verify — the Verifier agent will review your work.`,
		},
		RoleVerifier: {
			Role:          RoleVerifier,
			TokenBudget:   10_000,
			MaxIterations: 2,
			SystemPrompt: `You are the Verifier agent. You are adversarial by design.
Assume the implementation is BROKEN until proven otherwise.
Your job: review a task implementation and produce a structured verdict.

VERDICT: [APPROVED|REJECTED]
SCORE: [0.0-1.0]
EVIDENCE: [specific proof or gap]
ISSUES:
- [list]`,
		},
		RoleSummarizer: {
			Role:          RoleSummarizer,
			TokenBudget:   5_000,
			MaxIterations: 1,
			SystemPrompt: `You are the Summarizer agent.
Your job: produce a compact summary of completed work for handoff.
Include:
- What was implemented (≤3 sentences)
- What was verified (test results)
- Any open blockers
- Next recommended action

Keep it under 200 tokens.`,
		},
	}
}

// FormatRoleConfig renders a RoleConfig for display.
func FormatRoleConfig(cfg RoleConfig) string {
	return fmt.Sprintf("[%s] budget=%d tokens, max_iter=%d",
		cfg.Role, cfg.TokenBudget, cfg.MaxIterations)
}
