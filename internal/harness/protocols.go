package harness

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AgentProtocol defines how to communicate with a specific AI agent.
type AgentProtocol interface {
	// Name returns the agent name.
	Name() string
	// BuildArgs constructs command arguments for the agent.
	BuildArgs(prompt string, cfg AgentConfig) []string
	// ValidateConfig checks if the agent is properly configured.
	ValidateConfig(cfg AgentConfig) error
	// DefaultCommand returns the default command name.
	DefaultCommand() string
}

// ── Claude Protocol ──

type ClaudeProtocol struct{}

func (p *ClaudeProtocol) Name() string { return "claude" }

func (p *ClaudeProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"-p", prompt}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	return append(args, cfg.Args...)
}

func (p *ClaudeProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("claude command not specified")
	}
	return nil
}

func (p *ClaudeProtocol) DefaultCommand() string { return "claude" }

// ── Cursor Protocol ──

type CursorProtocol struct{}

func (p *CursorProtocol) Name() string { return "cursor" }

func (p *CursorProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"-p", prompt}
	return append(args, cfg.Args...)
}

func (p *CursorProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("cursor command not specified")
	}
	return nil
}

func (p *CursorProtocol) DefaultCommand() string { return "cursor" }

// ── Codex Protocol ──

type CodexProtocol struct{}

func (p *CodexProtocol) Name() string { return "codex" }

func (p *CodexProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"-p", prompt}
	return append(args, cfg.Args...)
}

func (p *CodexProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("codex command not specified")
	}
	return nil
}

func (p *CodexProtocol) DefaultCommand() string { return "codex" }

// ── Copilot Protocol ──

type CopilotProtocol struct{}

func (p *CopilotProtocol) Name() string { return "copilot" }

func (p *CopilotProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	// GitHub Copilot CLI uses --prompt flag
	args := []string{"--prompt", prompt}
	return append(args, cfg.Args...)
}

func (p *CopilotProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("copilot command not specified")
	}
	return nil
}

func (p *CopilotProtocol) DefaultCommand() string { return "gh" }

// ── Gemini Protocol ──

type GeminiProtocol struct{}

func (p *GeminiProtocol) Name() string { return "gemini" }

func (p *GeminiProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	// Gemini CLI uses --prompt flag
	args := []string{"--prompt", prompt}
	return append(args, cfg.Args...)
}

func (p *GeminiProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("gemini command not specified")
	}
	return nil
}

func (p *GeminiProtocol) DefaultCommand() string { return "gemini" }

// ── Windsurf Protocol ──

type WindsurfProtocol struct{}

func (p *WindsurfProtocol) Name() string { return "windsurf" }

func (p *WindsurfProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"-p", prompt}
	return append(args, cfg.Args...)
}

func (p *WindsurfProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("windsurf command not specified")
	}
	return nil
}

func (p *WindsurfProtocol) DefaultCommand() string { return "windsurf" }

// ── Generic Protocol (for any agent with --prompt flag) ──

type GenericProtocol struct {
	name    string
	command string
}

func (p *GenericProtocol) Name() string { return p.name }

func (p *GenericProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"--prompt", prompt}
	return append(args, cfg.Args...)
}

func (p *GenericProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("%s command not specified", p.name)
	}
	return nil
}

func (p *GenericProtocol) DefaultCommand() string { return p.command }

// ── Protocol Registry ──

var protocols = map[AgentID]AgentProtocol{
	AgentClaude: &ClaudeProtocol{},
	AgentCursor: &CursorProtocol{},
	AgentCodex:  &CodexProtocol{},
	"copilot":   &CopilotProtocol{},
	"gemini":    &GeminiProtocol{},
	"windsurf":  &WindsurfProtocol{},
}

// GetProtocol returns the protocol for an agent ID.
func GetProtocol(id AgentID) AgentProtocol {
	if p, ok := protocols[id]; ok {
		return p
	}
	return &GenericProtocol{name: string(id), command: string(id)}
}

