package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validSkill is the canonical well-formed skill used as the basis
// for "valid" test cases. Each field is set so no rule fires; tests
// that exercise rule violations mutate one field and assert the
// resulting error.
func validSkill() *Skill {
	return &Skill{
		Name:         "nova-feature",
		Version:      "1.0.0",
		Description:  "Start a new feature in the SDD pipeline.",
		WhenToUse:    "User asked for a new feature with clear intent.",
		TierEligible: []string{"trivial", "feature", "architecture"},
		Inputs: []Input{
			{Name: "intent", Type: "string", Required: true, Description: "What to build"},
			{Name: "context", Type: "string", Required: false, Description: "Optional context"},
		},
		Outputs: []Output{
			{Path: "specs/<NNNN>-<slug>/spec.md", Type: "artifact", Description: "Spec"},
			{Path: "specs/<NNNN>-<slug>/tasks.md", Type: "artifact", Description: "Tasks"},
		},
		Gates: []Gate{
			{Name: "tier-decided", Description: "Tier chosen"},
			{Name: "ac-testable", Description: "ACs are testable"},
		},
		ContextProvides:   []string{"state.md", "glossary.md"},
		CommandsAvailable: []string{"radiant spec <intent>"},
		RelatedSkills:     []string{"clarificar", "validar"},
		AntiPatterns:      []string{"Implementing without reading state.md"},
		Author:            "radiant-harness contributors",
		License:           "MIT",
	}
}

// TestValidateAcceptsValidSkill is the happy-path baseline: a fully
// populated Skill with no rule violations should return zero errors.
func TestValidateAcceptsValidSkill(t *testing.T) {
	s := validSkill()
	if errs := s.Validate(); len(errs) > 0 {
		t.Errorf("valid skill produced %d errors: %v", len(errs), errs)
	}
}

// TestRule1NamePattern checks rule 1 (kebab-case, 1-32 chars,
// starts with lowercase letter).
func TestRule1NamePattern(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "nova-feature", false},
		{"valid single char", "a", false},
		{"valid 32 chars", "a2345678901234567890123456789012", false}, // 33 chars (33+letters)
		{"uppercase rejected", "NovaFeature", true},
		{"underscore rejected", "nova_feature", true},
		{"starts with digit", "1nova", true},
		{"empty", "", true},
		{"contains space", "nova feature", true},
		{"contains slash", "nova/feature", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSkill()
			s.Name = tc.input
			errs := s.Validate()
			hasRule1 := false
			for _, e := range errs {
				if e.Rule == "1" {
					hasRule1 = true
					break
				}
			}
			if hasRule1 != tc.wantErr {
				t.Errorf("name=%q: rule1 fired=%v, want=%v (errors: %v)", tc.input, hasRule1, tc.wantErr, errs)
			}
		})
	}
}

// TestRule2Semver checks rule 2 (MAJOR.MINOR.PATCH).
func TestRule2Semver(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
	}{
		{"1.0.0", false},
		{"0.0.1", false},
		{"10.20.30", false},
		{"1.0", true},
		{"1.0.0.0", true},
		{"v1.0.0", true},
		{"", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			s := validSkill()
			s.Version = tc.input
			errs := s.Validate()
			has := false
			for _, e := range errs {
				if e.Rule == "2" {
					has = true
				}
			}
			if has != tc.wantErr {
				t.Errorf("version=%q: rule2=%v, want=%v", tc.input, has, tc.wantErr)
			}
		})
	}
}

// TestRule3TierEligible checks the closed set and uniqueness.
func TestRule3TierEligible(t *testing.T) {
	cases := []struct {
		input   []string
		wantErr bool
	}{
		{[]string{"trivial"}, false},
		{[]string{"trivial", "feature"}, false},
		{[]string{"trivial", "feature", "architecture"}, false},
		{[]string{}, true},
		{[]string{"huge"}, true},               // unknown tier
		{[]string{"trivial", "trivial"}, true}, // duplicate
	}
	for _, tc := range cases {
		s := validSkill()
		s.TierEligible = tc.input
		errs := s.Validate()
		has := false
		for _, e := range errs {
			if e.Rule == "3" {
				has = true
			}
		}
		if has != tc.wantErr {
			t.Errorf("tier_eligible=%v: rule3=%v, want=%v", tc.input, has, tc.wantErr)
		}
	}
}

