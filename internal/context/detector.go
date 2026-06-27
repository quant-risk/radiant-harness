// Package context implements the Context Engine — the layer that detects a
// project's domain, assembles a minimal CONTEXT.md with only relevant skills,
// and compresses context when token budgets are tight.
//
// Design principle: load the minimum needed. The engine reads only
// frontmatter metadata (not SKILL.md bodies) and recommends 3–7 skills
// instead of all 60. This keeps the agent's context window lean.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Domain is the detected primary domain of a project.
type Domain string

const (
	DomainFinance    Domain = "finance"
	DomainML         Domain = "ml"
	DomainFrontend   Domain = "frontend"
	DomainBackend    Domain = "backend"
	DomainOps        Domain = "ops"
	DomainBlockchain Domain = "blockchain"
	DomainSystems    Domain = "systems"
	DomainScience    Domain = "science"
	DomainGeneral    Domain = "general"
)

// Tier is the detected complexity tier of the current goal.
type Tier string

const (
	TierTrivial      Tier = "trivial"
	TierFeature      Tier = "feature"
	TierArchitecture Tier = "architecture"
	TierProduct      Tier = "product"
)

// DetectionResult holds the output of project analysis.
type DetectionResult struct {
	Domain            Domain   `json:"domain"`
	Tier              Tier     `json:"tier"`
	Signals           []string `json:"signals"`            // files/dirs that triggered detection
	RecommendedSkills []string `json:"recommended_skills"` // 3–7 skill names
	ActiveSpec        string   `json:"active_spec"`        // path to active spec, if any
	ProjectName       string   `json:"project_name"`       // inferred from dir name or go.mod
}

// signal is a filesystem marker with its detection weight.
type signal struct {
	path   string // relative path to check (file or dir)
	isDir  bool
	domain Domain
	weight int
}

// signals is the ordered list of detection heuristics.
// Higher weight wins when multiple domains match.
var signals = []signal{
	// Finance — highest specificity signals first
	{path: "go.mod", domain: DomainFinance, weight: 0}, // only combined with imports
	{path: "requirements.txt", domain: DomainML, weight: 0},
	{path: "pyproject.toml", domain: DomainML, weight: 0},
	{path: "package.json", domain: DomainFrontend, weight: 8},
	{path: "pom.xml", domain: DomainBackend, weight: 10},
	{path: "build.gradle", domain: DomainBackend, weight: 10},
	{path: "Cargo.toml", domain: DomainSystems, weight: 15},
	{path: "k8s", isDir: true, domain: DomainOps, weight: 12},
	{path: "helm", isDir: true, domain: DomainOps, weight: 12},
	{path: ".terraform", isDir: true, domain: DomainOps, weight: 12},
	{path: "terraform", isDir: true, domain: DomainOps, weight: 10},
	// Blockchain — .sol files are a high-specificity signal
	{path: "hardhat.config.js", domain: DomainBlockchain, weight: 20},
	{path: "hardhat.config.ts", domain: DomainBlockchain, weight: 20},
	{path: "truffle-config.js", domain: DomainBlockchain, weight: 20},
	{path: "foundry.toml", domain: DomainBlockchain, weight: 20},
	{path: "contracts", isDir: true, domain: DomainBlockchain, weight: 12},
}

// domainImportPatterns maps substrings found in source files to domains.
// Checked in Go (.go), Python (.py), and JS/TS (.js/.ts) files.
var domainImportPatterns = map[Domain][]string{
	DomainFinance: {
		"risk", "quant", "ifrs", "basel", "credit", "portfolio", "trading",
		"valuation", "derivative", "actuarial", "solvency", "regulatory",
		"aml", "kyc", "fraud", "capital", "provisioning",
	},
	DomainML: {
		"torch", "tensorflow", "sklearn", "scikit", "keras", "xgboost",
		"lightgbm", "catboost", "transformers", "huggingface", "jax",
		"numpy", "pandas", "scipy", "statsmodels",
	},
	DomainBlockchain: {
		"solidity", "web3", "ethers", "hardhat", "truffle", "anchor",
		"solana", "ethereum", "contract", "evm",
	},
	DomainScience: {
		"matplotlib", "seaborn", "sympy", "networkx", "biopython",
		"rdkit", "qiskit", "pennylane",
	},
}

