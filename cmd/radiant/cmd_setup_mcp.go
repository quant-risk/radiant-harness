package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func registerSetupMCPCmd(root *cobra.Command) {
	var agentFlag string
	var globalFlag bool
	var forceFlag bool
	var dryRunFlag bool

	cmd := &cobra.Command{
		Use:   "setup-mcp",
		Short: "Register radiant as an MCP server in your agent's config",
		Long: `Detects your agent and writes the MCP server entry so any prompt
can invoke radiant_run automatically.

Supported agents (auto-detected):
  Claude Code, Cursor, Windsurf, Zed, VSCode, Codex (OpenAI), OpenCode,
  Hermes (NousResearch), Kimi CLI (Moonshot), OpenClaw, Cline.

  radiant setup-mcp                  # auto-detect agent
  radiant setup-mcp --agent=codex    # specific agent (comma-separated for multiple)
  radiant setup-mcp --global         # write to user-level config (~/.claude/, etc.)
  radiant setup-mcp --dry-run        # show what would be written`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binaryPath, err := radiantBinaryPath()
			if err != nil {
				return fmt.Errorf("cannot determine radiant binary path: %w", err)
			}

			cwd, _ := os.Getwd()

			agents := resolveMCPAgents(agentFlag, cwd)
			if len(agents) == 0 {
				return fmt.Errorf("no supported agent detected in %s\n"+
					"Use --agent=claude|cursor|windsurf|zed|vscode|codex|opencode|"+
					"hermes|kimi|openclaw|cline to specify one", cwd)
			}

			for _, a := range agents {
				target, content, writeErr := mcpConfigFor(a, binaryPath, cwd, globalFlag)
				if writeErr != nil {
					fmt.Printf("  [skip] %s: %v\n", a, writeErr)
					continue
				}

				if dryRunFlag {
					fmt.Printf("  [dry-run] %s → %s\n%s\n", a, target, content)
					continue
				}

				if err2 := writeMCPConfig(target, content, forceFlag); err2 != nil {
					fmt.Printf("  [error] %s: %v\n", a, err2)
				} else {
					fmt.Printf("  ✓ %-10s → %s\n", a, target)
				}
			}

			if !dryRunFlag {
				fmt.Println()
				fmt.Println("Done. Any agent prompt now works:")
				fmt.Println(`  "use radiant-harness to: <your goal>"`)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agentFlag, "agent", "", "agent to configure: claude|cursor|windsurf|zed|vscode|codex|opencode|hermes|kimi|openclaw|cline (comma-separated; default: auto-detect)")
	cmd.Flags().BoolVar(&globalFlag, "global", false, "write to user-level config instead of project-level")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "overwrite existing MCP entry")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "show what would be written without writing")
	root.AddCommand(cmd)
}

// radiantBinaryPath returns the absolute path to the running radiant binary.
func radiantBinaryPath() (string, error) {
	self, err := os.Executable()
	if err == nil {
		self, err = filepath.EvalSymlinks(self)
	}
	if err == nil {
		return self, nil
	}
	// Fallback: find in PATH.
	return exec.LookPath("radiant")
}

// resolveMCPAgents returns the list of agents to configure.
// If agentFlag is set, use that. Otherwise auto-detect from cwd.
func resolveMCPAgents(agentFlag, cwd string) []string {
	if agentFlag != "" {
		var out []string
		for _, a := range strings.Split(agentFlag, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				out = append(out, a)
			}
		}
		return out
	}

	// Auto-detect by presence of config dirs/files.
	var detected []string
	checks := []struct {
		name string
		path string
	}{
		{"claude", filepath.Join(cwd, ".claude")},
		{"cursor", filepath.Join(cwd, ".cursor")},
		{"windsurf", filepath.Join(cwd, ".windsurf")},
		{"zed", filepath.Join(cwd, ".zed")},
		{"vscode", filepath.Join(cwd, ".vscode")},
		{"codex", filepath.Join(cwd, ".codex")},
		{"opencode", filepath.Join(cwd, ".opencode")},
		{"hermes", filepath.Join(cwd, ".hermes")},
		{"openclaw", filepath.Join(cwd, ".openclaw")},
	}
	for _, c := range checks {
		if _, err := os.Stat(c.path); err == nil {
			detected = append(detected, c.name)
		}
	}

	// Kimi CLI and Cline are global-only — detect by presence of their
	// global config dir as a fallback. They are appended AFTER the
	// project-local checks above so the project's primary agent stays
	// first in the list.
	home, _ := os.UserHomeDir()
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".kimi")); err == nil {
			detected = append(detected, "kimi")
		}
		if _, err := os.Stat(filepath.Join(home, ".cline")); err == nil {
			detected = append(detected, "cline")
		}
	}

	// Always include claude as fallback — it's the most common.
	if len(detected) == 0 {
		detected = []string{"claude"}
	}
	return detected
}

