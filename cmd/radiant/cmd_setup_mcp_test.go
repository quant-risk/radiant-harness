package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// helper: write a config file at path with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// helper: read a file as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ── Codex (OpenAI CLI) tests ────────────────────────────────────────────────

func TestMergeCodexTOML_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".codex", "config.toml")

	content, err := mergeCodexTOML(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeCodexTOML: %v", err)
	}

	if !strings.Contains(content, "[mcp_servers.radiant]") {
		t.Errorf("missing [mcp_servers.radiant] header in:\n%s", content)
	}
	if !strings.Contains(content, `command = "/usr/local/bin/radiant"`) {
		t.Errorf("missing command line in:\n%s", content)
	}
	if !strings.Contains(content, `args = ["mcp", "serve"]`) {
		t.Errorf("missing args line in:\n%s", content)
	}
}

func TestMergeCodexTOML_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".codex", "config.toml")
	existing := `# Existing config
model = "gpt-5"
max_tokens = 4096

[mcp_servers.other]
command = "/usr/bin/other"
args = ["run"]
`
	writeFile(t, target, existing)

	content, err := mergeCodexTOML(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeCodexTOML: %v", err)
	}

	// Existing top-level keys preserved.
	if !strings.Contains(content, `model = "gpt-5"`) {
		t.Errorf("existing 'model' lost:\n%s", content)
	}
	if !strings.Contains(content, `max_tokens = 4096`) {
		t.Errorf("existing 'max_tokens' lost:\n%s", content)
	}
	// Other MCP server preserved.
	if !strings.Contains(content, "[mcp_servers.other]") {
		t.Errorf("existing [mcp_servers.other] lost:\n%s", content)
	}
	if !strings.Contains(content, `command = "/usr/bin/other"`) {
		t.Errorf("other server's command lost:\n%s", content)
	}
	// New radiant block added.
	if !strings.Contains(content, "[mcp_servers.radiant]") {
		t.Errorf("new radiant block not added:\n%s", content)
	}
}

func TestMergeCodexTOML_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".codex", "config.toml")
	existing := `# Old config
[mcp_servers.radiant]
command = "/old/path/radiant"
args = ["mcp", "serve"]
# trailing comment
`
	writeFile(t, target, existing)

	content, err := mergeCodexTOML(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeCodexTOML: %v", err)
	}

	// Old path replaced.
	if strings.Contains(content, "/old/path/radiant") {
		t.Errorf("old path not replaced:\n%s", content)
	}
	// New path present.
	if !strings.Contains(content, "/usr/local/bin/radiant") {
		t.Errorf("new path not present:\n%s", content)
	}
	// Exactly one [mcp_servers.radiant] block (not duplicated).
	count := strings.Count(content, "[mcp_servers.radiant]")
	if count != 1 {
		t.Errorf("expected 1 radiant block, got %d:\n%s", count, content)
	}
}

func TestTomlQuote_EscapesSpecialChars(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"simple", `"simple"`},
		{`with"quote`, `"with\"quote"`},
		{`with\backslash`, `"with\\backslash"`},
		{"with\nnewline", `"with\nnewline"`},
		{"with\ttab", `"with\ttab"`},
	}
	for _, c := range cases {
		got := tomlQuote(c.in)
		if got != c.want {
			t.Errorf("tomlQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── OpenCode (sst/opencode) tests ───────────────────────────────────────────

func TestMergeOpenCodeConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".opencode", "config.json")

	content, err := mergeOpenCodeConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenCodeConfig: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, content)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("missing or wrong-typed 'mcp' key in:\n%s", content)
	}
	radiant, ok := mcp["radiant"].(map[string]any)
	if !ok {
		t.Fatalf("missing 'radiant' under 'mcp' in:\n%s", content)
	}
	if radiant["type"] != "local" {
		t.Errorf("type: got %v want 'local'", radiant["type"])
	}
	cmd, ok := radiant["command"].([]any)
	if !ok {
		t.Fatalf("'command' is not an array in:\n%s", content)
	}
	if len(cmd) != 3 || cmd[0] != "/usr/local/bin/radiant" ||
		cmd[1] != "mcp" || cmd[2] != "serve" {
		t.Errorf("command array: got %v want ['/usr/local/bin/radiant', 'mcp', 'serve']", cmd)
	}
}

func TestMergeOpenCodeConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".opencode", "config.json")
	existing := `{
  "theme": "dark",
  "provider": {
    "openai": { "apiKey": "sk-test" }
  },
  "mcp": {
    "github": {
      "type": "local",
      "command": ["/usr/bin/gh-mcp"],
      "environment": {}
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeOpenCodeConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenCodeConfig: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	// Unknown top-level keys preserved.
	if cfg["theme"] != "dark" {
		t.Errorf("'theme' lost:\n%s", content)
	}
	if _, ok := cfg["provider"]; !ok {
		t.Errorf("'provider' lost:\n%s", content)
	}

	// Existing MCP server preserved.
	mcp := cfg["mcp"].(map[string]any)
	if _, ok := mcp["github"]; !ok {
		t.Errorf("existing 'github' MCP server lost:\n%s", content)
	}

	// New radiant added.
	radiant := mcp["radiant"].(map[string]any)
	if radiant["type"] != "local" {
		t.Errorf("radiant type wrong: %v", radiant["type"])
	}
}

func TestMergeOpenCodeConfig_ReplacesExistingRadiant(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".opencode", "config.json")
	existing := `{
  "mcp": {
    "radiant": {
      "type": "local",
      "command": ["/old/path/radiant", "mcp", "serve"],
      "environment": {}
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeOpenCodeConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenCodeConfig: %v", err)
	}

	var cfg map[string]any
	_ = json.Unmarshal([]byte(content), &cfg)
	radiant := cfg["mcp"].(map[string]any)["radiant"].(map[string]any)
	cmd := radiant["command"].([]any)
	if cmd[0] != "/usr/local/bin/radiant" {
		t.Errorf("old path not replaced: %v", cmd[0])
	}
	// Exactly one radiant entry (not duplicated).
	count := 0
	for k := range cfg["mcp"].(map[string]any) {
		if k == "radiant" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 radiant entry, got %d", count)
	}
}

// ── Detection tests ─────────────────────────────────────────────────────────

func TestResolveMCPAgents_DetectsCodex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	agents := resolveMCPAgents("", dir)
	found := false
	for _, a := range agents {
		if a == "codex" {
			found = true
		}
	}
	if !found {
		t.Errorf("codex not detected in %s; got %v", dir, agents)
	}
}

func TestResolveMCPAgents_DetectsOpenCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	agents := resolveMCPAgents("", dir)
	found := false
	for _, a := range agents {
		if a == "opencode" {
			found = true
		}
	}
	if !found {
		t.Errorf("opencode not detected in %s; got %v", dir, agents)
	}
}

func TestResolveMCPAgents_ExplicitFlag(t *testing.T) {
	agents := resolveMCPAgents("codex,opencode", t.TempDir())
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %v", agents)
	}
	if agents[0] != "codex" || agents[1] != "opencode" {
		t.Errorf("expected [codex, opencode], got %v", agents)
	}
}

// ── mcpConfigFor tests ──────────────────────────────────────────────────────

func TestMCPConfigFor_Codex_Project(t *testing.T) {
	dir := t.TempDir()
	target, content, err := mcpConfigFor("codex", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor codex: %v", err)
	}
	if !strings.HasSuffix(target, filepath.Join(".codex", "config.toml")) {
		t.Errorf("target: got %q want suffix .codex/config.toml", target)
	}
	if !strings.Contains(content, "[mcp_servers.radiant]") {
		t.Errorf("content missing [mcp_servers.radiant]:\n%s", content)
	}
}

func TestMCPConfigFor_Codex_Global(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, _, err := mcpConfigFor("codex", "/usr/local/bin/radiant", dir, true)
	if err != nil {
		t.Fatalf("mcpConfigFor codex global: %v", err)
	}
	want := filepath.Join(home, ".codex", "config.toml")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
}

func TestMCPConfigFor_OpenCode_Project(t *testing.T) {
	dir := t.TempDir()
	target, content, err := mcpConfigFor("opencode", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor opencode: %v", err)
	}
	if !strings.HasSuffix(target, filepath.Join(".opencode", "config.json")) {
		t.Errorf("target: got %q want suffix .opencode/config.json", target)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Errorf("content not valid JSON: %v", err)
	}
}

