package main

// `radiant possess <task> <workdir> ...` — the entry-point MCP tool that
// the agent invokes when the user says "do X with this harness."
//
// Design rationale (2026-06-29):
//
// The previous `radiant_run` exposed the entire autonomous loop as a single
// MCP tool call. That design was the *wrong* size for an MCP tool:
//   - Real agents (Hermes, Codex GPT-5, OpenCode) failed because the
//     hidden complexity (4 phases × N iterations × sampling round-trips)
//     exceeded every host's outer tool-call timeout (Hermes 300 s, Codex
//     unbounded but throttled).
//   - Agents that did not see the tool fell back to running the harness
//     binary CLI commands directly (`radiant model`, `radiant profile`,
//     `radiant evaluate`), bypassing the harness loop entirely.
//
// The fix is to expose a *graduated* MCP surface:
//
//   1. `radiant_possess(task, workdir, profile)` — single tool, but
//      decomposed into bounded phases. Each phase = at most ONE
//      sampling/createMessage round-trip. State persisted to
//      `.radiant-harness/state/<task-id>.json` between phases so a
//      timeout or crash can resume from where the harness left off.
//
//   2. `radiant_phase_status(task_id)` — returns progress
//      (current_phase, completed_phases[], artifacts[], trace lines).
//
//   3. `radiant_skill_list()` + `radiant_skill_load(name)` — bounded
//      lookup tools. No sampling. The agent uses these to discover the
//      bundled domain skills and inject the relevant ones into its own
//      context *before* calling `radiant_possess`. This forces the
//      skills to be visible to the agent.
//
//   4. The legacy `radiant_run(goal)` is kept as a thin alias for
//      `radiant_possess(task=goal)` so existing setups keep working.
//
// All four primitives are non-blocking until the harness needs a sampling
// call. The agent calls them in any order, paused or inter-leaved with
// other tools, and the harness state file persists everything.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/hostdetect"
	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/scaffold"
	"github.com/spf13/cobra"
)

