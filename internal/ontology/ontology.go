// Package ontology is the harness world model: a formal, explicit graph of
// the entities the harness reasons about (Spec, Task, Gate, Skill, Domain,
// Phase, Verdict, Agent, Run, Artifact), the relations between them, and the
// axioms that constrain valid states.
//
// Why this exists: before this package, the harness's domain concepts were
// scattered across packages as independent Go structs (Task defined twice,
// Phase three times) with no shared semantic layer. Domain detection was
// heuristic classification, not ontological reasoning. This package gives the
// agent a single, queryable model of "how the project's concepts relate" —
// the world model that grounds skill selection, phase validation, and the
// exported boot manifest, instead of leaving each consumer to re-derive
// structure from filesystem guesses.
//
// The Go structs in other packages (internal/types.Task, internal/fleet.Task,
// internal/loop.Phase, …) remain the runtime *projections* of these entities;
// this package is the semantic *schema* they all answer to.
package ontology

import (
	"fmt"
	"sort"
	"strings"
)

// EntityKind is the type of a node in the harness ontology.
type EntityKind string

const (
	EntitySpec     EntityKind = "spec"
	EntityTask     EntityKind = "task"
	EntityGate     EntityKind = "gate"
	EntitySkill    EntityKind = "skill"
	EntityDomain   EntityKind = "domain"
	EntityPhase    EntityKind = "phase"
	EntityVerdict  EntityKind = "verdict"
	EntityAgent    EntityKind = "agent"
	EntityRun      EntityKind = "run"
	EntityArtifact EntityKind = "artifact"
)

// RelationKind is the type of a directed edge between entities.
type RelationKind string

const (
	RelContains   RelationKind = "contains"    // Spec   → Task
	RelVerifiedBy RelationKind = "verified_by" // Task   → Gate
	RelBelongsTo  RelationKind = "belongs_to"  // Gate   → Domain
	RelGoverns    RelationKind = "governs"     // Skill  → Domain
	RelProduces   RelationKind = "produces"    // Phase  → Artifact
	RelClaims     RelationKind = "claims"      // Agent  → Task
	RelTouches    RelationKind = "touches"     // Task   → Artifact
	RelExecutes   RelationKind = "executes"    // Run    → Phase
	RelPrecedes   RelationKind = "precedes"    // Phase  → Phase (loop order)
	RelEmits      RelationKind = "emits"       // Gate   → Verdict
)

// Entity is a typed node, identified by Name (unique within its Kind space,
// but Names are globally unique in a well-formed Ontology).
type Entity struct {
	Kind  EntityKind        `json:"kind"`
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Relation is a directed edge From → To of a given Kind.
type Relation struct {
	Kind RelationKind `json:"kind"`
	From string       `json:"from"`
	To   string       `json:"to"`
}

// Axiom is a named constraint over the graph. Check returns "" when the
// axiom holds, or a human-readable violation message when it does not.
type Axiom struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Check       func(*Ontology) string `json:"-"`
}

// Ontology is the harness world model: entities + relations + axioms.
type Ontology struct {
	Entities  map[string]Entity `json:"entities"`
	Relations []Relation        `json:"relations"`
	Axioms    []Axiom           `json:"-"`
}

// New returns an empty ontology ready to be populated.
func New() *Ontology {
	return &Ontology{
		Entities:  map[string]Entity{},
		Relations: nil,
		Axioms:    nil,
	}
}

// AddEntity inserts or overwrites an entity by name.
func (o *Ontology) AddEntity(e Entity) {
	o.Entities[e.Name] = e
}

// Relate adds a directed edge. It does not deduplicate; callers building from
// trusted schema data are expected to add each edge once.
func (o *Ontology) Relate(kind RelationKind, from, to string) {
	o.Relations = append(o.Relations, Relation{Kind: kind, From: from, To: to})
}

// Related returns the names of entities reachable from `name` by a single
// edge of the given relation kind (out-edges).
func (o *Ontology) Related(name string, kind RelationKind) []string {
	var out []string
	for _, r := range o.Relations {
		if r.From == name && r.Kind == kind {
			out = append(out, r.To)
		}
	}
	sort.Strings(out)
	return out
}

// RelatedInbound returns the names of entities that point at `name` via the
// given relation kind (in-edges).
func (o *Ontology) RelatedInbound(name string, kind RelationKind) []string {
	var out []string
	for _, r := range o.Relations {
		if r.To == name && r.Kind == kind {
			out = append(out, r.From)
		}
	}
	sort.Strings(out)
	return out
}

