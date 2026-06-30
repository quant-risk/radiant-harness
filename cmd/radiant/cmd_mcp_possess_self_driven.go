// Self-driven possession (v3.6.0+)
//
// When the host agent does not implement sampling/createMessage —
// either because hostdetect.ResolveSupport reported false from a prior
// probe, or because the live attempt returned JSON-RPC -32601 — the
// harness no longer degrades to a hollow stub. Instead it runs
// `runSelfDrivenPossess`, a deterministic 4-phase pipeline that:
//
//   1. **discover** — filesystem sniff + bundled-skill scan → CONTEXT.md
//   2. **plan**     — spec.md + tasks.md templated against the task slug
//   3. **execute**  — scaffold scripts/, docs/, AGENTS.md with placeholders
//      that the host agent can fill in with its own tools
//   4. **verify**   — every artefact gets a `[host-agent: fill in]`
//      marker so the next agent that opens the project knows exactly
//      which sections need its understanding
//
// The state file, phases, and bootstrap layout match the LLM-driven
// path so `radiant_phase_status` reports the same shape regardless of
// which path produced the artefacts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/hostdetect"
	"github.com/quant-risk/radiant-harness/v3/internal/skill"
)

// selfDrivenSkillHints maps task keywords (lowercased, whitespace-normalised)
// to bundled skill names. The match is best-effort: agents running in
// self-driven mode get a starting point for which skills to read, not a
// guarantee. Unrecognised tasks fall back to a single generic pointer
// to `nova-feature` (the SDD / spec-driven-development skill).
//
// IMPORTANT: every value MUST reference a skill that exists in
// `internal/skill/skills/<value>/SKILL.md`. Drift between this map and
// the bundle was the v3.7.1 hot-fix bug (4/13 hints pointed at ghosts
// like `credit-risk-modeling`, `risk-management`, `ml-modeling`,
// `regulatory-compliance`). `make audit-skills` enforces this invariant.
var selfDrivenSkillHints = []struct {
	keyword string
	skill   string
}{
	{"credit", "credit-risk"},
	{"risk", "credit-risk"},      // generic "risk" defaults to credit; specific risks (market-risk, operational-risk, liquidity-risk, model-risk) hit second-pass verbatim match below
	{"fraud", "fraud-detection"},
	{"model", "ml"},
	{"ml", "ml"},
	{"forecast", "ml"},
	{"spec", "nova-feature"},
	{"sdd", "nova-feature"},
	{"agent", "camada-agentica"},
	{"harness", "camada-agentica"},
	{"compliance", "regulatory"},
	{"regulatory", "regulatory"},
	{"basel", "regulatory"},
	{"ifrs", "regulatory"},
}

