package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
	"github.com/quant-risk/radiant-harness/v3/internal/config"
	"github.com/quant-risk/radiant-harness/v3/internal/engine"
	"github.com/quant-risk/radiant-harness/v3/internal/llm"
	"github.com/quant-risk/radiant-harness/v3/internal/loop"
	"github.com/quant-risk/radiant-harness/v3/internal/scaffold"
	"github.com/quant-risk/radiant-harness/v3/internal/skill"
)

func init() {
	// Light build: every LLM call goes through MCP sampling to the host
	// agent. Wrap the sampling backend in a factory the loop engine uses
	// to build per-phase clients (planner, implementer, validator).
	loop.SetHTTPBackendBuilder(func(m llm.Model) llm.Backend {
		c := llm.NewClient(m)
		c.SetWriter(os.Stdout)
		return c
	})
}

func resolveAgents(flag string, yes bool) []radiant.AgentID {
	if flag == "all" {
		return radiant.AllAgents()
	}
	if flag == "" {
		if yes {
			// --yes without --agent means "do the default thing": generate
			// for every supported agent. The user opted into bulk.
			return radiant.AllAgents()
		}
		// No flag, no --yes: refuse to guess. The operator must declare
		// which agent(s) they want — the harness is vendor-neutral and
		// doesn't privilege any particular CLI.
		return nil
	}
	var agents []radiant.AgentID
	for _, s := range strings.Split(flag, ",") {
		s = strings.TrimSpace(s)
		if radiant.IsValidAgent(s) {
			agents = append(agents, radiant.AgentID(s))
		}
	}
	return agents
}

func agentLabels(agents []radiant.AgentID) string {
	var labels []string
	for _, a := range agents {
		adapter := scaffold.GetAdapter(a)
		if adapter != nil {
			labels = append(labels, adapter.Label)
		} else {
			labels = append(labels, string(a))
		}
	}
	return strings.Join(labels, ", ")
}

// resolveModelSilent is a swallow-error variant of resolveModel used by
// the multi-agent --planner / --implementer flags. The reasoning: if a
// user explicitly types a model name that doesn't resolve, we'd rather
// emit a clear runtime warning and fall back to the default model than
// abort the entire run. The error is printed so the user can fix it.
// writeTraceToFile opens path (creating parent dirs as needed) and
// drains the engine's trace log to it as JSONL. Atomic on POSIX via
// temp + rename, so a crash mid-write leaves no torn file. On Windows
// strOrEmpty renders a string or "(none)" if empty. Used in state.md
// so a missing field is visually obvious to whoever reads it.
func strOrEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// slugify derives a kebab-case slug from free-form intent text. Best
// effort — falls back to lowercased ASCII with non-alphanumerics
// replaced by `-` and runs of `-` collapsed. Truncated to 48 chars.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 48 {
		out = out[:48]
		out = strings.TrimRight(out, "-")
	}
	return out
}

// nextSpecSeq scans `specs/` for the highest NNNN- prefix and
// returns next+1. Returns 1 if no specs exist or the directory
// is empty.
func nextSpecSeq(dir string) (int, error) {
	max := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[4] != '-' {
			continue
		}
		n, err := strconv.Atoi(name[:4])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// renderSpecMD produces spec.md from the interview answers. Follows
// the nova-feature skill template: Why, What, ACs, Non-goals. The
// frontmatter includes the fields required by radiant's quality
// auditor (name, description, alwaysApply) so the spec validates
// out-of-the-box.
func renderSpecMD(seq int, slug, intent, tier string, acs []string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: spec-%04d-%s\n", seq, slug)
	fmt.Fprintf(&b, "description: \"%s\"\n", strings.ReplaceAll(intent, `"`, `\"`))
	fmt.Fprintf(&b, "id: %04d\n", seq)
	fmt.Fprintf(&b, "slug: %s\n", slug)
	fmt.Fprintf(&b, "tier: %s\n", tier)
	b.WriteString("status: draft\n")
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %04d — %s\n\n", seq, slug)
	b.WriteString("## Why\n\n")
	fmt.Fprintf(&b, "%s\n\n", intent)
	b.WriteString("## What\n\n")
	b.WriteString("[Describe the user-visible behavior introduced by this feature.]\n\n")
	b.WriteString("## Acceptance criteria\n\n")
	for i, ac := range acs {
		// Header MUST include a title after the AC id (e.g. "### AC1: ...")
		// so `parseAcceptanceCriteria` in cmd_pr_review.go recognises it.
		// Body is on the next line.
		shortTitle := ac
		if len(shortTitle) > 80 {
			shortTitle = shortTitle[:77] + "..."
		}
		fmt.Fprintf(&b, "### AC%d: %s\n\n%s\n\n", i+1, shortTitle, ac)
	}
	b.WriteString("## Non-goals\n\n")
	b.WriteString("- [List what this feature does NOT do. Prevents scope creep.]\n\n")
	fmt.Fprintf(&b, "_Generated by `radiant spec` on %s (tier=%s)._\n", time.Now().UTC().Format("2006-01-02"), tier)
	return b.String()
}

// renderTasksMD produces tasks.md as a Markdown table with the AC
// coverage column. The coverage gate is enforced at command time —
// every task must declare which ACs it covers. The frontmatter
// matches `radiant validate`'s quality schema (name, description,
// alwaysApply).
func renderTasksMD(seq int, slug, tier string, tasks, gates, covers, acs []string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: tasks-%04d-%s\n", seq, slug)
	fmt.Fprintf(&b, "description: \"Tasks for spec %04d — %s\"\n", seq, slug)
	fmt.Fprintf(&b, "id: %04d\n", seq)
	fmt.Fprintf(&b, "slug: %s\n", slug)
	fmt.Fprintf(&b, "tier: %s\n", tier)
	b.WriteString("status: draft\n")
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %04d — Tasks: %s\n\n", seq, slug)
	fmt.Fprintf(&b, "_Tier: %s. Total ACs: %d. Total tasks: %d._\n\n", tier, len(acs), len(tasks))
	b.WriteString("| # | Task | Covers | Gate |\n")
	b.WriteString("|---|------|--------|------|\n")
	for i, t := range tasks {
		fmt.Fprintf(&b, "| %d | %s | %s | `%s` |\n", i+1, t, covers[i], gates[i])
	}
	b.WriteString("\n## Coverage check\n\n")
	b.WriteString("Every AC must appear in at least one task's Covers column:\n\n")
	covered := make(map[string]bool)
	for _, c := range covers {
		for _, ac := range strings.Split(c, ",") {
			ac = strings.TrimSpace(ac)
			if ac != "" {
				covered[ac] = true
			}
		}
	}
	for i := 1; i <= len(acs); i++ {
		key := strconv.Itoa(i)
		if covered[key] {
			fmt.Fprintf(&b, "- ✓ AC%d covered\n", i)
		} else {
			fmt.Fprintf(&b, "- ✗ AC%d NOT covered\n", i)
		}
	}
	b.WriteString("\n## Gates\n\n")
	b.WriteString("Each task's Gate command must exit 0 for the task to count as done.\n")
	b.WriteString("Commands must be in the gate allowlist (see `internal/policy`).\n")
	return b.String()
}

// upsertStateCurrentFeature updates the `current_feature`, `tier`,
// and `next_command` lines in state.md content. Idempotent — safe to
// call repeatedly.
func upsertStateCurrentFeature(body, feature, tier, nextCmd string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "- current_feature:"):
			lines[i] = fmt.Sprintf("- current_feature: %s", feature)
		case strings.HasPrefix(line, "- tier:"):
			lines[i] = fmt.Sprintf("- tier: %s", tier)
		case strings.HasPrefix(line, "- next_command:"):
			lines[i] = fmt.Sprintf("- next_command: %s", nextCmd)
		}
	}
	return strings.Join(lines, "\n")
}

// atomicWrite writes data to path via temp + rename so a crash
// mid-write leaves no torn file.
func atomicWrite(path, data string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// readFrontmatterVersion reads the `version:` field from a skill's
// frontmatter.yaml. Returns "" if the file is missing or has no
// version field. Used by `radiant update` to compare bundled vs.
// local skill versions. We don't unmarshal full YAML — a partial
// line scan is enough for one field and avoids a dependency in
// main.go (yaml.v3 already lives in internal/skill/).
func readFrontmatterVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		// Strip surrounding quotes (YAML permits both).
		v = strings.Trim(v, "\"'")
		return v
	}
	return ""
}

// generateAgentsMD returns the canonical AGENTS.md content. It is
// intentionally minimal (<=100 lines) — per the AI-dev video
// research, bloated AGENTS.md files hurt LLM behaviour. The
// canonical list of skills is appended as a one-line-per-skill
// section so an agent can grep the file to discover what exists.
//
// As of Sprint 14.3 this is a thin wrapper that delegates to
// `scaffold.GenerateAgentsMD()` — the SINGLE SOURCE OF TRUTH for
// the AGENTS.md template. Both `radiant init` and `radiant update`
// now produce identical content; the audit (`radiant camada-agentica`)
// cross-checks consistency.
func generateAgentsMD() string {
	return scaffold.GenerateAgentsMD()
}