// taskID hashes the (workdir, task) tuple into a stable identifier so a
// subsequent call from the same host can resume the same run.
func taskID(workdir, task string) string {
	h := sha256.New()
	h.Write([]byte(workdir))
	h.Write([]byte{0})
	h.Write([]byte(task))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// possessState is the on-disk shape persisted between phases.
type possessState struct {
	TaskID            string             `json:"task_id"`
	Workdir           string             `json:"workdir"`
	Task              string             `json:"task"`
	StartedAt         time.Time          `json:"started_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	CurrentPhase      string             `json:"current_phase"` // discover|plan|execute|verify|done
	Phases            map[string]*phaseResult `json:"phases"`
	Artifacts         []string           `json:"artifacts"`
	BootstrapDone     bool               `json:"bootstrap_done"`
	BootstrapMessages []string           `json:"bootstrap_messages"`
}

type phaseResult struct {
	Status    string    `json:"status"` // pending|in_progress|done|error
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Output    string    `json:"output,omitempty"`
	Tokens    int       `json:"tokens,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// possessDir returns the per-task state directory inside .radiant-harness.
func possessDir(workdir, id string) string {
	return filepath.Join(workdir, ".radiant-harness", "state", "possess-"+id)
}

func possessStatePath(workdir, id string) string {
	return filepath.Join(possessDir(workdir, id), "state.json")
}

func loadPossessState(workdir, id string) (*possessState, error) {
	p := possessStatePath(workdir, id)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s possessState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func savePossessState(s *possessState) error {
	s.UpdatedAt = time.Now().UTC()
	dir := filepath.Dir(possessStatePath(s.Workdir, s.TaskID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(possessStatePath(s.Workdir, s.TaskID), data, 0o644)
}

func newPossessState(workdir, task, id string) *possessState {
	now := time.Now().UTC()
	return &possessState{
		TaskID:    id,
		Workdir:   workdir,
		Task:      task,
		StartedAt: now,
		UpdatedAt: now,
		Phases: map[string]*phaseResult{
			"discover": {Status: "pending"},
			"plan":     {Status: "pending"},
			"execute":  {Status: "pending"},
			"verify":   {Status: "pending"},
		},
		CurrentPhase: "discover",
	}
}

// registerPossessCmd attaches `radiant mcp possess` etc. to the parent
// `mcp` command. The actual MCP tool handlers live in runMCPServe.
func registerPossessCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "possess",
		Short: "Run a bounded-phase possession loop locally (mirrors the MCP tool)",
		Long: `Same algorithm as the mcp__radiant__possess tool, but invokable
from the shell for debugging or one-off runs. Each phase is run inline
(no sampling; uses an inline stub). Useful for self-test and CI.`,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			task, _ := cmd.Flags().GetString("task")
			workdir, _ := cmd.Flags().GetString("workdir")
			if task == "" {
				return fmt.Errorf("--task required")
			}
			_, err := runPossessForCLI(cmd.Context(), workdir, task, os.Stdout)
			return err
		},
	}
	cmd.Flags().String("task", "", "The agent's original user prompt / goal")
	cmd.Flags().String("workdir", "", "Project working directory (default: $PWD)")
	parent.AddCommand(cmd)
}

// bootstrapPossess scaffolds the project layout needed for radiant-possession
// to proceed. It delegates to the canonical scaffold.Init path so MCP
// possession produces the same skills, manifest, state, docs/, specs/, and
// scripts/ structure as `radiant init`, then fills the task handoff file.
// Idempotent (only writes files that don't exist).
func bootstrapPossess(workdir string) ([]string, error) {
	res := scaffold.Init(scaffold.Config{
		TargetDir: workdir,
		Agents:    []radiant.AgentID{},
		Force:     false,
		Version:   version,
	})
	msgs := []string{
		fmt.Sprintf("scaffold init: written=%d skipped=%d", res.Written, res.Skipped),
	}
	if len(res.Errors) > 0 {
		return msgs, fmt.Errorf("scaffold init: %s", strings.Join(res.Errors, "; "))
	}

	dirs := []string{
		filepath.Join(workdir, ".radiant-harness"),
		filepath.Join(workdir, "docs"),
		filepath.Join(workdir, "specs"),
		filepath.Join(workdir, "scripts"),
		filepath.Join(workdir, ".agent-context"),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return msgs, err
			}
			msgs = append(msgs, "mkdir "+d)
		}
	}
	agentsMD := filepath.Join(workdir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		body := strings.Join([]string{
			"# AGENTS.md",
			"",
			"This project is being driven by radiant-harness.",
			"",
			"Any AI agent that opens this directory should:",
			"",
			"  1. Read this file in full.",
			"  2. Continue the work in .agent-context/ (if present).",
			"  3. Use the existing specs/ directory as the source of truth for",
			"     the current task; do not invent a new goal.",
			"  4. Run `radiant --version` to confirm the harness is on PATH.",
			"  5. Call `mcp__radiant__possess` (or `mcp__radiant__run` on older",
			"     setups) with the task from .agent-context/task.md, the",
			"     workdir set to this directory.",
			"",
			"The harness owns: scaffold layout, task decomposition, gating",
			"(per specs/0001-*/tasks.md), and trace emission.",
			"",
			"The agent owns: reading skills, calling real tools (read_file,",
			"write_file, run_gate via Bash), calling the harness MCP tool with",
			"the right arguments.",
			"",
		}, "\n")
		if err := os.WriteFile(agentsMD, []byte(body), 0o644); err != nil {
			return msgs, err
		}
		msgs = append(msgs, "write "+agentsMD)
	}
	taskFile := filepath.Join(workdir, ".agent-context", "task.md")
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		// Best-effort: empty task file is fine; possess will fill it from arguments.
		_ = os.WriteFile(taskFile, []byte("(radiant will set this on first call)\n"), 0o644)
		msgs = append(msgs, "write "+taskFile+" (placeholder)")
	}
	return msgs, nil
}

