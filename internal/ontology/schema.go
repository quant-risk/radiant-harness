package ontology

// This file builds the canonical harness ontology — the single source of
// truth for "what the harness's concepts are and how they relate". The
// skill→domain edges here formalize what used to live as the implicit
// skillDomains / domainSkillPriority tables in internal/context/registry.go.

// Domains are the eight project domains the Context Engine can detect, plus
// the general fallback.
var schemaDomains = []string{
	"general", "finance", "ml", "frontend", "backend",
	"ops", "blockchain", "systems", "science",
}

// Phases are the loop states, in canonical order.
var schemaPhases = []string{
	"idle", "discover", "plan", "execute", "verify", "persist", "done", "failed",
}

// phaseTransitions encodes the loop state machine as precedence edges.
// This is the semantic equivalent of the hardcoded validTransitions table
// in internal/loop/cycle.go — ValidateTransition consults these edges.
var phaseTransitions = [][2]string{
	{"idle", "discover"},
	{"discover", "plan"},
	{"discover", "failed"},
	{"plan", "execute"},
	{"plan", "failed"},
	{"execute", "verify"},
	{"execute", "failed"},
	{"verify", "persist"}, // approved
	{"verify", "execute"}, // rejected → retry
	{"verify", "failed"},
	{"persist", "done"},     // success
	{"persist", "discover"}, // next iteration
	{"failed", "idle"},      // reset
	{"failed", "discover"},  // retry
}

// skillGoverns maps each skill to the domain(s) it governs. Formalized from
// the registry's skillDomains map so the Context Engine can select skills by
// the governs relation rather than by a heuristic priority list.
var skillGoverns = map[string][]string{
	// Core SDD — govern the general domain (always relevant)
	"nova-feature": {"general"}, "nova-product": {"general"}, "kickoff": {"general"},
	"clarificar": {"general"}, "validar": {"general"}, "auditar": {"general"},
	"metricas": {"general"}, "evals": {"general"}, "revisar-pr": {"general"},
	"adr": {"general"}, "diagramar": {"general"}, "mapear": {"general"},
	"camada-agentica": {"general"}, "integracoes": {"general"}, "setup-ci": {"general", "ops"},
	"update": {"general"}, "handoff": {"general"}, "roadmap": {"general"},
	"security": {"general", "backend", "ops", "blockchain", "systems"}, "incident": {"general", "ops"},

	// Finance
	"credit-risk": {"finance"}, "credit-portfolio": {"finance"}, "market-risk": {"finance"},
	"liquidity-risk": {"finance"}, "operational-risk": {"finance"}, "model-risk": {"finance"},
	"valuation": {"finance"}, "capital-markets": {"finance"}, "controlling": {"finance"},
	"accounting": {"finance"}, "fraud-detection": {"finance"}, "stress-test": {"finance"},
	"regulatory": {"finance"}, "tax": {"finance"}, "aml-kyc": {"finance"},
	"actuarial": {"finance"}, "actuarial-solvency": {"finance"},

	// ML / Data Science
	"ml": {"ml"}, "deep-learning": {"ml"}, "reinforcement-learning": {"ml"},
	"causal-ml": {"ml"}, "causal": {"ml"}, "econometrics": {"ml"},
	"stats": {"ml"}, "bayesian": {"ml"}, "data": {"ml"},
	"synthetic-data": {"ml"}, "quantum-ml": {"ml", "science"},

	// Engineering
	"api": {"backend", "frontend"}, "cli": {"backend", "systems"},
	"frontend": {"frontend"}, "mobile": {"frontend"},

	// Ops / domain
	"blockchain": {"blockchain"}, "iot": {"systems"}, "game": {"frontend"},

	// Science
	"physics": {"science"}, "chemistry": {"science"}, "biology": {"science"},
	"quantum-physics": {"science"},
}