// nextADRSequence scans `docs/architecture/adr/` for the highest
// NNNN- prefix and returns next+1. Returns 1 if the directory is
// empty or doesn't exist yet. Same algorithm as nextSpecSeq but
// kept separate so the two domains can evolve independently.
func nextADRSequence(adrDir string) (int, error) {
	max := 0
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[4] != '-' {
			continue
		}
		n, err := strconv.Atoi(name[:4])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// featureCoverage is one row in the evals-report.md table — the
// per-feature fidelity snapshot.
type featureCoverage struct {
	Slug      string
	Total     int      // total ACs
	Covered   int      // ACs claimed in tasks.md coverage
	Uncovered []string // AC IDs not covered
	Score     float64  // covered / total (0..1)
}

// runEvals walks specs/ in scope, parses ACs + tasks coverage,
// and writes a fidelity report. Scopes:
//   - "all" — every feature in specs/
//   - "since-last-release" — every feature modified since the last
//     git tag (per git log --tags). If no tags exist, falls back
//     to all.
//   - <spec-path> — single feature (e.g. specs/0001-jwt/)
func runEvals(scope, outPath string) error {
	specsDir := "specs"

	// Resolve "since-last-release" to a set of feature slugs.
	var includeSlugs map[string]bool // nil = include all
	if scope == "since-last-release" {
		lastTag, err := lastGitTag()
		if err != nil || lastTag == "" {
			fmt.Println("  (no tags found; falling back to scope=all)")
		} else {
			fmt.Printf("  (scoping to features modified since %s)\n", lastTag)
			changed, err := specsChangedSince(lastTag)
			if err != nil {
				return fmt.Errorf("git log since %s: %w", lastTag, err)
			}
			includeSlugs = map[string]bool{}
			for _, s := range changed {
				includeSlugs[s] = true
			}
			if len(includeSlugs) == 0 {
				fmt.Println("  (no features modified since last release; reporting empty scope)")
			}
		}
	} else if scope != "all" && strings.HasPrefix(scope, "specs/") {
		// Treat scope as a single spec path.
		includeSlugs = map[string]bool{filepath.Base(scope): true}
	}

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s directory found — initialize with `radiant init` or `radiant spec`", specsDir)
		}
		return fmt.Errorf("read %s: %w", specsDir, err)
	}

	var coverages []featureCoverage
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_templates" || e.Name() == "quick" {
			continue
		}
		slug := e.Name()
		if includeSlugs != nil && !includeSlugs[slug] {
			continue
		}
		feat, err := computeFeatureCoverage(filepath.Join(specsDir, slug))
		if err != nil {
			fmt.Printf("  [skip] %s: %v\n", slug, err)
			continue
		}
		coverages = append(coverages, feat)
	}

	if len(coverages) == 0 {
		fmt.Println("  (no features found in specs/)")
		return nil
	}

	// Sort by score ascending so worst-covered features surface first.
	sort.Slice(coverages, func(i, j int) bool {
		return coverages[i].Score < coverages[j].Score
	})

	body := renderEvalsReport(scope, coverages)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	// Stdout summary
	totalACs := 0
	totalCovered := 0
	for _, c := range coverages {
		totalACs += c.Total
		totalCovered += c.Covered
	}
	overall := 0.0
	if totalACs > 0 {
		overall = float64(totalCovered) / float64(totalACs)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Features: %d\n", len(coverages))
	fmt.Printf("  ACs: %d total, %d claimed-covered (%.0f%%)\n",
		totalACs, totalCovered, overall*100)
	fmt.Printf("\n  Per-feature scores (worst first):\n")
	for _, c := range coverages {
		fmt.Printf("    %s — %d/%d (%.0f%%)\n",
			c.Slug, c.Covered, c.Total, c.Score*100)
	}
	if overall < 0.8 {
		fmt.Printf("\n  ⚠ fidelity below 80%% — review uncovered ACs above\n")
	} else if overall >= 1.0 {
		fmt.Printf("\n  ✓ 100%% fidelity — every AC claimed in tasks.md\n")
	}
	return nil
}

// computeFeatureCoverage parses one spec dir's spec.md + tasks.md
// and returns the coverage snapshot. Coverage = "the AC ID appears
// in at least one task's Coverage column" — i.e. is CLAIMED to
// be covered. The LLM via the evals skill does the real verification.
func computeFeatureCoverage(specDir string) (featureCoverage, error) {
	specMD, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		return featureCoverage{}, err
	}
	tasksMD, err := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if err != nil {
		// tasks.md missing = 0 coverage (the spec was never decomposed).
		tasksMD = []byte{}
	}

	acs := parseAcceptanceCriteria(string(specMD))
	if len(acs) == 0 {
		return featureCoverage{}, fmt.Errorf("no ACs found")
	}

	tasksBody := string(tasksMD)
	coveredSet := map[string]bool{}
	for _, ac := range acs {
		// Check if AC ID appears anywhere in tasks.md (the
		// Coverage column references ACs by ID).
		if strings.Contains(tasksBody, ac.ID) {
			coveredSet[ac.ID] = true
		}
	}

	var uncovered []string
	for _, ac := range acs {
		if !coveredSet[ac.ID] {
			uncovered = append(uncovered, ac.ID)
		}
	}

	slug := filepath.Base(specDir)
	score := float64(len(coveredSet)) / float64(len(acs))
	return featureCoverage{
		Slug:      slug,
		Total:     len(acs),
		Covered:   len(coveredSet),
		Uncovered: uncovered,
		Score:     score,
	}, nil
}

// renderEvalsReport produces the docs/evals-report.md content.
// Per the evals skill, the report MUST cite evidence per claim —
// for the MVP we cite spec.md:line / tasks.md:line as evidence.
// The LLM (via the skill) is responsible for filling in
// implementation evidence (file:line of the actual code).
func renderEvalsReport(scope string, coverages []featureCoverage) string {
	totalACs := 0
	totalCovered := 0
	for _, c := range coverages {
		totalACs += c.Total
		totalCovered += c.Covered
	}
	overall := 0.0
	if totalACs > 0 {
		overall = float64(totalCovered) / float64(totalACs)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Evals: %s\n\n", scope)
	fmt.Fprintf(&b, "> Generated by `radiant evals --scope=%s`. Per-AC\n", scope)
	b.WriteString("> evidence (file:line of implementing code) is filled in\n")
	b.WriteString("> by the LLM via the `evals` skill — the table below shows\n")
	b.WriteString("> the **claimed** coverage from tasks.md. The skill's job\n")
	b.WriteString("> is to verify each claim against actual code + test runs.\n\n")

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Features in scope | %d |\n", len(coverages))
	fmt.Fprintf(&b, "| Total ACs | %d |\n", totalACs)
	fmt.Fprintf(&b, "| Claimed-covered ACs | %d |\n", totalCovered)
	fmt.Fprintf(&b, "| Aggregate fidelity | **%.1f%%** |\n\n", overall*100)

	// Per-feature
	b.WriteString("## Per-feature fidelity\n\n")
	b.WriteString("| Feature | ACs | Covered | Score | Uncovered |\n")
	b.WriteString("|---------|-----|---------|-------|-----------|\n")
	for _, c := range coverages {
		uncovered := strings.Join(c.Uncovered, ", ")
		if uncovered == "" {
			uncovered = "—"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %.0f%% | %s |\n",
			c.Slug, c.Total, c.Covered, c.Score*100, uncovered)
	}
	b.WriteString("\n")

	// Per-AC detail (per-feature)
	b.WriteString("## AC-level detail\n\n")
	for _, c := range coverages {
		fmt.Fprintf(&b, "### %s\n\n", c.Slug)
		b.WriteString("| AC | Claimed covered | Evidence |\n")
		b.WriteString("|----|-----------------|----------|\n")
		b.WriteString("| _per AC_ | _TODO_ | _TODO (file:line of implementing code)_ |\n")
		b.WriteString("\n> Each TODO above is filled in by the LLM via the\n")
		b.WriteString("> `evals` skill: trace the AC's Given/When/Then to a test\n")
		b.WriteString("> that asserts it, and cite the file:line.\n\n")
	}

	// Footer
	b.WriteString("---\n\n")
	b.WriteString("_Generated by `radiant evals`. Re-run after every release; fidelity drifts._\n")
	return b.String()
}

// runCamadaAgentica audits the project's agentic layer:
//   - AGENTS.md present + references all bundled skills
//   - Native views present for the agents in use (--agents)
//   - Version consistency (AGENTS.md says skill X is vY, bundled
//     skill is vZ — drift!)
//
// Returns exit code 0 if everything is in sync, non-zero if any
// drift or missing files. With --fix, regenerates AGENTS.md
// (but does NOT touch native views; the user owns those).
func runCamadaAgentica(agentFlag string, fix bool) error {
	infos, err := skill.Bundle()
	if err != nil {
		return fmt.Errorf("load bundled skills: %w", err)
	}

	var agents []radiant.AgentID
	if agentFlag != "" {
		agents = resolveAgents(agentFlag, false)
	}

	// 1. Check AGENTS.md presence + contents.
	agentsPath := "AGENTS.md"
	agentsBody, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  [missing] AGENTS.md — run `radiant init` or `radiant update` to generate")
		} else {
			return fmt.Errorf("read %s: %w", agentsPath, err)
		}
	}

	// 2. Check skill references in AGENTS.md (presence only —
	//    full version parsing would couple us to the YAML format).
	if len(agentsBody) > 0 {
		var missing []string
		for _, info := range infos {
			if !strings.Contains(string(agentsBody), info.Name) {
				missing = append(missing, info.Name)
			}
		}
		if len(missing) > 0 {
			fmt.Printf("  [drift] AGENTS.md missing references to %d skill(s): %s\n",
				len(missing), strings.Join(missing, ", "))
		}
	}

	// 3. Check version drift between AGENTS.md and bundled skills.
	if len(agentsBody) > 0 {
		var drifted []string
		for _, info := range infos {
			// Look for "name (vX.Y.Z)" in AGENTS.md (the format generateAgentsMD emits).
			needle := fmt.Sprintf("%s (v%s)", info.Name, info.Version)
			if !strings.Contains(string(agentsBody), needle) {
				drifted = append(drifted, info.Name)
			}
		}
		if len(drifted) > 0 {
			fmt.Printf("  [version-drift] AGENTS.md version mismatch on %d skill(s): %s\n",
				len(drifted), strings.Join(drifted, ", "))
		}
	}

	// 4. Check native views presence for the agents in use.
	if len(agents) > 0 {
		for _, agent := range agents {
			adapter := scaffold.GetAdapter(agent)
			if adapter == nil {
				fmt.Printf("  [unknown-agent] %s\n", agent)
				continue
			}
			if _, err := os.Stat(adapter.InstTo); os.IsNotExist(err) {
				fmt.Printf("  [missing-view] %s — run `radiant views --agent=%s` to generate\n",
					adapter.InstTo, agent)
			} else {
				fmt.Printf("  [ok] %s (%s)\n", adapter.InstTo, adapter.Label)
			}
		}
	}

	// 5. Optional: regenerate AGENTS.md (without touching native views).
	if fix {
		body := generateAgentsMD()
		if err := atomicWrite(agentsPath, body); err != nil {
			return fmt.Errorf("regenerate %s: %w", agentsPath, err)
		}
		fmt.Printf("  [regenerated] %s\n", agentsPath)
	} else {
		fmt.Printf("\n  Re-run with --fix to regenerate AGENTS.md from current bundled skills.\n")
	}
	return nil
}

