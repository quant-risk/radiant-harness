package skill

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// bundledFS holds the canonical skills shipped with the radiant
// CLI. Use Bundle() to enumerate them, ExtractTo() to write them
// into a project during `radiant init`.
//
//go:embed all:skills
var bundledFS embed.FS

// SkillInfo is a lightweight descriptor for a bundled skill — name
// + description (from frontmatter.yaml). Used by `radiant skills
// list` and by AGENTS.md generation so any agent can scan what's
// available without parsing every SKILL.md.
type SkillInfo struct {
	Name              string
	Version           string
	Description       string
	TierEligible      []string
	CommandsAvailable []string
}

// Bundle returns one SkillInfo per valid skill directory embedded
// in the CLI binary. Skills without a frontmatter.yaml (legacy
// placeholders pending rewrite) are silently skipped — they
// remain on disk in case anyone references them by name, but
// they're not promoted to the canonical bundle.
//
// Invalid skills (failing Validate()) cause Bundle to return an
// error — never ship a binary with broken skills.
func Bundle() ([]SkillInfo, error) {
	var out []SkillInfo
	entries, err := bundledFS.ReadDir("skills")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip skills without frontmatter.yaml — they haven't been
		// migrated to the new schema yet. They'll be added in
		// subsequent sprints.
		fmPath := "skills/" + e.Name() + "/frontmatter.yaml"
		if _, err := fs.Stat(bundledFS, fmPath); err != nil {
			continue
		}
		s, err := LoadFromFS(bundledFS, "skills/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("load embedded skill %q: %w", e.Name(), err)
		}
		if errs := s.Validate(); len(errs) > 0 {
			return nil, fmt.Errorf("embedded skill %q is invalid: %d rule violations (first: %s)",
				e.Name(), len(errs), errs[0].Error())
		}
		out = append(out, SkillInfo{
			Name:              s.Name,
			Version:           s.Version,
			Description:       s.Description,
			TierEligible:      s.TierEligible,
			CommandsAvailable: s.CommandsAvailable,
		})
	}
	return out, nil
}

// LoadFromFS loads a skill from an arbitrary fs.FS — used by Bundle
// to read from the embedded FS, and by tests to read from a temp dir.
func LoadFromFS(fsys fs.FS, dir string) (*Skill, error) {
	fmBytes, err := fs.ReadFile(fsys, dir+"/frontmatter.yaml")
	if err != nil {
		return nil, fmt.Errorf("read frontmatter.yaml: %w", err)
	}
	var s Skill
	if err := yaml.Unmarshal(fmBytes, &s); err != nil {
		return nil, fmt.Errorf("parse frontmatter.yaml: %w", err)
	}
	// SKILL.md presence check (non-empty)
	skmdBytes, err := fs.ReadFile(fsys, dir+"/SKILL.md")
	if err != nil {
		return nil, fmt.Errorf("SKILL.md missing: %w", err)
	}
	if len(skmdBytes) == 0 {
		return nil, fmt.Errorf("SKILL.md is empty")
	}
	// Set SKMDPath to the embedded path so Validate can re-read it.
	// Note: Dir is left empty; Validate only checks Dir for rule 9.
	s.SKMDPath = dir + "/SKILL.md"
	return &s, nil
}

// ExtractTo writes every bundled skill into the target directory
// (e.g. `.radiant-harness/skills/`). Existing files are overwritten
// unless `force` is false — in which case they are skipped silently
// (the user's local edits win; they can use `radiant update` to
// review + accept changes explicitly).
func ExtractTo(targetDir string, force bool) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir target: %w", err)
	}
	entries, err := bundledFS.ReadDir("skills")
	if err != nil {
		return fmt.Errorf("read embedded skills: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip skills without frontmatter — they're placeholders.
		fmPath := "skills/" + e.Name() + "/frontmatter.yaml"
		if _, err := fs.Stat(bundledFS, fmPath); err != nil {
			continue
		}
		dest := filepath.Join(targetDir, e.Name())
		if !force {
			if _, err := os.Stat(dest); err == nil {
				// Already exists — skip
				continue
			}
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dest, err)
		}
		// Walk the embedded skill dir and copy every file.
		err := fs.WalkDir(bundledFS, "skills/"+e.Name(), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(bundledFS, path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel("skills/"+e.Name(), path)
			if err != nil {
				return err
			}
			destPath := filepath.Join(dest, rel)
			if err := os.WriteFile(destPath, data, 0o644); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("extract %s: %w", e.Name(), err)
		}
	}
	return nil
}
