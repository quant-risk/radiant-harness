// Package mode defines the operational mode of radiant-harness.
//
// In v2.42.0 the runtime no longer carries an explicit "mode" — the
// behaviour emerges from which subcommand the operator invokes:
//
//   - `radiant mcp-serve` is always Light: the harness runs as an
//     MCP server, sampling from the host agent for inference.
//     No API key required.
//
//   - Any other subcommand (`loop`, `run`, `fleet`, `init`, ...)
//     is always Full: the harness calls LLM HTTP endpoints
//     directly. API key required (env or .radiant.yaml).
//
// This package now just defines the Light/Full constants so other
// code can talk about the dichotomy without hardcoding strings.
// Resolution and detection live in the CLI layer where the
// invocation context is known.
package mode

// Mode is the operational mode of the harness. Used as a constant
// identifier in trace metadata and verifier prompts; never read
// from user input (no flag, no env var, no config field).
type Mode string

const (
	// Light — harness possesses the agent via MCP sampling. The host
	// agent (Claude Code, Hermes, Cursor, ...) performs LLM inference
	// with its own credentials. No API key required.
	Light Mode = "light"

	// Full — harness is autonomous, calls LLM HTTP endpoints directly
	// (OpenRouter, OpenAI, Anthropic, Groq, Mistral, xAI, ...).
	// Requires an API key in the environment or .radiant.yaml.
	Full Mode = "full"
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
	default:
		return "unknown mode"
	}
}

// IsValid reports whether m is one of the known modes.
func (m Mode) IsValid() bool {
	switch m {
	case Light, Full:
		return true
	}
	return false
}