// callSamplingOnce asks the host (via SamplingBackend) to perform the
// given phase. Bounded: ONE round-trip. Returns the assistant's text or
// an error. If the backend is non-MCP (debug/CI mode), fall back to
// running the phase as a deterministic stub.
func callSamplingOnce(ctx context.Context, backend llm.Backend, phase, system, userText string) (string, error) {
	msgs := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: userText},
	}
	resp, err := backend.Chat(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("sampling chat: %w", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("sampling: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

// runPossessForCLI is the entry point used by `radiant mcp possess --task=...`.
// In v3.6.0 it honours hostdetect.ResolveSupport the same way the MCP path
// does, so debugging the harness on a Codex box (or any host that we
// already know lacks sampling) produces the same self-driven scaffold
// instead of empty placeholders.
//
// The `RADIANT_FORCE_SAMPLING=1` env var escapes self-driven mode and
// runs the deterministic stub path — useful for verifying the stub
// output shape in a unit test or smoke run.
func runPossessForCLI(ctx context.Context, workdir, task string, w io.Writer) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	id := taskID(workdir, task)

	if os.Getenv("RADIANT_FORCE_SAMPLING") != "1" {
		detected := hostdetect.New().Detect()
		if detected.Agent != hostdetect.AgentUnknown {
			if supports, probed := hostdetect.ResolveSupport(detected.Agent); probed && !supports {
				fmt.Fprintf(w, "⚠ host %q has no sampling — routing `radiant mcp possess` to self-driven mode.\n",
					detected.Agent)
				fmt.Fprintln(w, "  Set RADIANT_FORCE_SAMPLING=1 to bypass and exercise the stub path.")
				fmt.Fprintln(w)
				return runSelfDrivenPossess(ctx, workdir, task, "standard", w,
					fmt.Sprintf("probe says %s has no sampling", detected.Agent))
			}
		}
	}

	st, err := loadPossessState(workdir, id)
	if err != nil {
		st = newPossessState(workdir, task, id)
	}
	fmt.Fprintf(w, "run id: %s\nworkdir: %s\ntask: %s\n\n", id, workdir, task)

	msgs, err := bootstrapPossess(workdir)
	if err != nil {
		return st, err
	}
	if !st.BootstrapDone {
		st.BootstrapDone = true
		st.BootstrapMessages = msgs
		_ = savePossessState(st)
	}

	fmt.Fprintln(w, "running phases (stub mode, no sampling)...")
	phases := []string{"discover", "plan", "execute", "verify"}
	descs := map[string]string{
		"discover": "Identify the project type, scan skills, surface relevant ones.",
		"plan":     "Decompose the task into ACs + tasks + gates.",
		"execute":  "Implement the code; run gates; iterate until green.",
		"verify":   "Adversarial verifier reviews the work; report verdict.",
	}
	for _, ph := range phases {
		st.Phases[ph].StartedAt = time.Now().UTC()
		st.Phases[ph].Status = "done"
		st.Phases[ph].EndedAt = time.Now().UTC()
		st.Phases[ph].Output = "[stub mode — set RADIANT_FORCE_SAMPLING=1 or wire MCP with sampling]\n\n" + descs[ph]
		st.CurrentPhase = ph
		_ = savePossessState(st)
		fmt.Fprintf(w, "  ✓ %s\n", ph)
	}
	st.CurrentPhase = "done"
	_ = savePossessState(st)

	fmt.Fprintf(w, "\nDone. State: %s\n", possessStatePath(workdir, id))
	return st, nil
}

// runPossessWithBackend is the MCP-bound path used by mcp__radiant__possess.
// It calls sampling/createMessage ONCE per phase. State is persisted after
// every phase so a timeout on phase N can be resumed at phase N.
//
// v3.6.0 routing:
//   - If hostdetect.ResolveSupport(detected agent).SupportsSampling is
//     false (probe-verified or well-attested), skip sampling entirely
//     and dispatch to runSelfDrivenPossess.
//   - Otherwise, try sampling. If the first call returns JSON-RPC -32601
//     we record it as evidence, then switch mid-run to the self-driven
//     pipeline so the rest of the phases still produce real artefacts.
func runPossessWithBackend(ctx context.Context, workdir, task, profile string, backend llm.Backend, w io.Writer) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	if profile == "" {
		profile = "standard"
	}
	id := taskID(workdir, task)

	// Pre-flight: if a prior probe already settled the question, skip
	// sampling without paying the cost of a doomed first call. Codex
	// is in knownSamplingUnsupported so the very first call here on a
	// Codex box dispatches to the self-driven pipeline immediately.
	detected := hostdetect.New().Detect()
	if detected.Agent != hostdetect.AgentUnknown {
		if supports, probed := hostdetect.ResolveSupport(detected.Agent); probed && !supports {
			fmt.Fprintf(w,
				"⚠ host %q does not implement sampling/createMessage "+
					"(probe-verified or well-attested; see hostdetect.ResolveSupport).\n"+
					"  Routing to self-driven pipeline.\n\n",
				detected.Agent)
			return runSelfDrivenPossess(ctx, workdir, task, profile, w,
				fmt.Sprintf("probe says %s has no sampling", detected.Agent))
		}
	}

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
		_ = savePossessState(st)
	}

	phases := []string{"discover", "plan", "execute", "verify"}
	// samplingUnsupportedLogged gates the warning so it only prints once
	// per possession run. After the first -32601 we hand off to the
	// self-driven pipeline, which still scaffolds a usable project
	// (spec.md, tasks.md, scripts/, docs/) instead of leaving it empty.
	samplingUnsupportedLogged := false
	for _, ph := range phases {
		r := st.Phases[ph]
		if r.Status == "done" {
			continue // already done from a previous run
		}
		r.Status = "in_progress"
		r.StartedAt = time.Now().UTC()
		st.CurrentPhase = ph
		_ = savePossessState(st)

		system, userText := phasePrompts(ph, task, workdir, st)
		out, err := callSamplingOnce(ctx, backend, ph, system, userText)
		if err != nil {
			if llm.IsSamplingUnsupported(err) {
				// Persist the failure as probe evidence so the NEXT
				// run's pre-flight short-circuits to self-driven
				// without paying the cost of this doomed call.
				recordProbeFromError(detected.Agent, err)
				if !samplingUnsupportedLogged {
					fmt.Fprintf(w,
						"\n⚠ sampling/createMessage is not implemented on this host "+
							"(JSON-RPC -32601).\n"+
							"  Switching to self-driven pipeline — the harness will scaffold\n"+
							"  spec.md, tasks.md, scripts/, docs/, .radiant-harness/ so the\n"+
							"  project is usable; the host agent (you) fills the\n"+
							"  [host-agent: fill in] markers with its own tools.\n\n")
					samplingUnsupportedLogged = true
				}
				// Mark the current phase as errored so the state file
				// tells the truth, then hand off to the deterministic
				// pipeline for the remaining work.
				r.Status = "error"
				r.Error = "host sampling unsupported: sampling/createMessage returned JSON-RPC -32601"
				r.EndedAt = time.Now().UTC()
				st.CurrentPhase = ph
				_ = savePossessState(st)
				return runSelfDrivenPossess(ctx, workdir, task, profile, w,
					"sampling unsupported mid-run")
			}
			r.Status = "error"
			r.Error = err.Error()
			r.EndedAt = time.Now().UTC()
			_ = savePossessState(st)
			fmt.Fprintf(w, "phase %s FAILED: %s\n", ph, err)
			return st, err
		}
		// Phase succeeded via sampling — record a positive probe so
		// future runs don't have to ask again.
		recordProbeFromError(detected.Agent, nil)
		r.Output = out
		r.Status = "done"
		r.EndedAt = time.Now().UTC()
		_ = savePossessState(st)
		fmt.Fprintf(w, "phase %s DONE (%d bytes)\n", ph, len(out))
	}
	st.CurrentPhase = "done"
	_ = savePossessState(st)

	// Emit artifact list as final result.
	files, _ := os.ReadDir(filepath.Join(workdir, "specs"))
	for _, f := range files {
		if f.IsDir() {
			st.Artifacts = append(st.Artifacts, filepath.Join("specs", f.Name()))
		}
	}
	sort.Strings(st.Artifacts)
	_ = savePossessState(st)

	fmt.Fprintf(w, "all phases done; trace=%s\n", possessStatePath(workdir, id))
	return st, nil
}