// Detect analyzes a project directory and returns a DetectionResult.
// It reads only directory listings and a small sample of source files —
// never full file contents — keeping detection fast and token-free.
func Detect(projectDir string) (*DetectionResult, error) {
	r := &DetectionResult{
		Domain: DomainGeneral,
		Tier:   TierFeature,
	}

	r.ProjectName = inferProjectName(projectDir)

	// Phase 1: filesystem signals
	domainScores := map[Domain]int{}
	for _, sig := range signals {
		p := filepath.Join(projectDir, sig.path)
		var hit bool
		if sig.isDir {
			info, err := os.Stat(p)
			hit = err == nil && info.IsDir()
		} else {
			_, err := os.Stat(p)
			hit = err == nil
		}
		if hit {
			r.Signals = append(r.Signals, sig.path)
			domainScores[sig.domain] += sig.weight
		}
	}

	// Phase 2: import-pattern scan (reads first 50 lines of select files)
	importScores, importSignals := scanImports(projectDir)
	for d, score := range importScores {
		domainScores[d] += score
	}
	r.Signals = append(r.Signals, importSignals...)

	// Phase 2b: module path scan — go.mod module line encodes project identity
	modScores, modSignals := scanModulePath(projectDir)
	for d, score := range modScores {
		domainScores[d] += score
	}
	r.Signals = append(r.Signals, modSignals...)

	// Phase 2c: docs scan — README.md / CLAUDE.md carry explicit domain intent
	docScores, docSignals := scanDocs(projectDir)
	for d, score := range docScores {
		domainScores[d] += score
	}
	r.Signals = append(r.Signals, docSignals...)

	// Phase 2d: directory name scan — top-level and internal/ dirs name the domain
	dirScores, dirSignals := scanDirNames(projectDir)
	for d, score := range dirScores {
		domainScores[d] += score
	}
	r.Signals = append(r.Signals, dirSignals...)

	// Phase 3: directory structure hints
	tierSignals := detectTier(projectDir)
	r.Signals = append(r.Signals, tierSignals.signals...)
	r.Tier = tierSignals.tier

	// Phase 4: pick winning domain
	bestScore := 0
	for d, score := range domainScores {
		if score > bestScore {
			bestScore = score
			r.Domain = d
		}
	}

	// Phase 5: active spec detection
	r.ActiveSpec = detectActiveSpec(projectDir)

	// Phase 6: recommend skills based on domain + tier
	r.RecommendedSkills = recommendSkills(r.Domain, r.Tier)

	return r, nil
}

// inferProjectName tries go.mod module path, then directory basename.
func inferProjectName(projectDir string) string {
	gomod := filepath.Join(projectDir, "go.mod")
	if data, err := os.ReadFile(gomod); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "module ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					segs := strings.Split(parts[1], "/")
					return segs[len(segs)-1]
				}
			}
		}
	}
	return filepath.Base(projectDir)
}

// tierResult combines detected tier with the signals that triggered it.
type tierResult struct {
	tier    Tier
	signals []string
}

