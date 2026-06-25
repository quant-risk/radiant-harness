// Package scaffold handles SDD pipeline scaffolding (init/update).
package scaffold

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/skill"
)

//go:embed all:templates
var templatesFS embed.FS

// Config holds scaffold configuration.
type Config struct {
	TargetDir string
	Agents    []radiant.AgentID
	Force     bool
	Version   string
}

// Result is the output of a scaffold operation.
type Result struct {
	Written int
	Skipped int
	Errors  []string
}

// Init scaffolds the SDD pipeline into the target directory.
//
// Order of operations (each non-fatal — we record errors and continue):
//  1. Walk legacy templates (CONVENTIONS.md, docs/, specs/, src/, hooks/)
//  2. Extract bundled skills from `internal/skill` to
//     `.radiant-harness/skills/<name>/{SKILL.md, frontmatter.yaml}`
//  3. Generate `.radiant-harness/AGENTS.md` — universal index, ≤100
//     lines (per video research #6 — LLM-generated AGENTS.md can
//     hurt results; we keep it minimal and link to docs)
//  4. Generate `.radiant-harness/state.md` — initial resume point
//  5. Generate agent-specific native views (Claude / Cursor /
//     Copilot / Windsurf) ONLY when --agent=<list> includes them
//  6. Write `.radiant-harness/manifest.json` — version + agents
func Init(cfg Config) Result {
	templates, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return Result{Errors: []string{"templates not found: " + err.Error()}}
	}

	os.MkdirAll(cfg.TargetDir, 0o755)

	var written, skipped int
	var errors []string

	wantsClaude := containsAgent(cfg.Agents, radiant.AgentClaude)

	// 1. Legacy templates (excluding .claude/ unless requested)
	fs.WalkDir(templates, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}

		if !wantsClaude && isClaudeLayout(path) {
			return nil
		}

		dest := filepath.Join(cfg.TargetDir, path)
		os.MkdirAll(filepath.Dir(dest), 0o755)

		if _, statErr := os.Stat(dest); statErr == nil && !cfg.Force {
			skipped++
			return nil
		}

		data, readErr := fs.ReadFile(templates, path)
		if readErr != nil {
			errors = append(errors, path+": "+readErr.Error())
			return nil
		}

		if writeErr := os.WriteFile(dest, data, 0o644); writeErr != nil {
			errors = append(errors, dest+": "+writeErr.Error())
			return nil
		}
		written++
		return nil
	})

	// 2. Extract bundled skills (the universal format)
	skillsDest := filepath.Join(cfg.TargetDir, ".radiant-harness", "skills")
	if err := skill.ExtractTo(skillsDest, cfg.Force); err != nil {
		errors = append(errors, "extract skills: "+err.Error())
	} else {
		// Count files written for the report.
		infos, _ := skill.Bundle()
		written += len(infos) * 2 // SKILL.md + frontmatter.yaml per skill
	}

	// 3. Generate AGENTS.md (universal index, minimal per video #6)
	agentsMDPath := filepath.Join(cfg.TargetDir, "AGENTS.md")
	agentsMD := generateAgentsMD()
	if !cfg.Force {
		if _, statErr := os.Stat(agentsMDPath); statErr == nil {
			skipped++
		} else if err := os.WriteFile(agentsMDPath, []byte(agentsMD), 0o644); err != nil {
			errors = append(errors, "AGENTS.md: "+err.Error())
		} else {
			written++
		}
	} else if err := os.WriteFile(agentsMDPath, []byte(agentsMD), 0o644); err != nil {
		errors = append(errors, "AGENTS.md: "+err.Error())
	} else {
		written++
	}

	// 4. Generate initial state.md (volatile memory for handoff)
	stateMDPath := filepath.Join(cfg.TargetDir, ".radiant-harness", "state.md")
	stateMD := generateInitialState()
	if err := os.WriteFile(stateMDPath, []byte(stateMD), 0o644); err != nil {
		errors = append(errors, "state.md: "+err.Error())
	} else {
		written++
	}

	// 5. Native views — opt-in via --agent=<list>
	for _, agent := range cfg.Agents {
		adapter := GetAdapter(agent)
		if adapter == nil {
			continue
		}
		extras := generateViews(adapter, templates)
		for _, e := range extras {
			dest := filepath.Join(cfg.TargetDir, e.Path)
			os.MkdirAll(filepath.Dir(dest), 0o755)

			if _, statErr := os.Stat(dest); statErr == nil && !cfg.Force {
				skipped++
				continue
			}

			if writeErr := os.WriteFile(dest, []byte(e.Content), 0o644); writeErr != nil {
				errors = append(errors, dest+": "+writeErr.Error())
				continue
			}
			written++
		}
	}

	// 6. Write manifest (version + selected agents)
	agentStrs := make([]string, len(cfg.Agents))
	for i, a := range cfg.Agents {
		agentStrs[i] = string(a)
	}
	manifest := radiant.Manifest{Version: cfg.Version, Agents: agentStrs}
	manifestPath := filepath.Join(cfg.TargetDir, ".radiant-harness", "manifest.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0o755)
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, append(data, '\n'), 0o644)
	written++

	return Result{Written: written, Skipped: skipped, Errors: errors}
}