// mcpEntry is the JSON structure for one MCP server entry.
type mcpEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// mcpConfigFor returns (targetPath, jsonContent, error) for a given agent.
func mcpConfigFor(agent, binaryPath, cwd string, global bool) (string, string, error) {
	entry := mcpEntry{
		Command: binaryPath,
		Args:    []string{"mcp", "serve"},
	}

	switch agent {
	case "claude":
		var target string
		if global {
			home, _ := os.UserHomeDir()
			target = filepath.Join(home, ".claude", "settings.json")
		} else {
			target = filepath.Join(cwd, ".claude", "settings.json")
		}
		content, err := mergeClaudeSettings(target, entry)
		return target, content, err

	case "cursor":
		target := filepath.Join(cwd, ".cursor", "mcp.json")
		content, err := mergeMCPJSON(target, entry)
		return target, content, err

	case "windsurf":
		target := filepath.Join(cwd, ".windsurf", "mcp.json")
		content, err := mergeMCPJSON(target, entry)
		return target, content, err

	case "zed":
		target := filepath.Join(cwd, ".zed", "settings.json")
		content, err := mergeZedSettings(target, entry)
		return target, content, err

	case "vscode":
		target := filepath.Join(cwd, ".vscode", "mcp.json")
		content, err := mergeMCPJSON(target, entry)
		return target, content, err

	case "codex":
		// Codex (OpenAI CLI) uses TOML config with [mcp_servers.<name>]
		// blocks. Project-level: .codex/config.toml. Global: ~/.codex/config.toml.
		var target string
		if global {
			home, _ := os.UserHomeDir()
			target = filepath.Join(home, ".codex", "config.toml")
		} else {
			target = filepath.Join(cwd, ".codex", "config.toml")
		}
		content, err := mergeCodexTOML(target, entry)
		return target, content, err

	case "opencode":
		// OpenCode (sst/opencode) uses JSON config with `mcp.<name>` block.
		// Project-level: .opencode/config.json. Global: ~/.config/opencode/config.json.
		var target string
		if global {
			home, _ := os.UserHomeDir()
			target = filepath.Join(home, ".config", "opencode", "config.json")
		} else {
			target = filepath.Join(cwd, ".opencode", "config.json")
		}
		content, err := mergeOpenCodeConfig(target, entry)
		return target, content, err

	case "hermes":
		// Hermes Agent (NousResearch) uses a YAML config file
		//   (~/.hermes/config.yaml or .hermes/config.yaml) with an
		//   `mcp_servers` top-level key whose value is a map of
		//   server-name → {command, args}.
		var target string
		if global {
			home, _ := os.UserHomeDir()
			target = filepath.Join(home, ".hermes", "config.yaml")
		} else {
			target = filepath.Join(cwd, ".hermes", "config.yaml")
		}
		content, err := mergeHermesConfig(target, entry)
		return target, content, err

	case "kimi":
		// Kimi CLI (Moonshot AI) stores MCP servers globally only:
		//   ~/.kimi/mcp.json  (or $KIMI_SHARE_DIR/mcp.json).
		// Standard mcpServers shape — same as Claude/Cursor.
		// No project-level file exists in Kimi; the --global flag is
		// always implicit for this agent.
		home, _ := os.UserHomeDir()
		target := filepath.Join(home, ".kimi", "mcp.json")
		content, err := mergeKimiMCP(target, entry)
		return target, content, err

	case "openclaw":
		// OpenClaw uses { mcp: { servers: { <name>: {command, args} } } }
		// with many sibling keys under `mcp` and `channels`/`gateway`/etc.
		// at the top level. Project-level: .openclaw/openclaw.json.
		// Global: ~/.openclaw/openclaw.json.
		var target string
		if global {
			home, _ := os.UserHomeDir()
			target = filepath.Join(home, ".openclaw", "openclaw.json")
		} else {
			target = filepath.Join(cwd, ".openclaw", "openclaw.json")
		}
		content, err := mergeOpenClawJSONConfig(target, entry)
		return target, content, err

	case "cline":
		// Cline CLI writes to ~/.cline/mcp.json with the standard
		// mcpServers shape. Cline's official examples include optional
		// `disabled` and `autoApprove` fields — mergeClineConfig emits
		// both for compatibility.
		// Cline is global-only via its CLI config; the VS Code
		// extension-managed file lives elsewhere and is not addressed
		// by `radiant setup-mcp`.
		home, _ := os.UserHomeDir()
		target := filepath.Join(home, ".cline", "mcp.json")
		content, err := mergeClineConfig(target, entry)
		return target, content, err

	default:
		return "", "", fmt.Errorf("unknown agent %q (supported: claude, cursor, windsurf, zed, vscode, codex, opencode, hermes, kimi, openclaw, cline)", agent)
	}
}