// runSelfDrivenPossess runs a full 4-phase loop without any LLM call.
// See package doc. Returns the same *possessState shape as the
// LLM-driven path so callers don't have to branch on the path taken.
func runSelfDrivenPossess(ctx context.Context, workdir, task, profile string, w io.Writer, reason string) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	if profile == "" {
		profile = "standard"
	}
	id := taskID(workdir, task)
	st, err := loadPossessState(workdir, id)
	if err != nil {
		st = newPossessState(workdir, task, id)
	}

	msgs, err := bootstrapPossess(workdir)
	if err != nil {
		return st, err
	}
	if !st.BootstrapDone {
		st.BootstrapDone = true
		st.BootstrapMessages = msgs
	}

	fmt.Fprintf(w, "run id: %s\nworkdir: %s\ntask: %s\nprofile: %s\n", id, workdir, task, profile)
	if reason != "" {
		fmt.Fprintf(w, "mode:    self-driven (%s)\n", reason)
	}
	fmt.Fprintln(w)

	slug := selfDrivenSlugify(task)
	if slug == "" {
		slug = "task"
	}
	specDir := filepath.Join(workdir, "specs", "0001-"+slug)

	phases := []struct {
		name string
		fn   func() error
	}{
		{"discover", func() error { return selfDrivenDiscover(workdir, task, profile, w) }},
		{"plan", func() error { return selfDrivenPlan(workdir, specDir, task, slug, profile, w) }},
		{"execute", func() error { return selfDrivenExecute(workdir, specDir, task, profile, w) }},
		{"verify", func() error { return selfDrivenVerify(workdir, specDir, w) }},
	}

	for _, ph := range phases {
		if r := st.Phases[ph.name]; r != nil && r.Status == "done" {
			continue
		}
		r := st.Phases[ph.name]
		r.Status = "in_progress"
		r.StartedAt = time.Now().UTC()
		st.CurrentPhase = ph.name
		_ = savePossessState(st)

		if err := ph.fn(); err != nil {
			r.Status = "error"
			r.Error = err.Error()
			r.EndedAt = time.Now().UTC()
			_ = savePossessState(st)
			fmt.Fprintf(w, "phase %s FAILED: %s\n", ph.name, err)
			return st, err
		}

		// Clear any prior transient error from the LLM-driven attempt
		// that this self-driven run replaced; the state file should
		// reflect the final outcome, not the in-flight failure.
		r.Status = "done"
		r.Error = ""
		r.EndedAt = time.Now().UTC()
		_ = savePossessState(st)
		fmt.Fprintf(w, "  ✓ %s\n", ph.name)
	}

	st.CurrentPhase = "done"
	_ = savePossessState(st)

	// Final artifact list.
	artifacts, _ := filepath.Glob(filepath.Join(specDir, "**"))
	for _, a := range artifacts {
		rel, _ := filepath.Rel(workdir, a)
		st.Artifacts = append(st.Artifacts, rel)
	}
	sort.Strings(st.Artifacts)
	_ = savePossessState(st)

	fmt.Fprintf(w, "\nall phases done (self-driven). state=%s\n", possessStatePath(workdir, id))
	fmt.Fprintf(w, "Next step: open the project in your agent and run radiant_phase_status(task_id=%q)\n", id)
	fmt.Fprintln(w, "to see exactly which files are templated vs empty. Fill the [host-agent: fill in] sections.")
	return st, nil
}

// selfDrivenSlugify lowercases, replaces non-alphanumeric with '-',
// trims, and caps length. Distinct from the helper in cmd/radiant/helpers.go
// which is the canonical slugify; we keep the renames local so future
// drift between the two stays visible.
func selfDrivenSlugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
		s = strings.Trim(s, "-")
	}
	return s
}

// selfDrivenMarker is the inline placeholder text written into every
// templated artefact. The next agent that opens the file is expected
// to replace it with real content. The marker includes the task id so
// the agent can correlate each placeholder back to its phase.
func selfDrivenMarker(taskID, phase string) string {
	return fmt.Sprintf("[host-agent: fill in — task_id=%s phase=%s]", taskID, phase)
}

