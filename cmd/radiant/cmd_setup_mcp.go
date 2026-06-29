package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// registerSetupMCPCmd registers `radiant setup-mcp` under root. The
// per-agent config merges (Codex TOML, OpenCode, Hermes YAML, Kimi,
// OpenClaw, Cline) live in cmd_setup_mcp_per_agent.go. The generic
// JSON merges (Claude / Cursor / Windsurf / VSCode / Zed) live below
// because they're the default format that five of the eleven
// supported agents share.
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
					"hermes|kimi|openclaw|cline|MiniMax to specify one", cwd)
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

	cmd.Flags().StringVar(&agentFlag, "agent", "", "agent to configure: claude|cursor|windsurf|zed|vscode|codex|opencode|hermes|kimi|openclaw|cline|MiniMax (comma-separated; default: auto-detect)")
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
// The first five agents (claude/cursor/windsurf/zed/vscode) use the
// generic JSON merge helpers in this file. The remaining six
// (codex/opencode/hermes/kimi/openclaw/cline) use specialized merge
// functions in cmd_setup_mcp_per_agent.go.
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

	case "MiniMax":
		// MiniMax Code (MiniMax AI) reads MCP servers from a JSON config
		// at .MiniMax/mcp.json (project) or ~/.MiniMax/mcp.json (global).
		// Format mirrors the standard mcpServers shape used by Claude/Cursor.
		// Override with $MINIMAX_CODE_CONFIG if the install uses a different path.
		var mmTarget string
		if cfg := os.Getenv("MINIMAX_CODE_CONFIG"); cfg != "" && global {
			mmTarget = cfg
		} else if global {
			home, _ := os.UserHomeDir()
			mmTarget = filepath.Join(home, ".MiniMax", "mcp.json")
		} else {
			mmTarget = filepath.Join(cwd, ".MiniMax", "mcp.json")
		}
		content, err := mergeMCPJSON(mmTarget, entry)
		return mmTarget, content, err

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
		return "", "", fmt.Errorf("unknown agent %q (supported: claude, cursor, windsurf, zed, vscode, codex, opencode, hermes, kimi, openclaw, cline, MiniMax)", agent)
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
