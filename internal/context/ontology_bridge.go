package context

import (
	"github.com/quant-risk/radiant-harness/internal/ontology"
)

// OntologySkillsForDomain returns the skills that govern the given domain
// according to the canonical harness ontology — the semantic source of truth.
// The heuristic domainSkillPriority list in registry.go is an *ordering* over
// this set; this function answers "which skills are even eligible" by relation
// rather than by hardcoded table.
func OntologySkillsForDomain(d Domain) []string {
	o := ontology.Default()
	return o.SkillsForDomain("domain:" + string(d))
}

// registryMatchesOntology reports skills whose registry domain mapping
// disagrees with the ontology. Used by tests to guarantee the two never drift.
// Returns nil when they agree.
func registryMatchesOntology() []string {
	o := ontology.Default()
	var mismatches []string

	for skill, domains := range skillDomains {
		for _, d := range domains {
			governed := o.Related(skill, ontology.RelGoverns)
			found := false
			for _, g := range governed {
				if g == "domain:"+string(d) {
					found = true
					break
				}
			}
			if !found {
				mismatches = append(mismatches, skill+"→"+string(d))
			}
		}
	}
	return mismatches
}
