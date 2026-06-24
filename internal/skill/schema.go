// Package skill implements the radiant-harness skill format — a
// vendor-neutral, agent-agnostic skill definition. See
// docs/SKILL-SCHEMA.md for the open specification this package
// implements.
//
// A skill is a directory:
//
//	<skill-name>/
//	├── SKILL.md            # required: human-readable instructions
//	├── frontmatter.yaml    # required: machine-readable contract
//	├── examples/           # optional: worked examples
//	└── scripts/            # optional: helper scripts
//
// This package provides:
//   - Skill: parsed representation of a skill
//   - Load: parse a skill from disk
//   - Validate: enforce the schema rules in docs/SKILL-SCHEMA.md §6
//   - Bundle: read embedded skills (via go:embed)
package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// nameRe matches the canonical skill name pattern: kebab-case,
// 1-32 chars, starting with a lowercase letter.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// semverRe matches MAJOR.MINOR.PATCH.
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// validTier is the closed set of tier identifiers used across the
// radiant-harness skill system.
var validTier = map[string]bool{
	"trivial":      true,
	"feature":      true,
	"architecture": true,
}

// validOutputType enumerates the artifact categories the harness
// understands. Skills produce one of these per output.
var validOutputType = map[string]bool{
	"artifact": true, // file written under specs/, docs/, or project root
	"report":   true, // diagnostic output (no file written)
	"commit":   true, // git commit produced
	"pr":       true, // pull request opened
	"decision": true, // ADR-style decision record
}

// validInputType enumerates the primitive types a skill input can be.
var validInputType = map[string]bool{
	"string": true,
	"number": true,
	"enum":   true,
	"object": true,
	"path":   true,
}

// Skill is the parsed representation of a single skill directory.
//
// Field semantics map directly to the YAML schema documented in
// docs/SKILL-SCHEMA.md §4. See that file for the canonical contract.
type Skill struct {
	// Name is the kebab-case identifier. Must match the parent
	// directory name (validation rule #9).
	Name string `yaml:"name"`

	// Version is semver. MAJOR bumps indicate contract changes;
	// old parsers may break.
	Version string `yaml:"version"`

	// Description is a one-paragraph summary used by agents to
	// decide applicability within 5 seconds.
	Description string `yaml:"description"`

	// WhenToUse is the 1-3 sentence decision criterion for
	// invocation. Plain text, no Markdown.
	WhenToUse string `yaml:"when_to_use"`

	// TierEligible is the subset of {trivial, feature, architecture}
	// that this skill can produce. Non-empty subset (rule #3).
	TierEligible []string `yaml:"tier_eligible"`

	// Inputs is the list of input fields the skill needs. Names
	// must be unique within the skill (rule #4).
	Inputs []Input `yaml:"inputs"`

	// Outputs is the list of artifacts the skill produces. Paths
	// must be unique within the skill (rule #5).
	Outputs []Output `yaml:"outputs"`

	// Gates is the list of validation steps before outputs are
	// accepted. Names must be unique (rule #6).
	Gates []Gate `yaml:"gates"`

	// ContextProvides lists files the skill expects to read before
	// executing (e.g. glossary.md, state.md). Optional.
	ContextProvides []string `yaml:"context_provides"`

	// CommandsAvailable lists equivalent CLI commands for non-agent
	// users. Optional.
	CommandsAvailable []string `yaml:"commands_available"`

	// RelatedSkills lists other skills this one references. Optional.
	RelatedSkills []string `yaml:"related_skills"`

	// AntiPatterns lists mistakes this skill helps avoid. Optional.
	AntiPatterns []string `yaml:"anti_patterns"`

	// Author is the skill author (free-form). Optional.
	Author string `yaml:"author"`

	// License is the SPDX license identifier. Optional.
	License string `yaml:"license"`

	// Dir is the absolute path of the skill directory on disk.
	// Set by Load, not from YAML.
	Dir string `yaml:"-"`

	// SKMDPath is the absolute path of SKILL.md.
	SKMDPath string `yaml:"-"`
}