// handleMCPRequest dispatches one JSON-RPC request to the
// appropriate MCP method handler.
func handleMCPRequest(req mcpRequest, tools []mcpTool, d *mcpDispatcher) mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "radiant-harness",
					"version": version,
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			},
		}
	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32602, Message: "invalid params"}}
		}
		result := callMCPTool(params.Name, params.Arguments, d)
		result.ID = req.ID
		return result
	default:
		return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

// runIncident scaffolds an incident document. The user fills in
// the timeline, RCA, and action items following the `incident`
// skill's blameless post-mortem template. Severity must be one
// of sev1..sev4 (validated).
func runIncident(severity, summary, outPath string) error {
	severity = strings.ToLower(strings.TrimSpace(severity))
	switch severity {
	case "sev1", "sev2", "sev3", "sev4":
		// ok
	default:
		return fmt.Errorf("invalid severity %q — expected sev1 | sev2 | sev3 | sev4", severity)
	}

	if outPath == "" {
		seq, err := nextIncidentSeq()
		if err != nil {
			return fmt.Errorf("compute next sequence: %w", err)
		}
		slug := slugify(summary)
		outPath = filepath.Join("docs", "incidents", fmt.Sprintf("%04d-%s.md", seq, slug))
	}

	body := renderIncidentDoc(severity, summary)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ created %s\n", outPath)
	fmt.Println("\n  Next (per the `incident` skill):")
	fmt.Println("    1. Acknowledge the alert in PagerDuty / Opsgenie / Slack.")
	fmt.Println("    2. Assign severity if you haven't already (you gave sev" + severity[3:] + ").")
	fmt.Println("    3. Name an incident commander within 5 min.")
	fmt.Println("    4. Mitigate (rollback / scale / failover) within 15 min.")
	fmt.Println("    5. Update status page for sev1/sev2.")
	fmt.Println("    6. Schedule a blameless post-mortem within 5 business days.")
	return nil
}

// nextIncidentSeq scans docs/incidents/ for the highest NNNN-
// prefix and returns next+1. Returns 1 if the directory is empty
// or doesn't exist yet.
func nextIncidentSeq() (int, error) {
	dir := "docs/incidents"
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[4] != '-' {
			continue
		}
		n, err := strconv.Atoi(name[:4])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

func runValidate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	body := string(data)
	issues := []string{}

	// Heuristic 1: must not be empty
	if len(body) < 100 {
		issues = append(issues, "file is suspiciously short (<100 bytes)")
	}

	// Heuristic 2: must have a top-level heading
	if !strings.HasPrefix(strings.TrimSpace(body), "#") {
		issues = append(issues, "no top-level markdown heading found")
	}

	// Heuristic 3: must not contain unfilled placeholders
	// (heuristic: <TODO or <fill in or `<...>` placeholder)
	placeholderPatterns := []string{
		"<TODO",
		"<fill in",
		"<FILL",
		"<your ",
		"<path/to/",
		"<n>",
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		for _, p := range placeholderPatterns {
			if strings.Contains(line, p) {
				issues = append(issues, fmt.Sprintf("line %d has placeholder '%s': %s", i+1, p, strings.TrimSpace(line)))
				break
			}
		}
	}

	// Heuristic 4: scaffold-specific section expectations
	if strings.HasSuffix(path, "-plan.md") || strings.HasSuffix(path, "-spec.md") || strings.HasSuffix(path, "-eval.md") || strings.HasSuffix(path, "-monitor.md") || strings.HasSuffix(path, "-request.md") {
		for _, sec := range []string{"## Decision tree", "## Workflow", "## Examples", "## Anti-patterns"} {
			if !strings.Contains(body, sec) {
				issues = append(issues, fmt.Sprintf("missing section: %s", sec))
			}
		}
	}

	if len(issues) == 0 {
		fmt.Printf("  ✓ %s: validated (%d lines, 0 issues)\n", path, len(lines))
		return nil
	}
	fmt.Printf("  ✗ %s: %d issue(s)\n", path, len(issues))
	for _, issue := range issues {
		fmt.Printf("    - %s\n", issue)
	}
	return fmt.Errorf("%s failed validation", path)
}

// runProfileScaffold produces a data profile scaffold following
// the radiant-data + radiant-drift skills. Captures schema, row
// counts, null rates, distributions, drift metrics, monitoring
// plan.

// renderIncidentDoc produces the incident document body. The
// user fills in the timeline + RCA + action items following
// the structure. MVP: template only; LLM via the `incident` skill
// fills in the actual content during the post-mortem.
func renderIncidentDoc(severity, summary string) string {
	return fmt.Sprintf(`# Incident %s — %s

> Generated by 'radiant incident'. Per the 'incident' skill:
> blameless post-mortem template. Fill in the timeline + RCA +
> action items; the skill's job is to remind you what goes where.

**Severity**: %s
**Date**: %s
**Duration**: <fill in: HH:MM from detection to resolution>
**Impact**: <fill in: customer-facing impact>
**Commander**: <name>
**Author**: <name>

## Timeline (UTC)

- HH:MM — detection (alert fired / customer report)
- HH:MM — commander named
- HH:MM — mitigation started (rollback / scale / failover)
- HH:MM — service restored
- HH:MM — root cause identified
- HH:MM — permanent fix deployed

## Root cause

What happened, and WHY. Not the symptom — the cause. Include the chain of events that led to the failure.

## Contributing factors

- What monitoring missed
- What tests didn't catch
- What process / runbook was unclear
- What communication failed

## What went well

- Fast detection
- Quick rollback
- Clear comms
- Good escalation

## Action items

| # | Action | Owner | Due | Tracked in |
|---|--------|-------|-----|------------|
| 1 | Add monitoring for X | @alice | 2026-07-01 | roadmap |
| 2 | Improve runbook for Y | @bob   | 2026-07-15 | roadmap |
| 3 | Add regression test for Z | @carol | 2026-07-01 | roadmap |

---

_Generated by 'radiant incident' on %s. See the 'incident' skill for the full playbook._
`,
		severity, summary, severity, time.Now().UTC().Format("2006-01-02"), time.Now().UTC().Format("2006-01-02"))
}

