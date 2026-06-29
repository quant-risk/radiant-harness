package main

// Per-agent MCP config merges. These six agents each have a non-trivial
// merge function beyond the generic JSON-shape used by Claude/Cursor/
// Windsurf/VSCode/Zed. Grouping them in this file (split off in
// Sprint 76) keeps cmd_setup_mcp.go focused on the routing and
// detection logic, and makes it cheap to add a 12th agent by appending
// one block.
//
// Imports needed only here:
//   - regexp              (Codex TOML block pattern)
//   - gopkg.in/yaml.v3    (Hermes YAML round-trip)
//
// All other imports (encoding/json, fmt, os, strings) are shared with
// the main setup-mcp file.

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Codex (OpenAI CLI) ──────────────────────────────────────────────────────
//
// Codex stores MCP config in TOML:
//
//	[mcp_servers.radiant]
//	command = "/usr/local/bin/radiant"
//	args = ["mcp", "serve"]
//
// We do a minimal TOML merge: find any existing `[mcp_servers.radiant]`
// block and replace it; otherwise append. Other sections are preserved
// verbatim.

// radiantBlockPattern matches a [mcp_servers.radiant] table block,
// including all scalar fields and inline tables/arrays. We capture
// up to (but not including) the next top-level section header or
// end of file. The leading `(?:\n\[|\z)` consumes the newline
// before the next section header so the replacement leaves a clean
// gap.
//
// RE2 doesn't support lookahead, so we match the trailing `\n[` as
// part of the captured text and trim it off in the replacement.
var radiantBlockPattern = regexp.MustCompile(`(?ms)^\[mcp_servers\.radiant\][\s\S]*?(?:\n\[|\z)`)

// tomlQuote returns a TOML-safe double-quoted string. Handles
// backslash and double-quote escaping per TOML 1.0 spec.
func tomlQuote(s string) string {
	// Escape backslashes first, then double quotes.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	// Newlines and control chars need to be escaped too, but our
	// values (binary path, command args) don't typically contain
	// them. Future-proof with a quick newline pass.
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}

// mergeCodexTOML returns the merged TOML content with the radiant
// MCP server entry. Existing non-radiant sections are preserved.
func mergeCodexTOML(path string, entry mcpEntry) (string, error) {
	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	// Build the new [mcp_servers.radiant] block.
	var sb strings.Builder
	sb.WriteString("[mcp_servers.radiant]\n")
	sb.WriteString("command = ")
	sb.WriteString(tomlQuote(entry.Command))
	sb.WriteString("\n")
	sb.WriteString("args = [")
	for i, a := range entry.Args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(tomlQuote(a))
	}
	sb.WriteString("]\n")

	// If existing contains a radiant block, replace it.
	if existing != "" {
		if loc := radiantBlockPattern.FindStringIndex(existing); loc != nil {
			// loc[1] includes the trailing "\n[" — trim that so we
			// don't lose the next section's header. The block capture
			// pattern ends with "\n[" (the start of the next section),
			// so slice off the last 2 chars.
			end := loc[1]
			if end >= 2 && existing[end-2:end] == "\n[" {
				end -= 2 // drop "\n[" so the next section's "[" is preserved
			}
			existing = existing[:loc[0]] + existing[end:]
		}
		// Trim trailing whitespace from the prefix, then append.
		merged := strings.TrimRight(existing, " \t\n") + "\n\n" + sb.String()
		return merged, nil
	}
	return sb.String(), nil
}

// ── OpenCode (sst/opencode) ──────────────────────────────────────────────────
//
// OpenCode stores MCP config in JSON:
//
//	{
//	  "$schema": "https://opencode.ai/config.json",
//	  "mcp": {
//	    "radiant": {
//	      "type": "local",
//	      "command": ["/usr/local/bin/radiant", "mcp", "serve"],
//	      "environment": {}
//	    }
//	  }
//	}
//
// Note: OpenCode uses `mcp` (not `mcpServers`), and `command` is an
// array (not a string). `type: "local"` distinguishes subprocess
// from remote (HTTP).

type openCodeServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment,omitempty"`
}

type openCodeConfig struct {
	Schema   string                            `json:"$schema,omitempty"`
	MCP      map[string]openCodeServer         `json:"mcp,omitempty"`
	OtherRaw map[string]json.RawMessage        `json:"-"` // preserve unknown keys
	raw      []byte                            // raw bytes for round-trip preservation
}