// Input is a single input field declared in the skill's contract.
type Input struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// Output is a single artifact the skill produces.
type Output struct {
	Path        string `yaml:"path"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// Gate is a single validation step in the skill's contract.
type Gate struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	OnFailure   string `yaml:"on_failure"`
}

// Load reads a skill from the given directory. It does NOT validate
// the skill against the schema — call Validate separately so callers
// can decide whether to treat warnings as errors.
//
// Returns the parsed Skill plus any error encountered reading files
// or parsing YAML.
func Load(dir string) (*Skill, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	fmPath := filepath.Join(abs, "frontmatter.yaml")
	fmBytes, err := os.ReadFile(fmPath)
	if err != nil {
		return nil, fmt.Errorf("read frontmatter.yaml: %w", err)
	}

	var s Skill
	if err := yaml.Unmarshal(fmBytes, &s); err != nil {
		return nil, fmt.Errorf("parse frontmatter.yaml: %w", err)
	}

	// SKILL.md must exist and be non-empty (rule #8).
	skmdPath := filepath.Join(abs, "SKILL.md")
	if info, err := os.Stat(skmdPath); err != nil {
		return nil, fmt.Errorf("SKILL.md missing: %w", err)
	} else if info.Size() == 0 {
		return nil, errors.New("SKILL.md is empty")
	}

	s.Dir = abs
	s.SKMDPath = skmdPath
	return &s, nil
}

// ValidationError describes a single rule violation in a Skill.
// Rule numbers correspond to docs/SKILL-SCHEMA.md §6.
type ValidationError struct {
	Rule  string // rule number, e.g. "1", "3", "9"
	Field string // path-like location, e.g. "name", "inputs[0].type"
	Msg   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("rule %s, field %s: %s", e.Rule, e.Field, e.Msg)
}

// Validate enforces the 10 rules from docs/SKILL-SCHEMA.md §6.
// Returns nil if the skill is valid; otherwise returns a slice of
// ValidationError — every violation is reported, not just the first.
func (s *Skill) Validate() []ValidationError {
	var errs []ValidationError

	// Rule 1: name pattern
	if !nameRe.MatchString(s.Name) {
		errs = append(errs, ValidationError{
			Rule:  "1",
			Field: "name",
			Msg:   fmt.Sprintf("%q does not match pattern ^[a-z][a-z0-9-]{0,31}$ (kebab-case, 1-32 chars)", s.Name),
		})
	}

	// Rule 2: semver
	if !semverRe.MatchString(s.Version) {
		errs = append(errs, ValidationError{
			Rule:  "2",
			Field: "version",
			Msg:   fmt.Sprintf("%q does not match semver MAJOR.MINOR.PATCH", s.Version),
		})
	}

	// Rule 3: tier_eligible is non-empty subset of valid tiers
	if len(s.TierEligible) == 0 {
		errs = append(errs, ValidationError{
			Rule:  "3",
			Field: "tier_eligible",
			Msg:   "must be a non-empty list",
		})
	}
	seenTier := map[string]bool{}
	for _, t := range s.TierEligible {
		if !validTier[t] {
			errs = append(errs, ValidationError{
				Rule:  "3",
				Field: "tier_eligible",
				Msg:   fmt.Sprintf("unknown tier %q (valid: trivial, feature, architecture)", t),
			})
		}
		if seenTier[t] {
			errs = append(errs, ValidationError{
				Rule:  "3",
				Field: "tier_eligible",
				Msg:   fmt.Sprintf("duplicate tier %q", t),
			})
		}
		seenTier[t] = true
	}

	// Rule 4: unique input names
	seenInput := map[string]bool{}
	for i, in := range s.Inputs {
		if in.Name == "" {
			errs = append(errs, ValidationError{
				Rule:  "4",
				Field: fmt.Sprintf("inputs[%d].name", i),
				Msg:   "name is required",
			})
			continue
		}
		if seenInput[in.Name] {
			errs = append(errs, ValidationError{
				Rule:  "4",
				Field: fmt.Sprintf("inputs[%d].name", i),
				Msg:   fmt.Sprintf("duplicate input name %q", in.Name),
			})
		}
		seenInput[in.Name] = true
		if !validInputType[in.Type] {
			errs = append(errs, ValidationError{
				Rule:  "4",
				Field: fmt.Sprintf("inputs[%d].type", i),
				Msg:   fmt.Sprintf("unknown type %q (valid: string, number, enum, object, path)", in.Type),
			})
		}
	}

	// Rule 5: unique output paths
	seenPath := map[string]bool{}
	for i, out := range s.Outputs {
		if out.Path == "" && out.Type != "report" {
			errs = append(errs, ValidationError{
				Rule:  "5",
				Field: fmt.Sprintf("outputs[%d].path", i),
				Msg:   "path is required (use \"-\" for ephemeral reports)",
			})
		}
		if seenPath[out.Path] {
			errs = append(errs, ValidationError{
				Rule:  "5",
				Field: fmt.Sprintf("outputs[%d].path", i),
				Msg:   fmt.Sprintf("duplicate output path %q", out.Path),
			})
		}
		seenPath[out.Path] = true
		if !validOutputType[out.Type] {
			errs = append(errs, ValidationError{
				Rule:  "5",
				Field: fmt.Sprintf("outputs[%d].type", i),
				Msg:   fmt.Sprintf("unknown type %q (valid: artifact, report, commit, pr, decision)", out.Type),
			})
		}
	}

	// Rule 6: unique gate names
	seenGate := map[string]bool{}
	for i, g := range s.Gates {
		if g.Name == "" {
			errs = append(errs, ValidationError{
				Rule:  "6",
				Field: fmt.Sprintf("gates[%d].name", i),
				Msg:   "name is required",
			})
			continue
		}
		if seenGate[g.Name] {
			errs = append(errs, ValidationError{
				Rule:  "6",
				Field: fmt.Sprintf("gates[%d].name", i),
				Msg:   fmt.Sprintf("duplicate gate name %q", g.Name),
			})
		}
		seenGate[g.Name] = true
	}

	// Rule 7: required fields present
	if strings.TrimSpace(s.Description) == "" {
		errs = append(errs, ValidationError{Rule: "7", Field: "description", Msg: "required"})
	}
	if strings.TrimSpace(s.WhenToUse) == "" {
		errs = append(errs, ValidationError{Rule: "7", Field: "when_to_use", Msg: "required"})
	}

	// Rule 8: SKILL.md exists and is non-empty — checked in Load.
	// If we got here without error, it exists.

	// Rule 9: name matches parent directory
	if s.Dir != "" {
		base := filepath.Base(s.Dir)
		if base != s.Name {
			errs = append(errs, ValidationError{
				Rule:  "9",
				Field: "name",
				Msg:   fmt.Sprintf("name %q does not match parent directory %q", s.Name, base),
			})
		}
	}

	// Rule 10 (recommended): SKILL.md includes all required sections.
	// We read SKILL.md and check for section headers.
	if s.SKMDPath != "" {
		if data, err := os.ReadFile(s.SKMDPath); err == nil {
			body := string(data)
			for _, section := range []string{"Decision tree", "Workflow", "Examples", "Anti-patterns", "Failure modes", "Related skills"} {
				if !strings.Contains(body, "## "+section) {
					errs = append(errs, ValidationError{
						Rule:  "10",
						Field: "SKILL.md",
						Msg:   fmt.Sprintf("recommended section ## %s not found", section),
					})
				}
			}
		}
	}

	return errs
}