// phasePrompts returns the (system, user) tuple for a given phase.
// The system prompt sets the agent's role + format expectation. The user
// prompt contains the goal + project context + the artifacts produced by
// prior phases (already persisted in `st`).
//
// Crucially, the user prompt is bounded: it includes only the prior
// phase outputs and a snapshot of the project layout — never the full
// conversation history. This keeps each phase's sampling call below the
// 120 s sampling-timeout cap.
//
// **Text-only contract (v3.5.1):** the sampling LLM does NOT have access
// to MCP tools (radiant doesn't send `tools` in the sampling/createMessage
// params; the v3.3.0+ architecture splits LLM-planning from host-execution).
// Therefore every phase prompt instructs the model to output Markdown
// content / structure / reasoning, NEVER calls to write_file / read_file /
// bash. The host agent applies the plan using its own tools after it
// receives the sampling response. This keeps mimo (Xiaomi) and other
// function-calling-less sampling LLMs safe from XML hallucination
// (validation: `phase_hallucination_test.go`).
func phasePrompts(phase, task, workdir string, st *possessState) (system, userText string) {
	// Each phase prompt starts with an unambiguous `## radiant-phase: <name>`
	// marker so the host (or our synthetic test host) can map a
	// sampling/createMessage request to its phase without ambiguity, even
	// when prior phase outputs are present in the prompt.
	switch phase {
	case "discover":
		system = "You are an expert software engineer analysing a project to plan work. " +
			"Output ONLY Markdown. Do not pretend to call tools you do not have."
		userText = fmt.Sprintf(
			"## radiant-phase: discover\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Identify the project layout (mention key manifest files such as go.mod, "+
				"package.json, requirements.txt, pyproject.toml), surface any existing "+
				"specs under specs/0001-*/, and list the bundled skills from radiant "+
				"that appear most relevant to the task. Do not write or modify files. "+
				"Reply with a Markdown summary 6–12 lines long, ending with a section "+
				"'## Skills to apply' listing exactly 1–3 skill names.",
			task, workdir)
	case "plan":
		system = "You are an expert software engineer decomposing a goal into acceptance criteria. " +
			"Output ONLY Markdown. Do not pretend to call tools you do not have."
		prior := strings.TrimSpace(st.Phases["discover"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: plan\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Discover output (verbatim):\n%s\n\n"+
				"Decompose the task into 2–5 acceptance criteria (each starting with 'AC<n>:') "+
				"and 2–5 ordered tasks. Output the full specs/0001-%s/tasks.md content "+
				"as a single fenced ```markdown``` block, then reply with a Markdown "+
				"summary 6–12 lines listing ACs and tasks.",
			task, workdir, prior, slugify(task))
	case "execute":
		system = "You are an expert software engineer implementing a plan. " +
			"Output ONLY Markdown (prose + fenced code blocks). " +
			"Do not pretend to call tools you do not have. " +
			"The host agent will apply every file change you describe."
		prior := strings.TrimSpace(st.Phases["plan"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: execute\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Plan output (verbatim):\n%s\n\n"+
				"Implement the code per the plan. For EACH file you intend to create or "+
				"modify, output a fenced ```language path=relative/path``` block with the "+
				"complete file contents — the host agent will write them. Then describe, "+
				"in prose, which gates should be run and the expected outcome. End with a "+
				"Markdown summary 6–12 lines: files touched, expected gate results, "+
				"iterations anticipated.",
			task, workdir, prior)
	case "verify":
		system = "You are an adversarial code reviewer. Default stance: REJECTED. " +
			"Output ONLY Markdown."
		prior := strings.TrimSpace(st.Phases["execute"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: verify\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Execute output (verbatim):\n%s\n\n"+
				"Verify the implementation against each AC by examining the produced "+
				"Markdown. Reply with EXACTLY these five lines (no preamble, no trailing "+
				"prose):\n\n"+
				"VERDICT: APPROVED|REJECTED\n"+
				"SCORE: <0.00–1.00>\n"+
				"EVIDENCE: <one short sentence>\n"+
				"ESCALATE: true|false\n"+
				"ISSUES:\n"+
				"- <one bullet per issue, or '-' for none>\n",
			task, workdir, prior)
	}
	return system, userText
}
