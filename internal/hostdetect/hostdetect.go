// Package hostdetect identifies which agent (if any) is currently
// invoking radiant-harness. Detection is two-layered:
//
//  1. Env-var fingerprint — every major agent sets at least one
//     distinguishing env var when it's running (e.g. Claude Code
//     sets CLAUDE_CODE_ENTRY). High confidence, cheap.
//
//  2. PPID walk — for agents that don't export env vars, or for
//     unknown hosts, the parent process name gives a fallback
//     signal. Medium/Low confidence.
//
// The result is a HostInfo describing what was found. Downstream
// code (e.g. internal/llm/pick.go in Sprint 80) decides what to do
// with the detection — typically: prefer the host's MCP sampling
// when available, fall back to API-key HTTP, else error clearly.
//
// Sprint 79 (v2.49.0) ships:
//   - the registry of agent fingerprints,
//   - Detect() returning HostInfo,
//   - the `radiant host-info` CLI subcommand.
package hostdetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentID identifies a host agent. Values are lowercase, hyphenated.
// Use AgentUnknown when no agent is detected.
type AgentID string

const (
	AgentUnknown     AgentID = ""
	AgentClaudeCode  AgentID = "claude-code"
	AgentCursor      AgentID = "cursor"
	AgentHermes      AgentID = "hermes"
	AgentKimiCLI     AgentID = "kimi-cli"
	AgentOpenClaw    AgentID = "openclaw"
	AgentCodex       AgentID = "codex"
	AgentCline       AgentID = "cline"
	AgentOpenCode    AgentID = "opencode"
	AgentVSCode      AgentID = "vscode-copilot"
)

// knownAgents lists every agent we know how to detect. The order is
// meaningful only for tie-breaking when multiple match.
var knownAgents = []AgentID{
	AgentClaudeCode,
	AgentCursor,
	AgentHermes,
	AgentKimiCLI,
	AgentOpenClaw,
	AgentCodex,
	AgentCline,
	AgentOpenCode,
	AgentVSCode,
}

// HostInfo is what Detect returns. Confidence is 0-100 where 0 means
// "no signal at all". SupportsSampling reflects whether the agent
// can answer MCP sampling/createMessage requests — if true, the
// harness can possess the agent without an API key.
type HostInfo struct {
	Agent            AgentID `json:"agent"`
	Confidence       int      `json:"confidence"`
	SupportsSampling bool     `json:"supports_sampling"`
	SampleEnvVars    []string `json:"sample_env_vars,omitempty"`
	PID              int      `json:"pid"`
	PPID             int      `json:"ppid"`
	ParentCmd        string   `json:"parent_cmd,omitempty"`
	DetectionSource  string   `json:"detection_source"` // "env" | "process-tree" | "none"
}

// agentSignature is the per-agent fingerprint. Each agent has 1+
// env-var keys and 1+ parent binary names that uniquely identify it.
// SupportsSampling documents whether the agent can answer MCP
// sampling/createMessage when it spawns the harness as an MCP
// server child process.
type agentSignature struct {
	ID               AgentID
	EnvVars          []string // any one matches → confidence boost
	ParentBinaries   []string // matches get Medium confidence
	SupportsSampling bool
}

