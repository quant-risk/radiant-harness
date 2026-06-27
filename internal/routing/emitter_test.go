package routing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEmitDirectAPIReturnsNothing(t *testing.T) {
	dir := t.TempDir()
	plan := Resolve("claude-sonnet-4-6", AgentRadiant, StrategyDirectAPI)
	written, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Errorf("direct_api should write 0 files, got %d", len(written))
	}
}

func TestEmitSubagentDelegation(t *testing.T) {
	dir := t.TempDir()
	plan := Resolve("claude-sonnet-4-6", AgentClaude, StrategySubagentDelegation)
	written, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}

	// Should write settings.json and slash command.
	if len(written) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(written), written)
	}

	// Verify settings.json exists and is valid JSON.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json invalid JSON: %v", err)
	}
	routing, ok := settings["model_routing"]
	if !ok {
		t.Fatal("settings.json missing model_routing")
	}
	routingMap, ok := routing.(map[string]any)
	if !ok {
		t.Fatal("model_routing is not an object")
	}
	if len(routingMap) != 7 {
		t.Errorf("expected 7 phases in routing, got %d", len(routingMap))
	}

	// Verify slash command exists.
	cmdPath := filepath.Join(dir, ".claude", "commands", "radiant-route.md")
	if _, err := os.Stat(cmdPath); err != nil {
		t.Fatalf("slash command not written: %v", err)
	}
}

func TestEmitDelegateTask(t *testing.T) {
	dir := t.TempDir()
	plan := Resolve("claude-sonnet-4-6", AgentHermes, StrategyDelegateTask)
	written, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("expected 1 file, got %d", len(written))
	}

	path := filepath.Join(dir, ".radiant-harness", "routing-hermes.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("routing-hermes.yaml not written: %v", err)
	}
	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("routing-hermes.yaml invalid YAML: %v", err)
	}
	if parsed["planner_model"] == nil {
		t.Error("routing-hermes.yaml missing planner_model")
	}
	if parsed["verifier_model"] == nil {
		t.Error("routing-hermes.yaml missing verifier_model")
	}
}

func TestEmitConfigPerRole(t *testing.T) {
	dir := t.TempDir()
	plan := Resolve("gpt-5", AgentOpenCode, StrategyConfigPerRole)
	written, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("expected 1 file, got %d", len(written))
	}

	path := filepath.Join(dir, ".opencode", "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.json not written: %v", err)
	}
	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("config.json invalid JSON: %v", err)
	}
	models, ok := parsed["models"]
	if !ok {
		t.Fatal("config.json missing models")
	}
	modelsMap, ok := models.(map[string]any)
	if !ok {
		t.Fatal("models is not an object")
	}
	if _, ok := modelsMap["planner"]; !ok {
		t.Error("config.json missing models.planner")
	}
	if _, ok := modelsMap["worker"]; !ok {
		t.Error("config.json missing models.worker")
	}
	if _, ok := modelsMap["reviewer"]; !ok {
		t.Error("config.json missing models.reviewer")
	}
}

func TestEmitAdvisoryCodex(t *testing.T) {
	dir := t.TempDir()
	// Pre-create AGENTS.md so the advisory has somewhere to go.
	agentsPath := filepath.Join(dir, "AGENTS.md")
	os.WriteFile(agentsPath, []byte("# Project\n\nSome existing content.\n"), 0o644)

	plan := Resolve("gpt-5", AgentCodex, StrategySingleModelAdvisory)
	written, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("expected 1 file, got %d", len(written))
	}

	data, _ := os.ReadFile(agentsPath)
	content := string(data)
	if !stringContains(content, "## Model Routing Advisory") {
		t.Error("AGENTS.md missing Model Routing Advisory section")
	}
	// Original content should be preserved.
	if !stringContains(content, "Some existing content") {
		t.Error("AGENTS.md original content was lost")
	}
}

func TestEmitIdempotent(t *testing.T) {
	dir := t.TempDir()
	plan := Resolve("claude-sonnet-4-6", AgentClaude, StrategySubagentDelegation)

	_, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data1, _ := os.ReadFile(settingsPath)

	// Emit again.
	_, err = Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(settingsPath)

	if string(data1) != string(data2) {
		t.Error("Emit is not idempotent — second call produced different output")
	}
}

func TestEmitAdvisoryReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	// Pre-existing advisory with old model names.
	os.WriteFile(agentsPath, []byte("# Project\n\n## Model Routing Advisory\n\nOLD ADVICE\n\n## Other Section\n\nstuff\n"), 0o644)

	plan := Resolve("gpt-5", AgentCodex, StrategySingleModelAdvisory)
	_, err := Emit(dir, plan)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(agentsPath)
	content := string(data)
	// Old advice should be gone.
	if stringContains(content, "OLD ADVICE") {
		t.Error("old advisory not replaced")
	}
	// New advice should be present.
	if !stringContains(content, "Model Routing Advisory") {
		t.Error("new advisory not found")
	}
	// Section after it should be preserved.
	if !stringContains(content, "## Other Section") {
		t.Error("section after advisory was lost")
	}
}

// stringContains is a simple helper to avoid importing strings.
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
