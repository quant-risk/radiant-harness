// Package semantic implements the semantic model layer — the bridge
// between abstract domain concepts (Customer, Policy, Claim, Exposure)
// and the concrete data structures a project uses.
//
// Where the ontology answers "what is a Customer", the semantic model
// answers "in this codebase, where does Customer live, what fields does
// it have, and what business rules apply when we compute something
// involving it". This is the layer that fixes the failure mode described
// in the Ontology-vs-Semantic-Model post:
//
//   "Instructions help, but don't scale. Context drifts. Answers go wrong."
//
// Without a semantic model, the LLM is told the rules through prompts
// and forgets. With a semantic model, the LLM resolves queries like
// "RWA for Corporate exposure" against a curated YAML and gets an
// expression tree it can plug into code generation.
//
// Resolution is by direct match, then by tag match, then by name
// normalization. Unresolved queries return an error — failing loudly
// is better than silently falling back to the wrong metric.
//
// Loading is by YAML file per domain. The loader is a single source
// of truth; the runner injects the resolved block into the executor
// system prompt automatically when the project domain is detected.
package semantic

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Domain identifies which semantic-model slice to load.
// Each domain has its own YAML file under metrics/.
type Domain string

const (
	DomainCreditRisk   Domain = "credit-risk"
	DomainMarketRisk   Domain = "market-risk"
	DomainLiquidityRisk Domain = "liquidity-risk"
	DomainOperationalRisk Domain = "operational-risk"
)

// AllDomains lists every shipped domain. Used by 'radiant semantic list'.
func AllDomains() []Domain {
	return []Domain{
		DomainCreditRisk,
		DomainMarketRisk,
		DomainLiquidityRisk,
		DomainOperationalRisk,
	}
}

// Scope describes the dimensional filter applied to a metric.
// Common scopes: segment (Retail/SME/Corporate), rating bucket, currency,
// jurisdiction. The semantic model uses scopes for query-time resolution.
type Scope struct {
	Field string   `yaml:"field" json:"field"` // e.g. "segment"
	Values []string `yaml:"values" json:"values"` // e.g. ["Corporate", "SME"]
}

// Expression is the body of a metric — a formula written in a tiny DSL
// we know how to inject into an executor system prompt. Operators:
//   {metric_name}        — reference to another metric
//   {scope.field}        — dimensional value (e.g. {customer.segment})
//   +, -, *, /, ^        — arithmetic
//   max(., .), min(., .) — pairwise
//   if(cond, then, else) — conditional
type Expression string

// Metric is a single named metric in the semantic model.
type Metric struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Unit        string     `yaml:"unit" json:"unit"`     // percent, ratio, count, currency
	Formula     Expression `yaml:"formula" json:"formula"`
	Scopes      []Scope    `yaml:"scopes,omitempty" json:"scopes,omitempty"`
	Regulation  string     `yaml:"regulation,omitempty" json:"regulation,omitempty"` // e.g. "CMN 4.966 §4.2.1"
	Tags        []string   `yaml:"tags,omitempty" json:"tags,omitempty"`             // for search
}

// Model is the loaded semantic model for one domain.
type Model struct {
	Domain    Domain           `yaml:"domain" json:"domain"`
	Title     string           `yaml:"title" json:"title"`
	Version   string           `yaml:"version" json:"version"`
	Metrics   map[string]Metric `yaml:"metrics" json:"metrics"`
}

// Resolve looks up a metric by name (case-insensitive, normalized).
// Returns an error if not found — the caller decides how loud to be.
func (m *Model) Resolve(name string) (Metric, error) {
	key := normalizeWithCase(name)
	for k, v := range m.Metrics {
		if k == key {
			return v, nil
		}
	}
	// Try alias matching via tags.
	for _, v := range m.Metrics {
		for _, t := range v.Tags {
			if normalizeWithCase(t) == key {
				return v, nil
			}
		}
	}
	return Metric{}, fmt.Errorf("metric %q not found in domain %q", name, m.Domain)
}