// mergeOpenCodeConfig reads the existing JSON config (if any), adds
// or replaces the radiant entry under `mcp`, and returns the merged
// JSON content. Unknown top-level keys are preserved verbatim.
func mergeOpenCodeConfig(path string, entry mcpEntry) (string, error) {
	cfg := openCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP:    map[string]openCodeServer{},
	}

	if data, err := os.ReadFile(path); err == nil {
		cfg.raw = data
		// Decode into a flexible map first so we can preserve unknown keys.
		var flexible map[string]json.RawMessage
		if err := json.Unmarshal(data, &flexible); err == nil {
			if raw, ok := flexible["mcp"]; ok {
				_ = json.Unmarshal(raw, &cfg.MCP)
			}
			cfg.OtherRaw = flexible
			delete(cfg.OtherRaw, "mcp")
		}
	}

	if cfg.MCP == nil {
		cfg.MCP = map[string]openCodeServer{}
	}

	// Build the radiant entry.
	cmd := append([]string{entry.Command}, entry.Args...)
	cfg.MCP["radiant"] = openCodeServer{
		Type:        "local",
		Command:     cmd,
		Environment: map[string]string{},
	}

	// Reconstruct the JSON. To preserve unknown keys, we have to
	// rebuild the map manually rather than re-marshalling cfg.
	out := make(map[string]any, len(cfg.OtherRaw)+1)
	for k, v := range cfg.OtherRaw {
		var x any
		if err := json.Unmarshal(v, &x); err == nil {
			out[k] = x
		}
	}
	// Put $schema at top if present.
	if cfg.Schema != "" {
		out["$schema"] = cfg.Schema
	}
	out["mcp"] = cfg.MCP

	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

// ── Hermes Agent (NousResearch) ──────────────────────────────────────────────
//
// Hermes stores configuration in YAML at ~/.hermes/config.yaml (or
// .hermes/config.yaml for project-level overrides). The MCP servers key
// is `mcp_servers` at the top level:
//
//	mcp_servers:
//	  time:
//	    command: "uvx"
//	    args: ["mcp-server-time", "--some-arg"]
//
// The rest of the config (model, terminal, browser, …) is large and
// user-customized. We MUST round-trip everything else byte-for-byte,
// which is why this is the only handler that needs yaml.v3 — every other
// agent in this file uses JSON.
//
// We decode the existing file into a generic map, locate (or create)
// `mcp_servers`, set `radiant`, and re-encode. Unknown keys are preserved
// verbatim because yaml.v3 round-trips through `map[string]any`.

// hermesEntry is the YAML shape of one MCP server. Hermes uses the same
// `command` + `args` shape as the stdio MCP standard, plus optional
// `timeout` (outer MCP server timeout), `cwd` (working directory), and
// a nested `sampling:` block (Hermes Nous Research's MCP sampling config)
// that the host reads to decide whether to respond to
// sampling/createMessage calls. Without that block the host silently
// drops the request, the harness times out, and the loop exits with
// `critical_failure`.
type hermesEntry struct {
	Command  string         `yaml:"command"`
	Args     []string       `yaml:"args,omitempty"`
	Timeout  int            `yaml:"timeout,omitempty"`
	Cwd      string         `yaml:"cwd,omitempty"`
	Sampling map[string]any `yaml:"sampling,omitempty"`
}

// hermesSamplingEnabled returns the default Hermes sampling block for the
// radiant MCP server. The defaults are calibrated to keep a long-running
// possession loop happy when the host (Hermes + xiaomi / mimo /
// OpenRouter) occasionally takes 30–40 s on a single sampling call (cold
// start or cumulative latency). Anything lower and the third call of a
// 3-call loop will time out, and the harness exits with critical_failure.
//
// To override per-user, edit ~/.hermes/config.yaml directly:
//
//	sampling:
//	  model: openrouter/google/gemini-2.5-flash  # force a faster model
//	  timeout: 60                                 # tighter cap
func hermesSamplingEnabled() map[string]any {
	return map[string]any{
		"enabled":         true,
		"timeout":         120,
		"max_tokens_cap":  8192,
		"max_tool_rounds": 5,
	}
}

// mergeHermesConfig reads the existing YAML config (if any), inserts or
// replaces `mcp_servers.radiant`, and returns the merged YAML content.
// Every other top-level key (model, terminal, browser, ...) is preserved.
//
// When `writeSamplingBlock` is true (the default for `radiant setup-mcp`),
// the `sampling:` nested key is populated with the defaults from
// hermesSamplingEnabled(). This is what makes the host respond to
// sampling/createMessage instead of failing with "sampling not enabled".
func mergeHermesConfig(path string, entry mcpEntry, writeSamplingBlock bool) (string, error) {
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &root); err != nil {
				return "", fmt.Errorf("hermes config at %s is not valid YAML: %w", path, err)
			}
		}
	}
	if root == nil {
		root = map[string]any{}
	}

	servers, _ := root["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	he := hermesEntry{
		Command: entry.Command,
		Args:    entry.Args,
		Timeout: 300, // outer MCP server timeout (long enough for a full radiant_run)
	}
	if writeSamplingBlock {
		he.Sampling = hermesSamplingEnabled()
	}
	servers["radiant"] = he
	root["mcp_servers"] = servers

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ── Kimi CLI (Moonshot AI) ───────────────────────────────────────────────────
//
// Kimi stores MCP servers GLOBALLY only — there is no project-level MCP
// config. The CLI command `kimi mcp add` writes to:
//
//	$HOME/.kimi/mcp.json                (or $KIMI_SHARE_DIR/mcp.json)
//
// Shape matches the Claude/Cursor standard:
//
//	{
//	  "mcpServers": {
//	    "context7": {
//	      "url": "https://mcp.context7.com/mcp",
//	      "headers": { "CONTEXT7_API_KEY": "..." }
//	    },
//	    "radiant": {
//	      "command": "/usr/local/bin/radiant",
//	      "args": ["mcp", "serve"]
//	    }
//	  }
//	}
//
// Kimi also supports ad-hoc configs via `kimi --mcp-config-file ...` but
// that's outside the scope of `setup-mcp`.