// RegisterProtocol registers a custom protocol.
func RegisterProtocol(id AgentID, protocol AgentProtocol) {
	protocols[id] = protocol
}

// ListProtocols returns all registered protocol names.
func ListProtocols() []string {
	var names []string
	for id := range protocols {
		names = append(names, string(id))
	}
	return names
}

// DetectAgentWithProtocol finds the best available agent using protocol registry.
func DetectAgentWithProtocol() (AgentID, string, AgentProtocol) {
	for id, protocol := range protocols {
		cmd := protocol.DefaultCommand()
		if _, err := exec.LookPath(cmd); err == nil {
			return id, cmd, protocol
		}
	}
	return "", "", nil
}

// ── Multi-Agent Runner ──

// MultiAgentRunner can run tasks across different agents.
type MultiAgentRunner struct {
	agents map[AgentID]*AgentRunner
}

// NewMultiAgentRunner creates a runner that can dispatch to multiple agents.
func NewMultiAgentRunner() *MultiAgentRunner {
	return &MultiAgentRunner{agents: make(map[AgentID]*AgentRunner)}
}

// Register registers an agent runner.
func (m *MultiAgentRunner) Register(id AgentID, runner *AgentRunner) {
	m.agents[id] = runner
}

// Run dispatches a task to the specified agent.
func (m *MultiAgentRunner) Run(ctx context.Context, agentID AgentID, prompt string) (*AgentResult, error) {
	runner, ok := m.agents[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %s not registered", agentID)
	}
	return runner.Run(ctx, prompt)
}

// RunBest picks the best available agent and runs.
func (m *MultiAgentRunner) RunBest(ctx context.Context, prompt string) (*AgentResult, error) {
	// Try agents in priority order
	priority := []AgentID{AgentClaude, AgentCursor, AgentCodex}
	for _, id := range priority {
		if runner, ok := m.agents[id]; ok {
			return runner.Run(ctx, prompt)
		}
	}
	return nil, fmt.Errorf("no agents available")
}

// AvailableAgents returns which agents are currently available.
func (m *MultiAgentRunner) AvailableAgents() []AgentID {
	var ids []AgentID
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids
}

// ── Agent Capability Detection ──

// AgentCapability represents what an agent can do.
type AgentCapability struct {
	SupportsStreaming bool
	SupportsModel     bool
	SupportsTools     bool
	SupportsMCP       bool
	MaxContextTokens  int
}

// DetectCapabilities detects what an agent supports.
func DetectCapabilities(id AgentID) AgentCapability {
	switch id {
	case AgentClaude:
		return AgentCapability{
			SupportsStreaming: true,
			SupportsModel:     true,
			SupportsTools:     true,
			SupportsMCP:       true,
			MaxContextTokens:  200000,
		}
	case AgentCursor:
		return AgentCapability{
			SupportsStreaming: true,
			SupportsModel:     true,
			SupportsTools:     true,
			SupportsMCP:       false,
			MaxContextTokens:  128000,
		}
	case AgentCodex:
		return AgentCapability{
			SupportsStreaming: false,
			SupportsModel:     true,
			SupportsTools:     true,
			SupportsMCP:       false,
			MaxContextTokens:  128000,
		}
	default:
		return AgentCapability{
			SupportsStreaming: false,
			SupportsModel:     false,
			SupportsTools:     false,
			SupportsMCP:       false,
			MaxContextTokens:  32000,
		}
	}
}

// String returns a human-readable capability summary.
func (c AgentCapability) String() string {
	var parts []string
	if c.SupportsStreaming {
		parts = append(parts, "streaming")
	}
	if c.SupportsModel {
		parts = append(parts, "model-override")
	}
	if c.SupportsTools {
		parts = append(parts, "tools")
	}
	if c.SupportsMCP {
		parts = append(parts, "mcp")
	}
	parts = append(parts, fmt.Sprintf("ctx=%d", c.MaxContextTokens))
	return strings.Join(parts, ", ")
}
