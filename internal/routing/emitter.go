package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Emit writes routing artifacts for the plan's strategy.
// Returns the list of files written and any error.
// direct_api strategy returns an empty file list (routing is runtime).
func Emit(projectDir string, plan *RoutingPlan) ([]string, error) {
	switch plan.Strategy {
	case StrategyDirectAPI:
		return nil, nil // no static artifacts

	case StrategySubagentDelegation:
		return emitSubagentDelegation(projectDir, plan)

	case StrategyDelegateTask:
		return emitDelegateTask(projectDir, plan)

	case StrategyConfigPerRole:
		return emitConfigPerRole(projectDir, plan)

	case StrategySingleModelAdvisory:
		return emitAdvisory(projectDir, plan)
	}
	return nil, fmt.Errorf("unknown strategy: %s", plan.Strategy)
}

// emitSubagentDelegation writes .claude/settings.json with model_routing
// and .claude/commands/radiant-route.md slash command.
func emitSubagentDelegation(projectDir string, plan *RoutingPlan) ([]string, error) {
	var written []string

	// Build settings.json with model_routing section.
	type routeEntry struct {
		Model string `json:"model"`
		Via   string `json:"via"`
	}
	routingMap := make(map[string]routeEntry)
	for _, phase := range AllPhases() {
		pr := plan.Phases[phase]
		routingMap[string(phase)] = routeEntry{Model: pr.Model, Via: pr.Via}
	}

	settings := map[string]any{
		"model_routing": routingMap,
	}

	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir .claude: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(settingsPath, data); err != nil {
		return nil, err
	}
	written = append(written, settingsPath)

	// Slash command with human-readable routing instructions.
	cmdDir := filepath.Join(claudeDir, "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir .claude/commands: %w", err)
	}
	cmdPath := filepath.Join(cmdDir, "radiant-route.md")
	cmdContent := buildClaudeSlashCommand(plan)
	if err := atomicWrite(cmdPath, []byte(cmdContent)); err != nil {
		return nil, err
	}
	written = append(written, cmdPath)

	return written, nil
}

