package ontology

import (
	"strings"
	"testing"
)

// ── Graph primitives ──────────────────────────────────────────────────────

func TestNewIsEmpty(t *testing.T) {
	o := New()
	if len(o.Entities) != 0 || len(o.Relations) != 0 {
		t.Fatal("New() should return an empty ontology")
	}
}

func TestAddEntityAndRelate(t *testing.T) {
	o := New()
	o.AddEntity(Entity{Kind: EntitySkill, Name: "validar"})
	o.AddEntity(Entity{Kind: EntityDomain, Name: "domain:general"})
	o.Relate(RelGoverns, "validar", "domain:general")

	if got := o.Related("validar", RelGoverns); len(got) != 1 || got[0] != "domain:general" {
		t.Errorf("Related = %v, want [domain:general]", got)
	}
	if got := o.RelatedInbound("domain:general", RelGoverns); len(got) != 1 || got[0] != "validar" {
		t.Errorf("RelatedInbound = %v, want [validar]", got)
	}
}

func TestRelatedSorted(t *testing.T) {
	o := New()
	o.Relate(RelGoverns, "z-skill", "domain:x")
	o.Relate(RelGoverns, "a-skill", "domain:x")
	got := o.RelatedInbound("domain:x", RelGoverns)
	if len(got) != 2 || got[0] != "a-skill" || got[1] != "z-skill" {
		t.Errorf("RelatedInbound not sorted: %v", got)
	}
}

func TestEntitiesOfKind(t *testing.T) {
	o := Default()
	domains := o.EntitiesOfKind(EntityDomain)
	if len(domains) != len(schemaDomains) {
		t.Errorf("got %d domains, want %d", len(domains), len(schemaDomains))
	}
}

// ── Default schema integrity ───────────────────────────────────────────────

func TestDefaultHasNoViolations(t *testing.T) {
	o := Default()
	if v := o.Violations(); len(v) != 0 {
		t.Fatalf("default ontology has axiom violations:\n%s", strings.Join(v, "\n"))
	}
}

func TestDefaultSkillCount(t *testing.T) {
	o := Default()
	skills := o.EntitiesOfKind(EntitySkill)
	if len(skills) != len(skillGoverns) {
		t.Errorf("got %d skill entities, want %d", len(skills), len(skillGoverns))
	}
}

func TestEverySkillGovernsADomain(t *testing.T) {
	o := Default()
	for _, s := range o.EntitiesOfKind(EntitySkill) {
		if len(o.Related(s, RelGoverns)) == 0 {
			t.Errorf("skill %q governs no domain", s)
		}
	}
}

// ── SkillsForDomain (the semantic replacement for heuristic routing) ───────

func TestSkillsForDomain_Finance(t *testing.T) {
	o := Default()
	skills := o.SkillsForDomain("domain:finance")
	if len(skills) == 0 {
		t.Fatal("finance domain has no governing skills")
	}
	want := map[string]bool{"credit-risk": true, "market-risk": true, "regulatory": true}
	found := map[string]bool{}
	for _, s := range skills {
		found[s] = true
	}
	for w := range want {
		if !found[w] {
			t.Errorf("expected %q to govern finance, not found in %v", w, skills)
		}
	}
}

func TestSkillsForDomain_CrossDomainSkill(t *testing.T) {
	o := Default()
	// security governs multiple domains, including backend
	backend := o.SkillsForDomain("domain:backend")
	var hasSecurity bool
	for _, s := range backend {
		if s == "security" {
			hasSecurity = true
		}
	}
	if !hasSecurity {
		t.Errorf("security should govern backend; backend skills: %v", backend)
	}
}

func TestSkillsForDomain_Unknown(t *testing.T) {
	o := Default()
	if got := o.SkillsForDomain("domain:nonexistent"); len(got) != 0 {
		t.Errorf("unknown domain should have no skills, got %v", got)
	}
}

// ── Phase transition validation ─────────────────────────────────────────────

func TestValidateTransition_Valid(t *testing.T) {
	o := Default()
	valid := [][2]string{
		{"idle", "discover"},
		{"discover", "plan"},
		{"plan", "execute"},
		{"execute", "verify"},
		{"verify", "persist"},
		{"verify", "execute"}, // rejected → retry
		{"persist", "done"},
		{"persist", "discover"}, // next iteration
	}
	for _, tr := range valid {
		if err := o.ValidateTransition(tr[0], tr[1]); err != nil {
			t.Errorf("transition %q→%q should be valid: %v", tr[0], tr[1], err)
		}
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	o := Default()
	invalid := [][2]string{
		{"idle", "execute"},    // skips discover/plan
		{"discover", "verify"}, // skips plan/execute
		{"execute", "persist"}, // skips verify (maker grading own work)
		{"done", "discover"},   // done is terminal
	}
	for _, tr := range invalid {
		if err := o.ValidateTransition(tr[0], tr[1]); err == nil {
			t.Errorf("transition %q→%q should be invalid", tr[0], tr[1])
		}
	}
}

func TestVerifyBeforePersistAxiom(t *testing.T) {
	// Build a broken ontology: persist reachable directly from execute,
	// with no verify→persist edge. The axiom must catch it.
	o := New()
	o.AddEntity(Entity{Kind: EntityPhase, Name: "execute"})
	o.AddEntity(Entity{Kind: EntityPhase, Name: "persist"})
	o.Relate(RelPrecedes, "execute", "persist")
	o.Axioms = defaultAxioms()

	v := o.Violations()
	var found bool
	for _, msg := range v {
		if strings.Contains(msg, "verify-before-persist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected verify-before-persist violation, got: %v", v)
	}
}

// ── Export ──────────────────────────────────────────────────────────────────

func TestExportContainsEntitiesAndRelations(t *testing.T) {
	o := Default()
	out := o.Export()
	for _, want := range []string{"# Harness Ontology", "## Entities", "## Relations", "skill", "governs"} {
		if !strings.Contains(out, want) {
			t.Errorf("Export missing %q", want)
		}
	}
}

func TestExportCompactIsTokenBudgeted(t *testing.T) {
	o := Default()
	out := o.ExportCompact()
	// Target ≤ ~300 tokens ≈ 1200 chars. Assert a generous ceiling.
	if len(out) > 1500 {
		t.Errorf("ExportCompact = %d chars, exceeds compact budget (~1500)", len(out))
	}
	for _, want := range []string{"HARNESS WORLD MODEL", "skill -governs-> domain", "phase -precedes-> phase", "axioms:"} {
		if !strings.Contains(out, want) {
			t.Errorf("ExportCompact missing %q", want)
		}
	}
}

func TestExportCompactListsAxioms(t *testing.T) {
	o := Default()
	out := o.ExportCompact()
	if !strings.Contains(out, "maker never grades own work") {
		t.Errorf("compact export should surface the verify-before-persist axiom description")
	}
}