// runTelemetryRotate caps the active telemetry log at `maxEntries`.
// When the log exceeds the cap, the oldest events are moved to
// an archive file (`telemetry-YYYY-MM-DD.jsonl`) so the user keeps
// full history without the active log growing unbounded.
//
// Idempotent: running it on a log under the cap is a no-op.
// Idempotent on a missing log: returns nil.
func runTelemetryRotate(maxEntries int) error {
	if maxEntries <= 0 {
		return fmt.Errorf("--max-entries must be > 0 (got %d)", maxEntries)
	}
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= maxEntries {
		fmt.Printf("  Log has %d entries; under cap %d. No rotation needed.\n", len(lines), maxEntries)
		return nil
	}

	// Split into keep (latest maxEntries) and archive (rest).
	keep := lines[len(lines)-maxEntries:]
	archive := lines[:len(lines)-maxEntries]
	archivePath := fmt.Sprintf("%s-%s.jsonl",
		strings.TrimSuffix(telemetryLogPath, ".jsonl"),
		time.Now().UTC().Format("2006-01-02"),
	)

	if err := os.WriteFile(archivePath, []byte(strings.Join(archive, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	if err := os.WriteFile(telemetryLogPath, []byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("rewrite log %s: %w", telemetryLogPath, err)
	}
	fmt.Printf("  ✓ Archived %d events to %s\n", len(archive), archivePath)
	fmt.Printf("  ✓ Active log now has %d entries (cap: %d)\n", len(keep), maxEntries)
	return nil
}

// runTelemetryExport dumps the local telemetry log in JSON or CSV.
// Default: JSON to stdout (pipe-friendly). Pass --output=<path> to
// write to a file. Pass --since=YYYY-MM-DD to filter.
//
// Privacy: exports ONLY the 4 fields already recorded locally
// (timestamp, command, hash, radiant_ver). The user must explicitly
// invoke this command AND pipe/save the output. No network egress.
//
// Format selection:
//
//	json — pretty-printed array of events (one per line in the log).
//	csv  — header row + one row per event.
//
// Disabled or missing log → no-op, returns nil.
func runTelemetryExport(format, output, since string) error {
	if format != "json" && format != "csv" {
		return fmt.Errorf("--format must be 'json' or 'csv' (got %q)", format)
	}
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	raw := strings.TrimRight(string(data), "\n")
	if raw == "" {
		return nil
	}

	// Build a normalized slice of events. Skip lines that don't parse
	// as JSON (defensive — shouldn't happen given how we record, but
	// user could have hand-edited the log).
	type ev struct {
		Timestamp  string `json:"timestamp"`
		Command    string `json:"command"`
		Hash       string `json:"hash"`
		RadiantVer string `json:"radiant_ver"`
	}
	var events []ev
	for _, line := range strings.Split(raw, "\n") {
		var e ev
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		// Apply --since filter (date portion of timestamp >= since).
		if since != "" {
			if len(e.Timestamp) < 10 || e.Timestamp[:10] < since {
				continue
			}
		}
		events = append(events, e)
	}

	var out string
	switch format {
	case "json":
		b, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		out = string(b) + "\n"
	case "csv":
		var sb strings.Builder
		sb.WriteString("timestamp,command,hash,radiant_ver\n")
		for _, e := range events {
			fmt.Fprintf(&sb, "%s,%s,%s,%s\n",
				csvField(e.Timestamp), csvField(e.Command),
				csvField(e.Hash), csvField(e.RadiantVer))
		}
		out = sb.String()
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(out), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", output, err)
		}
		fmt.Printf("  ✓ Exported %d events to %s (%s)\n", len(events), output, format)
	} else {
		fmt.Print(out)
	}
	return nil
}

// csvField quotes a value iff it contains a comma, double-quote, or newline.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// runTelemetrySummary reads the local telemetry log and prints
// aggregate stats: total events, top commands by frequency, and
// daily counts. All computation is local — no network access.
// Privacy guarantees are the same as `show` (only the local file).
func runTelemetrySummary() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	var events []telemetryEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev telemetryEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip malformed lines
		}
		events = append(events, ev)
	}
	if len(events) == 0 {
		fmt.Println("  (no events recorded)")
		return nil
	}

	// Count by command.
	cmdCounts := map[string]int{}
	// Count by day.
	dayCounts := map[string]int{}
	for _, ev := range events {
		cmdCounts[ev.Command]++
		// Timestamp is RFC3339; take the date prefix (first 10 chars).
		if len(ev.Timestamp) >= 10 {
			day := ev.Timestamp[:10]
			dayCounts[day]++
		}
	}

	// Top commands sorted descending.
	type cmdCount struct {
		cmd   string
		count int
	}
	var sortedCmds []cmdCount
	for c, n := range cmdCounts {
		sortedCmds = append(sortedCmds, cmdCount{c, n})
	}
	sort.Slice(sortedCmds, func(i, j int) bool {
		return sortedCmds[i].count > sortedCmds[j].count
	})

	// Daily counts sorted by date (string sort works for ISO dates).
	var sortedDays []string
	for d := range dayCounts {
		sortedDays = append(sortedDays, d)
	}
	sort.Strings(sortedDays)

	fmt.Printf("  Total events: %d\n", len(events))
	fmt.Printf("  Distinct commands: %d\n", len(cmdCounts))
	fmt.Printf("  Distinct days: %d\n", len(dayCounts))
	fmt.Println()
	fmt.Println("  Top commands:")
	for i, cc := range sortedCmds {
		if i >= 10 {
			break
		}
		fmt.Printf("    %-20s %d\n", cc.cmd, cc.count)
	}
	fmt.Println()
	fmt.Println("  Daily counts:")
	for _, d := range sortedDays {
		fmt.Printf("    %s  %d\n", d, dayCounts[d])
	}
	return nil
}

// recordTelemetry appends one telemetry event to the local log.
// PRIVACY-FIRST: this is a no-op unless the user has explicitly
// enabled telemetry via `radiant telemetry enable`. The event
// records only the command name + timestamp + 8-char content
// hash + CLI version — never args, paths, or env vars.
//
// Used by the release pipeline so that cutting a release is
// auditable locally. Composes naturally with `radiant telemetry
// show` / `radiant telemetry summary`.
func recordTelemetry(command string) {
	if !isTelemetryEnabled() {
		return
	}
	hash := shortHash(time.Now().UTC().Format(time.RFC3339Nano))
	ev := telemetryEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Command:    command,
		Hash:       hash,
		RadiantVer: version,
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return // best-effort; never fail the user's command
	}
	f, err := os.OpenFile(telemetryLogPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

// shortHash returns the first 8 chars of sha256(input). Used for
// the telemetry event's `hash` field — a stable identifier for the
// event without leaking the underlying content.
func shortHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:8]
}

// telemetryLogPath is the canonical location of the local
// telemetry log. Lives under .radiant-harness/ so it stays out
// of the user's source tree.
const telemetryLogPath = ".radiant-harness/telemetry.jsonl"

// telemetryEvent is one row in the telemetry log. PRIVACY-FIRST:
// only the command name (e.g. "spec"), a content hash, and the
// ISO-8601 timestamp are recorded. No args, no paths, no project
// metadata, no environment info.
type telemetryEvent struct {
	Timestamp  string `json:"timestamp"`   // ISO-8601 UTC
	Command    string `json:"command"`     // e.g. "spec", "release", "audit"
	Hash       string `json:"hash"`        // sha256 of redacted context (placeholder, 8 chars)
	RadiantVer string `json:"radiant_ver"` // CLI version (semver, no git sha)
}

// isTelemetryEnabled returns true when the user has run
// `radiant telemetry enable`. We detect enablement by checking
// for the existence of the telemetry log file. There is no
// separate "config" file — the log's existence IS the flag.
func isTelemetryEnabled() bool {
	_, err := os.Stat(telemetryLogPath)
	return err == nil
}

// runTelemetryStatus reports whether telemetry is enabled, what
// would be recorded if it were, and where the log lives.
func runTelemetryStatus() error {
	enabled := isTelemetryEnabled()
	fmt.Printf("  Telemetry: %s\n", boolStr(enabled))
	fmt.Printf("  Log path:  %s\n", telemetryLogPath)
	fmt.Println()
	fmt.Println("  When enabled, each radiant invocation records:")
	fmt.Println("    - timestamp (ISO-8601 UTC)")
	fmt.Println("    - command name (e.g. \"spec\", \"release\")")
	fmt.Println("    - 8-char hash of redacted context")
	fmt.Println("    - radiant CLI version (semver)")
	fmt.Println()
	fmt.Println("  NEVER recorded (privacy-first):")
	fmt.Println("    - command arguments")
	fmt.Println("    - file paths")
	fmt.Println("    - project names or git SHAs")
	fmt.Println("    - environment variables")
	fmt.Println("    - network endpoints")
	fmt.Println()
	if enabled {
		fmt.Printf("  Run 'radiant telemetry disable' to opt out and delete the log.\n")
	} else {
		fmt.Printf("  Run 'radiant telemetry enable' to opt in.\n")
	}
	return nil
}

// runTelemetryEnable creates the telemetry log file (empty).
// The act of creating the file IS the opt-in.
func runTelemetryEnable() error {
	if isTelemetryEnabled() {
		fmt.Printf("  Telemetry already enabled (log at %s)\n", telemetryLogPath)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(telemetryLogPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", telemetryLogPath, err)
	}
	defer f.Close()
	fmt.Printf("  ✓ Telemetry enabled. Log: %s\n", telemetryLogPath)
	fmt.Println("  Each subsequent radiant invocation will append one line.")
	fmt.Println("  Disable with 'radiant telemetry disable'.")
	return nil
}

// runTelemetryDisable removes the telemetry log file. Idempotent:
// returns nil even if the file doesn't exist.
func runTelemetryDisable() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry already disabled (no log file).")
		return nil
	}
	if err := os.Remove(telemetryLogPath); err != nil {
		return fmt.Errorf("remove %s: %w", telemetryLogPath, err)
	}
	fmt.Printf("  ✓ Telemetry disabled. Removed %s.\n", telemetryLogPath)
	return nil
}

// runTelemetryShow prints the last 50 events from the log, one
// per line. If telemetry is disabled, prints a helpful message.
func runTelemetryShow() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		fmt.Println("  (no events recorded)")
		return nil
	}
	// Last 50 events, most recent first.
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}
	fmt.Printf("  Last %d events:\n", len(lines)-start)
	for _, line := range lines[start:] {
		fmt.Printf("    %s\n", line)
	}
	return nil
}

// boolStr is a tiny helper to render "enabled" / "disabled".
func boolStr(b bool) string {
	if b {
		return "ENABLED"
	}
	return "disabled (opt-in)"
}

