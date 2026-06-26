package context

// skillDomains maps every bundled skill name to its primary domain(s).
// This is the canonical routing table — the assembler uses it to pick
// which skills to include in CONTEXT.md for a given domain.
//
// Maintained here (not in frontmatter) so the context package stays
// independent of the skill package's embed FS.
var skillDomains = map[string][]Domain{
	// Core SDD — always available regardless of domain
	"nova-feature":    {DomainGeneral},
	"nova-product":    {DomainGeneral},
	"kickoff":         {DomainGeneral},
	"clarificar":      {DomainGeneral},
	"validar":         {DomainGeneral},
	"auditar":         {DomainGeneral},
	"metricas":        {DomainGeneral},
	"evals":           {DomainGeneral},
	"revisar-pr":      {DomainGeneral},
	"adr":             {DomainGeneral},
	"diagramar":       {DomainGeneral},
	"mapear":          {DomainGeneral},
	"camada-agentica": {DomainGeneral},
	"integracoes":     {DomainGeneral},
	"setup-ci":        {DomainGeneral},
	"update":          {DomainGeneral},
	"handoff":         {DomainGeneral},
	"roadmap":         {DomainGeneral},
	"security":        {DomainGeneral},
	"incident":        {DomainGeneral},

	// Finance
	"credit-risk":        {DomainFinance},
	"credit-portfolio":   {DomainFinance},
	"market-risk":        {DomainFinance},
	"liquidity-risk":     {DomainFinance},
	"operational-risk":   {DomainFinance},
	"model-risk":         {DomainFinance},
	"valuation":          {DomainFinance},
	"capital-markets":    {DomainFinance},
	"controlling":        {DomainFinance},
	"accounting":         {DomainFinance},
	"fraud-detection":    {DomainFinance},
	"stress-test":        {DomainFinance},
	"regulatory":         {DomainFinance},
	"tax":                {DomainFinance},
	"aml-kyc":            {DomainFinance},
	"actuarial":          {DomainFinance},
	"actuarial-solvency": {DomainFinance},

	// ML / Data Science
	"ml":                     {DomainML},
	"deep-learning":          {DomainML},
	"reinforcement-learning": {DomainML},
	"causal-ml":              {DomainML},
	"causal":                 {DomainML},
	"econometrics":           {DomainML},
	"stats":                  {DomainML},
	"bayesian":               {DomainML},
	"data":                   {DomainML},
	"synthetic-data":         {DomainML},
	"quantum-ml":             {DomainML, DomainScience},

	// Tech / General Engineering
	"api":      {DomainBackend, DomainFrontend},
	"cli":      {DomainBackend, DomainSystems},
	"frontend": {DomainFrontend},
	"mobile":   {DomainFrontend},

	// Ops
	"blockchain": {DomainBlockchain},
	"iot":        {DomainSystems},
	"game":       {DomainFrontend},

	// Science
	"physics":         {DomainScience},
	"chemistry":       {DomainScience},
	"biology":         {DomainScience},
	"quantum-physics": {DomainScience},
}

// coreSkills are always recommended regardless of domain — they form
// the minimal viable context for any project.
var coreSkills = []string{
	"nova-feature",
	"validar",
	"adr",
}

// domainSkillPriority defines the preferred skill order for each domain.
// First N skills up to maxDomainSkills are included in CONTEXT.md.
var domainSkillPriority = map[Domain][]string{
	DomainFinance: {
		"credit-risk", "market-risk", "liquidity-risk", "operational-risk",
		"regulatory", "model-risk", "fraud-detection", "aml-kyc",
		"stress-test", "actuarial", "valuation", "capital-markets",
	},
	DomainML: {
		"ml", "deep-learning", "stats", "data", "bayesian",
		"causal-ml", "reinforcement-learning", "synthetic-data",
	},
	DomainFrontend: {
		"frontend", "api", "mobile",
	},
	DomainBackend: {
		"api", "cli", "security",
	},
	DomainOps: {
		"setup-ci", "security", "incident",
	},
	DomainBlockchain: {
		"blockchain", "api", "security",
	},
	DomainSystems: {
		"cli", "security", "api",
	},
	DomainScience: {
		"physics", "chemistry", "biology", "stats", "bayesian",
	},
	DomainGeneral: {
		"api", "cli", "security",
	},
}

// maxDomainSkills is the maximum number of domain-specific skills to
// include alongside the core skills.
const maxDomainSkills = 4

// maxTotalSkills is the hard cap on total recommended skills.
const maxTotalSkills = 10

// recommendSkills returns an ordered list of 3–7 skill names for the
// given domain and tier. Core skills always come first.
func recommendSkills(domain Domain, tier Tier) []string {
	seen := map[string]bool{}
	var result []string

	// Always include core skills
	for _, s := range coreSkills {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	// Add tier-specific skills
	switch tier {
	case TierProduct:
		for _, s := range []string{"nova-product", "kickoff", "mapear", "roadmap"} {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	case TierArchitecture:
		for _, s := range []string{"diagramar", "mapear"} {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}

	// Add domain-specific skills
	priority := domainSkillPriority[domain]
	added := 0
	for _, s := range priority {
		if added >= maxDomainSkills || len(result) >= maxTotalSkills {
			break
		}
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
			added++
		}
	}

	// Hard cap
	if len(result) > maxTotalSkills {
		result = result[:maxTotalSkills]
	}

	return result
}
