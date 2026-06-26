package improve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Proposal is a suggested edit to a skill's instructions or frontmatter.
type Proposal struct {
	Skill       string
	File        string // "SKILL.md" or "frontmatter.yaml"
	Category    FailureCategory
	Description string
	Before      string // original content excerpt
	After       string // proposed content excerpt
	Confidence  float64
	RunIDs      []string
	CreatedAt   time.Time
}

// ProposeEdits generates skill instruction proposals from detected failure patterns.
// It reads the skill files from the bundled skill registry in projectDir.
func ProposeEdits(patterns []FailurePattern, projectDir string) []Proposal {
	var proposals []Proposal

	for _, p := range patterns {
		if p.Category == CategoryUnknown || p.Confidence < 0.40 {
			continue
		}

		skillName := p.Skill
		if skillName == "" {
			skillName = inferSkillFromPattern(p)
		}

		prop := buildProposal(p, skillName, projectDir)
		if prop != nil {
			proposals = append(proposals, *prop)
		}
	}
	return proposals
}

// FormatProposals renders proposals as a human-readable diff summary.
func FormatProposals(proposals []Proposal) string {
	if len(proposals) == 0 {
		return "No proposals generated. Either no patterns detected or confidence too low.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d proposal(s) ready:\n\n", len(proposals)))

	for i, p := range proposals {
		sb.WriteString(fmt.Sprintf("%d. Skill: %s (%s)\n", i+1, p.Skill, p.File))
		sb.WriteString(fmt.Sprintf("   Pattern: %s (confidence: %.0f%%)\n", p.Category, p.Confidence*100))
		sb.WriteString(fmt.Sprintf("   Change: %s\n", p.Description))
		if p.Before != "" {
			sb.WriteString(fmt.Sprintf("   Before: %q\n", truncate(p.Before, 80)))
		}
		if p.After != "" {
			sb.WriteString(fmt.Sprintf("   After:  %q\n", truncate(p.After, 80)))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ── internals ─────────────────────────────────────────────────────────────────

func buildProposal(p FailurePattern, skillName, projectDir string) *Proposal {
	prop := &Proposal{
		Skill:      skillName,
		File:       "SKILL.md",
		Category:   p.Category,
		Confidence: p.Confidence,
		RunIDs:     p.RunIDs,
		CreatedAt:  time.Now(),
	}

	switch p.Category {
	case CategoryPrematureExit:
		prop.Description = "Add explicit completion gate: require ALL acceptance criteria to be verified before exiting"
		prop.Before = "## Gates\n- Linter passes"
		prop.After = "## Gates\n- Linter passes\n- ALL acceptance criteria verified (not just first)\n- Evidence cited for each AC"

	case CategoryMissingVerification:
		prop.Description = "Strengthen verification requirement: mandate evidence citation for each check"
		prop.Before = "Verify the implementation is correct."
		prop.After = "Verify the implementation is correct. For EACH check:\n1. Run it explicitly\n2. Quote the output as evidence\n3. Do not assume success without proof"

	case CategoryWrongScope:
		prop.Description = "Add scope guard: require explicit goal re-read at start of execute phase"
		prop.Before = "## Execute"
		prop.After = "## Execute\n> Before writing any code, re-read the GOAL. Stay within its exact scope."

	case CategoryBudgetWaste:
		prop.Description = "Add token-check gate: verify budget > 15% before starting large operations"
		prop.Before = "## Execute"
		prop.After = "## Execute\n> Check `radiant loop status` before proceeding. If budget < 15%, summarize and exit."

	case CategoryRepeatFailure:
		prop.Description = "Add retry limit guard: after 2 failures on same action, escalate or skip"
		prop.Before = "Retry if the action fails."
		prop.After = "Retry if the action fails — but maximum 2 retries per action.\nAfter 2 failures, record the blocker and move on."

	default:
		return nil
	}

	// Try to read the actual skill file for a better before/after
	if projectDir != "" {
		skillPath := filepath.Join(projectDir, ".radiant-harness", "skills", skillName, "SKILL.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			if !strings.Contains(string(data), prop.Before) {
				// The skill file doesn't have the exact before-text; use generic description
				prop.Before = ""
			}
		}
	}

	return prop
}

func inferSkillFromPattern(p FailurePattern) string {
	// Use a generic target skill based on failure category
	switch p.Category {
	case CategoryMissingVerification, CategoryPrematureExit:
		return "validar"
	case CategoryWrongScope:
		return "nova-feature"
	default:
		return "general"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
