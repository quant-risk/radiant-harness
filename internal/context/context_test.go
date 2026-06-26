package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- detector tests ---

func TestDetect_GoBackend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/myapi\ngo 1.22\n")
	writeFile(t, dir, "main.go", `package main
import "net/http"
func main() { http.ListenAndServe(":8080", nil) }`)

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProjectName != "myapi" {
		t.Errorf("ProjectName = %q, want %q", r.ProjectName, "myapi")
	}
	if r.Domain != DomainBackend && r.Domain != DomainGeneral {
		t.Errorf("unexpected domain %q for plain Go project", r.Domain)
	}
}

func TestDetect_FinanceProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/bank/credit-risk\ngo 1.22\n")
	writeFile(t, dir, "model.go", `package main
// credit risk pd model
// regulatory framework: ifrs9 basileia capital`)

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainFinance {
		t.Errorf("expected finance domain, got %q (signals: %v)", r.Domain, r.Signals)
	}
}

func TestDetect_MLProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "torch==2.1\ntransformers==4.40\nnumpy\n")
	writeFile(t, dir, "train.py", "import torch\nimport numpy as np\nfrom sklearn import metrics\n")

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainML {
		t.Errorf("expected ml domain, got %q", r.Domain)
	}
}

func TestDetect_FrontendProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"my-app","dependencies":{"react":"18"}}`)

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainFrontend {
		t.Errorf("expected frontend domain, got %q", r.Domain)
	}
}

func TestDetect_SystemsProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"my-daemon\"\nversion = \"0.1.0\"\n")

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainSystems {
		t.Errorf("expected systems domain, got %q", r.Domain)
	}
}

func TestDetect_OpsProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "k8s"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "go.mod", "module github.com/example/operator\ngo 1.22\n")

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainOps {
		t.Errorf("expected ops domain, got %q (signals: %v)", r.Domain, r.Signals)
	}
}

func TestDetect_BlockchainProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"nft-contract"}`)
	writeFile(t, dir, "contract.sol", "// SPDX-License-Identifier: MIT\npragma solidity ^0.8;\nimport web3;\n")

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainBlockchain {
		t.Errorf("expected blockchain domain, got %q (signals: %v)", r.Domain, r.Signals)
	}
}

func TestDetect_TierProduct(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs", "product"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs", "architecture"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0o755)
	os.MkdirAll(filepath.Join(dir, "specs"), 0o755)

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tier != TierProduct && r.Tier != TierArchitecture {
		t.Errorf("expected product/architecture tier, got %q (signals: %v)", r.Tier, r.Signals)
	}
}

func TestDetect_ActiveSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-login")
	os.MkdirAll(specDir, 0o755)
	writeFile(t, specDir, "spec.md", "# Login Feature\n")

	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.ActiveSpec, "specs/") {
		t.Errorf("expected active spec in specs/, got %q", r.ActiveSpec)
	}
}

func TestDetect_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	r, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != DomainGeneral {
		t.Errorf("expected general domain for empty project, got %q", r.Domain)
	}
	if len(r.RecommendedSkills) == 0 {
		t.Error("expected at least core skills recommended for empty project")
	}
}

// --- registry / recommend tests ---

func TestRecommendSkills_AlwaysIncludesCore(t *testing.T) {
	for _, domain := range []Domain{DomainFinance, DomainML, DomainFrontend, DomainGeneral} {
		skills := recommendSkills(domain, TierFeature)
		for _, core := range coreSkills {
			found := false
			for _, s := range skills {
				if s == core {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("domain=%s: core skill %q missing from recommendations %v", domain, core, skills)
			}
		}
	}
}

func TestRecommendSkills_FinanceHasFinanceSkills(t *testing.T) {
	skills := recommendSkills(DomainFinance, TierFeature)
	hasFinance := false
	for _, s := range skills {
		for _, fs := range []string{"credit-risk", "market-risk", "regulatory", "model-risk"} {
			if s == fs {
				hasFinance = true
			}
		}
	}
	if !hasFinance {
		t.Errorf("finance domain should include at least one finance skill, got %v", skills)
	}
}

func TestRecommendSkills_MaxLength(t *testing.T) {
	for _, domain := range []Domain{DomainFinance, DomainML, DomainOps, DomainGeneral} {
		for _, tier := range []Tier{TierTrivial, TierFeature, TierArchitecture, TierProduct} {
			skills := recommendSkills(domain, tier)
			if len(skills) > 10 {
				t.Errorf("domain=%s tier=%s: too many skills (%d > 10): %v", domain, tier, len(skills), skills)
			}
			if len(skills) == 0 {
				t.Errorf("domain=%s tier=%s: no skills recommended", domain, tier)
			}
		}
	}
}

func TestRecommendSkills_NoDuplicates(t *testing.T) {
	skills := recommendSkills(DomainFinance, TierProduct)
	seen := map[string]bool{}
	for _, s := range skills {
		if seen[s] {
			t.Errorf("duplicate skill %q in recommendations %v", s, skills)
		}
		seen[s] = true
	}
}

// --- compressor tests ---

func TestEstimateTokens_Empty(t *testing.T) {
	if n := estimateTokens(""); n != 0 {
		t.Errorf("empty string: want 0, got %d", n)
	}
}

func TestEstimateTokens_Prose(t *testing.T) {
	// 10 short words → roughly 10–15 tokens
	n := estimateTokens("the quick brown fox jumps over the lazy dog now")
	if n < 8 || n > 20 {
		t.Errorf("prose estimate out of range: %d", n)
	}
}

func TestCompress_UnderBudget(t *testing.T) {
	content := "# Title\n\nShort content.\n"
	result := Compress(content, 10000)
	if result.Truncated {
		t.Error("should not truncate when under budget")
	}
	if result.Content != content {
		t.Error("content should be unchanged when under budget")
	}
}

func TestCompress_OverBudget(t *testing.T) {
	// Build content larger than budget
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("This is a sentence with some words to build up token count meaningfully. ")
	}
	content := sb.String()
	budget := 50

	result := Compress(content, budget)
	if result.Compressed > budget*2 { // allow 2x tolerance for estimate imprecision
		t.Errorf("compressed tokens %d far exceeds budget %d", result.Compressed, budget)
	}
}

func TestCompress_NoBudget(t *testing.T) {
	content := "# Hello\n\nWorld.\n"
	result := Compress(content, 0)
	if result.Content != content {
		t.Error("zero budget should return content unchanged")
	}
}

func TestCompress_StripCompletedPhases(t *testing.T) {
	content := "# Title\n\n<!-- phase:done -->\nOld phase content.\n<!-- /phase:done -->\n\n## Active\n\nNew content.\n"
	result := Compress(content, 10000)
	if strings.Contains(result.Content, "Old phase content") {
		t.Error("completed phase content should be stripped")
	}
	if !strings.Contains(result.Content, "New content") {
		t.Error("active content should be preserved")
	}
}

func TestCheckBudget(t *testing.T) {
	tests := []struct {
		tokens, budget int
		want           BudgetStatus
	}{
		{500, 1000, BudgetOK},
		{750, 1000, BudgetWarning},
		{1000, 1000, BudgetExceeded},
		{1500, 1000, BudgetExceeded},
		{500, 0, BudgetOK}, // no budget
	}
	for _, tt := range tests {
		got := CheckBudget(tt.tokens, tt.budget, DefaultWarnThreshold)
		if got != tt.want {
			t.Errorf("CheckBudget(%d, %d) = %v, want %v", tt.tokens, tt.budget, got, tt.want)
		}
	}
}

// --- helpers ---

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