// signatures maps each known agent to its fingerprint. EnvVars is
// "any of" — having at least one match counts. ParentBinaries is
// the same shape (CLI binary names we know about).
var signatures = map[AgentID]agentSignature{
	AgentClaudeCode: {
		ID:               AgentClaudeCode,
		EnvVars:          []string{"CLAUDE_CODE_ENTRY", "CLAUDE_CODE_SSE_PORT", "CLAUDE_CODE_PID"},
		ParentBinaries:   []string{"claude-code", "claude", "Claude"},
		SupportsSampling: true,
	},
	AgentCursor: {
		ID:               AgentCursor,
		EnvVars:          []string{"CURSOR_TRACE_ID", "CURSOR_HOME", "CURSOR_USER_DATA_DIR"},
		ParentBinaries:   []string{"cursor", "Cursor", "cursor-server", "Cursor.exe"},
		SupportsSampling: true,
	},
	AgentHermes: {
		ID:               AgentHermes,
		EnvVars:          []string{"HERMES_VERSION", "HERMES_HOME", "HERMES_AGENT_HOME"},
		ParentBinaries:   []string{"hermes", "hermes-agent", "hermes-cli"},
		SupportsSampling: true,
	},
	AgentKimiCLI: {
		ID:               AgentKimiCLI,
		EnvVars:          []string{"KIMI_SHARE_DIR", "KIMI_VERSION", "KIMI_CONFIG_DIR"},
		ParentBinaries:   []string{"kimi", "kimi-cli", "kimi-shell"},
		SupportsSampling: true, // Kimi CLI exposes MCP integration
	},
	AgentOpenClaw: {
		ID:               AgentOpenClaw,
		EnvVars:          []string{"OPENCLAW_GATEWAY_URL", "OPENCLAW_VERSION", "OPENCLAW_WORKSPACE"},
		ParentBinaries:   []string{"openclaw", "openclaw-cli", "openclaw-gateway"},
		SupportsSampling: true, // OpenClaw's MCP server is its core
	},
	AgentCodex: {
		ID:               AgentCodex,
		EnvVars:          []string{"CODEX_HOME", "CODEX_THREAD_ID", "CODEX_RUNTIME", "CODEX_THREAD_ENV"},
		ParentBinaries:   []string{"codex", "codex-cli", "codex.exe", "OpenAI Codex"},
		SupportsSampling: true, // Codex supports MCP servers
	},
	AgentCline: {
		ID:               AgentCline,
		EnvVars:          []string{"CLINE_USER", "CLINE_VERSION", "CLINE_WORKSPACE"},
		ParentBinaries:   []string{"cline", "cline-host", "Cline", "Cline.exe"},
		SupportsSampling: true, // Cline supports MCP servers
	},
	AgentOpenCode: {
		ID:               AgentOpenCode,
		EnvVars:          []string{"OPENCODE_HOME", "OPENCODE_VERSION", "OPENCODE_CONFIG"},
		ParentBinaries:   []string{"opencode", "opencode-cli", "OpenCode"},
		SupportsSampling: true,
	},
	AgentVSCode: {
		ID:               AgentVSCode,
		EnvVars:          []string{"VSCODE_PID", "VSCODE_IPC_HOOK_CLI", "VSCODE_CWD"},
		ParentBinaries:   []string{"Code Helper", "Code", "code", "vscode", "code-server"},
		SupportsSampling: true, // VS Code Copilot supports MCP
	},
}

// Detector is the runtime detector. Pass os.Getenv-style functions
// or use defaults (os.LookupEnv and os.ReadFile via /proc).
type Detector struct {
	// LookupEnv defaults to os.LookupEnv. Tests override to inject
	// deterministic env without touching the process env.
	LookupEnv func(key string) (string, bool)
	// ReadProcComm defaults to reading /proc/<pid>/comm on Unix.
	// Tests override.
	ReadProcComm func(pid int) (string, error)
	// NowPID defaults to os.Getpid. Tests override.
	NowPID func() int
	// NowPPID defaults to os.Getppid. Tests override.
	NowPPID func() int
}

// New returns a Detector wired with the production defaults.
// (Reading /proc/<ppid>/comm is the typical path on macOS / Linux;
// Windows is deferred to Sprint 80+.)
func New() *Detector {
	return &Detector{
		LookupEnv:    os.LookupEnv,
		ReadProcComm: readProcCommDefault,
		NowPID:        os.Getpid,
		NowPPID:       os.Getppid,
	}
}