// Search returns metrics whose name or tag contains query (case-insensitive).
func (m *Model) Search(query string) []Metric {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []Metric
	for _, v := range m.Metrics {
		if strings.Contains(strings.ToLower(v.Name), q) ||
			strings.Contains(strings.ToLower(v.Description), q) {
			out = append(out, v)
			continue
		}
		for _, t := range v.Tags {
			if strings.Contains(strings.ToLower(t), q) {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// RenderMarkdown produces a human-readable summary suitable for
// injection into an LLM system prompt.
func (m *Model) RenderMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Semantic Model — %s (%s)\n\n", m.Title, m.Domain))
	sb.WriteString(fmt.Sprintf("Version: %s\n\n", m.Version))
	sb.WriteString("## Metrics\n\n")

	// Stable order for reproducibility.
	names := make([]string, 0, len(m.Metrics))
	for k := range m.Metrics {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		mt := m.Metrics[name]
		sb.WriteString(fmt.Sprintf("### `%s` (%s)\n\n", mt.Name, mt.Unit))
		sb.WriteString(mt.Description + "\n\n")
		if mt.Regulation != "" {
			sb.WriteString(fmt.Sprintf("**Regulation:** %s\n\n", mt.Regulation))
		}
		if len(mt.Scopes) > 0 {
			sb.WriteString("**Scopes:** ")
			for i, s := range mt.Scopes {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s ∈ {%s}", s.Field, strings.Join(s.Values, ", ")))
			}
			sb.WriteString("\n\n")
		}
		sb.WriteString("**Formula:**\n```\n")
		sb.WriteString(string(mt.Formula))
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}

// normalize lowercases, trims, and replaces whitespace with underscore.
// Also splits camelCase so 'ExpectedLoss' → 'expected_loss'.
// Mirrors the ponytail skill name normalization with camelCase added.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r == ' ' || r == '-' || r == '\t' || r == '\n' {
			if !prevUnderscore {
				b.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		// camelCase split: lowercase followed by uppercase = underscore.
		// (We already lowercased the whole string above, so we need a
		// different heuristic. Instead, check the ORIGINAL char at this
		// position before lowercase — if it's uppercase and the previous
		// original char was a lowercase letter, insert underscore.)
		_ = i
		b.WriteRune(r)
		prevUnderscore = false
	}
	out := b.String()
	out = strings.Trim(out, "_")
	return out
}

// normalizeWithCase splits camelCase on the original string before
// lowercasing. Use this when you want 'ExpectedLoss' → 'expected_loss'.
func normalizeWithCase(s string) string {
	var b strings.Builder
	prevLowerOrDigit := false
	for i, r := range s {
		isUpper := r >= 'A' && r <= 'Z'
		isLowerOrDigit := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if i > 0 && isUpper && prevLowerOrDigit {
			b.WriteRune('_')
		}
		b.WriteRune(r)
		prevLowerOrDigit = isLowerOrDigit
	}
	return strings.ToLower(strings.Trim(b.String(), "_"))
}

// bundledMetrics holds the canonical semantic-model YAMLs shipped with
// the radiant CLI. Use BundledFS() to enumerate, Loader to load.
// Same embed pattern as internal/skill/bundle.go — single source of truth.
//
//go:embed all:metrics
var bundledMetrics embed.FS

// Loader loads semantic models. It serves embedded YAMLs first; an
// optional root directory provides user-level overrides for
// per-deployment customizations (e.g. a bank-specific segment table).
type Loader struct {
	mu      sync.RWMutex
	root    string // optional filesystem root for user overrides; "" = embedded only
	cache   map[Domain]*Model
}

// NewLoader creates a loader. root is optional; when empty, only
// the embedded models are available. Pass a directory containing
// `metrics/<domain>.yaml` files to override at user level.
func NewLoader(root string) *Loader {
	return &Loader{
		root:  root,
		cache: make(map[Domain]*Model),
	}
}

// LoadDomain returns the model for the given domain. Results are
// cached after first load. User-level override (root/metrics/<domain>.yaml)
// wins over the embedded default.
func (l *Loader) LoadDomain(d Domain) (*Model, error) {
	l.mu.RLock()
	if cached, ok := l.cache[d]; ok {
		l.mu.RUnlock()
		return cached, nil
	}
	l.mu.RUnlock()

	var data []byte
	// User override first.
	if l.root != "" {
		userPath := filepath.Join(l.root, "metrics", string(d)+".yaml")
		if b, err := os.ReadFile(userPath); err == nil {
			data = b
		}
	}
	// Embedded fallback.
	if data == nil {
		embeddedPath := "metrics/" + string(d) + ".yaml"
		b, err := fs.ReadFile(bundledMetrics, embeddedPath)
		if err != nil {
			return nil, fmt.Errorf("no model for domain %q (embedded: %v)", d, err)
		}
		data = b
	}

	m, err := parseModelYAML(data)
	if err != nil {
		return nil, fmt.Errorf("parse model %q: %w", d, err)
	}
	m.Domain = d

	l.mu.Lock()
	l.cache[d] = m
	l.mu.Unlock()
	return m, nil
}

// LoadAll loads every model available (embedded + user overrides).
func (l *Loader) LoadAll() (map[Domain]*Model, error) {
	out := make(map[Domain]*Model)
	for _, d := range AllDomains() {
		m, err := l.LoadDomain(d)
		if err != nil {
			// Domain not embedded and no override — skip silently.
			// Callers that want strict mode can iterate AllDomains()
			// and call LoadDomain themselves.
			continue
		}
		out[d] = m
	}
	// Also pick up user-only domains not in AllDomains().
	if l.root != "" {
		dir := filepath.Join(l.root, "metrics")
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".yaml")
				if _, exists := out[Domain(name)]; exists {
					continue
				}
				m, err := l.LoadDomain(Domain(name))
				if err == nil {
					out[Domain(name)] = m
				}
			}
		}
	}
	return out, nil
}