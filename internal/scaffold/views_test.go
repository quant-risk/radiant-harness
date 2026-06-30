package scaffold

import (
	"strings"
	"testing"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
	"github.com/quant-risk/radiant-harness/v3/internal/skill"
)

// TestGenerateViewsForAgentKnownAgents checks that every adapter
// produces at least the instructions file + 1 skill view. We
// don't assert exact counts because the bundled skill set grows
// over time — but each agent must produce SOMETHING useful.
func TestGenerateViewsForAgentKnownAgents(t *testing.T) {
	for _, agent := range []radiant.AgentID{
		radiant.AgentClaude,
		radiant.AgentCodex,
		radiant.AgentCursor,
		radiant.AgentCopilot,
		radiant.AgentGemini,
		radiant.AgentWindsurf,
	} {
		views := GenerateViewsForAgent(agent)
		if len(views) < 2 {
			t.Errorf("%s: expected >= 2 views (instructions + skills), got %d", agent, len(views))
		}
		// First view is the instructions file (InstTo path).
		adapter := GetAdapter(agent)
		if views[0].Path != adapter.InstTo {
			t.Errorf("%s: first view path = %q, want %q", agent, views[0].Path, adapter.InstTo)
		}
	}
}

// TestGenerateViewsForAgentUnknown returns empty (not an error).
// Caller decides what to do with an unknown agent.
func TestGenerateViewsForAgentUnknown(t *testing.T) {
	views := GenerateViewsForAgent("nonexistent")
	if len(views) != 0 {
		t.Errorf("unknown agent should return 0 views, got %d", len(views))
	}
}

// TestGenerateViewsForAgentSkillLayouts checks that the per-agent
// layout choice is reflected in the generated paths.
func TestGenerateViewsForAgentSkillLayouts(t *testing.T) {
	cases := []struct {
		agent     radiant.AgentID
		skillsDir string // expected to appear in skill view paths
		layout    string // "skill-dir" (Claude/Codex) or "flat" (others)
	}{
		{radiant.AgentClaude, ".claude/skills", "skill-dir"},
		{radiant.AgentCodex, ".agents/skills", "skill-dir"},
		{radiant.AgentCursor, ".cursor/commands", "flat"},
		{radiant.AgentCopilot, ".github/prompts", "flat"},
		{radiant.AgentGemini, ".gemini/commands", "flat"},
		{radiant.AgentWindsurf, ".windsurf/workflows", "flat"},
	}
	for _, c := range cases {
		views := GenerateViewsForAgent(c.agent)
		// Find a skill view (any view whose path contains the agent's skills dir).
		var skillView *View
		for i := range views {
			if strings.Contains(views[i].Path, c.skillsDir) {
				skillView = &views[i]
				break
			}
		}
		if skillView == nil {
			t.Errorf("%s: no skill view found containing %q", c.agent, c.skillsDir)
			continue
		}
		// Layout check: skill-dir ends with SKILL.md; flat ends with <skill>.<ext>
		if c.layout == "skill-dir" {
			if !strings.HasSuffix(skillView.Path, "/SKILL.md") {
				t.Errorf("%s (skill-dir): expected path ending in SKILL.md, got %q", c.agent, skillView.Path)
			}
		} else {
			// Flat layout: path should end with .md or .prompt.md or .toml
			if !strings.Contains(skillView.Content, "# Skill:") &&
				!strings.HasSuffix(skillView.Path, ".md") &&
				!strings.HasSuffix(skillView.Path, ".prompt.md") &&
				!strings.HasSuffix(skillView.Path, ".toml") {
				t.Errorf("%s (flat): unexpected skill view path/content: %s", c.agent, skillView.Path)
			}
		}
	}
}

// TestGenerateViewsForAgentStripsFrontmatter checks that agents
// configured with InstFM="strip" (Codex, Copilot, Gemini, Windsurf)
// do NOT include the YAML frontmatter in their generated
// instructions file. Frontmatter is a Claude/Claude-Code convention.
func TestGenerateViewsForAgentStripsFrontmatter(t *testing.T) {
	stripAgents := []radiant.AgentID{
		radiant.AgentCodex,
		radiant.AgentCopilot,
		radiant.AgentGemini,
		radiant.AgentWindsurf,
	}
	for _, agent := range stripAgents {
		views := GenerateViewsForAgent(agent)
		if len(views) == 0 {
			t.Errorf("%s: no views generated", agent)
			continue
		}
		body := views[0].Content
		if strings.HasPrefix(strings.TrimSpace(body), "---") {
			t.Errorf("%s: instructions should not start with frontmatter delimiter (strip mode), got:\n%.200s", agent, body)
		}
	}
}

// TestGenerateViewsForAgentKeepsFrontmatter checks that agents
// configured with InstFM="keep" (Claude, Cursor) DO include the
// YAML frontmatter in their generated instructions file. Cursor's
// .mdc format requires it.
func TestGenerateViewsForAgentKeepsFrontmatter(t *testing.T) {
	keepAgents := []radiant.AgentID{
		radiant.AgentClaude,
		radiant.AgentCursor,
	}
	for _, agent := range keepAgents {
		views := GenerateViewsForAgent(agent)
		if len(views) == 0 {
			t.Errorf("%s: no views generated", agent)
			continue
		}
		body := views[0].Content
		if !strings.HasPrefix(strings.TrimSpace(body), "---") {
			t.Errorf("%s: instructions should start with frontmatter (keep mode), got:\n%.200s", agent, body)
		}
	}
}

// TestGenerateAgentsMDShape checks the canonical AGENTS.md format:
// ≤100 lines, includes every bundled skill by name, has the
// universal-index header. This is the regression guard for
// Sprint 14.3 (unify AGENTS.md templates) — both `Init` and
// `radiant update` must produce content matching this contract.
func TestGenerateAgentsMDShape(t *testing.T) {
	body := GenerateAgentsMD()
	lines := strings.Split(body, "\n")
	if len(lines) > 100 {
		t.Errorf("AGENTS.md should stay <=100 lines (video research #6); got %d", len(lines))
	}
	if !strings.HasPrefix(body, "# AGENTS.md") {
		t.Errorf("AGENTS.md should start with '# AGENTS.md'; got first line: %q", strings.SplitN(body, "\n", 2)[0])
	}
	infos, err := skill.Bundle()
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if !strings.Contains(body, info.Name) {
			t.Errorf("AGENTS.md should reference skill %q", info.Name)
		}
	}
}

// TestGenerateAgentsMDStable calls GenerateAgentsMD twice and
// ensures the output is byte-identical (no flakiness from map
// iteration order, time-dependent content, etc.).
func TestGenerateAgentsMDStable(t *testing.T) {
	a := GenerateAgentsMD()
	b := GenerateAgentsMD()
	if a != b {
		t.Errorf("GenerateAgentsMD is not stable across calls:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}
