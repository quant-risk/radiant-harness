// Package spec parses SDD specification files. The parsers are deliberately
// forgiving: real-world specs from humans and LLMs vary in formatting, so
// we normalize aggressively (whitespace, case, AC ID syntax) instead of
// failing on every stylistic deviation.
package spec

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

// scannerBufferSize is the max line length the parser tolerates. LLM-written
// specs occasionally emit JSON blobs or large examples inline; the default
// 64 KB is too tight.
const scannerBufferSize = 1024 * 1024

// Regular expressions. All of them are case-insensitive (we lower-case the
// line before matching when applicable) so "GIVEN", "given", "Given" all
// parse the same way.
var (
	acRe     = regexp.MustCompile(`(?i)^###\s+(AC[-_]?\d+)\s*:?\s*(.*)$`)
	givenRe  = regexp.MustCompile(`(?i)^[-\s]*\*?\*?Given\*?\*?\s*[:：]\s*(.+)$`)
	whenRe   = regexp.MustCompile(`(?i)^[-\s]*\*?\*?When\*?\*?\s*[:：]\s*(.+)$`)
	thenRe   = regexp.MustCompile(`(?i)^[-\s]*\*?\*?Then\*?\*?\s*[:：]\s*(.+)$`)
	andRe    = regexp.MustCompile(`(?i)^[-\s]*\*?\*?And\*?\*?\s*[:：]\s*(.+)$`)
	fmRe     = regexp.MustCompile(`^([a-zA-Z][\w-]*)\s*:\s*(.+)$`)
	summaryRe = regexp.MustCompile(`(?i)^#+\s*Summary\s*$`)
)

// ParseSpec reads and parses a spec.md file. Returns an error only on I/O
// failure; syntactic deviations are tolerated and reported through the
// returned Spec (e.g. ACs with missing Given/When/Then stay in the slice
// with empty fields so the validator can flag them).
func ParseSpec(path string) (*radiant.Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec: %w", err)
	}
	defer f.Close()

	spec := &radiant.Spec{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), scannerBufferSize)
	inFM := false
	var currentAC *radiant.AcceptanceCriteria

	for scanner.Scan() {
		line := scanner.Text()

		// Frontmatter ── open at first "---", close at second.
		if strings.TrimSpace(line) == "---" {
			inFM = !inFM
			continue
		}
		if inFM {
			if m := fmRe.FindStringSubmatch(line); m != nil {
				switch strings.ToLower(m[1]) {
				case "name":
					spec.Name = strings.TrimSpace(m[2])
				case "description":
					spec.Description = strings.TrimSpace(m[2])
				case "summary":
					spec.Summary = strings.TrimSpace(m[2])
				}
			}
			continue
		}

		// AC header (case-insensitive; tolerates "AC1", "AC-1", "AC_1", "AC: 1").
		if m := acRe.FindStringSubmatch(line); m != nil {
			if currentAC != nil {
				spec.ACs = append(spec.ACs, *currentAC)
			}
			currentAC = &radiant.AcceptanceCriteria{
				ID:    NormalizeACID(m[1]),
				Title: strings.TrimSpace(m[2]),
			}
			continue
		}

		// Given / When / Then / And. "And" extends the most recent clause.
		if currentAC != nil {
			if m := givenRe.FindStringSubmatch(line); m != nil {
				currentAC.Given = strings.TrimSpace(m[1])
				continue
			}
			if m := whenRe.FindStringSubmatch(line); m != nil {
				currentAC.When = strings.TrimSpace(m[1])
				continue
			}
			if m := thenRe.FindStringSubmatch(line); m != nil {
				currentAC.Then = strings.TrimSpace(m[1])
				continue
			}
			if m := andRe.FindStringSubmatch(line); m != nil {
				// Append "and …" to whichever clause is last non-empty so
				// multi-clause assertions don't silently lose data.
				extra := " and " + strings.TrimSpace(m[1])
				switch {
				case currentAC.Then != "":
					currentAC.Then += extra
				case currentAC.When != "":
					currentAC.When += extra
				case currentAC.Given != "":
					currentAC.Given += extra
				}
				continue
			}
		}

		// Summary section.
		if summaryRe.MatchString(line) {
			for scanner.Scan() {
				s := strings.TrimSpace(scanner.Text())
				if s == "" || strings.HasPrefix(s, "#") {
					break
				}
				if spec.Summary == "" {
					spec.Summary = s
				}
			}
		}
	}

	if currentAC != nil {
		spec.ACs = append(spec.ACs, *currentAC)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan spec: %w", err)
	}
	return spec, nil
}

// NormalizeACID canonicalizes an AC identifier from any of the forms that
// real-world specs use into the canonical "AC-N". AC1 → AC-1, AC_1 → AC-1,
// ACN → AC-N (no — that's ambiguous, so we don't touch bare "ACN"), "AC 1"
// → "AC-1".
func NormalizeACID(id string) string {
	id = strings.TrimSpace(strings.ToUpper(id))
	id = strings.ReplaceAll(id, "_", "-")
	id = strings.ReplaceAll(id, " ", "-")
	if !strings.Contains(id, "-") && strings.HasPrefix(id, "AC") && len(id) > 2 {
		id = "AC-" + id[2:]
	}
	return id
}

// ACTokens extracts AC-N tokens from a string and normalizes each. Used by
// fidelity checks and cross-referencing between spec.md and tasks.md.
func ACTokens(s string) []string {
	re := regexp.MustCompile(`(?i)AC[-_ ]?\d+`)
	matches := re.FindAllString(s, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		normalized := NormalizeACID(m)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}