// mcpRunFull implements the radiant_run MCP tool.
// Calls loop.Run() directly in-process — no exec.Command, no PATH dependency.
// When backend is non-nil (sampling mode), all LLM calls route through it
// instead of the HTTP API — no API key required.
func mcpRunFull(args json.RawMessage, backend llm.Backend) mcpResponse {
	var a struct {
		Goal      string  `json:"goal"`
		Profile   string  `json:"profile"`
		Model     string  `json:"model"`
		MaxIter   int     `json:"max_iter"`
		MaxCost   float64 `json:"max_cost"`
		MaxTime   string  `json:"max_time"`
		AutoRoute bool    `json:"auto_route"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Goal == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "radiant_run: goal is required"}}
	}
	if a.Profile == "" {
		a.Profile = "standard"
	}

	cwd, _ := os.Getwd()

	cfg, _ := config.Load(cwd)
	if cfg == nil {
		cfg = &config.Config{}
	}

	// Sampling mode: skip API key resolution entirely.
	if backend != nil {
		return mcpRunWithBackend(a, backend, cwd, cfg)
	}
	return mcpRunHTTP(a, cwd, cfg)
}

// mcpRunHTTP executes a full harness loop using the HTTP API (normal mode).
// Resolves API credentials from the environment.
func mcpRunHTTP(a struct {
	Goal      string  `json:"goal"`
	Profile   string  `json:"profile"`
	Model     string  `json:"model"`
	MaxIter   int     `json:"max_iter"`
	MaxCost   float64 `json:"max_cost"`
	MaxTime   string  `json:"max_time"`
	AutoRoute bool    `json:"auto_route"`
}, cwd string, cfg *config.Config) mcpResponse {

	apiKey, baseURL := resolveLoopLLMCreds("")

	modelID := a.Model
	if modelID == "" {
		modelID = os.Getenv("RADIANT_MODEL")
	}
	if modelID == "" && cfg.Model != "" {
		modelID = cfg.Model
	}
	if modelID == "" {
		modelID = "claude-sonnet-4-6"
	}

	autoRoute := a.AutoRoute || cfg.AutoRoute

	m := llm.Model{Model: modelID, APIKey: apiKey, BaseURL: baseURL}
	costPer1K, _ := loop.PriceFor(modelID)

	maxIter := a.MaxIter
	if maxIter == 0 && cfg.MaxIter > 0 {
		maxIter = cfg.MaxIter
	}

	var maxDuration time.Duration
	if a.MaxTime != "" {
		maxDuration, _ = time.ParseDuration(a.MaxTime)
	}

	runID := fmt.Sprintf("run-%d", time.Now().Unix())

	runCfg := loop.RunConfig{
		ExecutorModel: m,
		VerifierModel: m,
		PlannerModel:  m,
		Budget: loop.BudgetConfig{
			MaxIter:     maxIter,
			Profile:     loop.BudgetProfile(a.Profile),
			MaxDuration: maxDuration,
			MaxCostUSD:  a.MaxCost,
			CostPer1K:   costPer1K,
		},
		AutoRoute: autoRoute,
	}

	result, err := loop.Run(context.Background(), cwd, runID, a.Goal, runCfg)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run ID: %s\nGoal: %s\nModel: %s\n\n", runID, a.Goal, modelID))
	if err != nil {
		sb.WriteString(fmt.Sprintf("Loop failed: %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("Exit: %s\nIterations: %d\nElapsed: %s\nTokens: %d\n",
			result.ExitReason, result.Iterations,
			result.Elapsed.Round(time.Second), result.TokensUsed))
		if result.CostUSD > 0 {
			sb.WriteString(fmt.Sprintf("Cost: $%.4f\n", result.CostUSD))
		}
	}

	infos, _ := loop.ListTraceInfos(cwd)
	for _, info := range infos {
		if info.RunID == runID {
			events, readErr := loop.ReadTrace(filepath.Join(cwd, ".radiant-harness", "traces", runID+".jsonl"))
			if readErr == nil {
				exp := loop.ExportTrace(runID, modelID, events)
				sb.WriteString("\n---\n\n")
				sb.WriteString(loop.ExportTraceMarkdown(exp))
			}
			break
		}
	}

	isErr := err != nil
	return mcpResponse{JSONRPC: "2.0", Result: map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": sb.String()}},
		"isError": isErr,
	}}
}

// mcpRunWithBackend executes a full harness loop using a caller-supplied
// llm.Backend (sampling mode). The host agent provides LLM inference via
// MCP sampling/createMessage — no API key required.
func mcpRunWithBackend(a struct {
	Goal      string  `json:"goal"`
	Profile   string  `json:"profile"`
	Model     string  `json:"model"`
	MaxIter   int     `json:"max_iter"`
	MaxCost   float64 `json:"max_cost"`
	MaxTime   string  `json:"max_time"`
	AutoRoute bool    `json:"auto_route"`
}, backend llm.Backend, cwd string, cfg *config.Config) mcpResponse {

	modelID := a.Model
	if modelID == "" {
		modelID = os.Getenv("RADIANT_MODEL")
	}
	if modelID == "" && cfg.Model != "" {
		modelID = cfg.Model
	}
	// In sampling mode we don't strictly need a model ID — the host decides.
	if modelID == "" {
		modelID = "mcp-sampling"
	}

	maxIter := a.MaxIter
	if maxIter == 0 && cfg.MaxIter > 0 {
		maxIter = cfg.MaxIter
	}

	var maxDuration time.Duration
	if a.MaxTime != "" {
		maxDuration, _ = time.ParseDuration(a.MaxTime)
	}

	runID := fmt.Sprintf("run-%d", time.Now().Unix())

	runCfg := loop.RunConfig{
		Backend: backend, // ← sampling mode: no API key
		Budget: loop.BudgetConfig{
			MaxIter:     maxIter,
			Profile:     loop.BudgetProfile(a.Profile),
			MaxDuration: maxDuration,
			MaxCostUSD:  a.MaxCost,
		},
	}

	result, err := loop.Run(context.Background(), cwd, runID, a.Goal, runCfg)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run ID: %s\nGoal: %s\nModel: %s (sampling)\n\n", runID, a.Goal, modelID))
	if err != nil {
		sb.WriteString(fmt.Sprintf("Loop failed: %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("Exit: %s\nIterations: %d\nElapsed: %s\nTokens: %d\n",
			result.ExitReason, result.Iterations,
			result.Elapsed.Round(time.Second), result.TokensUsed))
		if result.CostUSD > 0 {
			sb.WriteString(fmt.Sprintf("Cost: $%.4f\n", result.CostUSD))
		}
	}

	infos, _ := loop.ListTraceInfos(cwd)
	for _, info := range infos {
		if info.RunID == runID {
			events, readErr := loop.ReadTrace(filepath.Join(cwd, ".radiant-harness", "traces", runID+".jsonl"))
			if readErr == nil {
				exp := loop.ExportTrace(runID, modelID, events)
				sb.WriteString("\n---\n\n")
				sb.WriteString(loop.ExportTraceMarkdown(exp))
			}
			break
		}
	}

	isErr := err != nil
	return mcpResponse{JSONRPC: "2.0", Result: map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": sb.String()}},
		"isError": isErr,
	}}
}

// callMCPTool dispatches a tools/call request to the matching
// radiant CLI command. Returns the stdout as a content array.
func callMCPTool(name string, args json.RawMessage, d *mcpDispatcher) mcpResponse {
	var argv []string
	argv = append(argv, name)
	// Each tool has its own CLI shape. Map tools to subcommands.
	switch name {
	case "radiant_spec":
		var a struct {
			Intent string `json:"intent"`
		}
		_ = json.Unmarshal(args, &a)
		argv = []string{"spec", a.Intent}
	case "radiant_adr":
		var a struct {
			Decision string `json:"decision"`
			Status   string `json:"status"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Status != "" {
			argv = []string{"adr", a.Decision, "--status=" + a.Status}
		} else {
			argv = []string{"adr", a.Decision}
		}
	case "radiant_product":
		var a struct {
			Vision   string `json:"vision"`
			MVPWeeks int    `json:"mvp_weeks"`
		}
		_ = json.Unmarshal(args, &a)
		if a.MVPWeeks > 0 {
			argv = []string{"product", a.Vision, "--mvp-weeks=" + strconv.Itoa(a.MVPWeeks)}
		} else {
			argv = []string{"product", a.Vision}
		}
	case "radiant_evals":
		var a struct {
			Scope string `json:"scope"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Scope == "" {
			a.Scope = "all"
		}
		argv = []string{"evals", "--scope=" + a.Scope}
	case "radiant_audit":
		var a struct {
			Scope string `json:"scope"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Scope == "" {
			a.Scope = "full"
		}
		argv = []string{"audit", "--scope=" + a.Scope}
	case "radiant_release":
		var a struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(args, &a)
		// Always dry-run via MCP for safety — never let an MCP
		// caller tag a release without explicit CLI confirmation.
		argv = []string{"release", a.Version, "--dry-run"}
	case "radiant_loop_start":
		var a struct {
			Goal      string `json:"goal"`
			RunID     string `json:"run_id"`
			Model     string `json:"model"`
			MaxIter   int    `json:"max_iter"`
			AutoRoute bool   `json:"auto_route"`
		}
		_ = json.Unmarshal(args, &a)
		argv = []string{"loop", "start", a.Goal}
		if a.RunID != "" {
			argv = append(argv, "--run-id="+a.RunID)
		}
		if a.Model != "" {
			argv = append(argv, "--model="+a.Model)
		}
		if a.MaxIter > 0 {
			argv = append(argv, "--max-iter="+strconv.Itoa(a.MaxIter))
		}
		if a.AutoRoute {
			argv = append(argv, "--auto-route")
		}
	case "radiant_loop_status":
		var a struct {
			RunID string `json:"run_id"`
			Model string `json:"model"`
		}
		_ = json.Unmarshal(args, &a)
		argv = []string{"loop", "status"}
		if a.RunID != "" {
			argv = append(argv, a.RunID)
		}
		if a.Model != "" {
			argv = append(argv, "--model="+a.Model)
		}
	case "radiant_loop_list":
		var a struct {
			Plain bool `json:"plain"`
		}
		_ = json.Unmarshal(args, &a)
		argv = []string{"loop", "list"}
		if a.Plain {
			argv = append(argv, "--plain")
		}
	case "radiant_run":
		return mcpRunFull(args, d.backend())
	default:
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "unknown tool: " + name}}
	}

	cmd := exec.Command("radiant", argv...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		// Return the error as content (text) so the MCP client
		// sees the failure message. Don't bubble up a JSON-RPC
		// error — tools/call errors are tool-call failures, not
		// protocol errors.
		return mcpResponse{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": string(stdout)}},
				"isError": true,
			},
		}
	}
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(stdout)}},
		},
	}
}