// selfDrivenDiscover fingerprints the project (stack + manifest files +
// skill hints) and writes `.radiant-harness/CONTEXT.md`. Pure local —
// no inference.
func selfDrivenDiscover(workdir, task, profile string, w io.Writer) error {
	manifest := sniffManifest(workdir)
	skillHints := pickSelfDrivenSkills(task)

	var b strings.Builder
	b.WriteString("# CONTEXT.md\n\n")
	b.WriteString("> Generated by `radiant-harness` self-driven mode (v3.6.0+).\n")
	b.WriteString("> The host agent does not implement MCP sampling, so this file is templated.\n")
	b.WriteString("> Edit the `[host-agent: fill in]` sections below.\n\n")
	b.WriteString("## Project fingerprint\n\n")
	if len(manifest) == 0 {
		b.WriteString("- <no recognised manifest file found>\n")
	}
	for _, m := range manifest {
		b.WriteString(fmt.Sprintf("- `%s`\n", m))
	}
	b.WriteString("\n## Task\n\n")
	b.WriteString("```\n")
	b.WriteString(strings.TrimSpace(task))
	b.WriteString("\n```\n\n")
	b.WriteString("## Profile\n\n")
	b.WriteString("- `" + profile + "`\n\n")
	b.WriteString("## Bundled skills to read first\n\n")
	if len(skillHints) == 0 {
		b.WriteString("- `nova-feature` (default — read this for SDD methodology)\n")
	} else {
		for _, s := range skillHints {
			b.WriteString("- `" + s + "`\n")
		}
	}
	b.WriteString("\n")

	dir := filepath.Join(workdir, ".radiant-harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "CONTEXT.md"), []byte(b.String()), 0o644)
}

// sniffManifest returns the relative paths of recognised project
// manifest files that exist in workdir. Order is deterministic for
// reproducible output.
func sniffManifest(workdir string) []string {
	candidates := []string{
		"go.mod", "go.sum",
		"package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json",
		"pyproject.toml", "requirements.txt", "setup.py", "Pipfile",
		"Cargo.toml", "Cargo.lock",
		"pom.xml", "build.gradle", "build.gradle.kts",
		"Gemfile", "composer.json",
		"AGENTS.md", "README.md", "CONTEXT.md",
		".radiant-harness/manifest.json",
	}
	var out []string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(workdir, c)); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// pickSelfDrivenSkills returns up to 3 skill names from the hint map
// whose keyword appears in the task, falling back to {"nova-feature"}.
func pickSelfDrivenSkills(task string) []string {
	t := strings.ToLower(task)
	seen := map[string]bool{}
	var out []string
	// First pass: hits from the hint map.
	for _, h := range selfDrivenSkillHints {
		if strings.Contains(t, h.keyword) && !seen[h.skill] {
			seen[h.skill] = true
			out = append(out, h.skill)
			if len(out) >= 3 {
				break
			}
		}
	}
	// Second pass: ask the skill index for any skill whose name
	// appears verbatim in the task (catches e.g. "credit-risk",
	// "market-risk", "ml" — even when the keyword map missed it).
	if rl := skillIndex(); rl != nil {
		for _, s := range rl {
			name := strings.ToLower(s)
			if strings.Contains(t, name) && !seen[name] {
				seen[name] = true
				out = append(out, name)
				if len(out) >= 3 {
					break
				}
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "nova-feature")
	}
	return out
}

// skillIndex is a best-effort lazy-loaded list of bundled skill names.
// Any error (no skills, no bundle dir) returns nil — pickSelfDrivenSkills
// handles the fallback itself.
func skillIndex() []string {
	bundle, err := skill.Bundle()
	if err != nil {
		return nil
	}
	var names []string
	for _, s := range bundle {
		if s.Name != "" {
			names = append(names, s.Name)
		}
	}
	return names
}

// selfDrivenPlan writes spec.md + tasks.md for specs/0001-<slug>/.
// Each file is a template with explicit AC/Tasks stubs and a marker
// at the bottom so the host agent knows what to fill in.
func selfDrivenPlan(workdir, specDir, task, slug, profile string, w io.Writer) error {
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return err
	}

	taskID := taskID(workdir, task)
	marker := selfDrivenMarker(taskID, "plan")

	specBody := fmt.Sprintf(`# spec.md — 0001-%s

> Templated by `+"`radiant-harness`"+` self-driven mode (v3.6.0+) on %s.
> Replace the [host-agent: fill in] markers with real acceptance criteria.

## Goal

`+"```\n%s\n```\n"+`

## Acceptance criteria

- AC1: %s (high-level — refine below)
- AC2: %s (high-level — refine below)
- AC3: %s (high-level — refine below)

## Non-goals

- %s (sketch; expand)

## Profile

- %s

---
%s
`, slug, time.Now().UTC().Format("2006-01-02"), strings.TrimSpace(task),
		marker, marker, marker, marker, profile, marker)
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specBody), 0o644); err != nil {
		return err
	}

	tasksBody := fmt.Sprintf(`# tasks.md — 0001-%s

> Templated by `+"`radiant-harness`"+` self-driven mode (v3.6.0+).
> The host agent should fill in concrete subtasks under each stub below.

## Tasks

1. %s — break into 2–3 concrete subtasks.
2. %s — break into 2–3 concrete subtasks.
3. %s — break into 2–3 concrete subtasks.

## Gates (suggested)

- `+"`go build ./...`"+` (or stack equivalent) → PASS
- `+"`go test ./...`"+` (or stack equivalent) → PASS
- %s — describe the manual verification step.

---
%s
`, slug, marker, marker, marker, marker, marker)
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasksBody), 0o644); err != nil {
		return err
	}
	return nil
}