// detectTier infers complexity tier from directory/file structure.
func detectTier(projectDir string) tierResult {
	var sigs []string
	score := 0

	checks := []struct {
		path  string
		isDir bool
		pts   int
		sig   string
	}{
		{"docs/product", true, 3, "docs/product/"},
		{"docs/architecture", true, 2, "docs/architecture/"},
		{"specs", true, 1, "specs/"},
		{"docs/adr", true, 2, "docs/adr/"},
		{".radiant-harness/state.md", false, 1, "state.md"},
	}

	for _, c := range checks {
		p := filepath.Join(projectDir, c.path)
		var hit bool
		if c.isDir {
			info, err := os.Stat(p)
			hit = err == nil && info.IsDir()
		} else {
			_, err := os.Stat(p)
			hit = err == nil
		}
		if hit {
			score += c.pts
			sigs = append(sigs, c.sig)
		}
	}

	// count existing specs
	specsDir := filepath.Join(projectDir, "specs")
	if entries, err := os.ReadDir(specsDir); err == nil {
		n := 0
		for _, e := range entries {
			if e.IsDir() {
				n++
			}
		}
		if n >= 3 {
			score += 2
		}
	}

	tier := TierFeature
	switch {
	case score >= 5:
		tier = TierProduct
	case score >= 3:
		tier = TierArchitecture
	case score <= 0:
		tier = TierTrivial
	}

	return tierResult{tier: tier, signals: sigs}
}

// scanImports samples source files in the project for domain-specific imports.
// Reads at most 50 lines per file, scans at most 20 files total.
func scanImports(projectDir string) (map[Domain]int, []string) {
	scores := map[Domain]int{}
	var signals []string

	// .sol files are a strong blockchain signal on their own
	solCount := 0
	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".sol" {
			solCount++
		}
		return nil
	})
	if solCount > 0 {
		scores[DomainBlockchain] += 15
		signals = append(signals, fmt.Sprintf("%d .sol files", solCount))
	}

	extensions := []string{".go", ".py", ".js", ".ts"}
	scanned := 0
	maxFiles := 20

	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || scanned >= maxFiles {
			return nil
		}
		// skip vendor, node_modules, .git
		rel, _ := filepath.Rel(projectDir, path)
		for _, skip := range []string{"vendor", "node_modules", ".git", "dist", "build"} {
			if strings.HasPrefix(rel, skip+string(os.PathSeparator)) || strings.HasPrefix(rel, skip+"/") {
				return nil
			}
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, wanted := range extensions {
			if ext == wanted {
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				scanned++
				// Sample first 50 lines only
				lines := strings.SplitN(string(data), "\n", 51)
				sample := strings.Join(lines, "\n")
				lower := strings.ToLower(sample)

				for domain, patterns := range domainImportPatterns {
					for _, pat := range patterns {
						if strings.Contains(lower, pat) {
							scores[domain] += 5
							sig := "import:" + pat
							signals = appendUnique(signals, sig)
							break // one match per domain per file is enough
						}
					}
				}
				break
			}
		}
		return nil
	})

	return scores, signals
}

// detectActiveSpec looks for a spec directory referenced in state.md or
// the most-recently-modified spec directory.
// domainKeywordPatterns maps substrings to domains for use in free-text sources
// (module paths, README, directory names). These are higher-specificity than
// import patterns because free text is less noisy.
var domainKeywordPatterns = map[Domain][]string{
	DomainFinance: {
		"quant", "trading", "fintech", "finance", "financial",
		"credit", "portfolio", "risk-mgmt", "hedging", "actuarial",
		"aml", "kyc", "compliance", "capital-markets", "settlement",
		"clearing", "custody", "derivatives", "fixed-income", "equities",
	},
	DomainML: {
		"ml", "machine-learning", "deep-learning", "nlp", "computer-vision",
		"ai", "inference", "training", "model-serving", "feature-store",
		"recommendation", "ranking", "classification", "regression",
	},
	DomainBlockchain: {
		"blockchain", "defi", "nft", "smart-contract", "solidity", "web3",
		"ethereum", "solana", "evm", "protocol",
	},
	DomainFrontend: {
		"frontend", "ui", "ux", "design-system", "webapp", "spa",
		"dashboard", "component-library",
	},
	DomainBackend: {
		"backend", "api", "microservice", "grpc", "graphql", "rest-api",
		"server", "service-mesh",
	},
	DomainOps: {
		"devops", "infra", "infrastructure", "sre", "observability",
		"monitoring", "deployment", "ci-cd", "gitops", "ops-platform",
	},
	DomainSystems: {
		"systems", "kernel", "embedded", "firmware", "driver", "runtime",
		"allocator", "scheduler", "wasm",
	},
	DomainScience: {
		"bioinformatics", "genomics", "simulation", "physics", "chemistry",
		"quantum", "scientific",
	},
}

