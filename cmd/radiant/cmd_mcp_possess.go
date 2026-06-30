package main

// `radiant possess <task> <workdir> ...` — the entry-point MCP tool that
// the agent invokes when the user says "do X with this harness."
//
// Design rationale (2026-06-29):
//
// Earlier versions wrapped the autonomous loop as a single
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
// All four primitives are non-blocking until the harness needs a sampling
// call. The agent calls them in any order, paused or inter-leaved with
// other tools, and the harness state file persists everything.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/v3/internal"
	"github.com/quant-risk/radiant-harness/v3/internal/hostdetect"
	"github.com/quant-risk/radiant-harness/v3/internal/llm"
	"github.com/quant-risk/radiant-harness/v3/internal/loop"
	"github.com/quant-risk/radiant-harness/v3/internal/possess"
	"github.com/quant-risk/radiant-harness/v3/internal/scaffold"
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
	TaskID            string                 `json:"task_id"`
	Workdir           string                 `json:"workdir"`
	Task              string                 `json:"task"`
	StartedAt         time.Time              `json:"started_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	CurrentPhase      string                 `json:"current_phase"` // discover|plan|execute|verify|done
	Phases            map[string]*phaseResult `json:"phases"`
	Artifacts         []string               `json:"artifacts"`
	BootstrapDone     bool                   `json:"bootstrap_done"`
	BootstrapMessages []string               `json:"bootstrap_messages"`
	// v3.7.2: extended fields used by `radiant_run_gate` /
	// `radiant_possess_async`. Persisted so a host can resume
	// across multiple MCP calls.
	Profile     string `json:"profile,omitempty"`     // lean|standard|thorough
	SpecDir     string `json:"spec_dir,omitempty"`     // workdir-relative path to specs/NNNN-slug
	Slug        string `json:"slug,omitempty"`        // kebab-case slug derived from the task
	Cancelled   bool   `json:"cancelled,omitempty"`   // set by AsyncGate.Cancel
	RunMode     string `json:"run_mode,omitempty"`    // self-driven | driver-fallback-v3.7.1 | etc.
	LastPhaseAt time.Time `json:"last_phase_at,omitempty"`
}

type phaseResult struct {
	Status    string    `json:"status"` // pending|in_progress|done|error
	Phase     string    `json:"phase,omitempty"`
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
//
// v3.6.0.1 — the empty-stub trap.
//
// The previous gate checked `detected.Agent != AgentUnknown` before
// routing to self-driven. When the host agent ran `radiant mcp possess`
// from a fresh shell (no `CODEX_HOME`, no `CLAUDE_CODE_ENTRY`, etc.),
// Detect() returned AgentUnknown and the run fell back to a stub
// path that ran bootstrapPossess and marked every phase as done
// with a `[stub mode]` placeholder — leaving the user with a folder
// of empty directories and zero deliverables, exactly the failure
// mode the v3.6.0 self-driven pipeline was supposed to fix.
//
// v3.6.0.1 flips the default: EVERY CLI invocation of
// `radiant mcp possess` dispatches to runSelfDrivenPossess. The user
// gets the templated scaffold (`specs/0001-<slug>/spec.md`,
// `tasks.md`, `scripts/run.sh`, `docs/README.md`,
// `.radiant-harness/{CONTEXT.md, handoff.md, verify.md}`) without
// having to first prove which agent they are. The old hollow-stub
// behaviour is preserved behind `RADIANT_FORCE_SAMPLING=1` for the
// rare case where the caller wants to verify the empty-stub shape
// in a unit test or smoke run.
func runPossessForCLI(ctx context.Context, workdir, task string, w io.Writer) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	id := taskID(workdir, task)

	if os.Getenv("RADIANT_FORCE_SAMPLING") != "1" {
		detected := hostdetect.New().Detect()
		reason := "CLI invocation without MCP wiring — defaulting to self-driven"
		if detected.Agent != hostdetect.AgentUnknown {
			if supports, probed := hostdetect.ResolveSupport(detected.Agent); probed && supports {
				// Detected host DOES support sampling; CLI path is still
				// self-driven because there's no MCP transport from a
				// direct shell call (the LLM loop runs only when the
				// host agent spawns `radiant mcp serve`). Use the
				// detected agent as a hint but don't claim sampling.
				reason = fmt.Sprintf("CLI invocation — host %q reports SupportsSampling=true but no MCP transport from a shell call", detected.Agent)
			} else if probed && !supports {
				reason = fmt.Sprintf("probe says %s has no sampling", detected.Agent)
			} else {
				reason = fmt.Sprintf("CLI invocation — host %q not yet probed", detected.Agent)
			}
		}
		fmt.Fprintf(w, "→ routing `radiant mcp possess` to self-driven mode (%s).\n", reason)
		if os.Getenv("RADIANT_FORCE_STUB") != "1" {
			fmt.Fprintln(w, "  Set RADIANT_FORCE_SAMPLING=1 to bypass and exercise the legacy stub path.")
		}
		fmt.Fprintln(w)
		return runSelfDrivenPossess(ctx, workdir, task, "standard", w, reason)
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
//
// v3.7.0 routing — drives real tool execution when the host supports it:
//
//   1. Pre-flight against hostdetect.ResolveSupport. If supports_sampling
//      is false (probe-verified or well-attested), fall through to the
//      self-driven scaffold path without ever opening the sampling
//      channel.
//
//   2. If the backend implements llm.ToolCapable (i.e. the host's MCP
//      server propagates a `tools` field through to its model),
//      run the agentic driver — internal/possess.Driver loops
//      sampling/createMessage with native tool_use blocks until the
//      model emits a VERDICT line. The driver dispatches every
//      tool_use through tools.Registry (read_file, write_file,
//      search_code, run_gate, …) so the model can edit the project,
//      run gates, and inspect the result without the harness
//      pretending it has finished.
//
//   3. Otherwise (no ToolCapable), fall back to the prior v3.6.x
//      contract: one sampling call per phase, text-only, with the
//      legacy code-block extraction still feeding the engine. If
//      any call returns -32601, hand off to the self-driven pipeline.
//
// The self-driven path is the universal fallback — it runs without
// LLM, with deterministic templates — so the v3.7.0 agentic capability
// is purely additive for hosts that implement it.
func runPossessWithBackend(ctx context.Context, workdir, task, profile string, backend llm.Backend, w io.Writer) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	if profile == "" {
		profile = "standard"
	}
	id := taskID(workdir, task)

	// Pre-flight: probe-verified or well-attested no-sampling hosts
	// bypass sampling entirely.
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

	// Bootstrap the project layout BEFORE delegating to the agentic
	// driver so the model can immediately write into specs/0001-*
	// without needing to mkdir itself.
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

	// v3.7.0 agentic path: the host's model can call tools natively.
	// We hand off the entire 4-phase loop to the driver in one shot
	// (no per-phase splitting) because tool_use + tool_result is an
	// inherently interleaved protocol — splitting it would force the
	// model into 4 separate turns instead of 1 long turn.
	if _, ok := backend.(llm.ToolCapable); ok {
		_, drvErr := runPossessWithDriver(ctx, workdir, task, profile, backend, w, st)
		if drvErr != nil {
			// v3.7.1: a -32601 mid-run → fall back to self-driven so
			// the workdir still lands with templated artefacts (the
			// 2026-06-29 Codex failure mode). Any other error
			// propagates unchanged.
			return routeAgenticErr(drvErr, ctx, workdir, task, profile, w, detected.Agent, st)
		}
		return st, nil
	}

	// Legacy / non-toolable host — per-phase sampling, code-block
	// extraction still does the actual work. If any call returns
	// -32601, hand off to the deterministic self-driven pipeline.
	fmt.Fprintf(w, "run id: %s\nworkdir: %s\ntask: %s\n\n", id, workdir, task)
	fmt.Fprintln(w, "running phases (text-only sampling; backend does not implement llm.ToolCapable)...")

	if !st.BootstrapDone {
		st.BootstrapDone = true
		// msgs was assigned earlier in the bootstrap block above.
		_ = msgs
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

// runPossessWithDriver is the v3.7.0+ path used when the host agent's
// MCP server implements llm.ToolCapable. The driver runs ONE long
// conversation that interleaves sampling with tool execution; the
// model emits tool_use blocks, we dispatch them through
// tools.Registry, feed tool_result echoes back, and loop until the
// model emits a VERDICT/REVIEW line.
//
// On any failure (driver error, ErrBackendToolsUnsupported, plain
// error from the sampling backend) we fall back to the v3.6.x path:
// re-mark phases as either errored or self-driven, so the project
// always lands in a usable state.
func runPossessWithDriver(ctx context.Context, workdir, task, profile string, backend llm.Backend, w io.Writer, st *possessState) (*possessState, error) {
	id := st.TaskID
	fmt.Fprintf(w, "run id: %s\nworkdir: %s\ntask: %s\nprofile: %s\n\n", id, workdir, task, profile)
	fmt.Fprintln(w, "→ running via v3.7.0 agentic tool-calling driver (backend implements llm.ToolCapable).")

	// Build the tool registry scoped to this project. The four built-in
	// tools (read_file, write_file, search_code, run_gate) all enforce
	// project-boundary checks via fsutil.PathIsSafe.
	reg, err := loop.RealRegistry(workdir)
	if err != nil {
		return st, fmt.Errorf("driver registry: %w", err)
	}

	driver, err := possess.NewDriver(possess.DriverConfig{
		Backend:     backend,
		ProjectRoot: workdir,
		Registry:    reg,
		Profile:     profile,
		MaxIter:     profileToMaxIter(profile),
		Out:         w,
	})
	if err != nil {
		return st, fmt.Errorf("driver init: %w", err)
	}

	// Build the system + task message. The task is one bounded
	// sentence the model needs to drive the 4 phases through tool
	// calls. The system prompt echoes the VERDICT/REVIEW format
	// before any tool runs so the model can finish on the right
	// line.
	system := agenticSystemPrompt(workdir, task)
	userTask := agenticTaskPrompt(task)

	tr, err := driver.Drive(ctx, system, userTask)
	if err != nil {
		// Driver failed — log to w, then check whether we can still
		// save the work and downgrade gracefully.
		fmt.Fprintf(w, "\n⚠ driver failed: %v\n", err)
		// Write whatever Trace the driver did manage into the
		// discover phase so the user has a paper trail. Anything
		// the model produced went into the project tree via tool
		// executions (the driver ran real write_file calls).
		st.Phases["discover"].Status = "error"
		st.Phases["discover"].Error = err.Error()
		st.Phases["discover"].Output = tr.TextSoFar
		st.Phases["discover"].EndedAt = time.Now().UTC()
		// If the driver complaint is "tools unsupported" the host
		// actually advertises Capability but the model never calls
		// anything — that means we should fall back to the
		// text-only/sampling-runs path for the rest of this run.
		// For now we mark a partial state and return so the caller
		// can decide. A future v3.7.x may chain to self-driven.
		st.CurrentPhase = "verify"
		_ = savePossessState(st)
		return st, fmt.Errorf("agentic driver failed (project tree may have partial writes; see %s): %w",
			possessStatePath(workdir, id), err)
	}

	// Driver succeeded (VERDICT/REVIEW surfaced). Surface the
	// summary and remember the tool-call log so the verifier can
	// audit it later.
	fmt.Fprintf(w, "\nDriver verdict: %s\nIterations: %d\nTool invocations: %d\n",
		tr.Verdict, tr.Iterations, len(tr.ToolInvocations))
	for _, ti := range tr.ToolInvocations {
		verb := "ok"
		if ti.Err != "" {
			verb = "err"
		}
		fmt.Fprintf(w, "  • %s [%s] %d bytes\n", ti.Name, verb, len(ti.Output))
	}

	for _, ph := range []string{"discover", "plan", "execute", "verify"} {
		st.Phases[ph].Status = "done"
		st.Phases[ph].StartedAt = tr.StartAt
		st.Phases[ph].EndedAt = tr.EndAt
		// We don't have per-phase output; shove the trace summary
		// into verify so the caller sees what happened.
		st.Phases[ph].Output = fmt.Sprintf("agentic driver completed; verdict=%s", tr.Verdict)
	}
	st.CurrentPhase = "done"

	// Re-walk specs/ to enumerate the artefacts the driver wrote.
	if files, ferr := os.ReadDir(filepath.Join(workdir, "specs")); ferr == nil {
		for _, f := range files {
			if f.IsDir() {
				st.Artifacts = append(st.Artifacts, filepath.Join("specs", f.Name()))
			}
		}
	}
	sort.Strings(st.Artifacts)
	_ = savePossessState(st)

	fmt.Fprintf(w, "\nall phases done (agentic). trace=%s\n", possessStatePath(workdir, id))
	// Positive probe evidence for future runs.
	recordProbeFromError(hostdetect.New().Detect().Agent, nil)
	return st, nil
}

// routeAgenticErr inspects the error returned by the agentic driver
// and decides whether to fall back to the self-driven pipeline or
// surface the error as fatal. v3.7.1 closes the 2026-06-29 hollow-
// stub trap on Codex: when the agentic driver returns
// ErrHostSamplingUnsupported (sentinel for -32601 mid-run) the
// caller falls through to runSelfDrivenPossess with the
// deterministic templates instead of leaving the workdir empty.
//
// All other errors (real backend bugs, timeouts, panic-like
// conditions) propagate unchanged.
func routeAgenticErr(err error, ctx context.Context, workdir, task, profile string, w io.Writer, detectedAgent hostdetect.AgentID, st *possessState) (*possessState, error) {
	if err == nil {
		return st, nil
	}
	if errors.Is(err, possess.ErrHostSamplingUnsupported) {
		fmt.Fprintf(w,
			"\n⚠ agentic driver failed with -32601 mid-run "+
				"(sampling/createMessage not implemented on this host).\n"+
				"  Falling back to self-driven scaffold. The harness will populate\n"+
				"  spec.md / tasks.md / scripts/run.sh / docs/README.md /\n"+
				"  .radiant-harness/{CONTEXT.md, handoff.md, verify.md} with\n"+
				"  [host-agent: fill in …] markers so the next agent can fill in.\n\n")
		// Persist the failure as probe evidence so future runs'
		// pre-flight (or this run's when run again) short-circuits
		// without paying the cost of another -32601.
		recordProbeFromError(detectedAgent, err)
		return runSelfDrivenPossess(ctx, workdir, task, profile, w,
			"sampling unsupported mid-run (driver fallback v3.7.1)")
	}
	return st, err
}

// agenticSystemPrompt tells the model the role + the strict
// format. The harness owns: spec.md / tasks.md scaffolding, gate
// invocation, scratch context. The model owns: deciding what the
// user asked for and emitting the verdict at the end.
func agenticSystemPrompt(workdir, task string) string {
	return fmt.Sprintf(
		"You are an expert software engineer working inside `radiant-harness` v3.7.0+. "+
			"Your job is to drive a 4-phase loop (discover → plan → execute → verify) "+
			"by calling the available tools. Each tool execution produces a real "+
			"side-effect inside the project directory `%s`.\n\n"+
			"RULES:\n"+
			"- Use read_file / search_code to inspect the project before you write anything.\n"+
			"- Use write_file to produce every artefact under specs/0001-*/, docs/, scripts/.\n"+
			"- Use run_gate to execute quality gates (go build, go test, scripts/run.sh). "+
			"Do not invent custom commands outside the gate allowlist.\n"+
			"- When the work is complete, end with EXACTLY ONE of these terminal blocks "+
			"on its own lines (no surrounding prose, no extra markdown):\n\n"+
			"VERDICT: APPROVED|REJECTED\n"+
			"SCORE: <0.00–1.00>\n"+
			"EVIDENCE: <one sentence>\n"+
			"ESCALATE: true|false\n"+
			"ISSUES:\n"+
			"- <one bullet, or '-' for none>\n\n"+
			"or (post-convergence review):\n\n"+
			"REVIEW: PASS|FAIL\n"+
			"SCORE: <0.00–1.00>\n"+
			"EVIDENCE: <one sentence>\n"+
			"FINDINGS:\n"+
			"- <one bullet, or '-' for none>\n\n"+
			"Do not output VERDICT lines until you've actually written the spec, "+
			"the tasks, and at least one runnable artefact, AND run a gate that "+
			"passed. The harness records every tool_use — pretending to finish "+
			"without tools is logged and counted as a failed run.\n\n"+
			"Task: %s",
		workdir, task)
}

// agenticTaskPrompt is the user turn that starts the loop. Kept
// short — the system prompt carries the rules.
func agenticTaskPrompt(task string) string {
	return fmt.Sprintf(
		"%s\n\n"+
			"Start by inspecting the project layout under the workdir with read_file "+
			"and search_code, then plan the spec, then execute.\n\n"+
			"Reference: %s/.radiant-harness/CONTEXT.md (already populated by the "+
			"bootstrap) lists the bundled skills surfaced from your task keywords.",
		task, "${workdir}") // ${workdir} substituted at run time below
}

// profileToMaxIter picks the agentic iteration cap from the
// possess profile. Conservative default — ModelBudget.Policy-style
// knobs land in a follow-up.
func profileToMaxIter(profile string) int {
	switch profile {
	case "lean":
		return 10
	case "thorough":
		return 60
	default: // "standard" or empty
		return 25
	}
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
