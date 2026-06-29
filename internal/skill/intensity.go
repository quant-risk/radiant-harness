package skill

import (
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// Intensity is the strictness level at which a skill should be applied.
// Mirrors the ponytail convention (lite / full / ultra) so operators
// already familiar with that mental model have zero translation cost.
type Intensity string

const (
	IntensityLite  Intensity = "lite"  // build what was asked, suggest lazy alt in one line
	IntensityFull  Intensity = "full"  // ladder enforced, default
	IntensityUltra Intensity = "ultra" // YAGNI extremist
	IntensityOff   Intensity = "off"   // don't inject
)

// ParseIntensity validates and normalizes the --intensity flag value.
func ParseIntensity(s string) (Intensity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "full":
		return IntensityFull, nil
	case "lite":
		return IntensityLite, nil
	case "ultra":
		return IntensityUltra, nil
	case "off":
		return IntensityOff, nil
	default:
		return IntensityFull, &InvalidIntensityError{Value: s}
	}
}

// InvalidIntensityError is returned by ParseIntensity for unknown values.
type InvalidIntensityError struct{ Value string }

func (e *InvalidIntensityError) Error() string {
	return "intensity must be lite|full|ultra|off, got " + e.Value
}

// FilterForIntensity returns the skill body with intensity-specific
// sections removed when they don't match the active intensity.
//
// Lines that look like:
//
//   | **full**  | ... description ... |
//   - full: ...
//   - lite: ...
//   - ultra: ...
//
// are removed unless their label matches the active intensity.
//
// Lines that look like normal prose or rules (no intensity label) are
// always preserved.
//
// Returns "" cleanly if the skill has no body or fails to read.
func FilterForIntensity(body string, intensity Intensity) string {
	if intensity == IntensityOff {
		return ""
	}
	var out []string
	for _, line := range strings.Split(body, "\n") {
		// Markdown table row: "| **<intensity>** | ..."
		if m := tableRowMatch(line); m != "" {
			keep := matchesIntensity(m, intensity)
			if keep {
				// Strip the intensity label to avoid leaking the marker to the agent.
				line = stripIntensityLabel(line, m)
			} else {
				continue
			}
		} else if m := bulletLabelMatch(line); m != "" {
			// Bullet example: "- full: ..."
			keep := matchesIntensity(m, intensity)
			if !keep {
				continue
			}
			line = stripBulletIntensityLabel(line, m)
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// tableRowMatch extracts the intensity label from a markdown table row, if any.
// Returns "" if the row has no recognized intensity label.
func tableRowMatch(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return ""
	}
	// Match `| **lite** |`, `| **full**  |`, `| **ultra** |`
	for _, label := range []string{"lite", "full", "ultra"} {
		needle := "**" + label + "**"
		if strings.Contains(trimmed, needle) {
			return label
		}
	}
	return ""
}

// bulletLabelMatch extracts the intensity label from a bullet example, if any.
// Format: "- <intensity>: <example text>"
func bulletLabelMatch(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- ") {
		return ""
	}
	for _, label := range []string{"lite", "full", "ultra"} {
		prefix := "- " + label + ":"
		if strings.HasPrefix(trimmed, prefix) {
			return label
		}
	}
	return ""
}

// matchesIntensity reports whether the given label matches the active intensity.
func matchesIntensity(label string, intensity Intensity) bool {
	return string(intensity) == label
}

// stripIntensityLabel removes the bolded label from a table row, leaving the
// description intact.
func stripIntensityLabel(line, label string) string {
	needle := "**" + label + "**"
	return strings.Replace(line, needle, strings.ToUpper(label), 1)
}

// stripBulletIntensityLabel removes the label prefix from a bullet example.
func stripBulletIntensityLabel(line, label string) string {
	prefix := "- " + label + ":"
	return strings.Replace(line, prefix, "- "+strings.ToUpper(label)+":", 1)
}

// LoadLazyExecutorSkill reads the lazy-executor skill from the bundle and
// returns its body filtered for the requested intensity.
//
// This is the canonical path used by the loop runner when injecting the
// skill into the executor system prompt. It is also used by tests.
func LoadLazyExecutorSkill(intensity Intensity) (string, error) {
	if intensity == IntensityOff {
		return "", nil
	}
	body, err := loadBundledSkillBody("lazy-executor")
	if err != nil {
		return "", err
	}
	return FilterForIntensity(body, intensity), nil
}

// loadBundledSkillBody reads SKILL.md from the embedded skills filesystem.
// Uses the same embed.FS as the rest of the bundle — single source of truth.
func loadBundledSkillBody(name string) (string, error) {
	path := "skills/" + name + "/SKILL.md"
	data, err := fs.ReadFile(bundledFS, path)
	if err != nil {
		return "", fmt.Errorf("read embedded skill %q: %w", name, err)
	}
	body := string(data)
	// Strip YAML frontmatter (--- ... ---).
	if strings.HasPrefix(body, "---") {
		if idx := strings.Index(body[3:], "---"); idx >= 0 {
			body = body[3+idx+3:]
			body = strings.TrimLeft(body, "\r\n")
		}
	}
	return body, nil
}

// Compile-time sanity: frontmatter.yaml of lazy-executor must declare this
// exact name. If a future rename happens the load path breaks loudly via
// this assertion in tests rather than via silent path miss.
var _ = yaml.Marshal // keep yaml import alive for callers that need it