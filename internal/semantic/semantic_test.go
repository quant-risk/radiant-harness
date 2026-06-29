package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"PD":                "pd",
		"Expected Loss":     "expected_loss",
		"ExpectedLoss":      "expectedloss",
		"  RWA  ":           "rwa",
		"capital_required":  "capital_required",
		"Capital-Required":  "capital_required",
		"Customer.Segment":  "customer.segment", // dots preserved (scope syntax)
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q): got %q want %q", in, got, want)
		}
	}
}

func TestLoaderLoadCreditRisk(t *testing.T) {
	// Find the metrics directory in the package's source tree.
	root := findRepoRoot(t)
	l := NewLoader(root)
	m, err := l.LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatalf("LoadDomain: %v", err)
	}
	if m.Domain != DomainCreditRisk {
		t.Errorf("domain: got %q want %q", m.Domain, DomainCreditRisk)
	}
	if m.Title == "" {
		t.Errorf("title should be populated")
	}
	// Spot-check that PD, LGD, EAD, RWA all loaded.
	for _, name := range []string{"pd", "lgd", "ead", "rwa", "expected_loss", "provision_min_ifrs9", "capital_required"} {
		if _, ok := m.Metrics[name]; !ok {
			t.Errorf("missing metric %q in credit-risk model", name)
		}
	}
}

func TestModelResolve(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	mt, err := m.Resolve("PD")
	if err != nil {
		t.Fatalf("Resolve PD: %v", err)
	}
	if mt.Unit != "percent" {
		t.Errorf("PD unit: got %q want percent", mt.Unit)
	}
	if !strings.Contains(string(mt.Formula), "rating_pd_table") {
		t.Errorf("PD formula should reference rating_pd_table, got: %s", mt.Formula)
	}
	if mt.Regulation == "" {
		t.Errorf("PD should have regulation reference")
	}
}

func TestModelResolve_NotFound(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Resolve("nonexistent_metric")
	if err == nil {
		t.Errorf("expected error for nonexistent metric")
	}
}

func TestModelResolve_TagMatch(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	// Aliases via tags
	_, err = m.Resolve("probability_of_default")
	if err != nil {
		t.Errorf("tag-based alias should resolve, got: %v", err)
	}
}

func TestModelSearch(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	hits := m.Search("default")
	if len(hits) == 0 {
		t.Errorf("Search 'default' should find PD or LGD")
	}
	hits = m.Search("basileia")
	if len(hits) < 4 {
		t.Errorf("Search 'basileia' should find at least 4 metrics, got %d", len(hits))
	}
}

func TestRenderMarkdown(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	out := m.RenderMarkdown()
	if !strings.Contains(out, "Credit Risk Metrics") {
		t.Errorf("rendered markdown should contain domain title")
	}
	if !strings.Contains(out, "### `PD`") {
		t.Errorf("rendered markdown should contain PD section")
	}
	if !strings.Contains(out, "CMN 4.966") {
		t.Errorf("rendered markdown should cite CMN 4.966")
	}
}

func TestRenderMarkdownCompact(t *testing.T) {
	root := findRepoRoot(t)
	m, err := NewLoader(root).LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	out := m.RenderMarkdownCompact()
	if !strings.Contains(out, "pd") {
		t.Errorf("compact render should list pd")
	}
	if !strings.Contains(out, "rwa") {
		t.Errorf("compact render should list rwa")
	}
	// Compact should be much shorter than full.
	full := m.RenderMarkdown()
	if len(out) > len(full)/2 {
		t.Errorf("compact render should be < half of full, got compact=%d full=%d", len(out), len(full))
	}
}

func TestLoaderLoadAll(t *testing.T) {
	root := findRepoRoot(t)
	models, err := NewLoader(root).LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if _, ok := models[DomainCreditRisk]; !ok {
		t.Errorf("LoadAll should include credit-risk")
	}
}

func TestLoaderCaching(t *testing.T) {
	root := findRepoRoot(t)
	l := NewLoader(root)
	m1, err := l.LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := l.LoadDomain(DomainCreditRisk)
	if err != nil {
		t.Fatal(err)
	}
	if m1 != m2 {
		t.Errorf("cached load should return same pointer")
	}
}

// findRepoRoot walks up from cwd to find the repo root (has go.mod).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find repo root")
	return ""
}

func TestEstimateFromTokens(t *testing.T) {
	// ~4 chars per token (heuristic).
	if got := EstimateFromTokens("hello world"); got != 3 {
		t.Errorf("EstimateFromTokens 'hello world' (11 chars): got %d want 3", got)
	}
}