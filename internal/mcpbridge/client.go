// Package mcpbridge implements an MCP stdio client that can register
// external MCP server tools into the local tools.Registry.
//
// Status (Sprint 72 / v2.41.0): stdio transport only. HTTP/SSE is
// Sprint 73+ territory.
//
// Wire format: JSON-RPC 2.0 over the server's stdin/stdout. Each
// request gets an incrementing ID; responses are matched by ID via
// a sync.Map. The client serialises writes with a mutex so multiple
// concurrent goroutines can safely invoke CallTool.
//
// Lifecycle:
//
//	client, err := mcpbridge.Dial(ctx, "github", "npx", []string{"-y", "@mcp/server-github"})
//	if err != nil { ... }
//	defer client.Close()
//
//	tools, err := client.ListTools(ctx)
//	for _, t := range tools {
//	    registry.Register(t.ToLocalTool(client))
//	}
package mcpbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Timeouts. Each RPC gets its own deadline so a hung server doesn't
// pin the loop forever. Defaults are conservative; raise them via
// the per-Call context if a workload needs longer.
const (
	initializeTimeout = 10 * time.Second
	listToolsTimeout  = 30 * time.Second
	callToolTimeout   = 60 * time.Second
)

// ErrTimeout is returned when an RPC exceeds its deadline.
var ErrTimeout = errors.New("mcp_bridge: timeout")

// ErrServerCrash is returned when the server subprocess dies
// unexpectedly (broken pipe, exit, etc.).
var ErrServerCrash = errors.New("mcp_bridge: server crashed")

// ErrProtocol is returned when the server response doesn't match
// JSON-RPC 2.0 (missing jsonrpc field, id mismatch, etc.).
var ErrProtocol = errors.New("mcp_bridge: protocol error")

// Client is an MCP stdio client. Not safe for concurrent Close
// calls; safe for concurrent CallTool calls.
type Client struct {
	name    string
	cmd     *exec.Cmd         // nil when constructed via NewClientWithStdio
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  io.Reader
	nextID  atomic.Int64
	pending sync.Map // map[int64]chan rpcResponse
	closed  atomic.Bool
	mu      sync.Mutex // serialises writes to stdin
}

// rpcResponse is what we read off the wire. Errors here are protocol-
// level; the JSON-RPC `error` field is captured separately.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Dial spawns the MCP server subprocess and performs the
// `initialize` handshake. The server is described by `command` and
// `args` (e.g. "npx", ["-y", "@mcp/server-github"]).
//
// The context controls the subprocess lifetime — cancelling it
// triggers a graceful Close.
func Dial(ctx context.Context, name, command string, args []string) (*Client, error) {
	if name == "" {
		return nil, errors.New("mcp_bridge: name is required")
	}
	if command == "" {
		return nil, errors.New("mcp_bridge: command is required")
	}

	cmd := exec.CommandContext(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp_bridge: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("mcp_bridge: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("mcp_bridge: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("mcp_bridge: start server: %w", err)
	}

	c := newClient(name, cmd, stdin, stdout, stderr)

	initCtx, cancel := context.WithTimeout(ctx, initializeTimeout)
	defer cancel()
	if err := c.initialize(initCtx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// NewClientWithStdio constructs a Client from explicit stdin/stdout/
// stderr streams. Used by tests to inject a pre-existing subprocess
// with custom env (Dial builds its own cmd and doesn't expose env).
//
// The returned Client does NOT own the underlying process — Close
// only closes the streams; the caller must manage the subprocess
// lifecycle separately (typically via cmd.Wait() in a goroutine).
func NewClientWithStdio(name string, stdin io.WriteCloser, stdout io.Reader, stderr io.Reader) *Client {
	c := newClient(name, nil, stdin, stdout, stderr)
	return c
}

// newClient is the shared constructor used by both Dial and
// NewClientWithStdio. Wires the streams, starts the reader
// goroutine, and drains stderr in the background.
func newClient(name string, cmd *exec.Cmd, stdin io.WriteCloser, stdout io.Reader, stderr io.Reader) *Client {
	c := &Client{
		name:   name,
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: stderr,
	}
	go c.readResponses()
	if stderr != nil {
		go func() { _, _ = io.Copy(io.Discard, stderr) }()
	}
	return c
}

// Handshake performs the MCP `initialize` + `notifications/initialized`
// sequence on a Client that was built via NewClientWithStdio (Dial
// calls this internally). Exposed so tests can build a Client from
// pre-wired streams and still perform the handshake.
func (c *Client) Handshake(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, initializeTimeout)
	defer cancel()
	return c.initialize(initCtx)
}

// initialize performs the MCP `initialize` handshake. Minimal payload:
// protocolVersion + clientInfo. The server's response is acknowledged
// with a `notifications/initialized` notification (no response expected).
func (c *Client) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "radiant-harness",
			"version": "2.41.0",
		},
		"capabilities": map[string]any{},
	}
	resp, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if len(resp) == 0 {
		return fmt.Errorf("initialize: empty result")
	}
	if err := c.notify("notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}
	return nil
}

// notify sends a JSON-RPC notification (no `id` field, no response expected).
func (c *Client) notify(method string, params any) error {
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed.Load() {
		return ErrServerCrash
	}
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	return nil
}

// call sends a JSON-RPC request and waits for the response. The
// context controls the call timeout.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	respCh := make(chan rpcResponse, 1)
	c.pending.Store(id, respCh)
	defer c.pending.Delete(id)

	c.mu.Lock()
	if c.closed.Load() {
		c.mu.Unlock()
		return nil, ErrServerCrash
	}
	if _, err := c.stdin.Write(data); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("write request: %w", err)
	}
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// readResponses is the background goroutine that reads JSON-RPC
// responses off stdout and dispatches them to waiting callers.
func (c *Client) readResponses() {
	for {
		if c.closed.Load() {
			return
		}
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			c.pending.Range(func(key, value any) bool {
				if ch, ok := value.(chan rpcResponse); ok {
					select {
					case ch <- rpcResponse{Error: &rpcError{Code: -1, Message: "server crashed"}}:
					default:
					}
				}
				return true
			})
			return
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.JSONRPC != "2.0" {
			continue
		}
		if ch, ok := c.pending.Load(resp.ID); ok {
			if respCh, ok := ch.(chan rpcResponse); ok {
				respCh <- resp
			}
		}
	}
}

// Close terminates the underlying streams. For clients built via
// Dial, this also gracefully terminates the subprocess (SIGTERM,
// then SIGKILL after 2 seconds). For clients built via
// NewClientWithStdio, only the streams are closed — the caller
// manages the subprocess.
//
// Idempotent — safe to call multiple times.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			c.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = c.cmd.Process.Kill()
			<-done
		}
	}
	return nil
}

// Name returns the bridge name (useful for trace attribution).
func (c *Client) Name() string { return c.name }