package mode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"light", Light, false},
		{"LIGHT", Light, false},
		{"lite", Light, false},
		{"full", Full, false},
		{"FULL", Full, false},
		{"autonomous", Full, false},
		{"auto", Auto, false},
		{"", Auto, false},
		{"  light  ", Light, false},
		{"weird", Auto, true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("Parse(%q): err=%v want_err=%v", c.in, err, c.wantErr)
		}
		if err == nil && got != c.want {
			t.Errorf("Parse(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestNeedsAPIKey(t *testing.T) {
	if Light.NeedsAPIKey() {
		t.Errorf("Light should not need API key")
	}
	if !Full.NeedsAPIKey() {
		t.Errorf("Full should need API key")
	}
	if Auto.NeedsAPIKey() {
		t.Errorf("Auto should not need API key (it's the default)")
	}
}

func TestHasLLMAPIKey(t *testing.T) {
	// Clear all known env vars before testing.
	for _, k := range []string{"OPENROUTER_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	if hasLLMAPIKey() {
		t.Errorf("expected false when no keys set")
	}
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	if !hasLLMAPIKey() {
		t.Errorf("expected true when OPENROUTER_API_KEY set")
	}
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	if !hasLLMAPIKey() {
		t.Errorf("expected true when ANTHROPIC_API_KEY set")
	}
}

func TestDetect_ProjectMCPConfig(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{"mcpServers": {"radiant": {"command": "/x", "args": ["mcp-serve"]}}}`
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Detect(dir); got != Light {
		t.Errorf("Detect: got %q want Light (project MCP config found)", got)
	}
}

func TestDetect_CursorConfig(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cur, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{"mcpServers": {"radiant": {"command": "/x", "args": ["mcp-serve"]}}}`
	if err := os.WriteFile(filepath.Join(cur, "mcp.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Detect(dir); got != Light {
		t.Errorf("Detect: got %q want Light (Cursor MCP config found)", got)
	}
}

func TestDetect_APIKeyFallback(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"OPENROUTER_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	if got := Detect(dir); got != Full {
		t.Errorf("Detect: got %q want Full (API key set, no MCP)", got)
	}
}

func TestDetect_DefaultsToLight(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"OPENROUTER_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	if got := Detect(dir); got != Light {
		t.Errorf("Detect: got %q want Light (no key, no MCP — safe default)", got)
	}
}

func TestResolve_FlagTakesPriority(t *testing.T) {
	t.Setenv("RADIANT_MODE", "full")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	r := Resolve("light", "/tmp", "full")
	if r.Mode != Light {
		t.Errorf("flag should win: got %q", r.Mode)
	}
	if r.Source != SourceFlag {
		t.Errorf("source: got %q want %q", r.Source, SourceFlag)
	}
}

func TestResolve_EnvOverridesConfig(t *testing.T) {
	t.Setenv("RADIANT_MODE", "light")
	r := Resolve("", "/tmp", "full")
	if r.Mode != Light {
		t.Errorf("env should win over config: got %q", r.Mode)
	}
	if r.Source != SourceEnv {
		t.Errorf("source: got %q want %q", r.Source, SourceEnv)
	}
}

func TestResolve_ConfigOverridesDetect(t *testing.T) {
	t.Setenv("RADIANT_MODE", "")
	// No MCP config, no API key — Detect would say Light.
	// Config says Full.
	r := Resolve("", "/tmp", "full")
	if r.Mode != Full {
		t.Errorf("config should win over detect: got %q", r.Mode)
	}
	if r.Source != SourceConfig {
		t.Errorf("source: got %q want %q", r.Source, SourceConfig)
	}
}

func TestResolution_String(t *testing.T) {
	r := Resolution{Mode: Light, Source: SourceFlag, Reason: "test"}
	s := r.String()
	if !strings.Contains(s, "light") || !strings.Contains(s, "flag") {
		t.Errorf("String() should include mode and source: got %q", s)
	}
}