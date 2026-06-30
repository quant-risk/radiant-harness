package scaffold

import radiant "github.com/quant-risk/radiant-harness/v3/internal"

// adapters is the registry of all supported agent adapters.
var adapters = map[radiant.AgentID]*radiant.AgentAdapter{
	radiant.AgentClaude: {
		ID: radiant.AgentClaude, Label: "Claude Code", Canonical: true,
		InstTo: "CONVENTIONS.md", InstFM: "keep",
		SkillsDir: ".claude/skills", SkillsLayout: "skill-dir",
	},
	radiant.AgentCodex: {
		ID: radiant.AgentCodex, Label: "OpenAI Codex",
		InstTo: "AGENTS.md", InstFM: "strip",
		SkillsDir: ".agents/skills", SkillsLayout: "skill-dir",
	},
	radiant.AgentCursor: {
		ID: radiant.AgentCursor, Label: "Cursor",
		InstTo: ".cursor/rules/sdd.mdc", InstFM: "keep",
		SkillsDir: ".cursor/commands", SkillsLayout: "flat", SkillsExt: "md",
	},
	radiant.AgentCopilot: {
		ID: radiant.AgentCopilot, Label: "GitHub Copilot",
		InstTo: ".github/copilot-instructions.md", InstFM: "strip",
		SkillsDir: ".github/prompts", SkillsLayout: "flat", SkillsExt: "prompt.md",
	},
	radiant.AgentGemini: {
		ID: radiant.AgentGemini, Label: "Gemini CLI",
		InstTo: "GEMINI.md", InstFM: "strip",
		SkillsDir: ".gemini/commands", SkillsLayout: "flat", SkillsExt: "toml",
	},
	radiant.AgentWindsurf: {
		ID: radiant.AgentWindsurf, Label: "Windsurf",
		InstTo: ".windsurf/rules/sdd.md", InstFM: "strip",
		SkillsDir: ".windsurf/workflows", SkillsLayout: "flat", SkillsExt: "md",
	},
}

// GetAdapter returns the adapter for the given agent ID.
func GetAdapter(id radiant.AgentID) *radiant.AgentAdapter {
	return adapters[id]
}

// AllAdapters returns all adapters.
func AllAdapters() map[radiant.AgentID]*radiant.AgentAdapter {
	return adapters
}