func TestMCPConfigFor_OpenCode_Global(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, _, err := mcpConfigFor("opencode", "/usr/local/bin/radiant", dir, true)
	if err != nil {
		t.Fatalf("mcpConfigFor opencode global: %v", err)
	}
	want := filepath.Join(home, ".config", "opencode", "config.json")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
}

func TestMCPConfigFor_UnknownAgent(t *testing.T) {
	_, _, err := mcpConfigFor("claude-cli-rocks", "/usr/local/bin/radiant", t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error should mention 'unknown agent': %v", err)
	}
}// ── Hermes Agent (NousResearch) tests ──────────────────────────────────────

func TestMergeHermesConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".hermes", "config.yaml")

	content, err := mergeHermesConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeHermesConfig: %v", err)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(content), &out); err != nil {
		t.Fatalf("output not valid YAML: %v\n%s", err, content)
	}
	servers, ok := out["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("missing or wrong-typed 'mcp_servers' key in:\n%s", content)
	}
	radiant, ok := servers["radiant"].(map[string]any)
	if !ok {
		t.Fatalf("missing 'radiant' under mcp_servers in:\n%s", content)
	}
	if radiant["command"] != "/usr/local/bin/radiant" {
		t.Errorf("command: got %v want '/usr/local/bin/radiant'", radiant["command"])
	}
	args, _ := radiant["args"].([]any)
	if len(args) != 2 || args[0] != "mcp" || args[1] != "serve" {
		t.Errorf("args: got %v want [mcp serve]", args)
	}
}

func TestMergeHermesConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".hermes", "config.yaml")
	existing := `model:
  default: "anthropic/claude-opus-4.6"
  provider: "auto"

terminal:
  backend: "local"
  cwd: "."
  timeout: 180

mcp_servers:
  time:
    command: "uvx"
    args: ["mcp-server-time", "--utc"]
`
	writeFile(t, target, existing)

	content, err := mergeHermesConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeHermesConfig: %v", err)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(content), &out); err != nil {
		t.Fatalf("output not valid YAML: %v\n%s", err, content)
	}

	if _, ok := out["model"].(map[string]any); !ok {
		t.Errorf("'model' lost:\n%s", content)
	}
	if _, ok := out["terminal"].(map[string]any); !ok {
		t.Errorf("'terminal' lost:\n%s", content)
	}

	servers := out["mcp_servers"].(map[string]any)
	if _, ok := servers["time"]; !ok {
		t.Errorf("existing 'time' MCP server lost:\n%s", content)
	}

	radiant := servers["radiant"].(map[string]any)
	if radiant["command"] != "/usr/local/bin/radiant" {
		t.Errorf("radiant command wrong: %v", radiant["command"])
	}
}

func TestMergeHermesConfig_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".hermes", "config.yaml")
	existing := `mcp_servers:
  radiant:
    command: "/old/path/radiant"
    args: ["mcp", "serve"]
`
	writeFile(t, target, existing)

	content, err := mergeHermesConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeHermesConfig: %v", err)
	}

	if strings.Contains(content, "/old/path/radiant") {
		t.Errorf("old path not replaced:\n%s", content)
	}

	var out map[string]any
	_ = yaml.Unmarshal([]byte(content), &out)
	servers := out["mcp_servers"].(map[string]any)
	if len(servers) != 1 {
		t.Errorf("expected exactly 1 mcp_server entry, got %d:\n%s", len(servers), content)
	}
}

// ── Kimi CLI tests ───────────────────────────────────────────────────────────

func TestMergeKimiMCP_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".kimi", "mcp.json")

	content, err := mergeKimiMCP(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeKimiMCP: %v", err)
	}

	var f struct {
		Servers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(content), &f); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, content)
	}
	if f.Servers["radiant"].Command != "/usr/local/bin/radiant" {
		t.Errorf("radiant command: got %q want '/usr/local/bin/radiant'", f.Servers["radiant"].Command)
	}
}

