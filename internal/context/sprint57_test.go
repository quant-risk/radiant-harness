package context_test

import (
	"os"
	"path/filepath"
	"testing"

	radctx "github.com/quant-risk/radiant-harness/internal/context"
)

// helpers ──────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── scanModulePath (via Detect) ────────────────────────────────────────────

func TestDetect_ModulePathFinance(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/quant-risk/portfolio-engine\n\ngo 1.21\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainFinance {
		t.Errorf("expected finance domain from module path quant-risk, got %q", r.Domain)
	}
	// Signal recorded
	found := false
	for _, s := range r.Signals {
		if len(s) > 7 && s[:7] == "module:" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected module: signal in %v", r.Signals)
	}
}

func TestDetect_ModulePathML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/acme/ml-inference\n\ngo 1.21\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainML {
		t.Errorf("expected ml domain from module path ml-inference, got %q", r.Domain)
	}
}

func TestDetect_ModulePathGeneric(t *testing.T) {
	dir := t.TempDir()
	// Generic module path — should not force any specific domain
	writeFile(t, dir, "go.mod", "module github.com/acme/awesome-tool\n\ngo 1.21\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Should stay general (no other signals)
	if r.Domain != radctx.DomainGeneral {
		t.Errorf("expected general domain for generic module path, got %q", r.Domain)
	}
}

// ── scanDocs (via Detect) ──────────────────────────────────────────────────

func TestDetect_ReadmeFinance(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Acme Trading Platform\n\nThis is a fintech application for portfolio management.\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainFinance {
		t.Errorf("expected finance domain from README, got %q", r.Domain)
	}
	found := false
	for _, s := range r.Signals {
		if len(s) > 5 && s[:5] == "docs:" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected docs: signal in %v", r.Signals)
	}
}

func TestDetect_ReadmeML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# ML Pipeline\n\nDeep learning inference service for NLP tasks.\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainML {
		t.Errorf("expected ml domain from README, got %q", r.Domain)
	}
}

func TestDetect_ClaudeMdOps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# Project\n\nThis is a devops and infrastructure automation project.\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainOps {
		t.Errorf("expected ops domain from CLAUDE.md, got %q", r.Domain)
	}
}

func TestDetect_ReadmeTrumpsWeakImport(t *testing.T) {
	// README says finance but there's no other signal — should win.
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Trading System\n\nCore trading and portfolio risk management system.\n")

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainFinance {
		t.Errorf("expected finance from README keywords, got %q", r.Domain)
	}
}

// ── scanDirNames (via Detect) ──────────────────────────────────────────────

func TestDetect_InternalDirFinance(t *testing.T) {
	dir := t.TempDir()
	// internal/trading directory → finance signal
	if err := os.MkdirAll(filepath.Join(dir, "internal", "trading"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainFinance {
		t.Errorf("expected finance from internal/trading dir, got %q", r.Domain)
	}
	found := false
	for _, s := range r.Signals {
		if len(s) > 4 && s[:4] == "dir:" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected dir: signal in %v", r.Signals)
	}
}

func TestDetect_TopLevelDirML(t *testing.T) {
	dir := t.TempDir()
	// Top-level ml/ dir → stronger weight than internal
	if err := os.MkdirAll(filepath.Join(dir, "ml"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainML {
		t.Errorf("expected ml domain from top-level ml/ dir, got %q", r.Domain)
	}
}

func TestDetect_CmdDirBlockchain(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "defi-bridge"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainBlockchain {
		t.Errorf("expected blockchain from cmd/defi-bridge, got %q", r.Domain)
	}
}

// ── multi-source agreement ─────────────────────────────────────────────────

func TestDetect_MultiSourceFinance(t *testing.T) {
	// Module path + README + internal dir all say finance → high confidence
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/acme/quant-engine\n\ngo 1.21\n")
	writeFile(t, dir, "README.md", "# Quant Engine\n\nTrading and portfolio management system.\n")
	if err := os.MkdirAll(filepath.Join(dir, "internal", "trading"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainFinance {
		t.Errorf("expected finance from multi-source agreement, got %q", r.Domain)
	}
}

func TestDetect_EmptyDirIsGeneral(t *testing.T) {
	dir := t.TempDir()
	r, err := radctx.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Domain != radctx.DomainGeneral {
		t.Errorf("empty dir should be general, got %q", r.Domain)
	}
}
