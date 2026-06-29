// Package main — diagram.go: C4-level Mermaid diagram templates.
//
// Used by `radiant views --diagram=context|container|component|code`.
// Each function returns a self-contained Markdown file with a fenced
// ```mermaid block. Operators edit the placeholder tokens (e.g. <API>)
// to match their project; the rendered diagram updates live in any
// Markdown viewer that supports Mermaid (GitHub, GitLab, VS Code
// with the Mermaid extension).
package main

import "fmt"

func renderDiagram(level string) (string, error) {
	switch level {
	case "context":
		return contextDiagram(), nil
	case "container":
		return containerDiagram(), nil
	case "component":
		return componentDiagram(), nil
	case "code":
		return codeDiagram(), nil
	default:
		return "", fmt.Errorf("unknown level %q — choose: context | container | component | code", level)
	}
}

func contextDiagram() string {
	return `# C4 Level 1 — System Context

High-level view: what is the system, who uses it, and what does it talk to.

` + "```mermaid" + `
C4Context
    title System Context diagram for <System>

    Person(user, "User", "A person who uses the system")
    System(system, "<System>", "The system we're documenting")
    System_Ext(externalA, "External System A", "Does X for us")
    System_Ext(externalB, "External System B", "Provides data")

    Rel(user, system, "Uses")
    Rel(system, externalA, "Sends events to")
    Rel(system, externalB, "Reads from")
` + "```" + `
`
}

func containerDiagram() string {
	return `# C4 Level 2 — Containers

Zoom into the system: what containers (apps, services, stores) make it up.

` + "```mermaid" + `
C4Container
    title Container diagram for <System>

    Person(user, "User", "A person who uses the system")

    System_Boundary(system, "<System>") {
        Container(web, "Web App", "React", "Browser SPA")
        Container(api, "API", "Go", "HTTP API")
        ContainerDb(db, "Database", "Postgres", "Stores domain data")
        ContainerQueue(queue, "Queue", "Redis", "Async work")
    }

    Rel(user, web, "Uses", "HTTPS")
    Rel(web, api, "Calls", "JSON/HTTPS")
    Rel(api, db, "Reads/writes", "SQL")
    Rel(api, queue, "Publishes events")
` + "```" + `
`
}

func componentDiagram() string {
	return `# C4 Level 3 — Components

Zoom into ONE container (the API in this example) and show its
internal building blocks. See the diagramar skill.

` + "```mermaid" + `
C4Component
    title Component diagram for <API>

    Container(web, "Web App", "React", "Browser UI")
    ContainerDb(database, "Database", "Postgres", "Stores data")

    Container_Boundary(api, "<API>") {
        Component(handler, "Handler", "HTTP layer", "Translates requests to commands")
        Component(svc, "Service", "Business logic", "Enforces invariants")
        Component(repo, "Repository", "Data access", "Owns SQL queries")
    }

    Rel(web, handler, "Calls", "JSON")
    Rel(handler, svc, "Invokes")
    Rel(svc, repo, "Uses")
    Rel(repo, database, "Reads/writes", "SQL")
` + "```" + `
`
}

func codeDiagram() string {
	return `# C4 Level 4 — Code

UML-style class diagram for a focused unit. The diagramar skill
recommends staying at Level 3 unless a specific class has
complex internal relationships worth visualising.

` + "```mermaid" + `
classDiagram
    class Service {
        +repo Repository
        +logger Logger
        +DoThing(input Input) (Output, error)
    }
    class Repository {
        <<interface>>
        +Get(id string) (Entity, error)
        +Put(entity Entity) error
    }
    Service --> Repository : depends on
`
}