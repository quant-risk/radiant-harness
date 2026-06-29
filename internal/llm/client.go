package llm

// Light-only shim: Client wraps SamplingBackend so that the Full-era
// engine, loop, run, and fleet packages compile in the Light build
// without any HTTP LLM client code.
//
// The Light build never reaches an HTTP provider. Every Chat() call goes
// through MCP sampling/createMessage to the host agent. If no host agent
// is connected, Chat() fails with a clear error instructing the user to
// wire up an agent via `radiant setup-mcp`.

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// Client is the Light build's stand-in for the Full's HTTP-backed Client.
// It carries the configured model (used only as a modelPreferences hint
// to the host agent) and delegates Chat/ChatStream to a SamplingBackend.
type Client struct {
	model    Model
	backend  *SamplingBackend
	maxRetry int
	writeMu  *sync.Mutex
}

// NewClient creates a Client that delegates to the MCP sampling backend.
// The host agent is whatever MCP client wired radiant in via setup-mcp
// (Claude Code, Cursor, Hermes, …). The model's Model field is forwarded
// as a modelPreferences hint; the host may ignore it per MCP spec §6.5.
func NewClient(m Model) *Client {
	return &Client{
		model:    m,
		maxRetry: MaxRetries,
	}
}

// SetWriter wires the SamplingBackend's output writer. Must be called
// before the first Chat() if the Client is used outside an MCP server
// process (e.g. from `radiant run`).
func (c *Client) SetWriter(w io.Writer) {
	c.backend = NewSamplingBackend(SamplingOptions{
		ModelHint: c.model.Model,
		MaxTokens: pickMaxTokens(c.model.MaxTokens),
		Out:       w,
	})
	if c.writeMu != nil {
		c.backend.SetWriteMu(c.writeMu)
	}
}

// SetWriteMu stores a mutex shared with the caller's JSON-RPC writer.
// Sampled requests and regular JSON-RPC responses then serialize cleanly.
func (c *Client) SetWriteMu(mu *sync.Mutex) {
	c.writeMu = mu
	if c.backend != nil {
		c.backend.SetWriteMu(mu)
	}
}

// Model returns the Model the Client was constructed with.
func (c *Client) Model() Model { return c.model }

// ModelID returns the model identifier string for the Backend interface.
// Same value used by the underlying SamplingBackend.
func (c *Client) ModelID() string {
	if c.backend != nil {
		return c.backend.ModelID()
	}
	if c.model.Model != "" {
		return c.model.Model
	}
	return "mcp-sampling"
}

// Chat sends messages through MCP sampling/createMessage and blocks
// until the host agent responds. Returns the same ChatResponse shape the
// Full HTTP Client returns so engine/loop/run don't need to know which
// transport is in use.
func (c *Client) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if c.backend == nil {
		// Default to os.Stdout so a Client built in-process (e.g. by the
		// MCP runtime) just works.
		c.SetWriter(os.Stdout)
	}
	return c.backend.Chat(ctx, messages)
}

// ChatStream is forwarded to the SamplingBackend (which delegates to
// Chat because MCP sampling is atomic; see sampling.go).
func (c *Client) ChatStream(ctx context.Context, messages []Message, cb StreamCallback) (*ChatResponse, error) {
	if c.backend == nil {
		c.SetWriter(os.Stdout)
	}
	return c.backend.ChatStream(ctx, messages, cb)
}

// SimpleChat is a convenience wrapper used by autodata/improve/etc. that
// only need a single system+user turn and want the raw string back. It
// matches the Full-era Client.SimpleChat signature so callers don't
// have to be rewritten for the Light build.
func (c *Client) SimpleChat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := c.Chat(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("sampling returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// pickMaxTokens returns the model's MaxTokens when set, otherwise 0
// (which makes SamplingBackend use its 8192 default).
func pickMaxTokens(n int) int {
	if n > 0 {
		return n
	}
	return 0
}

// ErrNoHostAgent is the canonical "no MCP sampling transport available"
// error. Surfaced so cmd_loop / cmd_run can show a friendly hint.
var ErrNoHostAgent = fmt.Errorf("no host agent connected: run `radiant setup-mcp` from inside an MCP agent (Claude Code, Cursor, Hermes, …) before invoking loop/run/fleet")