// mcpServer mirrors the standard .mcp.json schema. We only care
// about a few fields (name/command/args/env) — the rest is ignored.
// Keeping this lightweight means a user can paste a real .mcp.json
// from any MCP-aware tool and we just read what's relevant.
type mcpServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Notes   string            `json:"notes,omitempty"`
}

type mcpConfig struct {
	Servers map[string]mcpServer `json:"mcpServers"`
}

// runIntegrationsList reads the project's .mcp.json and either
// prints a table (default) or emits JSON. Optionally writes the
// canonical docs/engineering/integrations.md from the same data.
//
// Per the integracoes skill, this command NEVER writes .mcp.json.
// The user/agent must approve each MCP entry via the skill first.
// We only surface what's already declared.
func runIntegrationsList(jsonOut bool, docOut string) error {
	cfgPath := ".mcp.json"
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s found — invoke the /integracoes skill (or run `radiant init --all`) to declare MCPs", cfgPath)
		}
		return fmt.Errorf("read %s: %w", cfgPath, err)
	}

	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	if len(cfg.Servers) == 0 {
		fmt.Println("  (no MCP servers declared in .mcp.json)")
		return nil
	}

	// Sort by name for stable output.
	names := make([]string, 0, len(cfg.Servers))
	for n := range cfg.Servers {
		names = append(names, n)
	}
	sort.Strings(names)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg.Servers)
	}

	// Table output.
	fmt.Printf("  MCP servers declared in %s (%d):\n\n", cfgPath, len(names))
	fmt.Printf("    %-20s %-12s %-32s %s\n", "NAME", "COMMAND", "ARGS (truncated)", "ENV")
	fmt.Printf("    %-20s %-12s %-32s %s\n", "----", "-------", "--------------", "---")
	for _, name := range names {
		s := cfg.Servers[name]
		args := strings.Join(s.Args, " ")
		if len(args) > 32 {
			args = args[:29] + "..."
		}
		if args == "" {
			args = "(none)"
		}
		cmd := s.Command
		if cmd == "" && s.URL != "" {
			cmd = "<http>"
		}
		if cmd == "" {
			cmd = "?"
		}
		fmt.Printf("    %-20s %-12s %-32s %d vars\n", name, cmd, args, len(s.Env))
	}

	fmt.Printf("\n  To validate an MCP, invoke the /integracoes skill.\n")
	fmt.Printf("  To approve and persist a new MCP, edit .mcp.json manually — this command never writes it.\n")

	if docOut != "" {
		body := renderIntegrationsDoc(cfg.Servers)
		if err := os.MkdirAll(filepath.Dir(docOut), 0o755); err != nil {
			return err
		}
		if err := atomicWrite(docOut, body); err != nil {
			return fmt.Errorf("write %s: %w", docOut, err)
		}
		fmt.Printf("\n  ✓ wrote %s\n", docOut)
	}
	return nil
}

// renderIntegrationsDoc produces the canonical
// docs/engineering/integrations.md content from the current
// .mcp.json. This is what the integracoes skill writes — we're
// just regenerating from data we can read.
func renderIntegrationsDoc(servers map[string]mcpServer) string {
	var b strings.Builder
	b.WriteString("# Integrations and MCPs\n\n")
	b.WriteString("> Auto-generated by `radiant integrations list --write-docs` from\n")
	b.WriteString("> the project's `.mcp.json`. Per the integracoes skill, MCPs are\n")
	b.WriteString("> only listed here AFTER explicit approval — see the skill for the\n")
	b.WriteString("> approval flow.\n\n")

	b.WriteString("## Declared MCP servers\n\n")
	b.WriteString("| Name | Command | Args | Env vars |\n")
	b.WriteString("|------|---------|------|----------|\n")

	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		s := servers[name]
		cmd := s.Command
		if cmd == "" && s.URL != "" {
			cmd = "<http>"
		}
		args := strings.Join(s.Args, " ")
		if args == "" {
			args = "—"
		}
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %d |\n", name, cmd, args, len(s.Env))
	}

	b.WriteString("\n## How to connect\n\n")
	b.WriteString("- **Project-scoped:** `.mcp.json` at repo root — shareable with team. **No secrets.**\n")
	b.WriteString("- **Secrets:** via env var or the relevant MCP CLI (`claude mcp add`, etc.). **Never** commit tokens.\n")
	b.WriteString("\n## Approval log\n\n")
	b.WriteString("Add a row each time an MCP is approved (use the integracoes skill for the\n")
	b.WriteString("full interview — never skip the account-boundary step).\n\n")
	b.WriteString("| Date | MCP | Account/workspace | Approved by |\n")
	b.WriteString("|------|-----|-------------------|-------------|\n")
	b.WriteString("| _    | _   | _                 | _           |\n")
	return b.String()
}

// renderInception produces the 6-phase Lean Inception template.
// The user/agent fills in each phase one at a time following the
// nova-product skill. The template intentionally uses simple
// Markdown so it renders well in any viewer (GitHub, GitLab,
// Obsidian, IDE preview).
func renderInception(slug, vision string, mvpWeeks int) string {
	return fmt.Sprintf(`# Product Inception — %s

> **Lean Inception template.** Generated by 'radiant product'.
> Fill in the 6 phases below, then cut the MVP at the end.
> See the nova-product skill for guidance.

## 1. Why

> Vision line: "We help '<persona>' do '<job>' better than
> '<alternative>'."

**Vision**: %s

**Persona**: <name, role, where they work, what tools they use today>

**Job-to-be-done**: <what they're trying to accomplish when they find this product>

**Pain today**: <the cost of the current alternative>

**Why now**: <what changed that makes this urgent>

**Success metric**: <one number that proves the product worked — e.g. "40%% of weekly active users do X">

## 2. What (untagged brainstorm)

Brainstorm every feature you imagine. Do not filter yet.

- <feature>
- <feature>
- <feature>
- <feature>
- <feature>
- <feature>
- <feature>

## 3. Scope triage

Tag each feature above with one of:

- **MVP** — new user cannot succeed without it on day 1.
- **Growth** — what you add once MVP proves the Why.
- **Vision** — the end state, aspirational.

Rule: if you can cut a feature and a new user still gets value, it is NOT MVP.

## 4. Who (personas)

See 'personas.md' for full profiles. Summary here:

- **<Persona A>** — <one-line: role + goal>
- **<Persona B>** — <one-line: role + goal>
- **<Persona C>** — <one-line: role + goal>

## 5. How

<1-2 paragraphs: technical approach, business model, GTM, etc. Flag new bounded contexts or external integrations — they become the Where phase.>

## 6. When

Target MVP timeline: **%d weeks**.

| Quarter  | Milestone | Scope                              |
|----------|-----------|------------------------------------|
| Q1       | MVP       | <list MVP features here>           |
| Q2       | Growth    | <list Growth features here>        |
| Q3+      | Vision    | <list Vision features here>        |

## 7. Where (bounded contexts)

| Context       | Type            | Notes                              |
|---------------|-----------------|------------------------------------|
| <name>        | new / existing  | <one-line description>             |
| <name>        | new / existing  | <one-line description>             |

If most contexts are "new" → expect a longer architecture sprint after inception. If most are "existing" → brownfield path; scope the MVP to leverage what is there.

---

## MVP cut

The 3-7 features we ship first (in priority order):

1. <feature> — covers <persona>'s <top job>
2. <feature>
3. <feature>

Each becomes a spec under 'specs/<NNNN>-<slug>/' via the nova-feature skill. Do NOT bundle multiple MVP features into one spec — one feature per spec so each can ship independently.

After MVP is cut:

1. Update '.radiant-harness/state.md' with 'current_product' and 'mvp_features'.
2. For each MVP feature, open a FRESH context and run 'radiant spec <feature>'.
3. Close this inception context — don't start spec'ing in it.

---

_Generated by 'radiant product' on %s. See the 'nova-product' skill for the full Decision Tree, anti-patterns, and failure modes._
`,
		slug, vision, mvpWeeks, time.Now().UTC().Format("2006-01-02"))
}

// renderPersonasTemplate returns the starter personas.md file
// with 2-4 placeholder slots (default 3). The user/agent fills in
// each persona after the Who phase of the inception.
func renderPersonasTemplate() string {
	return `# Personas

> Generated by 'radiant product'. Fill in 2-4 personas — one
> paragraph each. See the nova-product skill for what each section
> needs to contain.

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>

---

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>

---

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>
`
}


