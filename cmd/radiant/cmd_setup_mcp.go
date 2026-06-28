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

func registerSetupMCPCmd(root *cobra.Command) {
	var agentFlag string
	var globalFlag bool
	var forceFlag bool
	var dryRunFlag bool

	cmd := &cobra.Command{
		Use:   "setup-mcp",
		Short: "Register radiant as an MCP server in your agent's config",
		Long: `Detects your agent (Claude Code, Cursor, Windsurf, Zed) and writes
the MCP server entry so any prompt can invoke radiant_run automatically.

  radiant setup-mcp                  # auto-detect agent
  radiant setup-mcp --agent=claude   # Claude Code only
  radiant setup-mcp --global         # write to user-level config (~/.claude/)
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
					"Use --agent=claude|cursor|windsurf|zed|vscode to specify one", cwd)
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

	cmd.Flags().StringVar(&agentFlag, "agent", "", "agent to configure: claude|cursor|windsurf|zed|vscode (default: auto-detect)")
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
	}
	for _, c := range checks {
		if _, err := os.Stat(c.path); err == nil {
			detected = append(detected, c.name)
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
		Args:    []string{"mcp-serve"},
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

	default:
		return "", "", fmt.Errorf("unknown agent %q (supported: claude, cursor, windsurf, zed, vscode)", agent)
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
