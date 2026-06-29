// Package mode defines and resolves the operational mode of radiant-harness.
//
// The harness operates in one of two modes:
//
//   - Light — "harness possesses the agent". The harness runs as an MCP
//     server; when it needs an LLM call, it emits sampling/createMessage
//     to the host agent (Claude Code, Hermes, Cursor, ...), which performs
//     inference with its own credentials. No API key required.
//
//   - Full — "harness is autonomous". The harness calls LLM HTTP endpoints
//     directly (OpenRouter, OpenAI, Anthropic, ...). Requires an API key
//     in the environment or .radiant.yaml.
//
// Resolution order (highest priority first):
//  1. CLI flag --mode=light|full
//  2. Environment variable RADIANT_MODE
//  3. Project config (.radiant.yaml → mode:)
//  4. Auto-detection (presence of MCP config → Light, else Full)
package mode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode is the operational mode of the harness.
type Mode string

const (
	// Light — harness possesses the agent via MCP sampling.
	Light Mode = "light"

	// Full — harness is autonomous via direct HTTP to LLM providers.
	Full Mode = "full"

	// Auto — auto-detect at resolution time.
	Auto Mode = "auto"
)

// String returns the lowercase name of the mode.
func (m Mode) String() string {
	return string(m)
}

// Description returns a one-sentence human description.
func (m Mode) Description() string {
	switch m {
	case Light:
		return "harness possesses the agent via MCP sampling (no API key)"
	case Full:
		return "harness is autonomous via direct HTTP to LLM providers (API key required)"
	case Auto:
		return "auto-detect at resolution time"
	default:
		return "unknown mode"
	}
}

// NeedsAPIKey reports whether the mode requires an LLM API key.
func (m Mode) NeedsAPIKey() bool {
	return m == Full
}

// Parse converts a string to a Mode. Accepts "light", "full", "auto"
// (case-insensitive, also accepts "lite" as alias for "light").
func Parse(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "light", "lite":
		return Light, nil
	case "full", "autonomous":
		return Full, nil
	case "auto", "":
		return Auto, nil
	default:
		return Auto, fmt.Errorf("unknown mode %q (expected light|full|auto)", s)
	}
}

// Detect inspects the environment and returns the most likely mode.
//
// Heuristics:
//   - CLAUDE_CONFIG_DIR set OR ~/.claude/settings.json has radiant MCP entry → Light
//   - OPENROUTER_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY in env → Full
//   - Otherwise → Light (default to agent-possessed for safety)
//
// projectDir is the project being worked on; used to check for project-local
// MCP config that would indicate the user is already inside an agent session.
func Detect(projectDir string) Mode {
	if hasMCPConfig(projectDir) {
		return Light
	}
	if hasLLMAPIKey() {
		return Full
	}
	// Default: assume Light. This biases toward "user is in an agent".
	// It will fail clearly with a useful error if sampling is not available.
	return Light
}

// hasMCPConfig reports whether the project (or user-level) has the radiant
// MCP server registered. If yes, the user is in an agent that supports sampling.
func hasMCPConfig(projectDir string) bool {
	// Check project-level .claude/settings.json (Claude Code project config).
	if projectDir != "" {
		settings := filepath.Join(projectDir, ".claude", "settings.json")
		if fileContainsRadiant(settings) {
			return true
		}
	}
	// Check user-level ~/.claude/settings.json.
	if home, err := os.UserHomeDir(); err == nil {
		settings := filepath.Join(home, ".claude", "settings.json")
		if fileContainsRadiant(settings) {
			return true
		}
	}
	// Check Cursor / Windsurf / Zed / VSCode project-level configs.
	if projectDir != "" {
		for _, sub := range []string{".cursor/mcp.json", ".windsurf/mcp.json", ".zed/settings.json", ".vscode/mcp.json"} {
			p := filepath.Join(projectDir, sub)
			if fileContainsRadiant(p) {
				return true
			}
		}
	}
	return false
}

// fileContainsRadiant reads a JSON-ish config file and returns true if it
// mentions a "radiant" key in any obvious MCP-server map shape. Best-effort:
// we don't fully parse every host's format, just grep the bytes.
func fileContainsRadiant(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Look for "radiant" as a JSON key (typical for mcpServers / context_servers maps).
	// We don't try to be clever — false positives are fine (Light is a safe default),
	// false negatives just mean the user has to pass --mode=light explicitly.
	return strings.Contains(string(data), `"radiant"`)
}

// hasLLMAPIKey reports whether any known LLM API key is set in the environment.
func hasLLMAPIKey() bool {
	for _, env := range []string{
		"OPENROUTER_API_KEY",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GROQ_API_KEY",
		"MISTRAL_API_KEY",
		"XAI_API_KEY",
	} {
		if strings.TrimSpace(os.Getenv(env)) != "" {
			return true
		}
	}
	return false
}

// Source describes where a resolved mode came from. Useful for diagnostics.
type Source string

const (
	SourceFlag    Source = "flag"     // explicit --mode flag
	SourceEnv     Source = "env"      // RADIANT_MODE env var
	SourceConfig  Source = "config"   // .radiant.yaml → mode:
	SourceDetect  Source = "detected" // auto-detect
	SourceDefault Source = "default"  // fell back to Light as safe default
)

// Resolution is the result of Resolve() — a mode plus where it came from.
type Resolution struct {
	Mode   Mode
	Source Source
	Reason string
}

// String renders a human-readable summary.
func (r Resolution) String() string {
	return fmt.Sprintf("%s (from %s: %s)", r.Mode, r.Source, r.Reason)
}

// Resolve picks the mode to use, given the resolution chain:
//
//  1. flag (highest priority, set explicitly by caller)
//  2. RADIANT_MODE env var
//  3. projectConfig.Mode (if non-empty)
//  4. auto-detect via Detect()
//
// Empty string arguments are treated as "not set".
func Resolve(flag, projectDir string, projectConfigMode string) Resolution {
	if m, err := Parse(flag); err == nil && m != Auto {
		return Resolution{Mode: m, Source: SourceFlag, Reason: "explicit --mode flag"}
	}
	if env := strings.TrimSpace(os.Getenv("RADIANT_MODE")); env != "" {
		if m, err := Parse(env); err == nil && m != Auto {
			return Resolution{Mode: m, Source: SourceEnv, Reason: "RADIANT_MODE=" + env}
		}
	}
	if projectConfigMode != "" {
		if m, err := Parse(projectConfigMode); err == nil && m != Auto {
			return Resolution{Mode: m, Source: SourceConfig, Reason: ".radiant.yaml mode: " + projectConfigMode}
		}
	}
	detected := Detect(projectDir)
	reason := "no API key found, defaulting to Light"
	if hasMCPConfig(projectDir) {
		reason = "MCP config detected — assume agent session"
	} else if hasLLMAPIKey() {
		reason = "LLM API key found in environment"
	}
	return Resolution{Mode: detected, Source: SourceDetect, Reason: reason}
}