// renderADR produces a Nygard-format ADR template. The user is
// expected to fill in Context, Decision, and Consequences after
// the file is generated.
func renderADR(seq int, decision, status string) string {
	if status == "" {
		status = "proposed"
	}
	switch status {
	case "proposed", "accepted", "deprecated", "superseded":
		// ok
	default:
		status = "proposed"
	}
	return fmt.Sprintf(`# %04d. %s

## Status

%s

> Status transitions: proposed → accepted (when team agrees) →
> deprecated or superseded (when replaced). Edit this section in
> place to record the transition.

## Context

What forces are at play? What problem are we solving? What
constraints exist?

**Alternatives considered** (fill in at least 2; ADRs are valuable
*because* they record what was rejected, not only what was chosen):

- **Alternative A: <name>** — <one-line description>
  - Pro: ...
  - Con: ...
- **Alternative B: <name>** — <one-line description>
  - Pro: ...
  - Con: ...

## Decision

We will <the chosen approach>.

(One paragraph. State the decision clearly so a reader who knows
nothing about the discussion can understand what was decided and
why.)

## Consequences

What becomes easier? What becomes harder? What trade-offs did we
accept?

### Positive

- ...

### Negative

- ...

### Neutral

- (Anything that changes but isn't clearly positive or negative)

---

_Generated by 'radiant adr' on %s. Edit the placeholders above.
See the 'adr' skill ('.radiant-harness/skills/adr/SKILL.md') for
the full Decision Tree, anti-patterns, and failure modes._
`, seq, decision, status, time.Now().UTC().Format("2006-01-02"))
}

// summaryFor produces the human-readable one-liner that goes into
// state.md's `last_summary` field. Combines the user's note with the
// feature slug (if any) so future sessions can grep it.
func summaryFor(note, feature string) string {
	if note != "" {
		return note
	}
	if feature != "" {
		return "Last session worked on " + feature
	}
	return "Last session"
}

// we fall back to a direct write — rename-over-existing is a no-op
// there.
func writeTraceToFile(e *engine.Engine, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }

	if err := e.WriteTraceJSONL(tmp); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write jsonl: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// runAutodata implements the "Autodata" pattern (Kulikov et al.,
// 2026) for skill authoring: prompt an LLM with a domain
// description + the schema contract; the LLM generates a
// candidate frontmatter.yaml + SKILL.md. We write to a draft
// location and surface a review prompt before installation.
//
// The LLM call is gated on API key presence (RADIANT_OPENAI_API_KEY
// or RADIANT_ANTHROPIC_API_KEY). Without a key, we emit a clear
// error with a stub template the user can fill manually.
func runAutodata(skillName, domain, out string, dryRun bool) error {
	if skillName == "" {
		return fmt.Errorf("skill name required")
	}
	if domain == "" {
		return fmt.Errorf("--domain prompt required (e.g. --domain='reinsurance pricing for P&C carriers')")
	}

	if out == "" {
		out = filepath.Join("internal", "skill", "skills", skillName)
	}

	// Check LLM availability (vendor-neutral: OpenRouter first,
	// then OpenAI, then Anthropic — never bias toward one).
	apiKey := ""
	provider := ""
	if v := os.Getenv("RADIANT_OPENROUTER_API_KEY"); v != "" {
		apiKey = v
		provider = "openrouter"
	} else if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		apiKey = v
		provider = "openrouter"
	} else if v := os.Getenv("RADIANT_OPENAI_API_KEY"); v != "" {
		apiKey = v
		provider = "openai"
	} else if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		apiKey = v
		provider = "openai"
	} else if v := os.Getenv("RADIANT_ANTHROPIC_API_KEY"); v != "" {
		apiKey = v
		provider = "anthropic"
	} else if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		apiKey = v
		provider = "anthropic"
	}

	if apiKey == "" {
		// No key — emit stub + clear instructions
		return emitAutodataStub(skillName, domain, out, dryRun)
	}

	// Resolve model. OpenRouter requires `provider/model` format
	// (e.g. "deepseek/deepseek-chat"); other providers use bare names.
	modelName := os.Getenv("RADIANT_MODEL")
	if modelName == "" {
		switch provider {
		case "openrouter":
			modelName = "deepseek/deepseek-chat"
		case "anthropic":
			modelName = "claude-sonnet-4-5"
		default:
			modelName = "gpt-4o"
		}
	}
	model, ok := resolveModelSilent(modelName, provider, apiKey)
	if !ok {
		return fmt.Errorf("could not resolve model %s (provider %s)", modelName, provider)
	}

	client := llm.NewClient(model)
	ctx, cancel := context.WithTimeout(context.Background(), 120*1_000_000_000) // 120s
	defer cancel()

	systemPrompt := autodataSystemPrompt(skillName)
	userPrompt := autodataUserPrompt(skillName, domain)

	fmt.Printf("  → Generating skill %q via %s ...\n", skillName, modelName)
	resp, err := client.SimpleChat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response into frontmatter + SKILL.md
	frontmatter, skillMD, err := parseAutodataResponse(resp)
	if err != nil {
		return fmt.Errorf("parse LLM response: %w\n\n--- raw response ---\n%s", err, resp)
	}

	if dryRun {
		fmt.Println("--- frontmatter.yaml ---")
		fmt.Println(frontmatter)
		fmt.Println("--- SKILL.md ---")
		fmt.Println(skillMD)
		return nil
	}

	// Write files
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", out, err)
	}
	if err := os.WriteFile(filepath.Join(out, "frontmatter.yaml"), []byte(frontmatter), 0o644); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ Generated %s/{frontmatter.yaml, SKILL.md}\n", out)
	fmt.Println("  ⚠ REVIEW before installing:")
	fmt.Println("    1. Edit frontmatter.yaml + SKILL.md")
	fmt.Println("    2. Validate: radiant skills validate")
	fmt.Println("    3. Install: copy to internal/skill/skills/ and rebuild")
	return nil
}

// emitAutodataStub writes a manual-fill template when no LLM
// API key is configured. Better than failing silently.
func emitAutodataStub(skillName, domain, out string, dryRun bool) error {
	frontmatter := fmt.Sprintf(`name: %s
version: 0.1.0
description: |
  <TODO: describe what this skill covers>
when_to_use: |
  <TODO: when should this skill be used?>
tier_eligible:
  - feature

inputs:
  - name: example_input
    type: string
    required: true
    description: <TODO: describe this input>

outputs:
  - path: docs/%s/example.md
    type: artifact
    description: <TODO: describe this output>

gates:
  - name: example-gate
    description: <TODO: describe a release-blocking gate>

context_provides:
  - state.md

commands_available: []

related_skills: []

anti_patterns:
  - <TODO: list common anti-patterns>

author: <TODO>
license: MIT
`, skillName, skillName)

	skillMD := fmt.Sprintf(`# Skill: %s

> Auto-generated stub (no LLM API key configured).
> Domain prompt: %s

## Decision tree

<TODO: ASCII flowchart>

## Workflow

### Step 1: <TODO>

<TODO>

## Examples

<TODO>

## Anti-patterns

<TODO>

## Failure modes

| Failure | Recovery |
|---------|----------|
| <TODO> | <TODO> |

## Related skills

<TODO>
`, skillName, domain)

	if dryRun {
		fmt.Println("--- frontmatter.yaml ---")
		fmt.Println(frontmatter)
		fmt.Println("--- SKILL.md ---")
		fmt.Println(skillMD)
		fmt.Println("\n(no LLM API key; set RADIANT_OPENROUTER_API_KEY (recommended; vendor-neutral), RADIANT_OPENAI_API_KEY, or RADIANT_ANTHROPIC_API_KEY to generate via LLM)")
		return nil
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", out, err)
	}
	if err := os.WriteFile(filepath.Join(out, "frontmatter.yaml"), []byte(frontmatter), 0o644); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ Stub written to %s (no LLM API key; set RADIANT_OPENROUTER_API_KEY (recommended; vendor-neutral), RADIANT_OPENAI_API_KEY, or RADIANT_ANTHROPIC_API_KEY to use LLM)\n", out)
	return nil
}

func autodataSystemPrompt(skillName string) string {
	return fmt.Sprintf(`You are an expert skill author for the radiant-harness CLI.
Generate a skill named %q following the open MIT schema (docs/SKILL-SCHEMA.md).

OUTPUT FORMAT (strict):
1. First emit "===FRONTMATTER===" on its own line.
2. Then emit valid YAML frontmatter with: name, version, description (multi-line, |), when_to_use (multi-line), tier_eligible (list of: trivial, feature, architecture), inputs (list with name/type/required/description; type must be: string, number, enum, object, path), outputs (list with path/type/description; output type must be: artifact, report, commit, pr, decision — NOT object/path/string), gates (list with name/description), context_provides (list), commands_available (list), related_skills (list), anti_patterns (list of quoted strings).
3. Then emit "===SKILLMD===" on its own line.
4. Then emit SKILL.md content with sections: "# Skill: <name>" (title), "## Decision tree", "## Workflow" (numbered steps), "## Examples" (2-3 concrete examples), "## Anti-patterns" (table or list), "## Failure modes" (table), "## Related skills".

CRITICAL RULES:
- inputs.*.type MUST be one of: string, number, enum, object, path (NOT list, NOT integer).
- outputs.*.type MUST be one of: artifact, report, commit, pr, decision (NOT object, NOT string).
- descriptions in YAML must NOT contain unquoted colons.
- SKILL.md must include "# Skill: <name>", "## Decision tree", "## Workflow", "## Examples", "## Anti-patterns", "## Failure modes", "## Related skills".
- Be specific; cite real methodologies, real metrics, real failure modes.
`, skillName)
}

func autodataUserPrompt(skillName, domain string) string {
	return fmt.Sprintf("Generate the skill %q for the domain: %s\n\nRemember the output format: ===FRONTMATTER=== then YAML, then ===SKILLMD=== then Markdown.", skillName, domain)
}

