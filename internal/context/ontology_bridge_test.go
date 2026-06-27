package context

import (
	"strings"
	"testing"
)

// TestRegistryMatchesOntology is the anti-drift guarantee: every (skill,
// domain) edge declared in registry.go's skillDomains must also exist in the
// canonical ontology. If someone adds a skill to one but not the other, this
// fails — the ontology stays the single source of truth.
func TestRegistryMatchesOntology(t *testing.T) {
	if mismatches := registryMatchesOntology(); len(mismatches) != 0 {
		t.Fatalf("registry/ontology drift on %d edges:\n  %s",
			len(mismatches), strings.Join(mismatches, "\n  "))
	}
}

func TestOntologySkillsForDomain_Finance(t *testing.T) {
	skills := OntologySkillsForDomain(DomainFinance)
	if len(skills) == 0 {
		t.Fatal("ontology returned no skills for finance")
	}
	var hasCreditRisk bool
	for _, s := range skills {
		if s == "credit-risk" {
			hasCreditRisk = true
		}
	}
	if !hasCreditRisk {
		t.Errorf("expected credit-risk to govern finance, got %v", skills)
	}
}

func TestOntologySkillsForDomain_General(t *testing.T) {
	skills := OntologySkillsForDomain(DomainGeneral)
	if len(skills) == 0 {
		t.Fatal("ontology returned no skills for general domain")
	}
}
