// Package routing implements agent-aware, tier-based model routing.
//
// The engine detects which agent is hosting the radiant session and
// emits routing artifacts that agent can actually consume. It replaces
// the heuristic AutoRoute in internal/llm/routing.go with a structured
// system that works across all modes: direct API, hosted-agent (static
// config), and hybrid.
//
// Design doc: docs/MODEL-ROUTING.md
package routing

// Strategy is how routing artifacts reach the hosting agent.
type Strategy string

const (
	// StrategyDirectAPI is the golden path: radiant calls LLM APIs
	// directly. Each phase can use a different model at runtime.
	StrategyDirectAPI Strategy = "direct_api"

	// StrategySubagentDelegation targets agents whose tooling can
	// spawn subagents with a specific model override (Claude Code's
	// Task tool). Emits settings.json + slash command.
	StrategySubagentDelegation Strategy = "subagent_delegation"

	// StrategyDelegateTask targets agents with a multi-agent
	// delegation primitive (Hermes Agent). Emits a routing YAML.
	StrategyDelegateTask Strategy = "delegate_task_routing"

	// StrategyConfigPerRole targets agents that read a config file
	// with per-role model slots (OpenCode, Roo Code, Kilo Code).
	StrategyConfigPerRole Strategy = "config_per_role"

	// StrategySingleModelAdvisory is the fallback: the agent uses one
	// model per session and cannot switch. Emits advisory text.
	StrategySingleModelAdvisory Strategy = "single_model_advisory"
)

// AgentID identifies a supported host agent. Re-exported conceptually
// from internal/types.go but kept as a plain string here to avoid an
// import dependency on the parent package.
type AgentID string

const (
	AgentClaude   AgentID = "claude"
	AgentCodex    AgentID = "codex"
	AgentCursor   AgentID = "cursor"
	AgentCopilot  AgentID = "copilot"
	AgentGemini   AgentID = "gemini"
	AgentWindsurf AgentID = "windsurf"
	AgentHermes   AgentID = "hermes"
	AgentOpenCode AgentID = "opencode"
	// AgentRadiant represents the harness itself in direct API mode.
	AgentRadiant AgentID = "radiant"
)

// Phase identifies which stage of the development cycle a model call
// belongs to. Each phase maps to exactly one tier.
type Phase string

const (
	PhaseResearch  Phase = "research"
	PhasePlan      Phase = "plan"
	PhaseImplement Phase = "implement"
	PhaseCorrect   Phase = "correct"
	PhaseVerify    Phase = "verify"
	PhasePersist   Phase = "persist"
	PhaseSummarize Phase = "summarize"
)

// Tier classifies a model by relative cost/capability.
type Tier string

const (
	TierTop    Tier = "top"    // expensive, strongest reasoning
	TierMid    Tier = "mid"    // balanced, good code generation
	TierBudget Tier = "budget" // cheap, fast, for trivial work
)

// RoutingPlan is the resolved set of per-phase model assignments.
// This is the output of Resolve() and the input to Emit().
type RoutingPlan struct {
	Agent    AgentID                `json:"agent"`
	Strategy Strategy               `json:"strategy"`
	Anchor   string                 `json:"anchor"` // user's session model
	Family   string                 `json:"family"` // "claude", "openai", etc.
	Phases   map[Phase]PhaseRouting `json:"phases"`
}

// PhaseRouting is one phase's model assignment.
type PhaseRouting struct {
	Phase string `json:"phase"`
	Model string `json:"model"` // canonical preset name, e.g. "claude-opus-4-8"
	Tier  string `json:"tier"`  // "top", "mid", "budget"
	Via   string `json:"via"`   // "main", "subagent", "api", "advisory"
}

// FamilyTier holds the three model tiers for a vendor family.
// If a family lacks a distinct model at a tier, the adjacent tier
// fills in (Top==Mid for a single-tier family).
type FamilyTier struct {
	Top    string `json:"top"`
	Mid    string `json:"mid"`
	Budget string `json:"budget"`
}