// scanModulePath reads the go.mod module declaration and scores domains
// based on keywords in the module path. Module paths encode project intent
// explicitly (e.g. "github.com/quant-risk/..." → finance score 20).
func scanModulePath(projectDir string) (map[Domain]int, []string) {
	scores := map[Domain]int{}
	var sigs []string

	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return scores, sigs
	}

	var modulePath string
	for _, line := range strings.SplitN(string(data), "\n", 5) {
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				modulePath = strings.ToLower(parts[1])
			}
			break
		}
	}
	if modulePath == "" {
		return scores, sigs
	}

	for domain, keywords := range domainKeywordPatterns {
		for _, kw := range keywords {
			if strings.Contains(modulePath, kw) {
				scores[domain] += 20
				sigs = appendUnique(sigs, "module:"+kw)
				break
			}
		}
	}
	return scores, sigs
}

// scanDocs reads README.md and CLAUDE.md (up to 200 lines each) for
// domain keywords. Documentation is the highest-signal free-text source.
func scanDocs(projectDir string) (map[Domain]int, []string) {
	scores := map[Domain]int{}
	var sigs []string

	candidates := []string{"README.md", "CLAUDE.md", "docs/README.md", "README.rst"}
	for _, rel := range candidates {
		data, err := os.ReadFile(filepath.Join(projectDir, rel))
		if err != nil {
			continue
		}
		lines := strings.SplitN(string(data), "\n", 201)
		content := strings.ToLower(strings.Join(lines, " "))

		for domain, keywords := range domainKeywordPatterns {
			for _, kw := range keywords {
				if strings.Contains(content, kw) {
					scores[domain] += 8
					sigs = appendUnique(sigs, "docs:"+kw)
					break // one match per domain per file
				}
			}
		}
	}
	return scores, sigs
}

// scanDirNames checks top-level and internal/ directory names for domain
// keywords. Directory names are strong structural signals (e.g. internal/trading/).
func scanDirNames(projectDir string) (map[Domain]int, []string) {
	scores := map[Domain]int{}
	var sigs []string

	scanLevel := func(dir string, weight int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			for domain, keywords := range domainKeywordPatterns {
				for _, kw := range keywords {
					// Match full dir name or dir containing keyword as segment
					if name == kw || strings.Contains(name, kw) {
						scores[domain] += weight
						rel, _ := filepath.Rel(projectDir, filepath.Join(dir, e.Name()))
						sigs = appendUnique(sigs, "dir:"+rel)
						break
					}
				}
			}
		}
	}

	// Top-level dirs (high weight — explicit project structure)
	scanLevel(projectDir, 12)
	// internal/ subdirs (medium weight)
	scanLevel(filepath.Join(projectDir, "internal"), 8)
	// cmd/ subdirs (medium weight)
	scanLevel(filepath.Join(projectDir, "cmd"), 8)

	return scores, sigs
}

func detectActiveSpec(projectDir string) string {
	// Check state.md for active spec reference
	stateFile := filepath.Join(projectDir, ".radiant-harness", "state.md")
	if data, err := os.ReadFile(stateFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "specs/") {
				// Extract specs/NNNN-slug pattern
				for _, f := range strings.Fields(line) {
					if strings.HasPrefix(f, "specs/") {
						return f
					}
				}
			}
		}
	}

	// Fall back to most-recently-modified spec dir
	specsDir := filepath.Join(projectDir, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var latest os.FileInfo
	var latestName string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if latest == nil || info.ModTime().After(latest.ModTime()) {
			latest = info
			latestName = e.Name()
		}
	}
	if latestName != "" {
		return "specs/" + latestName
	}
	return ""
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