// emitDelegateTask writes .radiant-harness/routing-hermes.yaml.
func emitDelegateTask(projectDir string, plan *RoutingPlan) ([]string, error) {
	type yamlPlan struct {
		PlannerModel     string `yaml:"planner_model"`
		ImplementerModel string `yaml:"implementer_model"`
		VerifierModel    string `yaml:"verifier_model"`
		SummarizerModel  string `yaml:"summarizer_model"`
	}

	yp := yamlPlan{
		PlannerModel:     plan.Phases[PhasePlan].Model,
		ImplementerModel: plan.Phases[PhaseImplement].Model,
		VerifierModel:    plan.Phases[PhaseVerify].Model,
		SummarizerModel:  plan.Phases[PhaseSummarize].Model,
	}

	data, err := yaml.Marshal(yp)
	if err != nil {
		return nil, err
	}

	rhDir := filepath.Join(projectDir, ".radiant-harness")
	if err := os.MkdirAll(rhDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(rhDir, "routing-hermes.yaml")
	if err := atomicWrite(path, data); err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// emitConfigPerRole writes agent-specific config JSON.
func emitConfigPerRole(projectDir string, plan *RoutingPlan) ([]string, error) {
	type roleConfig struct {
		Model string `json:"model"`
	}
	type opencodeConfig struct {
		Models struct {
			Planner  roleConfig `json:"planner"`
			Worker   roleConfig `json:"worker"`
			Reviewer roleConfig `json:"reviewer"`
		} `json:"models"`
	}

	var cfg opencodeConfig
	cfg.Models.Planner = roleConfig{Model: plan.Phases[PhasePlan].Model}
	cfg.Models.Worker = roleConfig{Model: plan.Phases[PhaseImplement].Model}
	cfg.Models.Reviewer = roleConfig{Model: plan.Phases[PhaseVerify].Model}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(projectDir, ".opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	if err := atomicWrite(path, data); err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// emitAdvisory injects a Model Routing Advisory section into the
// agent's instructions file. Creates the file if it doesn't exist.
func emitAdvisory(projectDir string, plan *RoutingPlan) ([]string, error) {
	path := agentInstructionsPath(projectDir, plan.Agent)
	if path == "" {
		return nil, nil // nowhere to write
	}

	section := buildAdvisorySection(plan)

	existing, _ := os.ReadFile(path)
	content := string(existing)

	// If the section already exists, replace it. Otherwise append.
	marker := "## Model Routing Advisory"
	if idx := strings.Index(content, marker); idx >= 0 {
		// Find the end of the section (next ## or end of file).
		rest := content[idx:]
		nextSection := strings.Index(rest[1:], "\n## ")
		var endIdx int
		if nextSection >= 0 {
			endIdx = idx + 1 + nextSection
		} else {
			endIdx = len(content)
		}
		content = content[:idx] + section + content[endIdx:]
	} else {
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + section
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// agentInstructionsPath returns the path to the agent's instructions file.
func agentInstructionsPath(projectDir string, agent AgentID) string {
	switch agent {
	case AgentCodex:
		return filepath.Join(projectDir, "AGENTS.md")
	case AgentGemini:
		return filepath.Join(projectDir, "GEMINI.md")
	case AgentCursor:
		return filepath.Join(projectDir, ".cursor", "rules", "sdd.mdc")
	case AgentCopilot:
		return filepath.Join(projectDir, ".github", "copilot-instructions.md")
	case AgentWindsurf:
		return filepath.Join(projectDir, ".windsurf", "rules", "sdd.md")
	}
	return ""
}

// buildAdvisorySection returns the advisory markdown block.
func buildAdvisorySection(plan *RoutingPlan) string {
	var sb strings.Builder
	sb.WriteString("## Model Routing Advisory\n\n")
	sb.WriteString("This project benefits from tiered model usage.\n")
	agentLabel := string(plan.Agent)
	if agentLabel == "" {
		agentLabel = "this agent"
	}
	sb.WriteString(fmt.Sprintf("Since %s uses one model per session, ", agentLabel))
	sb.WriteString("here's the efficiency guidance:\n\n")

	// Group by tier for readability.
	for _, tier := range []Tier{TierTop, TierMid, TierBudget} {
		var phases []string
		model := ""
		for _, phase := range AllPhases() {
			pr := plan.Phases[phase]
			if Tier(pr.Tier) == tier {
				phases = append(phases, pr.Phase)
				if model == "" {
					model = pr.Model
				}
			}
		}
		if len(phases) > 0 {
			sb.WriteString(fmt.Sprintf("- **%s tier** (%s): %s\n",
				tier, model, strings.Join(phases, ", ")))
		}
	}
	sb.WriteString("\nWhen using `radiant loop start`, multi-model routing is automatic.\n")
	return sb.String()
}

// buildClaudeSlashCommand creates the .claude/commands/radiant-route.md.
func buildClaudeSlashCommand(plan *RoutingPlan) string {
	var sb strings.Builder
	sb.WriteString("# Radiant Model Routing\n\n")
	sb.WriteString("This project uses tiered model routing for efficiency.\n\n")

	for _, phase := range AllPhases() {
		pr := plan.Phases[phase]
		sb.WriteString(fmt.Sprintf("**%s**: `%s` via %s\n", pr.Phase, pr.Model, pr.Via))
	}

	sb.WriteString("\n## When to use subagents\n\n")
	sb.WriteString("Use the Task tool with model override for:\n")
	for _, phase := range []Phase{PhaseResearch, PhasePlan, PhaseVerify, PhaseSummarize} {
		pr := plan.Phases[phase]
		if pr.Via == "subagent" {
			sb.WriteString(fmt.Sprintf("- %s: `Task(model=\"%s\", ...)`\n", pr.Phase, pr.Model))
		}
	}
	sb.WriteString("\n## When to use main model\n\n")
	for _, phase := range []Phase{PhaseImplement, PhaseCorrect} {
		pr := plan.Phases[phase]
		sb.WriteString(fmt.Sprintf("- %s: proceed with `%s`\n", pr.Phase, pr.Model))
	}
	return sb.String()
}

// atomicWrite writes data to path via temp file + rename.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".route-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	tmp.Close()
	return os.Rename(tmpName, path)
}