// EntitiesOfKind returns all entity names of a given kind, sorted.
func (o *Ontology) EntitiesOfKind(kind EntityKind) []string {
	var out []string
	for name, e := range o.Entities {
		if e.Kind == kind {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// SkillsForDomain returns the skills that govern the given domain — the
// semantic replacement for the heuristic skill-priority tables. A skill
// S governs domain D iff there is an edge S -governs-> D.
func (o *Ontology) SkillsForDomain(domain string) []string {
	return o.RelatedInbound(domain, RelGoverns)
}

// ValidateTransition reports whether moving from phase `from` to phase `to`
// is permitted by the loop's precedence relation. A transition is valid iff
// there is an edge from -precedes-> to. Self-loops (retries) and the terminal
// "done"/"failed" sinks are encoded as explicit edges in the default schema.
func (o *Ontology) ValidateTransition(from, to string) error {
	for _, next := range o.Related(from, RelPrecedes) {
		if next == to {
			return nil
		}
	}
	return fmt.Errorf("ontology: invalid phase transition %q → %q", from, to)
}

// Violations runs every axiom and returns the non-empty violation messages.
// An empty result means the ontology is internally consistent.
func (o *Ontology) Violations() []string {
	var out []string
	for _, a := range o.Axioms {
		if a.Check == nil {
			continue
		}
		if msg := a.Check(o); msg != "" {
			out = append(out, fmt.Sprintf("%s: %s", a.ID, msg))
		}
	}
	return out
}

// hasEntity is a small helper used by axioms.
func (o *Ontology) hasEntity(name string) bool {
	_, ok := o.Entities[name]
	return ok
}

// Export renders the full ontology as a human/agent-readable outline. This is
// the verbose form; see ExportCompact for the token-budgeted world model.
func (o *Ontology) Export() string {
	var sb strings.Builder
	sb.WriteString("# Harness Ontology\n\n")

	kinds := []EntityKind{
		EntitySpec, EntityTask, EntityGate, EntitySkill, EntityDomain,
		EntityPhase, EntityVerdict, EntityAgent, EntityRun, EntityArtifact,
	}
	sb.WriteString("## Entities\n")
	for _, k := range kinds {
		names := o.EntitiesOfKind(k)
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "- **%s** (%d): %s\n", k, len(names), strings.Join(names, ", "))
	}

	sb.WriteString("\n## Relations\n")
	byKind := map[RelationKind][]Relation{}
	var order []RelationKind
	for _, r := range o.Relations {
		if _, seen := byKind[r.Kind]; !seen {
			order = append(order, r.Kind)
		}
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}
	for _, k := range order {
		fmt.Fprintf(&sb, "- **%s** (%d edges)\n", k, len(byKind[k]))
	}

	return sb.String()
}

// ExportCompact renders the world model in a token-budgeted form suitable for
// injection into any LLM's context (target ≤ ~300 tokens). It states the
// entity kinds and the relation schema as compact triples, which is enough
// for an agent to reason about project structure without the full edge list.
func (o *Ontology) ExportCompact() string {
	var sb strings.Builder
	sb.WriteString("HARNESS WORLD MODEL\n")
	sb.WriteString("entities: ")
	var kinds []string
	seen := map[EntityKind]bool{}
	for _, e := range o.Entities {
		if !seen[e.Kind] {
			seen[e.Kind] = true
			kinds = append(kinds, string(e.Kind))
		}
	}
	sort.Strings(kinds)
	sb.WriteString(strings.Join(kinds, ", "))
	sb.WriteString("\nschema:\n")

	// One representative triple per relation kind, in declaration order.
	schema := []struct {
		rel      RelationKind
		from, to string
	}{
		{RelContains, "spec", "task"},
		{RelVerifiedBy, "task", "gate"},
		{RelBelongsTo, "gate", "domain"},
		{RelGoverns, "skill", "domain"},
		{RelProduces, "phase", "artifact"},
		{RelClaims, "agent", "task"},
		{RelTouches, "task", "artifact"},
		{RelExecutes, "run", "phase"},
		{RelEmits, "gate", "verdict"},
		{RelPrecedes, "phase", "phase"},
	}
	for _, s := range schema {
		fmt.Fprintf(&sb, "  %s -%s-> %s\n", s.from, s.rel, s.to)
	}
	sb.WriteString("axioms:\n")
	for _, a := range o.Axioms {
		fmt.Fprintf(&sb, "  - %s\n", a.Description)
	}
	return sb.String()
}
