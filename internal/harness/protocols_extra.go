package harness

import "fmt"

// ── Aider Protocol ──

type AiderProtocol struct{}

func (p *AiderProtocol) Name() string            { return "aider" }
func (p *AiderProtocol) DefaultCommand() string  { return "aider" }

func (p *AiderProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"--message", prompt}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	return append(args, cfg.Args...)
}

func (p *AiderProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("aider command not specified")
	}
	return nil
}

// ── Continue Protocol ──

type ContinueProtocol struct{}

func (p *ContinueProtocol) Name() string            { return "continue" }
func (p *ContinueProtocol) DefaultCommand() string  { return "continue" }

func (p *ContinueProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"--prompt", prompt}
	return append(args, cfg.Args...)
}

func (p *ContinueProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("continue command not specified")
	}
	return nil
}

// ── Cline Protocol ──

type ClineProtocol struct{}

func (p *ClineProtocol) Name() string            { return "cline" }
func (p *ClineProtocol) DefaultCommand() string  { return "cline" }

func (p *ClineProtocol) BuildArgs(prompt string, cfg AgentConfig) []string {
	args := []string{"--prompt", prompt}
	return append(args, cfg.Args...)
}

func (p *ClineProtocol) ValidateConfig(cfg AgentConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("cline command not specified")
	}
	return nil
}

// ── Aider AgentID ──
const AgentAider AgentID = "aider"

func init() {
	// Register extra protocols
	RegisterProtocol("aider", &AiderProtocol{})
	RegisterProtocol("continue", &ContinueProtocol{})
	RegisterProtocol("cline", &ClineProtocol{})
}
