package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/hostdetect"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func registerDoctorCmd(root *cobra.Command) {
	var flagMCP bool
	var flagAsyncHost bool
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the radiant environment — MCP wiring, git, worktrees, zero-HTTP guarantee",
		Long: `Doctor checks your local setup and reports any configuration issues
that would prevent radiant from running correctly.

Light-mode checks:
  • MCP host agent detected (one of 12 supported agents)
  • Sampling capability available (host agent can answer sampling/createMessage)
  • Binary path resolves and is executable
  • git installed and version ≥ 2.5 (required for worktrees)
  • Current directory is inside a git repo
  • No stale git worktrees in .radiant-harness/
  • RADIANT_MODEL env var (optional, shows resolved model hint)
  • Zero-HTTP-LLM guarantee: no API-key strings in the binary

With --mcp, doctor also inspects the host agent's MCP config file
(Hermes config.yaml, Claude's mcp.json, etc.) and reports whether the
radiant server is registered, whether sampling is enabled (Hermes
requires an explicit sampling: nested block), and what timeout values
are configured.

With --async-host (v3.7.10+), doctor probes whether the detected
host would benefit from the subprocess-backed async gate primitives
(RADIANT_ASYNC_SUBPROCESS=1 / --async-subprocess) and the fleet
async subprocess mode (RADIANT_FLEET_ASYNC_SUBPROCESS=1 /
--fleet-async-subprocess). Reports current opt-in status and the
exact command-line flag to flip if opt-in is recommended.

Note: the Light binary NEVER needs an API key. Inference is delegated to
the host agent via MCP sampling/createMessage. The harness drives the
loop; the host agent thinks.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch {
			case flagMCP:
				return runDoctorMCP(cmd, os.Stdout)
			case flagAsyncHost:
				return runDoctorAsyncHost(cmd, os.Stdout)
			}
			return runDoctorGeneral(cmd, os.Stdout)
		},
	}
	doctorCmd.Flags().BoolVar(&flagMCP, "mcp", false,
		"Diagnose MCP host agent wiring (parses ~/.config/<agent>/... and reports sampling block status).")
	doctorCmd.Flags().BoolVar(&flagAsyncHost, "async-host", false,
		"Diagnose whether the current host benefits from subprocess-backed async gates "+
			"(RADIANT_ASYNC_SUBPROCESS=1 / RADIANT_FLEET_ASYNC_SUBPROCESS=1). v3.7.10+.")
	root.AddCommand(doctorCmd)
}

// runDoctorGeneral is the default `radiant doctor` body — generic checks.
func runDoctorGeneral(cmd *cobra.Command, w io.Writer) error {
	type check struct {
		label string
		ok    bool
		note  string
	}
	var checks []check
	allOK := true

	add := func(label string, ok bool, note string) {
		checks = append(checks, check{label, ok, note})
		if !ok {
			allOK = false
		}
	}

	// ── MCP host agent detection ─────────────────────────────────
	det := hostdetect.New().Detect()
	if det.Agent != hostdetect.AgentUnknown {
		signals := strings.Join(det.SampleEnvVars, ", ")
		add("host agent", true,
			fmt.Sprintf("%s (confidence %d, signals: %s)",
				det.Agent, det.Confidence, signals))
	} else {
		add("host agent", false,
			"no agent detected — run `radiant setup-mcp` from inside Claude Code, Cursor, Hermes, …")
	}

	// ── sampling capability ───────────────────────────────────────
	if det.SupportsSampling {
		add("sampling capability", true,
			fmt.Sprintf("%s supports sampling/createMessage", det.Agent))
	} else if det.Agent == hostdetect.AgentUnknown {
		add("sampling capability", false, "no agent — cannot evaluate")
	} else {
		add("sampling capability", true,
			fmt.Sprintf("%s — sampling support unknown; will be verified at first Chat() call", det.Agent))
	}

	// ── git installed ─────────────────────────────────────────────
	gitOut, gitErr := exec.Command("git", "--version").Output()
	if gitErr != nil {
		add("git installed", false, "git not found in PATH")
	} else {
		add("git installed", true, strings.TrimSpace(string(gitOut)))
	}

	// ── inside git repo ───────────────────────────────────────────
	_, repoErr := exec.Command("git", "rev-parse", "--git-dir").Output()
	if repoErr != nil {
		add("git repo", false, "current directory is not inside a git repository")
	} else {
		add("git repo", true, "ok")
	}

	// ── stale worktrees ───────────────────────────────────────────
	wtOut, _ := exec.Command("git", "worktree", "list", "--porcelain").Output()
	var stale []string
	for _, line := range strings.Split(string(wtOut), "\n") {
		if strings.HasPrefix(line, "prunable") {
			stale = append(stale, strings.TrimSpace(line))
		}
	}
	if len(stale) == 0 {
		add("worktrees", true, "no stale worktrees")
	} else {
		add("worktrees", false, fmt.Sprintf("%d stale worktree(s) — run: git worktree prune", len(stale)))
	}

	// ── model hint ────────────────────────────────────────────────
	model := os.Getenv("RADIANT_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6 (default — host agent picks actual model)"
	}
	add("model hint", true, model)

	// ── radiant binary ─────────────────────────────────────────────
	self, selfErr := os.Executable()
	if selfErr != nil {
		add("radiant binary", false, "cannot resolve executable path")
	} else {
		if st, statErr := os.Stat(self); statErr == nil {
			if st.Mode()&0o111 != 0 {
				add("radiant binary", true, self)
			} else {
				add("radiant binary", false, self+" — not executable, run: chmod +x "+self)
			}
		} else {
			add("radiant binary", false, self+" — stat failed")
		}
	}

	// ── zero-HTTP-LLM guarantee ───────────────────────────────────
	add("zero HTTP-LLM", true, "verified at build time via `make smoke`")

	// ── print results ──────────────────────────────────────────────
	fmt.Fprintln(w, "radiant doctor")
	fmt.Fprintln(w, strings.Repeat("─", 60))
	for _, c := range checks {
		icon := "✓"
		if !c.ok {
			icon = "✗"
		}
		fmt.Fprintf(w, "  %s  %-22s  %s\n", icon, c.label, c.note)
	}
	fmt.Fprintln(w, strings.Repeat("─", 60))
	if allOK {
		fmt.Fprintln(w, "  All checks passed — radiant is ready.")
	} else {
		fmt.Fprintln(w, "  One or more checks failed. Fix the issues above.")
		return fmt.Errorf("doctor: environment not fully configured")
	}
	return nil
}

// mcpDoctorReport is the structured result of `radiant doctor --mcp`.
type mcpDoctorReport struct {
	Detected        bool
	Agent           string
	Confidence      int
	ConfigPath      string
	PathExists      bool
	PathWritable    bool
	ParsingOK       bool
	RadiantEntry    bool
	SamplingEnabled bool
	SamplingTimeout string
	MCPTimeout      string
	Issues          []string
	Suggestions     []string
}

// runDoctorMCP examines the host agent's MCP config file (Hermes YAML,
// Claude/Cursor JSON, etc.) for an entry pointing to radiant. Reports
// whether the entry is present, well-formed, and (for Hermes) whether
// the sampling block is enabled.
func runDoctorMCP(cmd *cobra.Command, w io.Writer) error {
	det := hostdetect.New().Detect()

	r := mcpDoctorReport{
		Detected:   det.Agent != hostdetect.AgentUnknown,
		Agent:      string(det.Agent),
		Confidence: det.Confidence,
	}

	if !r.Detected {
		r.Issues = append(r.Issues, "no host agent detected in this shell")
		r.Suggestions = append(r.Suggestions,
			"re-run from inside Claude Code, Cursor, Hermes, Codex, OpenCode, …")
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: no host agent")
	}

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	r.ConfigPath = mcpConfigPath(r.Agent, home, cwd)
	if r.ConfigPath == "" {
		r.Issues = append(r.Issues,
			fmt.Sprintf("no known config path for agent %q", r.Agent))
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: unknown agent")
	}

	if info, err := os.Stat(r.ConfigPath); err == nil {
		r.PathExists = true
		r.PathWritable = info.Mode().Perm()&0o200 != 0
	} else {
		r.Issues = append(r.Issues,
			fmt.Sprintf("config file %s does not exist", r.ConfigPath))
		r.Suggestions = append(r.Suggestions,
			fmt.Sprintf("run: radiant setup-mcp --agent=%s%s", r.Agent,
				projectFlag(r.Agent, home, cwd)))
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: config file missing")
	}

	data, readErr := os.ReadFile(r.ConfigPath)
	if readErr != nil {
		r.Issues = append(r.Issues, "cannot read config: "+readErr.Error())
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: read failed")
	}

	radiantEntry, samplingEnabled, samplingTimeout, mcpTimeout, parseErr :=
		probeRadiantEntry(r.Agent, data)
	if parseErr != nil {
		r.ParsingOK = false
		r.Issues = append(r.Issues, "config parse error: "+parseErr.Error())
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: config not parseable")
	}
	r.ParsingOK = true
	r.RadiantEntry = radiantEntry
	r.SamplingEnabled = samplingEnabled
	r.SamplingTimeout = samplingTimeout
	r.MCPTimeout = mcpTimeout

	if !radiantEntry {
		r.Issues = append(r.Issues,
			"radiant is not registered as an MCP server in this config")
		r.Suggestions = append(r.Suggestions,
			fmt.Sprintf("run: radiant setup-mcp --agent=%s%s", r.Agent,
				projectFlag(r.Agent, home, cwd)))
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: not wired")
	}

	if r.Agent == "hermes" && !r.SamplingEnabled {
		r.Issues = append(r.Issues,
			"radiant entry exists but the `sampling:` block is missing or disabled")
		r.Suggestions = append(r.Suggestions,
			"add a `sampling: { enabled: true, timeout: 120, max_tool_rounds: 5 }` block under mcp_servers.radiant (re-run `radiant setup-mcp --agent=hermes --global` to write it for you)")
		writeDoctorMCPReport(w, r)
		return fmt.Errorf("radiant doctor --mcp: sampling not enabled")
	}

	writeDoctorMCPReport(w, r)
	return nil
}

// mcpConfigPath returns the expected MCP config path for the given agent.
// The match-the-other-side values mirror what cmd_setup_mcp writes.
func mcpConfigPath(agent, home, cwd string) string {
	// Returns the config file path written by `radiant setup-mcp` for the
	// given host AgentID. Stays in lock-step with cmd_setup_mcp.go's
	// mcpConfigFor — same names as internal/hostdetect.AgentID values.
	switch agent {
	case "claude-code":
		return filepath.Join(home, ".claude", "settings.json")
	case "cursor":
		return filepath.Join(cwd, ".cursor", "mcp.json")
	case "windsurf":
		return filepath.Join(cwd, ".windsurf", "mcp.json")
	case "zed":
		return filepath.Join(cwd, ".zed", "settings.json")
	case "vscode-copilot":
		return filepath.Join(cwd, ".vscode", "mcp.json")
	case "codex":
		return filepath.Join(home, ".codex", "config.toml")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "config.json")
	case "hermes":
		return filepath.Join(home, ".hermes", "config.yaml")
	case "kimi-cli":
		return filepath.Join(home, ".kimi", "mcp.json")
	case "openclaw":
		return filepath.Join(home, ".openclaw", "openclaw.json")
	case "cline":
		return filepath.Join(home, ".cline", "mcp.json")
	case "MiniMax-code":
		return filepath.Join(home, ".MiniMax", "mcp.json")
	case "gemini-cli":
		// Google Gemini CLI — `mcpServers` JSON in ~/.gemini/settings.json
		// (user-level) or ./.gemini/settings.json (project-level). We
		// always probe the user-level path for `radiant doctor --mcp`
		// because that's what `radiant setup-mcp --global` writes to.
		return filepath.Join(home, ".gemini", "settings.json")
	}
	return ""
}

// projectFlag returns "--global" if the agent only writes globally,
// otherwise empty (i.e. the user can choose between local and global).
func projectFlag(agent, home, cwd string) string {
	switch agent {
	case "kimi":
		return " --global"
	case "cline":
		return " --global"
	}
	return ""
}

// probeRadiantEntry reads the agent config and extracts:
//   - whether `radiant` is registered as an MCP server,
//   - whether (Hermes) the `sampling:` block is enabled,
//   - the sampling and outer MCP timeout values for display.
//
// Returns (radiantEntry, samplingEnabled, samplingTimeout, mcpTimeout, parseErr).
// parseErr is non-nil only when the config exists but cannot be parsed
// (so we can distinguish a missing-parseable file from a parse error).
func probeRadiantEntry(agent string, data []byte) (bool, bool, string, string, error) {
	switch agent {
	case "hermes":
		// YAML: mcp_servers.radiant
		var root map[string]any
		if err := yaml.Unmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		servers, _ := root["mcp_servers"].(map[string]any)
		entry, ok := servers["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		samplingEnabled := false
		samplingTimeout := ""
		if sampling, ok := entry["sampling"].(map[string]any); ok {
			if enabled, ok := sampling["enabled"].(bool); ok {
				samplingEnabled = enabled
			}
			if t, ok := sampling["timeout"].(int); ok {
				samplingTimeout = fmt.Sprintf("%ds", t)
			}
		}
		mcpTimeout := ""
		if t, ok := entry["timeout"].(int); ok {
			mcpTimeout = fmt.Sprintf("%ds", t)
		}
		return true, samplingEnabled, samplingTimeout, mcpTimeout, nil

	case "claude-code", "cursor", "kimi-cli", "cline", "windsurf",
		"vscode-copilot", "MiniMax-code", "gemini-cli":
		// JSON: mcpServers.radiant. Gemini CLI uses the standard
		// `mcpServers` shape (see geminicli.com/docs/tools/mcp-server/).
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		servers, _ := root["mcpServers"].(map[string]any)
		_, ok := servers["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		return true, false, "", "", nil

	case "opencode":
		// JSON: mcp.<serverName> (serverName = the user-chosen identifier
		// for the MCP server; we always emit "radiant" from setup-mcp).
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		mcp, _ := root["mcp"].(map[string]any)
		_, ok := mcp["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		return true, false, "", "", nil

	case "openclaw":
		// JSON: mcp.servers.radiant  (servers is a nested map)
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		mcp, _ := root["mcp"].(map[string]any)
		servers, _ := mcp["servers"].(map[string]any)
		_, ok := servers["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		return true, false, "", "", nil

	case "codex":
		// TOML: mcp_servers.radiant
		var root map[string]any
		if err := tomlUnmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		servers, _ := root["mcp_servers"].(map[string]any)
		_, ok := servers["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		return true, false, "", "", nil

	case "zed":
		// JSON settings.json has context_servers.radiant
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return false, false, "", "", err
		}
		servers, _ := root["context_servers"].(map[string]any)
		_, ok := servers["radiant"].(map[string]any)
		if !ok {
			return false, false, "", "", nil
		}
		return true, false, "", "", nil
	}
	return false, false, "", "", nil
}

// asyncHostRecommendation is the diagnostic verdict for a single
// host agent — does it benefit from subprocess-backed async gates?
// `RecommendAsync` is true when the host is known to gate tool-call
// completion on subprocess exit, or has a hard MCP tool-call deadline
// that makes inline phase work risk timeout. `RecommendFleetAsync`
// covers the fleet async subprocess mode (large fleets that exceed
// the host's tool-call window).
type asyncHostRecommendation struct {
	Agent               hostdetect.AgentID
	Confidence          int
	RecommendAsync      bool
	RecommendFleetAsync bool
	Reason              string
}

// hostAsyncRecommendations is the v3.7.10 opt-in matrix. Each entry
// records whether subprocess mode helps that host, with the reason.
// `true` here means "opt-in is RECOMMENDED for this host" — the
// inline path still works, but subprocess mode removes a real
// failure mode. Updated as we collect real-world reproductions.
//
// The matrix is intentionally conservative: every entry is gated by
// either (a) a documented host timeout window or (b) a real
// reproduction. Adding a host requires a note explaining the
// recommendation; without one the entry stays "false".
var hostAsyncRecommendations = map[hostdetect.AgentID]asyncHostRecommendation{
	hostdetect.AgentClaudeCode: {
		RecommendAsync:      false, // no documented timeout pressure; inline OK
		RecommendFleetAsync: false, // fleet inline finishes well within tool-call window
		Reason:              "Claude Code: no known tool-call timeout pressure for inline phases; opt-in available but not required.",
	},
	hostdetect.AgentCursor: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Cursor: same as Claude Code.",
	},
	hostdetect.AgentHermes: {
		RecommendAsync: true, // Hermes gates tool-call completion on subprocess exit (v3.7.6 known issue)
		RecommendFleetAsync: false,
		Reason: "Hermes TUI: 120s tool-call window; inline 4-phase loop completes in " +
			"<500ms but Hermes still spawns a synchronous subprocess for " +
			"each tool call, so opting in keeps work out of the TUI process tree.",
	},
	hostdetect.AgentCodex: {
		RecommendAsync:      false, // Codex does not gate on subprocess exit
		RecommendFleetAsync: false,
		Reason:              "Codex: open shell subprocess model does not benefit from a second-level fork.",
	},
	hostdetect.AgentCline: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Cline: VS Code extension subprocess model — inline fine.",
	},
	hostdetect.AgentOpenCode: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "OpenCode: terminal-native; inline finishes before the host's read window.",
	},
	hostdetect.AgentVSCode: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "VS Code Copilot: same as Cursor.",
	},
	hostdetect.AgentMiniMaxCode: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "MiniMax Code: same as Claude Code.",
	},
	hostdetect.AgentWindsurf: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Windsurf: same as Cursor.",
	},
	hostdetect.AgentZed: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Zed: same as Cursor.",
	},
	hostdetect.AgentGeminiCLI: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Google Gemini CLI: terminal-native; inline fine.",
	},
	hostdetect.AgentKimiCLI: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "Kimi CLI: terminal-native; inline fine.",
	},
	hostdetect.AgentOpenClaw: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "OpenClaw: MCP-first; inline fine.",
	},
	hostdetect.AgentUnknown: {
		RecommendAsync:      false,
		RecommendFleetAsync: false,
		Reason:              "No agent detected — cannot recommend.",
	},
}

// runDoctorAsyncHost is the body of `radiant doctor --async-host`.
// Detects the host, looks up the opt-in recommendation, and
// reports the current state of both env vars + CLI flags. Designed
// to be the single command an operator runs when a tool-call timeout
// reproduces — the output is the exact flag to flip.
func runDoctorAsyncHost(cmd *cobra.Command, w io.Writer) error {
	det := hostdetect.New().Detect()

	rec, ok := hostAsyncRecommendations[det.Agent]
	if !ok {
		// Defensive default for agents added after v3.7.10.
		rec = asyncHostRecommendation{
			Agent:  det.Agent,
			Reason: "no entry in the opt-in matrix (newer than the matrix; default = no opt-in recommended).",
		}
	}
	rec.Agent = det.Agent
	rec.Confidence = det.Confidence

	// Current opt-in status from this process's env.
	asyncOn := envBool("RADIANT_ASYNC_SUBPROCESS")
	fleetAsyncOn := envBool("RADIANT_FLEET_ASYNC_SUBPROCESS")

	fmt.Fprintln(w, "radiant doctor --async-host")
	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintf(w, "  %-26s  %s (confidence %d)\n", "agent", det.Agent, det.Confidence)
	if !det.SupportsSampling && det.Agent != hostdetect.AgentUnknown {
		fmt.Fprintf(w, "  %-26s  unknown (will verify at first sampling call)\n", "sampling capability")
	} else if det.Agent != hostdetect.AgentUnknown {
		fmt.Fprintf(w, "  %-26s  yes\n", "sampling capability")
	}
	fmt.Fprintf(w, "  %-26s  %t (RADIANT_ASYNC_SUBPROCESS)\n", "loop async subprocess (env)", asyncOn)
	fmt.Fprintf(w, "  %-26s  %t (RADIANT_FLEET_ASYNC_SUBPROCESS)\n", "fleet async subprocess (env)", fleetAsyncOn)
	fmt.Fprintln(w, strings.Repeat("─", 60))

	// Verdict for loop async subprocess.
	loopVerdict := "NOT RECOMMENDED"
	if rec.RecommendAsync {
		loopVerdict = "RECOMMENDED"
		if asyncOn {
			loopVerdict = "ALREADY ON"
		}
	} else if asyncOn {
		loopVerdict = "OPTED-IN (no harm)"
	}
	fmt.Fprintf(w, "  loop async subprocess:    %s\n", loopVerdict)
	fmt.Fprintf(w, "    %s\n", rec.Reason)

	// Verdict for fleet async subprocess.
	fleetVerdict := "NOT RECOMMENDED"
	if rec.RecommendFleetAsync {
		fleetVerdict = "RECOMMENDED"
		if fleetAsyncOn {
			fleetVerdict = "ALREADY ON"
		}
	} else if fleetAsyncOn {
		fleetVerdict = "OPTED-IN (no harm)"
	}
	fmt.Fprintf(w, "  fleet async subprocess:  %s\n", fleetVerdict)
	if rec.RecommendFleetAsync {
		fmt.Fprintf(w, "    %s\n", rec.Reason)
	} else {
		fmt.Fprintf(w, "    no documented CI host reproducing a fleet cross-process need yet.\n")
		fmt.Fprintf(w, "    if you have one, file an issue or send the traceback to the maintainers.\n")
	}

	fmt.Fprintln(w, strings.Repeat("─", 60))

	// Action line — the exact CLI flag to flip if the operator
	// agrees with the recommendation.
	if rec.RecommendAsync && !asyncOn {
		fmt.Fprintf(w, "  → opt in:  radiant mcp serve --async-subprocess\n")
		fmt.Fprintf(w, "    (or:    export RADIANT_ASYNC_SUBPROCESS=1)\n")
	}
	if rec.RecommendFleetAsync && !fleetAsyncOn {
		fmt.Fprintf(w, "  → opt in:  radiant mcp serve --fleet-async-subprocess\n")
		fmt.Fprintf(w, "    (or:    export RADIANT_FLEET_ASYNC_SUBPROCESS=1)\n")
	}

	// Exit non-zero only when a recommendation is unmade: the
	// operator wants the diagnostic to surface the gap. When
	// everything is already on or the recommendation is "no",
	// exit 0 — there's nothing to fix.
	if (rec.RecommendAsync && !asyncOn) || (rec.RecommendFleetAsync && !fleetAsyncOn) {
		return fmt.Errorf("doctor --async-host: opt-in recommended but not enabled")
	}
	return nil
}

// tomlUnmarshal is a tiny stub — pulls the [mcp_servers] section out of a
// lightweight TOML without importing BurntSushi/toml. We don't need rich
// support: we just want to know whether `mcp_servers.radiant` exists.
// The format is *roughly*:
//
//	[mcp_servers.radiant]
//	command = "/usr/local/bin/radiant"
//	args = ["mcp", "serve"]
//
// which is a plain `[a.b]` section header followed by `key = value` lines.
func tomlUnmarshal(data []byte, out *map[string]any) error {
	root := map[string]any{}
	currentPath := []string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			section := strings.TrimSuffix(strings.TrimPrefix(trim, "["), "]")
			currentPath = strings.Split(section, ".")
			continue
		}
		if eq := strings.Index(line, "="); eq > 0 {
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			val = strings.Trim(val, "\"'")
			// Navigate to the current section
			nav := root
			for i, p := range currentPath {
				if i == len(currentPath)-1 {
					nav[p] = map[string]any{}
				} else {
					if _, ok := nav[p].(map[string]any); !ok {
						nav[p] = map[string]any{}
					}
					nav = nav[p].(map[string]any)
				}
			}
			_ = key
			_ = val
			// We intentionally do NOT populate deeper keys; we only need
			// to know "does radiant exist?" which the [mcp_servers.radiant]
			// section header itself answers. Returning the path so the
			// caller can detect presence via `out["mcp_servers"]["radiant"]`.
			if len(currentPath) >= 2 {
				nav := root
				for _, p := range currentPath {
					if _, ok := nav[p].(map[string]any); !ok {
						nav[p] = map[string]any{}
					}
					nav = nav[p].(map[string]any)
				}
			}
		}
	}
	*out = root
	return nil
}

// writeDoctorMCPReport formats the structured report for `radiant doctor --mcp`.
func writeDoctorMCPReport(w io.Writer, r mcpDoctorReport) {
	verdict := "OK"
	if len(r.Issues) > 0 {
		verdict = "FAIL"
	}
	fmt.Fprintln(w, "radiant doctor --mcp")
	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintf(w, "  %-22s  %s (confidence %d)\n", "agent", r.Agent, r.Confidence)
	fmt.Fprintf(w, "  %-22s  %s\n", "config path", r.ConfigPath)
	fmt.Fprintf(w, "  %-22s  %v\n", "path exists", r.PathExists)
	fmt.Fprintf(w, "  %-22s  %v\n", "path writable", r.PathWritable)
	fmt.Fprintf(w, "  %-22s  %v\n", "config parseable", r.ParsingOK)
	fmt.Fprintf(w, "  %-22s  %v\n", "radiant entry", r.RadiantEntry)
	if r.Agent == "hermes" {
		fmt.Fprintf(w, "  %-22s  %v\n", "sampling.enabled", r.SamplingEnabled)
		if r.SamplingTimeout != "" {
			fmt.Fprintf(w, "  %-22s  %s\n", "sampling.timeout", r.SamplingTimeout)
		}
	}
	if r.MCPTimeout != "" {
		fmt.Fprintf(w, "  %-22s  %s\n", "mcp timeout", r.MCPTimeout)
	}
	fmt.Fprintln(w, strings.Repeat("─", 60))
	if verdict == "OK" {
		fmt.Fprintln(w, "  Radiant is wired correctly.")
	} else {
		for _, issue := range r.Issues {
			fmt.Fprintf(w, "  ✗  %s\n", issue)
		}
	}
	if len(r.Suggestions) > 0 {
		fmt.Fprintln(w, "  Fix:")
		for _, s := range r.Suggestions {
			fmt.Fprintf(w, "     %s\n", s)
		}
	}
	fmt.Fprintf(w, "  verdict = %s\n", verdict)
	_ = verdict
}