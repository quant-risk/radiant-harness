package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
}