// generateAgentsMD produces the universal AGENTS.md. ≤100 lines,
// links to docs, lists skills + commands. Video research #6: keep
// this minimal — LLM-generated AGENTS.md can hurt task success.
func generateAgentsMD() string {
	infos, _ := skill.Bundle()
	var b strings.Builder

	b.WriteString("# AGENTS.md\n\n")
	b.WriteString("> **Universal project index.** Read this first. If you are\n")
	b.WriteString("> any LLM agent working on this project, follow the instructions\n")
	b.WriteString("> below. For the open spec and methodology, see:\n")
	b.WriteString("> https://github.com/quant-risk/radiant-harness (and `docs/SKILL-SCHEMA.md`)\n\n")
	b.WriteString("## How this project uses radiant-harness\n\n")
	b.WriteString("This project ships with vendor-neutral workflow skills at\n")
	b.WriteString("`.radiant-harness/skills/`. Read `SKILL.md` in each skill\n")
	b.WriteString("directory for instructions, and `frontmatter.yaml` for the\n")
	b.WriteString("machine-readable contract (inputs, outputs, gates).\n\n")
	b.WriteString("**Important:** Keep `AGENTS.md` minimal. Do not paste full\n")
	b.WriteString("documentation here — link to it instead. LLM-generated\n")
	b.WriteString("AGENTS.md can hurt task success rates (per video research).\n")
	b.WriteString("Always review and edit this file after `radiant init`.\n\n")

	b.WriteString("## Available skills\n\n")
	b.WriteString("| Skill | When to use | CLI command |\n")
	b.WriteString("|-------|-------------|-------------|\n")
	if len(infos) == 0 {
		b.WriteString("| (no skills bundled) | — | — |\n\n")
	} else {
		for _, info := range infos {
			cmd := ""
			for _, c := range info.CommandsAvailable {
				cmd = c
				break
			}
			desc := strings.SplitN(info.Description, "\n", 2)[0]
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Fprintf(&b, "| `%s` | %s | `%s` |\n", info.Name, desc, cmd)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Quick reference\n\n")
	b.WriteString("- Start a feature: `radiant spec <intent>`\n")
	b.WriteString("- Resume a session: `radiant state` (read) / `radiant handoff` (write)\n")
	b.WriteString("- Run a feature: `radiant run specs/<NNNN>-<slug>/`\n")
	b.WriteString("- Validate: `radiant validate specs/<NNNN>-<slug>/`\n")
	b.WriteString("- List skills: `radiant skills list`\n")
	b.WriteString("- Validate a skill: `radiant skills validate <dir>`\n\n")

	b.WriteString("## Where to find more\n\n")
	b.WriteString("- Spec templates: `specs/_templates/` (per tier)\n")
	b.WriteString("- Project conventions: `CLAUDE.md` (if generated) or `docs/CONVENTIONS.md`\n")
	b.WriteString("- Glossary: `docs/glossary.md`\n")
	b.WriteString("- Architecture: `docs/architecture/context-map.md` (when present)\n")
	b.WriteString("- Resume point: `.radiant-harness/state.md` (volatile, updated by handoff)\n\n")

	b.WriteString("## For the agent\n\n")
	b.WriteString("- **Vendor-neutral.** Skills are plain Markdown + YAML. No\n")
	b.WriteString("  Claude/Cursor/etc. namespaces. Any modern LLM can consume them.\n")
	b.WriteString("- **Tier-aware.** Trivial/Feature/Architecture determines which\n")
	b.WriteString("  artifacts are required. Don't add ceremony beyond the tier.\n")
	b.WriteString("- **Tests are non-negotiable.** Every AC must map to a task\n")
	b.WriteString("  with a gate command. The harness runs the gates; you write them.\n")
	b.WriteString("- **Plan is self-contained.** After spec.md + tasks.md exist,\n")
	b.WriteString("  close the planning context and open a fresh one for implementation.\n")

	return b.String()
}

// generateInitialState produces the volatile session memory. The
// handoff skill updates this; new sessions read it.
func generateInitialState() string {
	return fmt.Sprintf(`# State

## Current position
- current_feature: null
- tier: null
- next_command: radiant spec <intent>
- blockers: []
- open_questions: []

## Last session
- last_updated: %s
- last_summary: "Project initialized. Run 'radiant spec <intent>' to start a feature."

## Notes
This file is read by 'radiant handoff' (resume) and written by it (pause).
Don't commit it — it's volatile session memory, not project documentation.
`, time.Now().UTC().Format(time.RFC3339))
}

func containsAgent(agents []radiant.AgentID, target radiant.AgentID) bool {
	for _, a := range agents {
		if a == target {
			return true
		}
	}
	return false
}

func isClaudeLayout(path string) bool {
	p := filepath.ToSlash(path)
	return strings.HasPrefix(p, ".claude/")
}

// View represents a generated file for a non-Claude agent.
type View struct {
	Path    string
	Content string
}

func generateViews(adapter *radiant.AgentAdapter, templates fs.FS) []View {
	claudemd, err := fs.ReadFile(templates, "CONVENTIONS.md")
	if err != nil {
		return nil
	}

	var views []View
	body := string(claudemd)

	if adapter.InstFM == "strip" {
		body = stripFrontmatter(body)
	}

	body = strings.ReplaceAll(body, "skills", adapter.SkillsDir)
	views = append(views, View{Path: adapter.InstTo, Content: body})

	skillsDir := "skills"
	entries, _ := fs.ReadDir(templates, skillsDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := skillsDir + "/" + e.Name() + "/SKILL.md"
		data, readErr := fs.ReadFile(templates, skillPath)
		if readErr != nil {
			continue
		}

		if adapter.SkillsLayout == "skill-dir" {
			outPath := adapter.SkillsDir + "/" + e.Name() + "/SKILL.md"
			views = append(views, View{Path: outPath, Content: string(data)})
		} else {
			ext := adapter.SkillsExt
			if ext == "" {
				ext = "md"
			}
			outPath := adapter.SkillsDir + "/" + e.Name() + "." + ext
			content := string(data)
			if adapter.ID == radiant.AgentGemini {
				content = toToml(content)
			}
			views = append(views, View{Path: outPath, Content: content})
		}
	}
	return views
}

// GenerateViewsForAgent returns the list of native-view files
// that would be written for the given agent. Used by `radiant views`
// to let users regenerate a single agent's view on demand (e.g.
// after adding a new skill, or switching from Cursor to Codex).
// Exported so cmd/radiant can call it without duplicating the
// template walking logic.
//
// Reads from TWO sources:
//  1. CONVENTIONS.md (the canonical instructions file) — gives
//     us the agent's instructions body.
//  2. The bundled skills in internal/skill/ — gives us the SKILL.md
//     files to mirror into the agent's skills directory. This is
//     the canonical source (scaffold's templates/skills/ is empty
//     by design — it was a stub during early development).
func GenerateViewsForAgent(agent radiant.AgentID) []View {
	adapter := GetAdapter(agent)
	if adapter == nil {
		return nil
	}
	templates, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return nil
	}
	claudemd, err := fs.ReadFile(templates, "CONVENTIONS.md")
	if err != nil {
		return nil
	}

	var views []View
	body := string(claudemd)
	if adapter.InstFM == "strip" {
		body = stripFrontmatter(body)
	}
	body = strings.ReplaceAll(body, "skills", adapter.SkillsDir)
	views = append(views, View{Path: adapter.InstTo, Content: body})

	// Pull skill views from the canonical bundled skill set.
	infos, err := skill.Bundle()
	if err != nil {
		return views // at least the instructions file is there
	}
	for _, info := range infos {
		s, err := skill.LoadFromFS(skill.BundledFS(), "skills/"+info.Name)
		if err != nil {
			continue
		}
		// SKILL.md body is what we mirror. Read it directly.
		skmd, err := fs.ReadFile(skill.BundledFS(), "skills/"+info.Name+"/SKILL.md")
		if err != nil {
			continue
		}
		_ = s // not used; we just need the metadata check to succeed

		var outPath, content string
		if adapter.SkillsLayout == "skill-dir" {
			outPath = adapter.SkillsDir + "/" + info.Name + "/SKILL.md"
			content = string(skmd)
		} else {
			ext := adapter.SkillsExt
			if ext == "" {
				ext = "md"
			}
			outPath = adapter.SkillsDir + "/" + info.Name + "." + ext
			content = string(skmd)
			if adapter.ID == radiant.AgentGemini {
				content = toToml(content)
			}
		}
		views = append(views, View{Path: outPath, Content: content})
	}
	return views
}

func stripFrontmatter(text string) string {
	if !strings.HasPrefix(text, "---") {
		return text
	}
	end := strings.Index(text[3:], "\n---")
	if end == -1 {
		return text
	}
	body := text[end+7:]
	return strings.TrimLeft(body, "\n")
}

func toToml(raw string) string {
	lines := strings.SplitN(raw, "---", 3)
	if len(lines) < 3 {
		return raw
	}

	fm := lines[1]
	body := strings.TrimLeft(lines[2], "\n")

	name := ""
	desc := ""
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
		if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	if desc == "" {
		desc = name
	}

	desc = strings.ReplaceAll(desc, "\\", "\\\\")
	desc = strings.ReplaceAll(desc, "\"", "\\\"")
	body = strings.ReplaceAll(body, "\\", "\\\\")
	body = strings.ReplaceAll(body, "\"\"\"", "\\\"\\\"\\\"")
	body = strings.ReplaceAll(body, "${", "\\${")

	return fmt.Sprintf("description = \"%s\"\nprompt = \"\"\"\n%s\n\"\"\"\n", desc, body)
}
