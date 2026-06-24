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

	radiant "github.com/quant-risk/radiant-harness/internal"
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
func Init(cfg Config) Result {
	templates, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return Result{Errors: []string{"templates not found: " + err.Error()}}
	}

	os.MkdirAll(cfg.TargetDir, 0o755)

	var written, skipped int
	var errors []string

	wantsClaude := containsAgent(cfg.Agents, radiant.AgentClaude)

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

	// Generate agent-specific views for ALL selected agents
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

	// Write manifest
	agentStrs := make([]string, len(cfg.Agents))
	for i, a := range cfg.Agents {
		agentStrs[i] = string(a)
	}
	manifest := radiant.Manifest{Version: cfg.Version, Agents: agentStrs}
	manifestPath := filepath.Join(cfg.TargetDir, ".radiant-harness", "manifest.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0o755)
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, append(data, '\n'), 0o644)

	return Result{Written: written, Skipped: skipped, Errors: errors}
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