// Detect runs all heuristics and returns the best match. Returns
// AgentUnknown + Confidence 0 when nothing matches.
func (d *Detector) Detect() HostInfo {
	if d.LookupEnv == nil {
		d.LookupEnv = os.LookupEnv
	}
	if d.ReadProcComm == nil {
		d.ReadProcComm = readProcCommDefault
	}
	if d.NowPID == nil {
		d.NowPID = os.Getpid
	}
	if d.NowPPID == nil {
		d.NowPPID = os.Getppid
	}

	pid := d.NowPID()
	ppid := d.NowPPID()
	parentCmd, _ := d.ReadProcComm(ppid)

	// Layer 1: env-var fingerprint. Try every known agent; pick the
	// one with the highest number of env-var matches (ties: pick the
	// first in knownAgents order).
	var bestEnv AgentID
	bestEnvHits := 0
	var bestEnvVars []string
	for _, id := range knownAgents {
		sig := signatures[id]
		hits := matchAny(sig.EnvVars, d.LookupEnv)
		if hits == 0 {
			continue
		}
		if hits > bestEnvHits {
			bestEnv = id
			bestEnvHits = hits
			bestEnvVars = collectMatched(sig.EnvVars, d.LookupEnv)
		}
	}

	if bestEnvHits > 0 {
		confidence := 60 + 15*bestEnvHits // 1 hit = 75, 2 hits = 90, 3+ = 100
		if confidence > 100 {
			confidence = 100
		}
		return HostInfo{
			Agent:            bestEnv,
			Confidence:       confidence,
			SupportsSampling: signatures[bestEnv].SupportsSampling,
			SampleEnvVars:    bestEnvVars,
			PID:              pid,
			PPID:             ppid,
			ParentCmd:        parentCmd,
			DetectionSource:  "env",
		}
	}

	// Layer 2: parent-process walk. If the parent comm matches a
	// known binary, give that Medium confidence.
	if parentCmd != "" {
		for _, id := range knownAgents {
			sig := signatures[id]
			for _, pb := range sig.ParentBinaries {
				if strings.EqualFold(strings.TrimSuffix(parentCmd, filepath.Ext(parentCmd)), pb) {
					return HostInfo{
						Agent:            id,
						Confidence:       50,
						SupportsSampling: sig.SupportsSampling,
						PID:              pid,
						PPID:             ppid,
						ParentCmd:        parentCmd,
						DetectionSource:  "process-tree",
					}
				}
			}
		}
	}

	// Nothing found.
	return HostInfo{
		Agent:           AgentUnknown,
		Confidence:      0,
		PID:             pid,
		PPID:            ppid,
		ParentCmd:       parentCmd,
		DetectionSource: "none",
	}
}

// matchAny returns the number of keys in needles that are non-empty
// in env. We check value presence (key set + non-empty) rather than
// just the key being set, since empty env vars are usually noise.
func matchAny(needles []string, lookup func(string) (string, bool)) int {
	hits := 0
	for _, k := range needles {
		if v, ok := lookup(k); ok && v != "" {
			hits++
		}
	}
	return hits
}

// collectMatched returns the subset of needles whose env value is
// non-empty. Used for reporting which env vars triggered the match.
func collectMatched(needles []string, lookup func(string) (string, bool)) []string {
	var out []string
	for _, k := range needles {
		if v, ok := lookup(k); ok && v != "" {
			out = append(out, k)
		}
	}
	return out
}

// String returns a human-readable label for the given AgentID.
// Used by the `radiant host-info` command and by tests.
func (a AgentID) String() string {
	if a == "" {
		return "(unknown)"
	}
	return string(a)
}

// readProcCommDefault reads /proc/<pid>/comm on Linux/macOS.
// Returns an error if /proc isn't mounted (Windows, BSD without procfs).
func readProcCommDefault(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", err
	}
	// comm ends with a newline.
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("empty comm for pid %d", pid)
	}
	return name, nil
}
