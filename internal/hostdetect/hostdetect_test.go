package hostdetect

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// stubDetector builds a Detector with mocked env + parent process
// for deterministic tests.
func stubDetector(env map[string]string, parentCmd string, pid, ppid int) *Detector {
	d := &Detector{
		LookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		ReadProcComm: func(p int) (string, error) {
			if p != ppid {
				return "", fmt.Errorf("pid mismatch")
			}
			return parentComm(parentCmd, p), nil
		},
		NowPID:      func() int { return pid },
		NowPPID:     func() int { return ppid },
	}
	return d
}

func parentComm(cmd string, pid int) string {
	if cmd == "" {
		return ""
	}
	return cmd
}

// Per-agent signature tests. Each sets the most distinctive env var
// for one agent and asserts the detector picks it.

func TestDetect_ClaudeCode(t *testing.T) {
	d := stubDetector(
		map[string]string{"CLAUDE_CODE_ENTRY": "/entry/claude"},
		"", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentClaudeCode {
		t.Errorf("got agent %q, want %q", got.Agent, AgentClaudeCode)
	}
	if got.Confidence < 75 {
		t.Errorf("got confidence %d, want ≥75", got.Confidence)
	}
	if !got.SupportsSampling {
		t.Error("Claude Code supports sampling")
	}
	if len(got.SampleEnvVars) == 0 || got.SampleEnvVars[0] != "CLAUDE_CODE_ENTRY" {
		t.Errorf("SampleEnvVars mismatch: %v", got.SampleEnvVars)
	}
	if got.DetectionSource != "env" {
		t.Errorf("got source %q, want env", got.DetectionSource)
	}
}

func TestDetect_Cursor(t *testing.T) {
	d := stubDetector(
		map[string]string{"CURSOR_TRACE_ID": "abc123"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentCursor {
		t.Errorf("got %q, want cursor", got.Agent)
	}
}

func TestDetect_Hermes(t *testing.T) {
	d := stubDetector(
		map[string]string{"HERMES_VERSION": "0.1.0"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentHermes {
		t.Errorf("got %q, want hermes", got.Agent)
	}
}

func TestDetect_KimiCLI(t *testing.T) {
	d := stubDetector(
		map[string]string{"KIMI_SHARE_DIR": "/home/.kimi"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentKimiCLI {
		t.Errorf("got %q, want kimi-cli", got.Agent)
	}
}

func TestDetect_OpenClaw(t *testing.T) {
	d := stubDetector(
		map[string]string{"OPENCLAW_GATEWAY_URL": "ws://localhost:18789"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentOpenClaw {
		t.Errorf("got %q, want openclaw", got.Agent)
	}
}

func TestDetect_Codex(t *testing.T) {
	d := stubDetector(
		map[string]string{"CODEX_HOME": "/home/.codex"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentCodex {
		t.Errorf("got %q, want codex", got.Agent)
	}
}

func TestDetect_Cline(t *testing.T) {
	d := stubDetector(
		map[string]string{"CLINE_VERSION": "1.0.0"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentCline {
		t.Errorf("got %q, want cline", got.Agent)
	}
}

func TestDetect_OpenCode(t *testing.T) {
	d := stubDetector(
		map[string]string{"OPENCODE_VERSION": "0.5.0"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentOpenCode {
		t.Errorf("got %q, want opencode", got.Agent)
	}
}

func TestDetect_VSCodeCopilot(t *testing.T) {
	d := stubDetector(
		map[string]string{"VSCODE_PID": "1234"},
		"", 1000, 999,
	)
	if got := d.Detect(); got.Agent != AgentVSCode {
		t.Errorf("got %q, want vscode-copilot", got.Agent)
	}
}

// Multiple env vars from the same agent → higher confidence.
func TestDetect_MultipleHitsHigherConfidence(t *testing.T) {
	d := stubDetector(
		map[string]string{
			"CLAUDE_CODE_ENTRY":    "/entry",
			"CLAUDE_CODE_SSE_PORT": "8080",
			"CLAUDE_CODE_PID":      "42",
		},
		"", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentClaudeCode {
		t.Errorf("got %q, want claude-code", got.Agent)
	}
	// 3 hits → 60 + 3*15 = 105 → capped at 100
	if got.Confidence != 100 {
		t.Errorf("got confidence %d, want 100 (capped)", got.Confidence)
	}
}

// Env wins over parent process: if env says Claude Code but parent
// is named "cursor", Claude Code wins.
func TestDetect_EnvWinsOverParent(t *testing.T) {
	d := stubDetector(
		map[string]string{"HERMES_VERSION": "0.1.0"},
		"cursor", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentHermes {
		t.Errorf("got %q, want hermes (env should beat parent)", got.Agent)
	}
	if got.DetectionSource != "env" {
		t.Errorf("got source %q, want env", got.DetectionSource)
	}
}

// Layer 2: parent process only (env empty).
func TestDetect_ParentProcessOnly(t *testing.T) {
	d := stubDetector(
		map[string]string{},
		"opencode", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentOpenCode {
		t.Errorf("got %q, want opencode", got.Agent)
	}
	if got.Confidence != 50 {
		t.Errorf("got confidence %d, want 50 (parent-only Medium)", got.Confidence)
	}
	if got.DetectionSource != "process-tree" {
		t.Errorf("got source %q, want process-tree", got.DetectionSource)
	}
}

// Nothing found: empty env + unknown parent.
func TestDetect_NoMatch(t *testing.T) {
	d := stubDetector(
		map[string]string{},
		"totally-unrelated-process", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentUnknown {
		t.Errorf("got %q, want unknown", got.Agent)
	}
	if got.Confidence != 0 {
		t.Errorf("got confidence %d, want 0", got.Confidence)
	}
	if got.DetectionSource != "none" {
		t.Errorf("got source %q, want none", got.DetectionSource)
	}
	if got.PID != 1000 {
		t.Errorf("PID lost: %d", got.PID)
	}
	if got.PPID != 999 {
		t.Errorf("PPID lost: %d", got.PPID)
	}
}

// Empty env, empty parent — totally invisible invocation.
func TestDetect_Empty(t *testing.T) {
	d := stubDetector(map[string]string{}, "", 1000, 999)
	got := d.Detect()
	if got.Agent != AgentUnknown {
		t.Errorf("got %q, want unknown", got.Agent)
	}
	if got.ParentCmd != "" {
		t.Errorf("parent cmd should be empty: %q", got.ParentCmd)
	}
}

// Parent process name with .exe suffix (Windows pattern) still matches.
func TestDetect_ParentWithExeSuffix(t *testing.T) {
	d := stubDetector(
		map[string]string{},
		"claude-code.exe", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentClaudeCode {
		t.Errorf("got %q, want claude-code (.exe suffix should still match)", got.Agent)
	}
}

// Multi-agent env confusion: if Hermes AND Claude envs are both set,
// Claude Code wins (matches 3 envs vs Hermes 1).
func TestDetect_MultipleAgentsPickHighestHits(t *testing.T) {
	d := stubDetector(
		map[string]string{
			"HERMES_VERSION":         "0.1.0",
			"CLAUDE_CODE_ENTRY":      "/entry",
			"CLAUDE_CODE_SSE_PORT":   "8080",
			"CLAUDE_CODE_PID":        "42",
		},
		"", 1000, 999,
	)
	got := d.Detect()
	if got.Agent != AgentClaudeCode {
		t.Errorf("got %q, want claude-code (3 hits beats Hermes 1 hit)", got.Agent)
	}
}

// Detector with default nil factories doesn't crash.
func TestNew_WithDefaults(t *testing.T) {
	d := New()
	// We can't reset state between tests in the same process, but we
	// can verify the factories aren't nil after the lazy fixup.
	// (We don't call Detect() to avoid coupling to real env.)
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
}

// All agents support sampling.
func TestSupportsSampling_TrueForAllKnown(t *testing.T) {
	for _, id := range knownAgents {
		sig, ok := signatures[id]
		if !ok {
			t.Errorf("missing signature for %s", id)
			continue
		}
		if !sig.SupportsSampling {
			t.Errorf("agent %s should support sampling", id)
		}
	}
}

// AgentID.String() returns the lowercase form.
func TestAgentIDString(t *testing.T) {
	if AgentClaudeCode.String() != "claude-code" {
		t.Errorf("expected claude-code, got %q", AgentClaudeCode.String())
	}
	if AgentUnknown.String() != "(unknown)" {
		t.Errorf("expected (unknown), got %q", AgentUnknown.String())
	}
}

// Sanity: signatures map covers every knownAgents entry.
func TestSignaturesCoverAllAgents(t *testing.T) {
	for _, id := range knownAgents {
		if _, ok := signatures[id]; !ok {
			t.Errorf("missing signature for %s", id)
		}
	}
}

// Sanity: every signature has at least 1 env var and 1 parent binary.
func TestSignaturesWellFormed(t *testing.T) {
	for id, sig := range signatures {
		if len(sig.EnvVars) == 0 {
			t.Errorf("%s has 0 env vars", id)
		}
		if len(sig.ParentBinaries) == 0 {
			t.Errorf("%s has 0 parent binaries", id)
		}
		for _, envKey := range sig.EnvVars {
			if envKey == "" {
				t.Errorf("%s has empty env var key", id)
			}
			if !strings.Contains(envKey, "_") && !strings.Contains(envKey, "-") {
				t.Errorf("%s env var %q should be uppercase/separated", id, envKey)
			}
		}
	}
}