// mergeClaudeSettings merges the radiant MCP entry into .claude/settings.json.
func mergeClaudeSettings(path string, entry mcpEntry) (string, error) {
	var settings map[string]json.RawMessage
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]json.RawMessage)
	}

	// Read existing mcpServers.
	servers := make(map[string]mcpEntry)
	if raw, ok := settings["mcpServers"]; ok {
		_ = json.Unmarshal(raw, &servers)
	}
	servers["radiant"] = entry

	b, _ := json.Marshal(servers)
	settings["mcpServers"] = json.RawMessage(b)

	out, err := json.MarshalIndent(settings, "", "  ")
	return string(out), err
}

// mergeMCPJSON merges into a generic {mcpServers: {...}} file (Cursor, Windsurf, VSCode).
func mergeMCPJSON(path string, entry mcpEntry) (string, error) {
	type mcpFile struct {
		Servers map[string]mcpEntry `json:"mcpServers"`
	}
	var f mcpFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	if f.Servers == nil {
		f.Servers = make(map[string]mcpEntry)
	}
	f.Servers["radiant"] = entry
	out, err := json.MarshalIndent(f, "", "  ")
	return string(out), err
}

// mergeZedSettings merges into .zed/settings.json under context_servers.
func mergeZedSettings(path string, entry mcpEntry) (string, error) {
	var settings map[string]json.RawMessage
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]json.RawMessage)
	}

	type zedServer struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	servers := make(map[string]zedServer)
	if raw, ok := settings["context_servers"]; ok {
		_ = json.Unmarshal(raw, &servers)
	}
	servers["radiant"] = zedServer(entry)

	b, _ := json.Marshal(servers)
	settings["context_servers"] = json.RawMessage(b)

	out, err := json.MarshalIndent(settings, "", "  ")
	return string(out), err
}

// writeMCPConfig writes content to path, creating parent dirs as needed.
func writeMCPConfig(path, content string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		// File exists — check if radiant entry is already there.
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), `"radiant"`) {
			return fmt.Errorf("already configured (use --force to overwrite)")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content+"\n"), 0o644)
}

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
// `command` + `args` shape as the stdio MCP standard.
type hermesEntry struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
}

// mergeHermesConfig reads the existing YAML config (if any), inserts or
// replaces `mcp_servers.radiant`, and returns the merged YAML content.
// Every other top-level key (model, terminal, browser, ...) is preserved.
func mergeHermesConfig(path string, entry mcpEntry) (string, error) {
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
	servers["radiant"] = hermesEntry{
		Command: entry.Command,
		Args:    entry.Args,
	}
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