func TestMergeKimiMCP_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".kimi", "mcp.json")
	existing := `{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": { "CONTEXT7_API_KEY": "ctx7sk-test" }
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeKimiMCP(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeKimiMCP: %v", err)
	}

	var f struct {
		Servers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(content), &f); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if _, ok := f.Servers["context7"]; !ok {
		t.Errorf("existing 'context7' lost:\n%s", content)
	}
	if _, ok := f.Servers["radiant"]; !ok {
		t.Errorf("radiant not added:\n%s", content)
	}
}

// ── OpenClaw tests ───────────────────────────────────────────────────────────

func TestMergeOpenClawJSONConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".openclaw", "openclaw.json")

	content, err := mergeOpenClawJSONConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenClawJSONConfig: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, content)
	}
	mcp := cfg["mcp"].(map[string]any)
	servers := mcp["servers"].(map[string]any)
	radiant := servers["radiant"].(map[string]any)
	if radiant["command"] != "/usr/local/bin/radiant" {
		t.Errorf("command: got %v want '/usr/local/bin/radiant'", radiant["command"])
	}
}

func TestMergeOpenClawJSONConfig_PreservesSiblings(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".openclaw", "openclaw.json")
	existing := `{
  "mcp": {
    "sessionIdleTtlMs": 600000,
    "servers": {
      "context7": {
        "command": "uvx",
        "args": ["context7-mcp"]
      }
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeOpenClawJSONConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenClawJSONConfig: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	mcp := cfg["mcp"].(map[string]any)
	if mcp["sessionIdleTtlMs"].(float64) != 600000 {
		t.Errorf("sessionIdleTtlMs lost: %v", mcp["sessionIdleTtlMs"])
	}

	servers := mcp["servers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Errorf("existing context7 lost: %v", servers)
	}
	if _, ok := servers["radiant"]; !ok {
		t.Errorf("radiant not added: %v", servers)
	}
}

func TestMergeOpenClawJSONConfig_PreservesTopLevel(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".openclaw", "openclaw.json")
	existing := `{
  "channels": { "telegram": { "token": "..." } },
  "gateway": { "port": 18789 },
  "mcp": {
    "servers": {
      "context7": { "command": "uvx", "args": ["context7-mcp"] }
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeOpenClawJSONConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeOpenClawJSONConfig: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	if _, ok := cfg["channels"]; !ok {
		t.Errorf("'channels' lost: %s", content)
	}
	if _, ok := cfg["gateway"]; !ok {
		t.Errorf("'gateway' lost: %s", content)
	}
	if _, ok := cfg["mcp"]; !ok {
		t.Errorf("'mcp' lost: %s", content)
	}
}

// ── Cline tests ──────────────────────────────────────────────────────────────

func TestMergeClineConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".cline", "mcp.json")

	content, err := mergeClineConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeClineConfig: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, content)
	}
	servers := out["mcpServers"].(map[string]any)
	radiant := servers["radiant"].(map[string]any)
	if radiant["command"] != "/usr/local/bin/radiant" {
		t.Errorf("command: got %v", radiant["command"])
	}
	if radiant["disabled"] != false {
		t.Errorf("disabled: got %v want false", radiant["disabled"])
	}
	aa, ok := radiant["autoApprove"].([]any)
	if !ok || len(aa) != 0 {
		t.Errorf("autoApprove: got %v want []", radiant["autoApprove"])
	}
}

func TestMergeClineConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".cline", "mcp.json")
	existing := `{
  "mcpServers": {
    "local-server": {
      "command": "node",
      "args": ["/path/to/server.js"],
      "env": { "API_KEY": "your_api_key" },
      "disabled": false,
      "autoApprove": []
    }
  }
}
`
	writeFile(t, target, existing)

	content, err := mergeClineConfig(target, mcpEntry{
		Command: "/usr/local/bin/radiant",
		Args:    []string{"mcp", "serve"},
	})
	if err != nil {
		t.Fatalf("mergeClineConfig: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	servers := out["mcpServers"].(map[string]any)
	if _, ok := servers["local-server"]; !ok {
		t.Errorf("existing local-server lost: %s", content)
	}
	if _, ok := servers["radiant"]; !ok {
		t.Errorf("radiant not added: %s", content)
	}
}

// ── Detection tests (Sprint 75 additions) ───────────────────────────────────

func TestResolveMCPAgents_DetectsHermes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".hermes"), 0o755); err != nil {
		t.Fatal(err)
	}
	agents := resolveMCPAgents("", dir)
	found := false
	for _, a := range agents {
		if a == "hermes" {
			found = true
		}
	}
	if !found {
		t.Errorf("hermes not detected in %s; got %v", dir, agents)
	}
}

func TestResolveMCPAgents_DetectsOpenClaw(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".openclaw"), 0o755); err != nil {
		t.Fatal(err)
	}
	agents := resolveMCPAgents("", dir)
	found := false
	for _, a := range agents {
		if a == "openclaw" {
			found = true
		}
	}
	if !found {
		t.Errorf("openclaw not detected in %s; got %v", dir, agents)
	}
}

// ── mcpConfigFor tests (Sprint 75 additions) ────────────────────────────────

func TestMCPConfigFor_Hermes_Project(t *testing.T) {
	dir := t.TempDir()
	target, content, err := mcpConfigFor("hermes", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor hermes: %v", err)
	}
	if !strings.HasSuffix(target, filepath.Join(".hermes", "config.yaml")) {
		t.Errorf("target: got %q want suffix .hermes/config.yaml", target)
	}
	if !strings.Contains(content, "mcp_servers:") {
		t.Errorf("missing 'mcp_servers:' in:\n%s", content)
	}
	// yaml.v3 emits bare paths unquoted when they don't need escaping,
	// and quoted strings when they do. Accept either quoting style
	// for the command line as long as the path is on a `command:` line.
	if !strings.Contains(content, "/usr/local/bin/radiant") {
		t.Errorf("missing command path in:\n%s", content)
	}
	if !strings.Contains(content, "command:") {
		t.Errorf("missing 'command:' key in:\n%s", content)
	}
}

func TestMCPConfigFor_Hermes_Global(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, _, err := mcpConfigFor("hermes", "/usr/local/bin/radiant", dir, true)
	if err != nil {
		t.Fatalf("mcpConfigFor hermes global: %v", err)
	}
	want := filepath.Join(home, ".hermes", "config.yaml")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
}

func TestMCPConfigFor_Kimi(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, content, err := mcpConfigFor("kimi", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor kimi: %v", err)
	}
	want := filepath.Join(home, ".kimi", "mcp.json")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
	if !strings.Contains(content, `"radiant"`) {
		t.Errorf("missing radiant entry in:\n%s", content)
	}
}

func TestMCPConfigFor_OpenClaw_Project(t *testing.T) {
	dir := t.TempDir()
	target, content, err := mcpConfigFor("openclaw", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor openclaw: %v", err)
	}
	if !strings.HasSuffix(target, filepath.Join(".openclaw", "openclaw.json")) {
		t.Errorf("target: got %q want suffix .openclaw/openclaw.json", target)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Errorf("content not valid JSON: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Errorf("missing 'mcp' object:\n%s", content)
	} else {
		servers, ok := mcp["servers"].(map[string]any)
		if !ok {
			t.Errorf("missing 'mcp.servers':\n%s", content)
		} else if _, ok := servers["radiant"]; !ok {
			t.Errorf("missing mcp.servers.radiant:\n%s", content)
		}
	}
}

func TestMCPConfigFor_OpenClaw_Global(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, _, err := mcpConfigFor("openclaw", "/usr/local/bin/radiant", dir, true)
	if err != nil {
		t.Fatalf("mcpConfigFor openclaw global: %v", err)
	}
	want := filepath.Join(home, ".openclaw", "openclaw.json")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
}

func TestMCPConfigFor_Cline(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	target, content, err := mcpConfigFor("cline", "/usr/local/bin/radiant", dir, false)
	if err != nil {
		t.Fatalf("mcpConfigFor cline: %v", err)
	}
	want := filepath.Join(home, ".cline", "mcp.json")
	if target != want {
		t.Errorf("target: got %q want %q", target, want)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Errorf("content not valid JSON: %v", err)
	}
	servers := cfg["mcpServers"].(map[string]any)
	radiant := servers["radiant"].(map[string]any)
	if _, ok := radiant["disabled"]; !ok {
		t.Errorf("missing 'disabled' field in cline entry: %s", content)
	}
	if _, ok := radiant["autoApprove"]; !ok {
		t.Errorf("missing 'autoApprove' field in cline entry: %s", content)
	}
}
