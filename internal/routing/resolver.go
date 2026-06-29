package routing

import (
	"fmt"
	"strings"
)

// Resolve builds a RoutingPlan from an anchor model + detected agent.
// The anchor is the user's session model (e.g. "claude-sonnet-4-6").
// The agent and strategy determine how routing artifacts are delivered.
//
// If the anchor's family is unknown, every phase gets the anchor
// (passthrough) — the resolver never returns an empty model.
func Resolve(anchor string, agent AgentID, strategy Strategy) *RoutingPlan {
	family := FamilyOf(anchor)

	phases := make(map[Phase]PhaseRouting, len(AllPhases()))
	for _, phase := range AllPhases() {
		model := anchor // default: passthrough
		tier := TierForPhase(phase)

		if family != "" {
			if m := ModelForTier(family, tier); m != "" {
				model = m
			}
		}

		via := viaFor(strategy, phase)
		phases[phase] = PhaseRouting{
			Phase: string(phase),
			Model: model,
			Tier:  string(tier),
			Via:   via,
		}
	}

	return &RoutingPlan{
		Agent:    agent,
		Strategy: strategy,
		Anchor:   anchor,
		Family:   family,
		Phases:   phases,
	}
}

// ResolveAndDetect calls DetectAgent first, then Resolve.
// Convenience for callers who don't already know the agent.
func ResolveAndDetect(anchor string, projectDir string) *RoutingPlan {
	agent, strategy := DetectAgent(projectDir)
	return Resolve(anchor, agent, strategy)
}

// viaFor determines the delivery mechanism for a phase, given the
// overall strategy.
//
//   - direct_api: everything is "api"
//   - subagent_delegation: research/plan/verify/summarize go to
//     subagents; implement/correct stay on "main"
//   - delegate_task: everything is "delegate"
//   - config_per_role: everything is "config"
//   - single_model_advisory: everything is "advisory"
func viaFor(strategy Strategy, phase Phase) string {
	switch strategy {
	case StrategyDirectAPI:
		return "api"

	case StrategySubagentDelegation:
		switch phase {
		case PhaseResearch, PhasePlan, PhaseVerify, PhaseSummarize:
			return "subagent"
		case PhaseImplement, PhaseCorrect:
			return "main"
		case PhasePersist:
			return "main"
		}
		return "main"

	case StrategyDelegateTask:
		return "delegate"

	case StrategyConfigPerRole:
		return "config"

	case StrategySingleModelAdvisory:
		return "advisory"
	}
	return "advisory"
}

// FormatPlan renders a RoutingPlan as a human-readable table.
func FormatPlan(plan *RoutingPlan) string {
	var sb strings.Builder
	agentLabel := string(plan.Agent)
	if agentLabel == "" {
		agentLabel = "(unknown)"
	}

	sb.WriteString("Detected agent: " + agentLabel + "\n")
	sb.WriteString("Strategy:       " + string(plan.Strategy) + "\n")
	sb.WriteString("Anchor model:   " + plan.Anchor + "\n")
	if plan.Family != "" {
		sb.WriteString("Family:         " + plan.Family + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-14s %-24s %-8s %s\n", "PHASE", "MODEL", "TIER", "VIA"))
	sb.WriteString(strings.Repeat("-", 60) + "\n")
	for _, phase := range AllPhases() {
		pr := plan.Phases[phase]
		sb.WriteString(fmt.Sprintf("%-14s %-24s %-8s %s\n",
			pr.Phase, pr.Model, pr.Tier, pr.Via))
	}
	return sb.String()
}
