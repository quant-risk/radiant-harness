package routing

import (
	"os"
	"path/filepath"
)

// DetectAgent examines the project directory and environment to
// determine which agent is hosting this radiant session.
//
// Returns the primary agent and the routing strategy it supports.
// Detection is layered: the first match wins.
func DetectAgent(projectDir string) (AgentID, Strategy) {
	// 1. radiant loop mode — always highest priority.
	if fileExists(filepath.Join(projectDir, ".radiant-harness", "loop.json")) {
		return AgentRadiant, StrategyDirectAPI
	}

	// 2. Claude Code — can delegate to subagents with model override.
	if dirExists(filepath.Join(projectDir, ".claude")) {
		return AgentClaude, StrategySubagentDelegation
	}

	// 3. OpenCode — reads config with per-role model slots.
	if dirExists(filepath.Join(projectDir, ".opencode")) {
		return AgentOpenCode, StrategyConfigPerRole
	}

	// 4. Cursor — .cursor/rules/ exists.
	if dirExists(filepath.Join(projectDir, ".cursor", "rules")) {
		return AgentCursor, StrategySingleModelAdvisory
	}

	// 5. Copilot — copilot instructions exist.
	if fileExists(filepath.Join(projectDir, ".github", "copilot-instructions.md")) {
		return AgentCopilot, StrategySingleModelAdvisory
	}

	// 6. Windsurf — .windsurf/ exists.
	if dirExists(filepath.Join(projectDir, ".windsurf")) {
		return AgentWindsurf, StrategySingleModelAdvisory
	}

	// 7. Codex CLI — on PATH and AGENTS.md present.
	if hasOnPath("codex") && fileExists(filepath.Join(projectDir, "AGENTS.md")) {
		return AgentCodex, StrategySingleModelAdvisory
	}

	// 8. Gemini CLI — on PATH.
	if hasOnPATH("gemini") {
		return AgentGemini, StrategySingleModelAdvisory
	}

	// 9. Hermes Agent — global tool, checked after project-local IDE configs.
	home, _ := os.UserHomeDir()
	if dirExists(filepath.Join(home, ".hermes")) {
		return AgentHermes, StrategyDelegateTask
	}

	// 10. Default fallback.
	return "", StrategySingleModelAdvisory
}

// AgentStrategy returns the routing strategy for a given agent ID.
// Used by callers who already know the agent (e.g. from --agent flag).
func AgentStrategy(agent AgentID) Strategy {
	switch agent {
	case AgentClaude:
		return StrategySubagentDelegation
	case AgentHermes:
		return StrategyDelegateTask
	case AgentOpenCode:
		return StrategyConfigPerRole
	case AgentRadiant:
		return StrategyDirectAPI
	case AgentCodex, AgentGemini, AgentCursor, AgentCopilot, AgentWindsurf:
		return StrategySingleModelAdvisory
	}
	return StrategySingleModelAdvisory
}

// fileExists returns true if path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// hasOnPath checks if a binary is on $PATH (POSIX-style, using PATH env).
// Named hasOnPath to avoid collision with hasOnPATH.
func hasOnPATH(name string) bool {
	return hasOnPath(name)
}

// hasOnPath checks if a binary is on $PATH.
func hasOnPath(name string) bool {
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return true
		}
	}
	return false
}