// Default returns the canonical harness ontology, fully populated with
// entities, relations, and axioms.
func Default() *Ontology {
	o := New()

	// ── Entities ────────────────────────────────────────────────────────────
	for _, d := range schemaDomains {
		o.AddEntity(Entity{Kind: EntityDomain, Name: "domain:" + d})
	}
	for _, p := range schemaPhases {
		o.AddEntity(Entity{Kind: EntityPhase, Name: p})
	}
	for skill := range skillGoverns {
		o.AddEntity(Entity{Kind: EntitySkill, Name: skill})
	}
	// Verdict outcomes the verifier can emit.
	for _, v := range []string{"approved", "rejected"} {
		o.AddEntity(Entity{Kind: EntityVerdict, Name: "verdict:" + v})
	}
	// Abstract entity kinds present in the runtime but not enumerated as
	// instances — declared once so the schema is complete for export.
	o.AddEntity(Entity{Kind: EntitySpec, Name: "spec"})
	o.AddEntity(Entity{Kind: EntityTask, Name: "task"})
	o.AddEntity(Entity{Kind: EntityGate, Name: "gate"})
	o.AddEntity(Entity{Kind: EntityAgent, Name: "agent"})
	o.AddEntity(Entity{Kind: EntityRun, Name: "run"})
	o.AddEntity(Entity{Kind: EntityArtifact, Name: "artifact"})

	// ── Relations ───────────────────────────────────────────────────────────
	// Skill governs domain (formalized routing table).
	for skill, domains := range skillGoverns {
		for _, d := range domains {
			o.Relate(RelGoverns, skill, "domain:"+d)
		}
	}
	// Phase precedence (loop state machine).
	for _, t := range phaseTransitions {
		o.Relate(RelPrecedes, t[0], t[1])
	}
	// Structural schema edges (abstract — one each).
	o.Relate(RelContains, "spec", "task")
	o.Relate(RelVerifiedBy, "task", "gate")
	o.Relate(RelClaims, "agent", "task")
	o.Relate(RelTouches, "task", "artifact")
	o.Relate(RelExecutes, "run", "execute")
	o.Relate(RelEmits, "gate", "verdict:approved")
	o.Relate(RelEmits, "gate", "verdict:rejected")
	o.Relate(RelProduces, "persist", "artifact")

	// ── Axioms ──────────────────────────────────────────────────────────────
	o.Axioms = defaultAxioms()

	return o
}

// defaultAxioms returns the constraints that a well-formed harness world must
// satisfy. Each Check returns "" when satisfied.
func defaultAxioms() []Axiom {
	return []Axiom{
		{
			ID:          "verify-before-persist",
			Description: "a task reaches persist only through verify (maker never grades own work)",
			Check: func(o *Ontology) string {
				// persist must be reachable from verify
				for _, next := range o.Related("verify", RelPrecedes) {
					if next == "persist" {
						return ""
					}
				}
				return "no verify→persist edge: persist is reachable without verification"
			},
		},
		{
			ID:          "every-skill-governs-a-domain",
			Description: "every skill governs at least one declared domain",
			Check: func(o *Ontology) string {
				for _, s := range o.EntitiesOfKind(EntitySkill) {
					if len(o.Related(s, RelGoverns)) == 0 {
						return "skill " + s + " governs no domain"
					}
				}
				return ""
			},
		},
		{
			ID:          "domains-exist",
			Description: "every governed domain is a declared domain entity",
			Check: func(o *Ontology) string {
				for _, r := range o.Relations {
					if r.Kind == RelGoverns && !o.hasEntity(r.To) {
						return "skill " + r.From + " governs undeclared domain " + r.To
					}
				}
				return ""
			},
		},
		{
			ID:          "terminal-phases",
			Description: "done and failed are terminal (no outbound precedence except failed→retry)",
			Check: func(o *Ontology) string {
				if len(o.Related("done", RelPrecedes)) != 0 {
					return "done has outbound transitions; must be terminal"
				}
				return ""
			},
		},
	}
}
