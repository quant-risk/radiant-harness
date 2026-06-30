// Package hostdetect — empirical sampling-support probe (v3.6.0+).
//
// The declared `agentSignature.SupportsSampling` value is a best-guess
// based on docs / vendor marketing. Empirically we have observed that
// some agents declared as supporting sampling/createMessage actually
// return JSON-RPC -32601 at runtime (Codex GPT-5 first, then later
// Cline / OpenCode / Kimi / OpenClaw / VSCode). The v3.5.1 handling
// caught the -32601 inside the possession flow and degraded to stub
// mode, but that left the project scaffold empty: the harness had
// nothing useful to show for the run.
//
// v3.6.0 fixes this two ways:
//
//   1. Persistent state file at ~/.radiant-harness/agent-capabilities.json
//      records the empirical probe result per agent. Detect() prefers
//      the recorded value when present, falling back to the declared
//      constant only when no probe has ever run.
//
//   2. Probe runs at the most reliable moment we have — the first
//      `initialize` exchange between the host agent and the radiant MCP
//      server (cmd_mcp_runtime.go::handleMCPRequestLight). That's the
//      earliest point where we have a working sampling channel (when
//      we have it at all) AND it does not delay the user's task.
//
// The state file is written incrementally, so a crash mid-probe does
// not corrupt prior data.
package hostdetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProbeEvidence is a short human-readable note about why we believe
// what we believe. Persisted alongside the boolean so the next run can
// show the user *how* we got the answer.
type ProbeEvidence string

const (
	// EvidenceProbeOK is recorded when the agent answered a tiny
	// sampling/createMessage ping inside the configured timeout.
	EvidenceProbeOK ProbeEvidence = "probe ok"
	// EvidenceUnsupported32601 is recorded when the agent returned
	// JSON-RPC -32601 (method not found) for sampling/createMessage.
	EvidenceUnsupported32601 ProbeEvidence = "json-rpc -32601"
	// EvidenceUnsupportedTimeout is recorded when the agent did not
	// answer within the configured sampling timeout.
	EvidenceUnsupportedTimeout ProbeEvidence = "probe timeout"
	// EvidenceUnsupportedIO is recorded when the sampling channel
	// itself could not be opened (no stdio transport / missing host).
	EvidenceUnsupportedIO ProbeEvidence = "probe io error"
	// EvidenceDeclared is the default when the constant is used and
	// the probe has never run.
	EvidenceDeclared ProbeEvidence = "declared (unverified)"
)

// ProbeResult is the empirically-verified sampling-support state for
// one agent.
type ProbeResult struct {
	Agent            AgentID       `json:"agent"`
	SupportsSampling bool          `json:"supports_sampling"`
	ProbedAt         time.Time     `json:"probed_at"`
	Evidence         ProbeEvidence `json:"evidence"`
	ProbeRoundTrips  int           `json:"probe_round_trips,omitempty"`
}

// capabilitiesFile is the on-disk shape of ~/.radiant-harness/agent-capabilities.json.
// Keys are AgentID values so the file is greppable.
type capabilitiesFile map[AgentID]ProbeResult

// capMu serialises capabilities reads/writes inside the same process.
// Cross-process safety is provided by file rename (atomic on POSIX).
var capMu sync.Mutex

// capabilitiesPath returns the absolute path of the capabilities file.
// Created lazily on first write; reads return an empty map if missing.
func capabilitiesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".radiant-harness", "agent-capabilities.json"), nil
}