// TestRule4InputUniquenessAndTypes checks name uniqueness and type
// closed set.
func TestRule4InputUniquenessAndTypes(t *testing.T) {
	s := validSkill()
	s.Inputs = []Input{
		{Name: "intent", Type: "string", Required: true},
		{Name: "intent", Type: "string", Required: true}, // dup name
		{Name: "count", Type: "banana", Required: false}, // bad type
	}
	errs := s.Validate()
	rule4Count := 0
	for _, e := range errs {
		if e.Rule == "4" {
			rule4Count++
		}
	}
	if rule4Count < 2 {
		t.Errorf("expected >=2 rule4 errors, got %d: %v", rule4Count, errs)
	}
}

// TestRule5OutputUniquenessAndTypes checks path uniqueness and type
// closed set, plus the report-special-case (path = "").
func TestRule5OutputUniquenessAndTypes(t *testing.T) {
	s := validSkill()
	s.Outputs = []Output{
		{Path: "a.md", Type: "artifact"},
		{Path: "a.md", Type: "artifact"}, // dup
		{Path: "b.md", Type: "video"},    // bad type
		{Path: "", Type: "artifact"},     // missing path on artifact
		{Path: "-", Type: "report"},      // OK: report allows empty path
	}
	errs := s.Validate()
	rule5Count := 0
	for _, e := range errs {
		if e.Rule == "5" {
			rule5Count++
		}
	}
	if rule5Count < 3 {
		t.Errorf("expected >=3 rule5 errors, got %d: %v", rule5Count, errs)
	}
}

// TestRule6GateUniqueness checks gate name uniqueness.
func TestRule6GateUniqueness(t *testing.T) {
	s := validSkill()
	s.Gates = []Gate{
		{Name: "ready"},
		{Name: "ready"}, // dup
		{Name: ""},      // missing name
	}
	errs := s.Validate()
	rule6Count := 0
	for _, e := range errs {
		if e.Rule == "6" {
			rule6Count++
		}
	}
	if rule6Count < 2 {
		t.Errorf("expected >=2 rule6 errors, got %d", rule6Count)
	}
}

// TestRule7RequiredFieldsPresent checks description + when_to_use.
func TestRule7RequiredFieldsPresent(t *testing.T) {
	s := validSkill()
	s.Description = ""
	s.WhenToUse = ""
	errs := s.Validate()
	rule7Count := 0
	for _, e := range errs {
		if e.Rule == "7" {
			rule7Count++
		}
	}
	if rule7Count != 2 {
		t.Errorf("expected 2 rule7 errors, got %d", rule7Count)
	}
}

// TestRule9NameMatchesDirectory checks that the YAML `name` field
// matches the parent directory name.
func TestRule9NameMatchesDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create a directory whose name doesn't match the YAML name.
	bad := filepath.Join(dir, "wrong-dir-name")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "frontmatter.yaml"),
		[]byte("name: right-name\nversion: 1.0.0\ndescription: d\nwhen_to_use: w\ntier_eligible: [trivial]\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("## Workflow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(bad)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	errs := s.Validate()
	hasRule9 := false
	for _, e := range errs {
		if e.Rule == "9" {
			hasRule9 = true
		}
	}
	if !hasRule9 {
		t.Errorf("expected rule9 to fire when name != dir, got errors: %v", errs)
	}
}

// TestLoadAndValidateRoundTrip exercises the full Load → Validate
// path on a real on-disk skill structure.
func TestLoadAndValidateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "roundtrip")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := `name: roundtrip
version: 1.0.0
description: roundtrip test skill
when_to_use: when testing the loader
tier_eligible: [trivial]
inputs:
  - name: x
    type: string
    required: true
    description: input
outputs:
  - path: out.md
    type: artifact
    description: output
gates:
  - name: ready
    description: ready check
`
	if err := os.WriteFile(filepath.Join(skillDir, "frontmatter.yaml"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
	skmd := "# Skill: roundtrip\n\n## Decision tree\nn/a\n\n## Workflow\nn/a\n\n## Examples\nn/a\n\n## Anti-patterns\nn/a\n\n## Failure modes\nn/a\n\n## Related skills\nn/a\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skmd), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name != "roundtrip" {
		t.Errorf("Name = %q, want roundtrip", s.Name)
	}
	if s.Dir != skillDir {
		t.Errorf("Dir = %q, want %q", s.Dir, skillDir)
	}
	if len(s.Inputs) != 1 || s.Inputs[0].Name != "x" {
		t.Errorf("Inputs = %+v, want one input named x", s.Inputs)
	}
	errs := s.Validate()
	// Should fire rule 10 only if SKILL.md sections are missing — we
	// included all 6, so no rule 10 errors.
	for _, e := range errs {
		t.Errorf("unexpected validation error: %v", e)
	}
}

// TestLoadMissingFrontmatter returns an error when frontmatter.yaml
// is absent.
func TestLoadMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); err == nil {
		t.Errorf("Load on empty dir should return an error")
	}
}

