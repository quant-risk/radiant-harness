package routing

import (
	"testing"
)

func TestResolveClaudeSubagent(t *testing.T) {
	plan := Resolve("claude-sonnet-4-6", AgentClaude, StrategySubagentDelegation)

	if plan.Anchor != "claude-sonnet-4-6" {
		t.Errorf("anchor = %q", plan.Anchor)
	}
	if plan.Family != "claude" {
		t.Errorf("family = %q, want claude", plan.Family)
	}
	if plan.Strategy != StrategySubagentDelegation {
		t.Errorf("strategy = %q", plan.Strategy)
	}

	// Research should use opus via subagent.
	research := plan.Phases[PhaseResearch]
	if research.Model != "claude-opus-4-8" {
		t.Errorf("research model = %q, want claude-opus-4-8", research.Model)
	}
	if research.Tier != "top" {
		t.Errorf("research tier = %q, want top", research.Tier)
	}
	if research.Via != "subagent" {
		t.Errorf("research via = %q, want subagent", research.Via)
	}

	// Implement should use sonnet via main.
	impl := plan.Phases[PhaseImplement]
	if impl.Model != "claude-sonnet-4-6" {
		t.Errorf("implement model = %q, want claude-sonnet-4-6", impl.Model)
	}
	if impl.Via != "main" {
		t.Errorf("implement via = %q, want main", impl.Via)
	}

	// Verify should use opus via subagent.
	verify := plan.Phases[PhaseVerify]
	if verify.Model != "claude-opus-4-8" {
		t.Errorf("verify model = %q, want claude-opus-4-8", verify.Model)
	}
	if verify.Via != "subagent" {
		t.Errorf("verify via = %q, want subagent", verify.Via)
	}

	// Summarize should use haiku via subagent.
	summ := plan.Phases[PhaseSummarize]
	if summ.Model != "claude-haiku-4-5" {
		t.Errorf("summarize model = %q, want claude-haiku-4-5", summ.Model)
	}
}

func TestResolveDirectAPI(t *testing.T) {
	plan := Resolve("claude-sonnet-4-6", AgentRadiant, StrategyDirectAPI)

	research := plan.Phases[PhaseResearch]
	if research.Via != "api" {
		t.Errorf("research via = %q, want api", research.Via)
	}
	if research.Model != "claude-opus-4-8" {
		t.Errorf("research model = %q, want claude-opus-4-8", research.Model)
	}
}

func TestResolveGLM(t *testing.T) {
	plan := Resolve("glm-5.2", AgentRadiant, StrategyDirectAPI)

	if plan.Family != "glm" {
		t.Fatalf("family = %q, want glm", plan.Family)
	}

	research := plan.Phases[PhaseResearch]
	if research.Model != "glm-5.2" {
		t.Errorf("research model = %q, want glm-5.2", research.Model)
	}

	impl := plan.Phases[PhaseImplement]
	if impl.Model != "glm-5.2-air" {
		t.Errorf("implement model = %q, want glm-5.2-air", impl.Model)
	}
}

func TestResolveUnknownAnchorPassthrough(t *testing.T) {
	plan := Resolve("totally-unknown-model", AgentCodex, StrategySingleModelAdvisory)

	if plan.Family != "" {
		t.Errorf("family = %q, want empty", plan.Family)
	}

	// All phases should get the anchor (passthrough).
	for _, phase := range AllPhases() {
		pr := plan.Phases[phase]
		if pr.Model != "totally-unknown-model" {
			t.Errorf("%s model = %q, want passthrough", phase, pr.Model)
		}
	}
}

func TestResolveSingleTierFamily(t *testing.T) {
	// Groq: top and mid are the same model.
	plan := Resolve("groq-llama-3.3-70b", AgentRadiant, StrategyDirectAPI)

	research := plan.Phases[PhaseResearch]
	impl := plan.Phases[PhaseImplement]
	if research.Model != impl.Model {
		t.Errorf("groq top and mid should be same: research=%s, impl=%s",
			research.Model, impl.Model)
	}

	// Budget should be the smaller model.
	budget := plan.Phases[PhasePersist]
	if budget.Model != "groq-llama-3.3-8b" {
		t.Errorf("groq budget model = %q, want groq-llama-3.3-8b", budget.Model)
	}
}

func TestResolveAllPhasesPresent(t *testing.T) {
	plan := Resolve("gpt-5", AgentRadiant, StrategyDirectAPI)

	for _, phase := range AllPhases() {
		pr, ok := plan.Phases[phase]
		if !ok {
			t.Errorf("phase %q missing from plan", phase)
			continue
		}
		if pr.Model == "" {
			t.Errorf("phase %q has empty model", phase)
		}
		if pr.Tier == "" {
			t.Errorf("phase %q has empty tier", phase)
		}
		if pr.Via == "" {
			t.Errorf("phase %q has empty via", phase)
		}
	}
}

func TestResolveAdvisoryAllViaAdvisory(t *testing.T) {
	plan := Resolve("gpt-5", AgentCodex, StrategySingleModelAdvisory)

	for _, phase := range AllPhases() {
		pr := plan.Phases[phase]
		if pr.Via != "advisory" {
			t.Errorf("%s via = %q, want advisory", phase, pr.Via)
		}
	}
}

func TestFormatPlan(t *testing.T) {
	plan := Resolve("claude-sonnet-4-6", AgentClaude, StrategySubagentDelegation)
	out := FormatPlan(plan)

	// Should contain key headers.
	if !contains(out, "PHASE") {
		t.Error("FormatPlan missing PHASE header")
	}
	if !contains(out, "claude") {
		t.Error("FormatPlan missing family")
	}
	if !contains(out, "claude-opus-4-8") {
		t.Error("FormatPlan missing model")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && len(substr) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