// selfDrivenExecute populates scripts/ with a single entrypoint script
// and a docs/README.md. The script is intentionally a no-op so the
// host agent can replace it with the real logic without a layout
// reshuffle.
func selfDrivenExecute(workdir, specDir, task, profile string, w io.Writer) error {
	taskID := taskID(workdir, task)
	marker := selfDrivenMarker(taskID, "execute")

	// scripts/run.sh — POSIX entrypoint the host agent fills in.
	if err := os.MkdirAll(filepath.Join(workdir, "scripts"), 0o755); err != nil {
		return err
	}
	runSh := fmt.Sprintf(`#!/usr/bin/env bash
# scripts/run.sh — entrypoint for spec 0001-%s
#
# Generated by radiant-harness self-driven mode (v3.6.0+).
# Replace the %s marker with the real command.

set -euo pipefail
cd "$(dirname "$0")/.."

# %s
echo "[radiant] run.sh is still templated — fill in the implementation." >&2
exit 2
`, selfDrivenSlugify(task), marker, marker)
	if err := os.WriteFile(filepath.Join(workdir, "scripts", "run.sh"), []byte(runSh), 0o755); err != nil {
		return err
	}

	// docs/README.md — explains the project to the next reader.
	if err := os.MkdirAll(filepath.Join(workdir, "docs"), 0o755); err != nil {
		return err
	}
	docsMD := fmt.Sprintf(`# docs/README.md

> Templated by `+"`radiant-harness`"+` self-driven mode on %s.

## What this project does

`+"```\n%s\n```\n"+`

## Layout produced by the harness

| Path | Origin | Status |
|---|---|---|
| `+"`AGENTS.md`"+` | generated (radiant-harness v3.6.0+) | templated |
| `+"`docs/README.md`"+` | generated this file | templated |
| `+"`docs/CONTEXT.md`"+` | generated (moved to .radiant-harness/CONTEXT.md in self-driven mode) | templated |
| `+"`specs/0001-%s/spec.md`"+` | templated | templated |
| `+"`specs/0001-%s/tasks.md`"+` | templated | templated |
| `+"`scripts/run.sh`"+` | templated entrypoint | templated |

## Next step

The host agent should read every templated file, replace each
`+"`%s`"+` marker with the real content, and then run the entrypoint
`+"`./scripts/run.sh`"+` to validate end-to-end.

---
%s
`, time.Now().UTC().Format("2006-01-02"),
		strings.TrimSpace(task), selfDrivenSlugify(task), selfDrivenSlugify(task), selfDrivenMarker(taskID, "docs"), marker)
	if err := os.WriteFile(filepath.Join(workdir, "docs", "README.md"), []byte(docsMD), 0o644); err != nil {
		return err
	}

	// .radiant-harness/handoff.md — short note explaining the state
	// to the next agent.
	handoff := fmt.Sprintf(`# .radiant-harness/handoff.md

> Generated by radiant-harness v3.6.0+ self-driven mode.

**Task ID:** %s
**Started:** %s
**Mode:** self-driven (sampling/createMessage is not implemented on the host)

## What the harness did

The harness could not drive the 4-phase loop through
`+"`sampling/createMessage`"+`. Instead, it ran the same phases using
deterministic templates:

- `+"`discover`"+` → `+"`docs/CONTEXT.md`"+` (project fingerprint, suggested skills)
- `+"`plan`"+`     → `+"`specs/0001-%s/spec.md`"+` + `+"`specs/0001-%s/tasks.md`"+`
- `+"`execute`"+`  → `+"`scripts/run.sh`"+`, `+"`docs/README.md`"+`
- `+"`verify`"+`   → this file + the `+"`radiant_phase_status`"+` snapshot

## What the host agent should do

1. Read `+"`docs/CONTEXT.md`"+` and the spec/tasks in `+"`specs/0001-%s/`"+`.
2. Replace every `+"`%s`"+` marker with the real content.
3. Run `+"`./scripts/run.sh`"+` and confirm it works end-to-end.
4. Call `+"`mcp__radiant__phase_status(task_id=\"%s\")`"+` to mark the run done.

If sampling becomes available (e.g. you switch to an agent that
implements `+"`sampling/createMessage`"+`), the next run will use the
LLM-driven path automatically — the harness detects this through
`+"`hostdetect.ResolveSupport`"+`.
`, taskID, time.Now().UTC().Format(time.RFC3339),
		selfDrivenSlugify(task), selfDrivenSlugify(task), selfDrivenSlugify(task), selfDrivenMarker(taskID, "execute"), taskID)
	if err := os.WriteFile(filepath.Join(workdir, ".radiant-harness", "handoff.md"), []byte(handoff), 0o644); err != nil {
		return err
	}

	return nil
}