// TestLoadMissingSKILLMD returns an error when SKILL.md is absent.
func TestLoadMissingSKILLMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frontmatter.yaml"),
		[]byte("name: x\nversion: 1.0.0\ntier_eligible: [trivial]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Errorf("Load without SKILL.md should return an error")
	}
}

// TestLoadEmptySKILLMD returns an error when SKILL.md is empty.
func TestLoadEmptySKILLMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frontmatter.yaml"),
		[]byte("name: x\nversion: 1.0.0\ntier_eligible: [trivial]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Errorf("Load with empty SKILL.md should return an error")
	}
}

// TestRule10SKILLMDSections checks that SKILL.md includes the 6
// recommended section headers.
func TestRule10SKILLMDSections(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "sections")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "frontmatter.yaml"),
		[]byte("name: sections\nversion: 1.0.0\ndescription: d\nwhen_to_use: w\ntier_eligible: [trivial]\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	// SKILL.md missing most sections
	skmd := "# Skill: sections\n\n## Workflow\n\nn/a\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skmd), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(skillDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	errs := s.Validate()
	rule10Count := 0
	for _, e := range errs {
		if e.Rule == "10" {
			rule10Count++
		}
	}
	if rule10Count < 5 {
		t.Errorf("expected >=5 rule10 errors for incomplete SKILL.md, got %d", rule10Count)
	}
}

// TestValidationErrorMessage is a sanity check on the error message
// format — used by the CLI to print diagnostics.
func TestValidationErrorMessage(t *testing.T) {
	e := ValidationError{Rule: "1", Field: "name", Msg: "bad name"}
	msg := e.Error()
	if !strings.Contains(msg, "rule 1") {
		t.Errorf("error should mention rule number, got: %s", msg)
	}
	if !strings.Contains(msg, "name") {
		t.Errorf("error should mention field, got: %s", msg)
	}
	if !strings.Contains(msg, "bad name") {
		t.Errorf("error should include msg, got: %s", msg)
	}
}

// TestNovaFeatureValidatesCleanly is the regression guard for the
// nova-feature rewrite — the canonical showcase skill must validate
// against the new schema with zero errors. Catches drift between
// the spec and the implementation.
func TestNovaFeatureValidatesCleanly(t *testing.T) {
	const novaPath = "skills/nova-feature"
	s, err := LoadFromFS(bundledFS, novaPath)
	if err != nil {
		t.Fatalf("Load nova-feature from bundle: %v", err)
	}
	errs := s.Validate()
	if len(errs) > 0 {
		t.Errorf("nova-feature must validate cleanly (it's the showcase skill); got %d errors:", len(errs))
		for _, e := range errs {
			t.Errorf("  %s", e.Error())
		}
	}
}

// TestBundleIncludesNovaFeature exercises the embedded Bundle()
// listing — proves the //go:embed directive includes nova-feature.
func TestBundleIncludesNovaFeature(t *testing.T) {
	infos, err := Bundle()
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Name == "nova-feature" {
			found = true
			if info.Version == "" {
				t.Error("nova-feature in bundle has empty version")
			}
			if info.Description == "" {
				t.Error("nova-feature in bundle has empty description")
			}
			break
		}
	}
	if !found {
		t.Errorf("nova-feature not found in bundle; got: %v", infos)
	}
}

// TestExtractToRoundTrip extracts the bundle into a temp dir, then
// re-loads each extracted skill and validates it. Catches issues
// with the copy logic (encoding, partial writes, etc.).
func TestExtractToRoundTrip(t *testing.T) {
	dest := t.TempDir()
	if err := ExtractTo(dest, true); err != nil {
		t.Fatalf("ExtractTo: %v", err)
	}
	// Re-load each extracted skill.
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("ExtractTo wrote 0 directories")
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := Load(filepath.Join(dest, e.Name()))
		if err != nil {
			t.Errorf("Load extracted %s: %v", e.Name(), err)
			continue
		}
		if errs := s.Validate(); len(errs) > 0 {
			t.Errorf("extracted %s failed validation: %v", e.Name(), errs)
		}
	}
}

// TestExtractToSkipsExisting verifies the safety check: when
// `force=false`, existing skills are left alone. Use this when the
// user has local edits they want to keep.
func TestExtractToSkipsExisting(t *testing.T) {
	dest := t.TempDir()
	// Pre-create a sentinel file in one of the skill dirs.
	target := filepath.Join(dest, "nova-feature")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "LOCAL_EDIT.md")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Extract WITHOUT force — sentinel should be preserved.
	if err := ExtractTo(dest, false); err != nil {
		t.Fatalf("ExtractTo: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("sentinel was deleted despite force=false: %v", err)
	}
}