// mergeKimiMCP returns the merged mcpServers JSON for Kimi's global
// config file. Other top-level keys (none currently, but safe for
// future additions) are preserved.
func mergeKimiMCP(path string, entry mcpEntry) (string, error) {
	type kimiFile struct {
		Servers map[string]mcpEntry `json:"mcpServers"`
	}
	var f kimiFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	if f.Servers == nil {
		f.Servers = make(map[string]mcpEntry)
	}
	f.Servers["radiant"] = entry
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// ── OpenClaw ────────────────────────────────────────────────────────────────
//
// OpenClaw saves third-party MCP server definitions under:
//
//	~/.openclaw/openclaw.json           (or .openclaw/openclaw.json)
//	{
//	  "channels": { ... },
//	  "gateway":  { ... },
//	  "mcp": {
//	    "sessionIdleTtlMs": 600000,
//	    "servers": {
//	      "context7": {
//	        "command": "uvx",
//	        "args": ["context7-mcp"]
//	      },
//	      "radiant": {
//	        "command": "/usr/local/bin/radiant",
//	        "args": ["mcp", "serve"]
//	      }
//	    }
//	  }
//	}
//
// The `mcp` key has many siblings (`sessionIdleTtlMs`, `defaultTools...`)
// and there are unrelated top-level keys (`channels`, `gateway`,
// `skills`, ...). We preserve unknown keys at BOTH levels.

// openClawServer is the JSON shape OpenClaw uses for stdio MCP servers.
// We omit `type`/`transport` for stdio (the default) and `url`/`headers`
// for remote servers — neither applies to a local binary entry.
type openClawServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// mergeOpenClawJSONConfig preserves every top-level key and every
// sibling of `mcp.servers` except the `servers` map itself, which is
// what we merge into.
func mergeOpenClawJSONConfig(path string, entry mcpEntry) (string, error) {
	root := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &root)
	}

	// Extract or create `mcp` object.
	mcp := map[string]json.RawMessage{}
	if raw, ok := root["mcp"]; ok {
		_ = json.Unmarshal(raw, &mcp)
	}
	// Extract or create `mcp.servers` map.
	servers := map[string]openClawServer{}
	if raw, ok := mcp["servers"]; ok {
		_ = json.Unmarshal(raw, &servers)
	}
	servers["radiant"] = openClawServer{
		Command: entry.Command,
		Args:    entry.Args,
	}
	srvBytes, err := json.Marshal(servers)
	if err != nil {
		return "", err
	}
	mcp["servers"] = srvBytes
	mcpBytes, err := json.Marshal(mcp)
	if err != nil {
		return "", err
	}
	root["mcp"] = mcpBytes

	// Decode-and-rebuild to preserve original key ordering better than
	// json.Marshal(map) does. We use a two-step: RawMessage preserves
	// the bytes; the final MarshalIndent sorts keys alphabetically,
	// which is acceptable for OpenClaw (it accepts diff-friendly JSON).
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// ── Cline (CLI) ──────────────────────────────────────────────────────────────
//
// Cline CLI persists MCP servers in ~/.cline/mcp.json with the standard
// `mcpServers` shape. Cline's official examples include optional
// `disabled` and `autoApprove` fields on each entry; we emit both for
// shape parity with their docs:
//
//	{
//	  "mcpServers": {
//	    "local-server": {
//	      "command": "node",
//	      "args": ["/path/to/server.js"],
//	      "env": { "API_KEY": "..." },
//	      "disabled": false,
//	      "autoApprove": []
//	    }
//	  }
//	}
//
// VSCode-extension users manage their config through the Cline UI
// panel; that file lives at a separate path and is intentionally NOT
// addressed by `radiant setup-mcp`.

// clineEntry extends mcpEntry with Cline's two optional fields.
type clineEntry struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Disabled   bool     `json:"disabled"`
	AutoApprove []string `json:"autoApprove"`
}

// mergeClineConfig returns the merged mcpServers JSON for Cline's CLI
// config file, emitting `disabled: false` and `autoApprove: []` on
// every entry to match the documented shape.
func mergeClineConfig(path string, entry mcpEntry) (string, error) {
	type clineFile struct {
		Servers map[string]clineEntry `json:"mcpServers"`
	}
	var f clineFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	if f.Servers == nil {
		f.Servers = make(map[string]clineEntry)
	}
	f.Servers["radiant"] = clineEntry{
		Command:    entry.Command,
		Args:       entry.Args,
		Disabled:   false,
		AutoApprove: []string{},
	}
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
