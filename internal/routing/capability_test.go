package routing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAgentRadiantLoop(t *testing.T) {
	dir := t.TempDir()
	rhDir := filepath.Join(dir, ".radiant-harness")
	os.MkdirAll(rhDir, 0o755)
	os.WriteFile(filepath.Join(rhDir, "loop.json"), []byte("{}"), 0o644)

	agent, strategy := DetectAgent(dir)
	if agent != AgentRadiant {
		t.Errorf("agent = %q, want %q", agent, AgentRadiant)
	}
	if strategy != StrategyDirectAPI {
		t.Errorf("strategy = %q, want %q", strategy, StrategyDirectAPI)
	}
}

func TestDetectAgentClaude(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)

	agent, strategy := DetectAgent(dir)
	if agent != AgentClaude {
		t.Errorf("agent = %q, want %q", agent, AgentClaude)
	}
	if strategy != StrategySubagentDelegation {
		t.Errorf("strategy = %q, want %q", strategy, StrategySubagentDelegation)
	}
}

func TestDetectAgentCursor(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o755)

	agent, strategy := DetectAgent(dir)
	if agent != AgentCursor {
		t.Errorf("agent = %q, want %q", agent, AgentCursor)
	}
	if strategy != StrategySingleModelAdvisory {
		t.Errorf("strategy = %q, want %q", strategy, StrategySingleModelAdvisory)
	}
}

func TestDetectAgentCopilot(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	os.WriteFile(filepath.Join(dir, ".github", "copilot-instructions.md"),
		[]byte("# Copilot"), 0o644)

	agent, strategy := DetectAgent(dir)
	if agent != AgentCopilot {
		t.Errorf("agent = %q, want %q", agent, AgentCopilot)
	}
	if strategy != StrategySingleModelAdvisory {
		t.Errorf("strategy = %q, want %q", strategy, StrategySingleModelAdvisory)
	}
}

func TestDetectAgentWindsurf(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".windsurf"), 0o755)

	agent, strategy := DetectAgent(dir)
	if agent != AgentWindsurf {
		t.Errorf("agent = %q, want %q", agent, AgentWindsurf)
	}
	if strategy != StrategySingleModelAdvisory {
		t.Errorf("strategy = %q, want %q", strategy, StrategySingleModelAdvisory)
	}
}

func TestDetectAgentDefaultFallback(t *testing.T) {
	// Use a fake HOME so ~/.hermes on the test runner's machine
	// doesn't trigger a false Hermes detection.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	agent, strategy := DetectAgent(dir)
	if agent != "" {
		t.Errorf("agent = %q, want empty", agent)
	}
	if strategy != StrategySingleModelAdvisory {
		t.Errorf("strategy = %q, want %q", strategy, StrategySingleModelAdvisory)
	}
}

func TestDetectAgentPriorityClaudeOverCursor(t *testing.T) {
	// Both .claude/ and .cursor/rules/ exist -> Claude wins.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o755)

	agent, _ := DetectAgent(dir)
	if agent != AgentClaude {
		t.Errorf("when .claude and .cursor coexist, agent = %q, want %q",
			agent, AgentClaude)
	}
}

func TestDetectAgentPriorityRadiantOverClaude(t *testing.T) {
	// loop.json + .claude/ -> radiant wins.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	rhDir := filepath.Join(dir, ".radiant-harness")
	os.MkdirAll(rhDir, 0o755)
	os.WriteFile(filepath.Join(rhDir, "loop.json"), []byte("{}"), 0o644)

	agent, _ := DetectAgent(dir)
	if agent != AgentRadiant {
		t.Errorf("when loop.json and .claude coexist, agent = %q, want %q",
			agent, AgentRadiant)
	}
}

func TestAgentStrategy(t *testing.T) {
	tests := []struct {
		agent AgentID
		want  Strategy
	}{
		{AgentClaude, StrategySubagentDelegation},
		{AgentHermes, StrategyDelegateTask},
		{AgentOpenCode, StrategyConfigPerRole},
		{AgentRadiant, StrategyDirectAPI},
		{AgentCodex, StrategySingleModelAdvisory},
		{AgentGemini, StrategySingleModelAdvisory},
		{AgentCursor, StrategySingleModelAdvisory},
		{AgentCopilot, StrategySingleModelAdvisory},
		{AgentWindsurf, StrategySingleModelAdvisory},
	}
	for _, tt := range tests {
		t.Run(string(tt.agent), func(t *testing.T) {
			if got := AgentStrategy(tt.agent); got != tt.want {
				t.Errorf("AgentStrategy(%s) = %s, want %s",
					tt.agent, got, tt.want)
			}
		})
	}
}