// selfDrivenVerify finalises the run by writing a sanity report under
// .radiant-harness/verify.md and stamping the task-id in the state.
// No LLM is invoked; the report is a deterministic summary of what
// the agent did get plus what is still missing.
func selfDrivenVerify(workdir, specDir string, w io.Writer) error {
	// Walk every artefact the harness emitted, count templated vs
	// "filled" (heuristic: any line containing a [host-agent:
	// fill in] marker means templated).
	var totalFiles, templatedFiles int
	_ = filepath.WalkDir(workdir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		// Skip our internal state and large blobs.
		base := filepath.Base(path)
		if base == "state.json" || strings.HasSuffix(base, ".tmp") {
			return nil
		}
		if strings.HasSuffix(path, "/.git/") {
			return nil
		}
		totalFiles++
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), selfDrivenMarker("", "")) {
			templatedFiles++
		}
		return nil
	})
	filledFiles := totalFiles - templatedFiles

	body := fmt.Sprintf(`# .radiant-harness/verify.md

> Generated by radiant-harness self-driven mode on %s.

## Summary

| Metric | Value |
|---|---|
| Files the harness produced | %d |
| Still templated (need host agent's input) | %d |
| Already filled by the host agent | %d |

## Verdict

VERDICT: SELF_DRIVEN_BOOTSTRAPPED
SCORE: 0.50
EVIDENCE: harness scaffolded the project without sampling; host agent still needs to fill %d templated sections.
ESCALATE: false
ISSUES:
- %d files still contain placeholder markers — host agent must apply real content.

---
*This report is for the next agent that opens the project. It does not
assert task quality — it asserts that the scaffold was emitted.*
`, time.Now().UTC().Format(time.RFC3339), totalFiles, templatedFiles, filledFiles, templatedFiles, templatedFiles)
	return os.WriteFile(filepath.Join(workdir, ".radiant-harness", "verify.md"), []byte(body), 0o644)
}

// recordProbeFromError converts an error returned by a sampling chat
// call into a ProbeResult and persists it. Used by both
// runPossessWithBackend (when sampling fails) and runMCPServe's
// pre-flight probe goroutine (when the host answers).
func recordProbeFromError(agent hostdetect.AgentID, err error) {
	if agent == hostdetect.AgentUnknown {
		return
	}
	var supports bool
	var evidence hostdetect.ProbeEvidence
	switch {
	case err == nil:
		supports = true
		evidence = hostdetect.EvidenceProbeOK
	default:
		supports = false
		switch {
		case errorContains(err, "-32601") || errorContains(err, "method not found"):
			evidence = hostdetect.EvidenceUnsupported32601
		case errorContains(err, "deadline exceeded") || errorContains(err, "timeout") || errorContains(err, "no host agent"):
			evidence = hostdetect.EvidenceUnsupportedTimeout
		default:
			evidence = hostdetect.EvidenceUnsupportedIO
		}
	}
	_ = hostdetect.RecordProbe(hostdetect.ProbeResult{
		Agent:            agent,
		SupportsSampling: supports,
		ProbedAt:         time.Now().UTC(),
		Evidence:         evidence,
	})
}

// errorContains is a tiny helper to avoid pulling strings everywhere
// for one call site. Case-insensitive substring match.
func errorContains(err error, needle string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(needle))
}

// jsonMustMarshal is a debug helper kept local so future contributors
// don't need to import encoding/json just to one-line a debug log.
func jsonMustMarshal(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
