// Package boot implements the Bootstrap Protocol — the universal entry point
// that any LLM or IDE can call to understand a project without prior knowledge.
//
// The boot manifest is intentionally small (≤500 tokens in Markdown mode,
// machine-readable JSON mode also available) so agents spend their token
// budget on actual work, not on orientation.
package boot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ctx "github.com/quant-risk/radiant-harness/internal/context"
	"github.com/quant-risk/radiant-harness/internal/routing"
)

// AgentFlavor controls which IDE/agent the manifest is tailored for.
type AgentFlavor string

const (
	FlavorGeneric  AgentFlavor = "generic"
	FlavorClaude   AgentFlavor = "claude"
	FlavorCursor   AgentFlavor = "cursor"
	FlavorCopilot  AgentFlavor = "copilot"
	FlavorGemini   AgentFlavor = "gemini"
	FlavorWindsurf AgentFlavor = "windsurf"
	FlavorCodex    AgentFlavor = "codex"
)

// Options controls boot manifest generation.
type Options struct {
	// Flavor tailors the manifest to a specific IDE/agent.
	Flavor AgentFlavor
	// JSON outputs a machine-readable manifest instead of Markdown.
	JSON bool
	// BudgetProfile is "lean" | "standard" | "thorough".
	BudgetProfile string
}

// Manifest is the structured representation of a boot manifest.
type Manifest struct {
	Version     string       `json:"version"`
	Project     ProjectInfo  `json:"project"`
	Skills      []string     `json:"recommended_skills"`
	Commands    []string     `json:"available_commands"`
	Loop        LoopInfo     `json:"loop"`
	Budget      BudgetInfo   `json:"budget_estimate"`
	ContextFile string       `json:"context_file"`
	ActiveSpec  string       `json:"active_spec,omitempty"`
	Routing     *RoutingInfo `json:"routing,omitempty"`
}

// ProjectInfo summarizes the project.
type ProjectInfo struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Tier   string `json:"tier"`
}

// LoopInfo describes how to start the autonomous loop.
type LoopInfo struct {
	Pattern string `json:"pattern"`
	Start   string `json:"start_command"`
	Status  string `json:"status_command"`
}

// BudgetInfo estimates token cost for common operations.
type BudgetInfo struct {
	Profile     string `json:"profile"`
	ContextMin  int    `json:"context_min_tokens"`
	ContextMax  int    `json:"context_max_tokens"`
	LoopPerIter int    `json:"loop_per_iteration_tokens"`
}

// RoutingInfo describes the detected model routing plan.
// Populated when a routing-capable agent is detected.
type RoutingInfo struct {
	Agent    string                  `json:"agent,omitempty"`
	Strategy string                  `json:"strategy,omitempty"`
	Anchor   string                  `json:"anchor,omitempty"`
	Family   string                  `json:"family,omitempty"`
	Phases   map[string]RoutingPhase `json:"phases,omitempty"`
}

// RoutingPhase is one phase's model in the boot manifest.
type RoutingPhase struct {
	Model string `json:"model"`
	Tier  string `json:"tier"`
	Via   string `json:"via"`
}

var budgetProfiles = map[string]BudgetInfo{
	"lean": {
		Profile:     "lean",
		ContextMin:  500,
		ContextMax:  2000,
		LoopPerIter: 3000,
	},
	"standard": {
		Profile:     "standard",
		ContextMin:  2000,
		ContextMax:  8000,
		LoopPerIter: 8000,
	},
	"thorough": {
		Profile:     "thorough",
		ContextMin:  8000,
		ContextMax:  30000,
		LoopPerIter: 20000,
	},
}