// parseAutodataResponse splits the LLM response on the
// ===FRONTMATTER=== / ===SKILLMD=== markers.
func parseAutodataResponse(resp string) (frontmatter, skillMD string, err error) {
	const fmMarker = "===FRONTMATTER==="
	const mdMarker = "===SKILLMD==="
	fmIdx := strings.Index(resp, fmMarker)
	mdIdx := strings.Index(resp, mdMarker)
	if fmIdx < 0 || mdIdx < 0 || mdIdx <= fmIdx {
		return "", "", fmt.Errorf("missing markers; expected %q and %q", fmMarker, mdMarker)
	}
	frontmatter = strings.TrimSpace(resp[fmIdx+len(fmMarker) : mdIdx])
	skillMD = strings.TrimSpace(resp[mdIdx+len(mdMarker):])
	return frontmatter, skillMD, nil
}

func resolveModelSilent(modelName, provider, apiKey string) (llm.Model, bool) {
	m, err := resolveModel(modelName, provider, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ could not resolve --planner/--implementer model %q: %v\n  → falling back to default --model\n", modelName, err)
		return llm.Model{}, false
	}
	return m, true
}

func resolveModel(modelName, provider, apiKey string) (llm.Model, error) {
	// Check environment variables for API key first
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Try preset
	if preset, ok := llm.GetPreset(modelName, apiKey); ok {
		return preset, nil
	}

	if apiKey == "" {
		return llm.Model{}, fmt.Errorf("no API key provided. Use --api-key flag or set OPENROUTER_API_KEY env var")
	}

	// Build custom model
	providerType := llm.Provider(provider)
	if providerType == "" {
		providerType = llm.ProviderOpenRouter
	}

	return llm.Model{
		Provider:  providerType,
		Model:     modelName,
		APIKey:    apiKey,
		MaxTokens: 8192,
	}, nil
}

// runDoctor prints a diagnostic report of the local environment. Each
// check prints ✓ / ⚠ / ✗ and explains what to do if something's wrong.
// The function never returns an error — diagnostics are informational.
func runDoctor(root string) error {
	fmt.Println("  radiant doctor — environment diagnostic")
	fmt.Println()

	checkOK := func(label string) {
		fmt.Printf("  ✓ %s\n", label)
	}
	checkWarn := func(label, advice string) {
		fmt.Printf("  ⚠ %s\n    %s\n", label, advice)
	}
	checkFail := func(label, advice string) {
		fmt.Printf("  ✗ %s\n    %s\n", label, advice)
	}

	// 1. PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		checkFail("PATH not set", "export PATH=$PATH:/usr/local/bin:/opt/homebrew/bin")
	} else {
		checkOK("PATH set (" + strconv.Itoa(len(pathEnv)) + " chars)")
	}

	// 2. Supported agents on PATH
	fmt.Println("\n  Agents:")
	agents := []string{"claude", "codex", "copilot", "cursor", "gemini"}
	for _, name := range agents {
		if _, err := exec.LookPath(name); err == nil {
			checkOK(name + " available")
		} else {
			checkWarn(name+" not found", "agent is optional — install only the ones you use")
		}
	}

	// 3. LLM provider API keys
	fmt.Println("\n  LLM providers:")
	providers := []struct {
		name   string
		envKey string
		note   string
	}{
		{"OpenRouter", "OPENROUTER_API_KEY", "covers all OpenRouter presets"},
		{"OpenAI", "OPENAI_API_KEY", "for direct OpenAI access"},
		{"Anthropic", "ANTHROPIC_API_KEY", "for direct Anthropic (requires native client)"},
		{"Groq", "GROQ_API_KEY", "for Groq-hosted models (ultra-low latency)"},
		{"Mistral", "MISTRAL_API_KEY", "for Mistral-hosted models"},
		{"xAI", "XAI_API_KEY", "for Grok models"},
	}
	anyKey := false
	for _, p := range providers {
		if os.Getenv(p.envKey) != "" {
			checkOK(p.name + ": " + p.envKey + " set")
			anyKey = true
		} else {
			checkWarn(p.name+": "+p.envKey+" not set", p.note)
		}
	}
	if !anyKey {
		fmt.Println("\n  ⚠ No LLM API key set. `radiant run` will fail without one.")
		fmt.Println("    Set one of the env vars above, or pass --api-key=… to `radiant run`.")
	}

	// 4. Gate binaries (test runners, type checkers)
	fmt.Println("\n  Gate binaries:")
	gates := []string{"node", "npm", "pnpm", "yarn", "go", "make", "pytest", "python3", "cargo"}
	for _, name := range gates {
		if _, err := exec.LookPath(name); err == nil {
			checkOK(name + " available")
		} else {
			checkWarn(name+" not found", "install if you plan to use it as a gate command")
		}
	}

	// 5. .radiant-harness state directory
	fmt.Println("\n  Project state:")
	stateDir := filepath.Join(root, ".radiant-harness")
	if info, err := os.Stat(stateDir); err == nil {
		if info.IsDir() {
			checkOK(stateDir + " exists")
		}
	} else {
		checkWarn(stateDir+" not found", "run `radiant init .` to create the harness state directory")
	}

	// 6. Version
	fmt.Println("\n  Version:")
	fmt.Printf("    radiant v%s\n", version)
	fmt.Printf("    Go module: github.com/quant-risk/radiant-harness\n")

	fmt.Println()
	return nil
}

// evalRun captures one iteration of the eval loop.
type evalRun struct {
	LatencyMs    int64  `json:"latency_ms"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Error        string `json:"error,omitempty"`
}

// evalResult is the full eval output for one model.
type evalResult struct {
	Model       string    `json:"model"`
	Runs        int       `json:"runs"`
	Successful  int       `json:"successful"`
	MedianMs    int64     `json:"median_latency_ms"`
	MeanMs      int64     `json:"mean_latency_ms"`
	TotalInTok  int       `json:"total_input_tokens"`
	TotalOutTok int       `json:"total_output_tokens"`
	TotalCost   float64   `json:"total_cost_usd"`
	Iterations  []evalRun `json:"iterations"`
}

// runEval sends `prompt` to `model` exactly `runs` times and reports
// latency / token / cost statistics. The output is a markdown table
// plus an optional JSON file via --output for trend tracking.
func runEval(ctx context.Context, model, prompt string, runs int, outputPath string) error {
	// Light build: never reads API key env vars. Inference is delegated
	// to the host agent via MCP sampling/createMessage. If no agent is
	// connected, the SamplingBackend returns a clear "no host agent"
	// error after 5s (see internal/llm/sampling.go).

	// Resolve model preset without an API key — the preset table is
	// metadata (provider + model id + max tokens), not credentials.
	m, ok := llm.GetPreset(model, "")
	if !ok {
		return fmt.Errorf("unknown model preset %q; run `radiant models` for the list", model)
	}
	client := llm.NewClient(m)

	fmt.Printf("  radiant eval — model=%s runs=%d\n", model, runs)
	fmt.Printf("  prompt: %s\n\n", truncateForDisplay(prompt, 80))
	fmt.Println("  (delegating to host agent via MCP sampling — no API key required)")

	results := evalResult{Model: model, Runs: runs, Iterations: make([]evalRun, runs)}

	var latencies []int64
	for i := 0; i < runs; i++ {
		start := time.Now()
		resp, err := client.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}})
		latency := time.Since(start).Milliseconds()

		run := evalRun{LatencyMs: latency}
		if err != nil {
			run.Error = err.Error()
			fmt.Printf("  [%d/%d] ✗ %s (%dms)\n", i+1, runs, err, latency)
		} else {
			run.InputTokens = resp.Usage.PromptTokens
			run.OutputTokens = resp.Usage.CompletionTokens
			fmt.Printf("  [%d/%d] ✓ %dms, %d+%d tok\n",
				i+1, runs, latency, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			latencies = append(latencies, latency)
			results.Successful++
			results.TotalInTok += run.InputTokens
			results.TotalOutTok += run.OutputTokens
		}
		results.Iterations[i] = run
	}

	// Compute median + mean from successful runs.
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		results.MedianMs = latencies[len(latencies)/2]
		var sum int64
		for _, l := range latencies {
			sum += l
		}
		results.MeanMs = sum / int64(len(latencies))
	}
	results.TotalCost = llm.CostUSD(model, results.TotalInTok, results.TotalOutTok)

	fmt.Println()
	fmt.Printf("  Median latency : %dms\n", results.MedianMs)
	fmt.Printf("  Mean latency   : %dms\n", results.MeanMs)
	fmt.Printf("  Success rate   : %d/%d\n", results.Successful, runs)
	fmt.Printf("  Total tokens   : %d in + %d out = %d\n",
		results.TotalInTok, results.TotalOutTok, results.TotalInTok+results.TotalOutTok)
	fmt.Printf("  Estimated cost : %s\n", llm.FormatCost(results.TotalCost))

	if outputPath != "" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal results: %w", err)
		}
		if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		fmt.Printf("  Saved JSON to %s\n", outputPath)
	}
	return nil
}

func truncateForDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// resolveLoopLLMCreds is a Light-build stub. The Light binary never
// reaches an HTTP LLM provider — every inference goes through MCP
// sampling to the host agent. The signature is preserved so callers
// (cmd_loop, mcpRunFull) compile unchanged. The returned API key and
// base URL are always empty; callers must rely on the SamplingBackend
// in internal/llm/client.go for actual inference.
func resolveLoopLLMCreds(baseURLOverride string) (apiKey, baseURL string) {
	return "", ""
}