// LoadCapabilities returns the persisted state, or an empty map when
// the file is missing / unreadable. Never returns an error — bad data
// is treated as "no prior probe", which falls back to declared values.
func LoadCapabilities() map[AgentID]ProbeResult {
	capMu.Lock()
	defer capMu.Unlock()
	p, err := capabilitiesPath()
	if err != nil {
		return map[AgentID]ProbeResult{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return map[AgentID]ProbeResult{}
	}
	var got capabilitiesFile
	if err := json.Unmarshal(data, &got); err != nil || got == nil {
		return map[AgentID]ProbeResult{}
	}
	out := make(map[AgentID]ProbeResult, len(got))
	for k, v := range got {
		out[k] = v
	}
	return out
}

// SaveCapabilities writes the given map atomically. Missing parent
// dirs are created. Errors are returned but not fatal — the probe is a
// best-effort hint, not a hard contract.
func SaveCapabilities(caps map[AgentID]ProbeResult) error {
	if caps == nil {
		return nil
	}
	capMu.Lock()
	defer capMu.Unlock()
	p, err := capabilitiesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	cp := make(capabilitiesFile, len(caps))
	for k, v := range caps {
		cp[k] = v
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// RecordProbe merges one ProbeResult into the persisted state and
// writes atomically. The new value overwrites any prior entry for the
// same agent. Use this from inside the MCP server's first initialize
// call, or from `radiant setup-mcp` after a manual probe.
func RecordProbe(r ProbeResult) error {
	caps := LoadCapabilities()
	caps[r.Agent] = r
	return SaveCapabilities(caps)
}

// ResolveSupport returns the empirically-verified sampling support for
// an agent. The boolean is the value to use at runtime; probed reports
// whether the answer came from a prior probe (true) or from the
// declared constant (false).
//
// Resolution order (most authoritative first):
//  1. Persisted probe result in ~/.radiant-harness/agent-capabilities.json
//  2. Hard-coded `knownSamplingUnsupported` (agents we have empirically
//     observed to return -32601 in production as of v3.6.0)
//  3. The declared signature constant
//
// Unrecognised agents return (false, false) — same shape as
// "unverified and predicted unsupported".
func ResolveSupport(agent AgentID) (supports, probed bool) {
	if r, ok := LoadCapabilities()[agent]; ok {
		return r.SupportsSampling, true
	}
	if knownSamplingUnsupported[agent] {
		return false, true // well-attested, treat as probed
	}
	if sig, ok := signatures[agent]; ok {
		return sig.SupportsSampling, false
	}
	return false, false
}

// knownSamplingUnsupported is the closed set of agents whose MCP server
// has been empirically observed to return JSON-RPC -32601 for
// sampling/createMessage in production. New entries should be added
// with the date and a short note linking to the case / issue.
//
// Update this when:
//   - A new agent is added to the signatures map and we probe it
//     negatively;
//   - A listed agent ADDS sampling support (move to the signatures
//     `SupportsSampling: true` block + drop from this map).
//
// As of v3.6.0 the only confirmed-no-sampling entry is Codex (GPT-5
// path was discovered on 2026-06-29 in the Hermes mimo case — full
// trace in .radiant-harness/state/).
var knownSamplingUnsupported = map[AgentID]bool{
	AgentCodex: true, // 2026-06-29: GPT-5 + OpenAI Codex CLI return -32601
}

// knownSyncHosts is the closed set of agents whose MCP server uses
// synchronous `wait_for_tool_result` semantics — i.e. they block the
// tool call until the host's model finishes. Calling
// `sampling/createMessage` from inside one of these tools causes a
// deadlock (the host is busy waiting on the tool call, the tool is
// waiting on the sampling response — neither can make progress).
//
// The harness auto-routes `mcp__radiant__possess` on these hosts to
// the async gate primitives (`radiant_possess_async` +
// `radiant_run_gate`) instead of the synchronous possess loop,
// which closes the 120 s deadlock documented in AGENTS-FOR-TASKS.md
// § Hermes TUI workstream.
//
// Membership is decided per-vendor by inspecting the host's MCP
// runtime documentation or running an empirical deadlock probe.
// Add entries when:
//
//   - A vendor confirms their TUI/SDK is synchronous (open issue);
//   - A session against a previously-listed host completes
//     end-to-end inside one MCP round-trip (rare — see PR that
//     moves the agent OFF this list).
//
// As of v3.7.x, the only confirmed entry is Hermes TUI (the 2026-
// 06-29 protocol-level deadlock reproduction). Update this comment
// when the picture changes.
var knownSyncHosts = map[AgentID]bool{
	AgentHermes: true, // Hermes TUI synchronous wait_for_tool_result; v3.7.x first appeared in AGENTS-FOR-TASKS.md § Hermes-TUI workstream.
}

// IsSyncHost reports whether the agent's MCP server uses
// synchronous tool-call semantics that deadlock on a nested
// sampling/createMessage round-trip. The harness uses this to
// auto-route `mcp__radiant__possess` to the async primitives on
// sync hosts.
func IsSyncHost(agent AgentID) bool {
	if agent == "" {
		return false
	}
	return knownSyncHosts[agent]
}
