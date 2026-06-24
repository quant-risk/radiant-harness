// Package quality provides SDD pipeline validation scripts.
package quality

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

var ignoreDirs = map[string]bool{
	"node_modules": true, ".git": true, ".radiant-harness": true,
	".spec-driven": true, ".agents": true, ".cursor": true,
	".gemini": true, ".windsurf": true,
}

var noFrontmatterOK = map[string]bool{
	"RELEASING.md": true, "CHANGELOG.md": true,
}

// AuditPipeline validates SDD pipeline structure.
func AuditPipeline(root string) radiant.ScriptResult {
	var errors []string
	files := walkMarkdown(root)

	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		if isGenerated(rel) {
			continue
		}

		text, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(text)
		base := filepath.Base(f)

		if noFrontmatterOK[base] {
			continue
		}

		fm := parseFrontmatter(content)
		if fm == nil {
			errors = append(errors, rel+": missing frontmatter")
			continue
		}
		if fm["name"] == "" {
			errors = append(errors, rel+": frontmatter missing 'name'")
		}
		if fm["description"] == "" {
			errors = append(errors, rel+": frontmatter missing 'description'")
		}

		isSkill := strings.Contains(rel, ".claude/skills/") ||
			strings.HasSuffix(rel, "/skill.template.md") ||
			strings.HasSuffix(rel, "/subagent.template.md")

		if isSkill {
			if _, ok := fm["alwaysApply"]; ok {
				errors = append(errors, rel+": skill dialect must not have 'alwaysApply'")
			}
		} else {
			apply, ok := fm["alwaysApply"]
			if !ok {
				errors = append(errors, rel+": doc missing 'alwaysApply'")
			} else if apply != "true" && apply != "false" {
				errors = append(errors, rel+": invalid alwaysApply: "+apply)
			}
		}
	}

	// Check broken relative links
	linkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, f := range files {
		text, _ := os.ReadFile(f)
		content := string(text)
		rel, _ := filepath.Rel(root, f)

		for _, m := range linkRe.FindAllStringSubmatch(content, -1) {
			target := strings.TrimSpace(m[1])
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			// Skip placeholder patterns (NNNN, XXXX, <placeholder>)
			if strings.Contains(target, "NNNN") || strings.Contains(target, "XXXX") || strings.Contains(target, "<") {
				continue
			}
			target = strings.Split(target, "#")[0]
			if target == "" {
				continue
			}
			absTarget := filepath.Join(filepath.Dir(f), target)
			if _, err := os.Stat(absTarget); os.IsNotExist(err) {
				errors = append(errors, rel+": broken link -> "+target)
			}
		}
	}

	// Check specs without spec.md
	specsDir := filepath.Join(root, "specs")
	if entries, err := os.ReadDir(specsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && len(e.Name()) >= 4 {
				num := e.Name()[:4]
				if num >= "0000" && num <= "9999" {
					specFile := filepath.Join(specsDir, e.Name(), "spec.md")
					if _, err := os.Stat(specFile); os.IsNotExist(err) {
						errors = append(errors, "specs/"+e.Name()+": feature missing 'spec.md'")
					}
				}
			}
		}
	}

	return radiant.ScriptResult{OK: len(errors) == 0, Errors: errors}
}

func walkMarkdown(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if ignoreDirs[info.Name()] || strings.HasPrefix(info.Name(), ".tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func parseFrontmatter(text string) map[string]string {
	if !strings.HasPrefix(text, "---") {
		return nil
	}
	idx := strings.Index(text[3:], "\n---")
	if idx == -1 {
		return nil
	}
	block := text[3 : 3+idx]
	keys := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(block))
	fmRe := regexp.MustCompile(`^([A-Za-z_][\w-]*):\s*(.+)$`)
	for scanner.Scan() {
		if m := fmRe.FindStringSubmatch(scanner.Text()); m != nil {
			keys[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return keys
}

func isGenerated(rel string) bool {
	r := filepath.ToSlash(rel)
	return r == "AGENTS.md" || r == "GEMINI.md" ||
		r == ".github/copilot-instructions.md" || strings.HasPrefix(r, ".github/prompts/")
}
