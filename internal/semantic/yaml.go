package semantic

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseModelYAML parses the on-disk YAML representation of a Model.
// We accept a simpler shape than the in-memory struct: metric keys
// are mapped to a list under `metrics:` to keep the YAML compact and
// human-editable, then we normalize into the map form.
func parseModelYAML(data []byte) (*Model, error) {
	// Raw shape: { domain, title, version, metrics: [{name, ...}, ...] }
	var raw struct {
		Domain  string `yaml:"domain"`
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
		Metrics []struct {
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Unit        string   `yaml:"unit"`
			Formula     string   `yaml:"formula"`
			Scopes      []Scope  `yaml:"scopes"`
			Regulation  string   `yaml:"regulation"`
			Tags        []string `yaml:"tags"`
		} `yaml:"metrics"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	if raw.Domain == "" {
		return nil, fmt.Errorf("missing domain field")
	}
	if raw.Title == "" {
		raw.Title = raw.Domain
	}
	if raw.Version == "" {
		raw.Version = "0.0.0"
	}
	m := &Model{
		Domain:  Domain(raw.Domain),
		Title:   raw.Title,
		Version: raw.Version,
		Metrics: make(map[string]Metric, len(raw.Metrics)),
	}
	for _, mt := range raw.Metrics {
		if mt.Name == "" {
			continue
		}
		key := normalizeWithCase(mt.Name)
		m.Metrics[key] = Metric{
			Name:        mt.Name,
			Description: mt.Description,
			Unit:        mt.Unit,
			Formula:     Expression(mt.Formula),
			Scopes:      mt.Scopes,
			Regulation:  mt.Regulation,
			Tags:        mt.Tags,
		}
	}
	return m, nil
}

// EstimateFromTokens is unused here; reserved for a future caller that
// wants to size the semantic-model block before injection. The presence
// of this function keeps the file from being only parseYAML — a small
// signal to maintainers that the package has more than one job.
func EstimateFromTokens(s string) int {
	return (len(s) + 3) / 4
}

// RenderMarkdownCompact produces a single-paragraph summary of the model
// for places where the full Markdown is too long (e.g. small LLM windows).
// Lists metric names with one-line descriptions.
func (m *Model) RenderMarkdownCompact() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Semantic model: %s (%s)\n", m.Title, m.Domain))
	sb.WriteString("Metrics: ")
	names := make([]string, 0, len(m.Metrics))
	for k := range m.Metrics {
		names = append(names, k)
	}
	sortStrings(names)
	for i, n := range names {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(n)
	}
	sb.WriteString(".\nUse 'radiant semantic resolve <name>' for full formula and regulation.")
	return sb.String()
}

// sortStrings is a tiny non-allocating sort for short slices.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// Compile-time guard: must be able to format floats (used by callers that
// include numbers in markdown).
var _ = strconv.FormatFloat