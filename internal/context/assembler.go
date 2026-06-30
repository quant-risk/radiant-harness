package context

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/skill"
	"gopkg.in/yaml.v3"
)

// AssembleOptions controls how CONTEXT.md is generated.
type AssembleOptions struct {
	// BudgetTokens is the soft token ceiling for the assembled context.
	// Zero means no limit. When set, skills are dropped (lowest priority
	// first) until the estimate fits within the budget.
	BudgetTokens int

	// IncludeActiveSpec adds the active spec's tasks.md (if found) to
	// the context so the agent knows what it's working on.
	IncludeActiveSpec bool

	// ExtraSkills are additional skill names to include beyond the
	// recommended set (e.g. user-requested via --skills flag).
	ExtraSkills []string
}

// Assemble builds a minimal CONTEXT.md for projectDir and writes it to
// .radiant-harness/CONTEXT.md. It loads only frontmatter metadata —
// never SKILL.md bodies — keeping the assembled context lean.
//
// Returns the path written and estimated token count.
func Assemble(projectDir string, result *DetectionResult, opts AssembleOptions) (path string, tokens int, err error) {
	skillNames := mergeSkillNames(result.RecommendedSkills, opts.ExtraSkills)

	// Load frontmatter for each recommended skill
	type entry struct {
		name string
		info skill.SkillInfo
		fm   frontmatterSnippet
	}

	// Try loading from local .radiant-harness/skills/ first, then bundled FS.
	localSkillsDir := filepath.Join(projectDir, ".radiant-harness", "skills")
	bundledFS := skill.BundledFS()

	var entries []entry
	for _, name := range skillNames {
		fm, loadErr := loadFrontmatter(localSkillsDir, name, bundledFS)
		if loadErr != nil {
			// Skill not found — skip silently (bundled skills may be a subset)
			continue
		}
		entries = append(entries, entry{name: name, fm: fm})
	}

	// Build the markdown document
	var sb strings.Builder
	sb.WriteString("# Radiant Context\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s  \n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))
	sb.WriteString(fmt.Sprintf("Domain: **%s** | Tier: **%s** | Project: **%s**\n\n", result.Domain, result.Tier, result.ProjectName))

	if result.ActiveSpec != "" {
		sb.WriteString(fmt.Sprintf("Active spec: `%s`\n\n", result.ActiveSpec))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("## Available Skills\n\n")
	sb.WriteString("_Full skill instructions are loaded on-demand. Run `radiant skills show <name>` for complete guidance._\n\n")

	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("### %s\n\n", e.name))
		if e.fm.Description != "" {
			// Truncate description to first sentence / 120 chars
			desc := firstSentence(e.fm.Description, 120)
			sb.WriteString(fmt.Sprintf("%s\n\n", desc))
		}
		if e.fm.WhenToUse != "" {
			sb.WriteString(fmt.Sprintf("**When**: %s\n\n", firstSentence(e.fm.WhenToUse, 100)))
		}
		if len(e.fm.CommandsAvailable) > 0 {
			sb.WriteString(fmt.Sprintf("**Command**: `%s`\n\n", e.fm.CommandsAvailable[0]))
		}
	}

	// Active spec tasks (if requested and available)
	if opts.IncludeActiveSpec && result.ActiveSpec != "" {
		tasksPath := filepath.Join(projectDir, result.ActiveSpec, "tasks.md")
		if data, readErr := os.ReadFile(tasksPath); readErr == nil {
			sb.WriteString("---\n\n")
			sb.WriteString("## Active Spec Tasks\n\n")
			sb.WriteString(string(data))
			sb.WriteString("\n")
		}
	}

	// Boot instructions — always last
	sb.WriteString("---\n\n")
	sb.WriteString("## Loop Instructions\n\n")
	sb.WriteString("1. `radiant boot` — print this manifest at any time\n")
	sb.WriteString("2. `radiant context assemble` — refresh this file\n")
	sb.WriteString("3. `radiant loop start \"<goal>\"` — start autonomous loop\n")
	sb.WriteString("4. `radiant loop status` — check current loop state\n")

	content := sb.String()

	// Apply budget trim if needed
	if opts.BudgetTokens > 0 {
		content = trimToTokenBudget(content, opts.BudgetTokens)
	}

	// Write atomically
	outDir := filepath.Join(projectDir, ".radiant-harness")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("mkdir .radiant-harness: %w", err)
	}
	outPath := filepath.Join(outDir, "CONTEXT.md")
	if err := atomicWrite(outPath, []byte(content)); err != nil {
		return "", 0, err
	}

	est := estimateTokens(content)
	return outPath, est, nil
}

// frontmatterSnippet is a minimal parse of frontmatter.yaml — only the
// fields the assembler needs. This avoids pulling in the full skill.Skill
// type (which requires SKILL.md to be read too).
type frontmatterSnippet struct {
	Description       string   `yaml:"description"`
	WhenToUse         string   `yaml:"when_to_use"`
	CommandsAvailable []string `yaml:"commands_available"`
	TierEligible      []string `yaml:"tier_eligible"`
}

// loadFrontmatter reads frontmatter.yaml from local skills dir first,
// then falls back to the bundled FS. Returns error if not found anywhere.
func loadFrontmatter(localDir, name string, bundled fs.FS) (frontmatterSnippet, error) {
	var fm frontmatterSnippet

	// Try local first
	localPath := filepath.Join(localDir, name, "frontmatter.yaml")
	if data, err := os.ReadFile(localPath); err == nil {
		if yamlErr := yaml.Unmarshal(data, &fm); yamlErr == nil {
			return fm, nil
		}
	}

	// Fall back to bundled FS
	bundledPath := "skills/" + name + "/frontmatter.yaml"
	if data, err := fs.ReadFile(bundled, bundledPath); err == nil {
		if yamlErr := yaml.Unmarshal(data, &fm); yamlErr == nil {
			return fm, nil
		}
	}

	return fm, fmt.Errorf("skill %q not found in local or bundled FS", name)
}

// mergeSkillNames combines recommended and extra skills, deduplicating.
func mergeSkillNames(recommended, extra []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, s := range append(recommended, extra...) {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// firstSentence returns up to maxChars of s, truncated at the first
// period/newline boundary if shorter, else hard-truncated with "…".
func firstSentence(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	// Collapse newlines to spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")

	if len(s) <= maxChars {
		return s
	}
	// Try to cut at a sentence boundary
	for _, sep := range []string{". ", "! ", "? "} {
		if i := strings.Index(s[:maxChars], sep); i >= 0 {
			return s[:i+1]
		}
	}
	return s[:maxChars] + "…"
}

// atomicWrite writes data to path via a temp file + rename so the
// file is never partially written.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ctx-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	return os.Rename(tmpName, path)
}
