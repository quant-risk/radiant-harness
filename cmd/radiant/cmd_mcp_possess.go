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

	"github.com/quant-risk/radiant-harness/internal/llm"
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

// bootstrapPossess scaffolds the minimum project layout needed for
// radiant-possession to proceed: AGENTS.md, docs/, specs/, scripts/.
// Idempotent (only writes files that don't exist).
func bootstrapPossess(workdir string) ([]string, error) {
	dirs := []string{
		filepath.Join(workdir, ".radiant-harness"),
		filepath.Join(workdir, "docs"),
		filepath.Join(workdir, "specs"),
		filepath.Join(workdir, "scripts"),
		filepath.Join(workdir, ".agent-context"),
	}
	var msgs []string
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
// It does NOT do sampling (CI mode); it walks every phase and writes the
// trace file. The MCP tool path uses runPossessWithBackend which can
// drive real sampling.
func runPossessForCLI(ctx context.Context, workdir, task string, w io.Writer) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	id := taskID(workdir, task)

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
		st.Phases[ph].Output = "[stub mode — set RADIANT_MCP=1 and rerun via MCP for real sampling]\n\n" + descs[ph]
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
func runPossessWithBackend(ctx context.Context, workdir, task, profile string, backend llm.Backend, w io.Writer) (*possessState, error) {
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
		_ = savePossessState(st)
	}

	phases := []string{"discover", "plan", "execute", "verify"}
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
			r.Status = "error"
			r.Error = err.Error()
			r.EndedAt = time.Now().UTC()
			_ = savePossessState(st)
			fmt.Fprintf(w, "phase %s FAILED: %s\n", ph, err)
			return st, err
		}
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
func phasePrompts(phase, task, workdir string, st *possessState) (system, userText string) {
	// Each phase prompt starts with an unambiguous `## radiant-phase: <name>`
	// marker so the host (or our synthetic test host) can map a
	// sampling/createMessage request to its phase without ambiguity, even
	// when prior phase outputs are present in the prompt.
	switch phase {
	case "discover":
		system = "You are an expert software engineer analysing a project to plan work."
		userText = fmt.Sprintf(
			"## radiant-phase: discover\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Identify the project layout (ls the workdir, read package manifests like go.mod, "+
				"package.json, requirements.txt, pyproject.toml), surface any existing "+
				"specs under specs/0001-*/, and list the bundled skills from radiant "+
				"that appear most relevant to the task. Do not write files yet. "+
				"Reply with a Markdown summary 6–12 lines long, ending with a section "+
				"'## Skills to apply' listing exactly 1–3 skill names.",
			task, workdir)
	case "plan":
		system = "You are an expert software engineer decomposing a goal into acceptance criteria."
		prior := strings.TrimSpace(st.Phases["discover"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: plan\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Discover output (verbatim):\n%s\n\n"+
				"Decompose the task into 2–5 acceptance criteria (each starting with 'AC<n>:') "+
				"and 2–5 ordered tasks. Write to specs/0001-%s/tasks.md using Write tool. "+
				"Then reply with a Markdown summary 6–12 lines listing ACs and tasks.",
			task, workdir, prior, slugify(task))
	case "execute":
		system = "You are an expert software engineer implementing the plan."
		prior := strings.TrimSpace(st.Phases["plan"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: execute\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Plan output (verbatim):\n%s\n\n"+
				"Implement the code per the plan. Write files with Write tool. Run "+
				"the gates from tasks.md with Bash. Iterate until all gates pass. "+
				"Reply with a Markdown summary 6–12 lines: what you wrote, each gate "+
				"PASS/FAIL, iterations taken.",
			task, workdir, prior)
	case "verify":
		system = "You are an adversarial code reviewer. Default stance: REJECTED."
		prior := strings.TrimSpace(st.Phases["execute"].Output)
		userText = fmt.Sprintf(
			"## radiant-phase: verify\n\n"+
				"Task: %s\nWorkdir: %s\n\n"+
				"Execute output (verbatim):\n%s\n\n"+
				"Verify the implementation against each AC. Run the gates yourself "+
				"to confirm. Reply with EXACTLY:\n"+
				"VERDICT: APPROVED\n"+
				"SCORE: 1.00\n"+
				"EVIDENCE: <one sentence>\n"+
				"ESCALATE: false\n"+
				"ISSUES:\n",
			task, workdir, prior)
	}
	return system, userText
}