// Generate produces a boot manifest for projectDir.
func Generate(projectDir string, opts Options) (*Manifest, error) {
	detection, err := ctx.Detect(projectDir)
	if err != nil {
		return nil, fmt.Errorf("detect project: %w", err)
	}

	profile := opts.BudgetProfile
	if profile == "" {
		profile = "standard"
	}
	budget, ok := budgetProfiles[profile]
	if !ok {
		budget = budgetProfiles["standard"]
	}

	// Check if CONTEXT.md exists — if not, note it needs assembly
	contextFile := filepath.Join(projectDir, ".radiant-harness", "CONTEXT.md")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		contextFile = "(not yet generated — run: radiant context assemble)"
	} else {
		rel, _ := filepath.Rel(projectDir, contextFile)
		contextFile = rel
	}

	m := &Manifest{
		Version: "2.0",
		Project: ProjectInfo{
			Name:   detection.ProjectName,
			Domain: string(detection.Domain),
			Tier:   string(detection.Tier),
		},
		Skills:   detection.RecommendedSkills,
		Commands: coreCommands(detection),
		Loop: LoopInfo{
			Pattern: "discover → plan → execute → verify → persist",
			Start:   fmt.Sprintf(`radiant loop start "<goal>" --budget=%d`, budget.ContextMax*5),
			Status:  "radiant loop status",
		},
		Budget:      budget,
		ContextFile: contextFile,
		ActiveSpec:  detection.ActiveSpec,
	}

	// Populate routing info from the routing package.
	anchor := inferAnchor(detection)
	rAgent, rStrategy := routing.DetectAgent(projectDir)
	if anchor != "" && rStrategy != "" {
		plan := routing.Resolve(anchor, rAgent, rStrategy)
		if plan != nil {
			phases := make(map[string]RoutingPhase, len(plan.Phases))
			for _, pr := range plan.Phases {
				phases[pr.Phase] = RoutingPhase{
					Model: pr.Model,
					Tier:  pr.Tier,
					Via:   pr.Via,
				}
			}
			m.Routing = &RoutingInfo{
				Agent:    string(plan.Agent),
				Strategy: string(plan.Strategy),
				Anchor:   plan.Anchor,
				Family:   plan.Family,
				Phases:   phases,
			}
		}
	}

	return m, nil
}

// RenderMarkdown produces the ≤500-token Markdown manifest.
func RenderMarkdown(m *Manifest, flavor AgentFlavor) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Radiant Boot — %s\n\n", m.Project.Name))
	sb.WriteString(fmt.Sprintf("Domain: **%s** | Tier: **%s**\n\n", m.Project.Domain, m.Project.Tier))

	if m.ActiveSpec != "" {
		sb.WriteString(fmt.Sprintf("Active spec: `%s`\n\n", m.ActiveSpec))
	}

	sb.WriteString("## Recommended Skills\n\n")
	for _, s := range m.Skills {
		sb.WriteString(fmt.Sprintf("- `%s`\n", s))
	}
	sb.WriteString("\n")

	sb.WriteString("## Getting Started\n\n")
	sb.WriteString("```sh\n")
	sb.WriteString("# 1. Load minimal context (only relevant skills)\n")
	sb.WriteString("radiant context assemble\n\n")
	sb.WriteString("# 2. Start autonomous loop\n")
	sb.WriteString(m.Loop.Start + "\n\n")
	sb.WriteString("# 3. Check progress\n")
	sb.WriteString(m.Loop.Status + "\n")
	sb.WriteString("```\n\n")

	// Flavor-specific hints
	switch flavor {
	case FlavorClaude:
		sb.WriteString("## Claude Code Tips\n\n")
		sb.WriteString("- Context is in `.radiant-harness/CONTEXT.md` — load it at session start\n")
		sb.WriteString("- Skills are in `.radiant-harness/skills/<name>/SKILL.md`\n")
		sb.WriteString("- Hooks auto-load context: `radiant views --agent=claude` to regenerate\n\n")
	case FlavorCursor:
		sb.WriteString("## Cursor Tips\n\n")
		sb.WriteString("- Rules live in `.cursor/rules/` — run `radiant views --agent=cursor` to refresh\n")
		sb.WriteString("- Use `@CONTEXT.md` to pin minimal context in chat\n\n")
	case FlavorCopilot:
		sb.WriteString("## Copilot Tips\n\n")
		sb.WriteString("- Instructions in `.github/copilot-instructions.md`\n")
		sb.WriteString("- Run `radiant views --agent=copilot` to regenerate\n\n")
	}

	sb.WriteString(fmt.Sprintf("Context file: `%s`  \n", m.ContextFile))
	sb.WriteString(fmt.Sprintf("Budget profile: **%s** (%d–%d tokens/context, %d tokens/iter)\n",
		m.Budget.Profile, m.Budget.ContextMin, m.Budget.ContextMax, m.Budget.LoopPerIter))

	return sb.String()
}

// RenderJSON produces a machine-readable JSON manifest.
func RenderJSON(m *Manifest) (string, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// coreCommands returns the most relevant radiant commands for this project.
func coreCommands(detection *ctx.DetectionResult) []string {
	cmds := []string{
		"radiant boot",
		"radiant context assemble",
		"radiant context detect",
		"radiant loop start \"<goal>\"",
		"radiant loop status",
	}
	if detection.ActiveSpec != "" {
		cmds = append(cmds, "radiant validate "+detection.ActiveSpec)
	}
	return cmds
}

// inferAnchor picks a sensible default anchor model.
// Future: read from .radiant-harness/config; for now use a balanced default.
func inferAnchor(detection *ctx.DetectionResult) string {
	return "claude-sonnet-4-6"
}
