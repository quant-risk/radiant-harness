package casetest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Phase is one bounded step in the harness possess flow.
type Phase string

const (
	PhaseDiscover Phase = "discover"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
)

// Event is a single observable thing the driver saw during a run,
// emitted to a sink so we can render the Markdown report afterwards.
type Event struct {
	At      time.Time
	Kind    string // "init" | "tool-list" | "sampling" | "phase-done" | "tool-call" | "final" | "err"
	Phase   Phase
	Latency time.Duration // time the host observed before responding
	Text    string
	Extra   map[string]any
}

// Config is the driver's runtime configuration.
type Config struct {
	Binary      string        // absolute path to radiant binary (defaults to os.Executable())
	Workdir     string        // project dir the harness operates on (case unpack target)
	ColdStartMs int           // simulated per-call sampling latency (default 25 s)
	JitterMs    int           // ± random spread on each call (default 5 s)
	SamplingTO  time.Duration // per-call sampling timeout the harness enforces (default 120 s)
	Profile     string        // harness profile (default "standard")
	OnEvent     func(Event)   // optional sink; nil = silent
}

// Driver owns a single radiant mcp serve subprocess and drives a JSON-RPC
// session through it.
type Driver struct {
	cfg    Config
	proc   *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	outCh  chan readResult
	readerDone chan struct{}
	events []Event
}

type readResult struct {
	line []byte
	err  error
}

// Run starts the subprocess, drives the full MCP session, and returns
// the final report + the events that occurred.
func Run(ctx context.Context, c *Case, cfg Config) (*Report, error) {
	if cfg.Binary == "" {
		cfg.Binary, _ = os.Executable()
	}
	if cfg.ColdStartMs == 0 {
		cfg.ColdStartMs = 25000
	}
	if cfg.JitterMs == 0 {
		cfg.JitterMs = 5000
	}
	if cfg.SamplingTO == 0 {
		cfg.SamplingTO = 120 * time.Second
	}
	if cfg.Profile == "" {
		cfg.Profile = "standard"
	}
	d := &Driver{cfg: cfg}

	if err := d.spawn(ctx); err != nil {
		return nil, err
	}
	defer d.close()

	if err := d.handshake(ctx); err != nil {
		return nil, err
	}
	if err := d.runPossess(ctx, c); err != nil {
		return nil, err
	}

	r := renderReport(c, d.cfg, d.events)
	return r, nil
}

// spawn launches `radiant mcp serve` and wires stdio pipes. The harness's
// own project-root auto-detect runs from the child's cwd, which we set
// to `cfg.Workdir`.
func (d *Driver) spawn(ctx context.Context) error {
	// Reset the harness state for this task so repeat runs start clean.
	// State is keyed by SHA-256(workdir || 0x00 || task), so different
	// prompts can coexist in the same dir but reusing the SAME prompt
	// would otherwise short-circuit to "phases done, return success"
	// before sending a single sampling call.
	if d.cfg.Workdir != "" {
		stateDir := filepath.Join(d.cfg.Workdir, ".radiant-harness", "state")
		os.RemoveAll(stateDir)
	}
	cmd := exec.CommandContext(ctx, d.cfg.Binary, "mcp", "serve",
		"--sampling-timeout="+d.cfg.SamplingTO.String(),
		"--cwd="+d.cfg.Workdir,
	)
	cmd.Env = append(os.Environ(),
		"RADIANT_SAMPLING_TIMEOUT="+d.cfg.SamplingTO.String(),
		"RADIANT_INTERNAL=1",
	)
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		pipeR.Close()
		pipeW.Close()
		return err
	}
	cmd.Stdin = pipeR
	cmd.Stdout = stdoutW
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		pipeR.Close()
		pipeW.Close()
		stdoutR.Close()
		stdoutW.Close()
		return fmt.Errorf("start child radiant mcp serve: %w", err)
	}
	_ = pipeR.Close()
	_ = stdoutW.Close()
	d.stdin = pipeW
	d.stdout = stdoutR
	d.proc = cmd

	// Dedicate a reader goroutine that streams raw bytes from the harness's
	// stdout into a channel. Per-line readers (with timeout) just pull from
	// the channel. This is the only safe pattern: bufio.Scanner is not safe
	// for concurrent use and we don't want per-call goroutines leaking.
	d.outCh = make(chan readResult, 16)
	d.readerDone = make(chan struct{})
	go func() {
		defer close(d.readerDone)
		sc := bufio.NewScanner(stdoutR)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			d.outCh <- readResult{line: line}
		}
		if err := sc.Err(); err != nil {
			d.outCh <- readResult{err: err}
		}
		// final line: EOF marker (channel close is detected via readerDone)
	}()
	return nil
}

func (d *Driver) close() {
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.stdout != nil {
		_ = d.stdout.Close()
	}
	// Drain pending reads so the reader goroutine can exit cleanly.
	go func() {
		// Wait briefly for goroutine to finish, then close stdout to
		// unblock it if it's mid-Scan. We don't want to leak.
		select {
		case <-d.readerDone:
			return
		case <-time.After(2 * time.Second):
			_ = d.stdout.Close()
			<-d.readerDone
		}
	}()
	if d.proc != nil {
		_ = d.proc.Wait()
	}
}

// handshake runs initialize + tools/list.
func (d *Driver) handshake(ctx context.Context) error {
	if err := d.write(ctx, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]string{"name": "radiant-test-case-host", "version": "1.0"},
		},
	}); err != nil {
		return err
	}
	if _, err := d.readJSON(ctx); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	d.push(Event{Kind: "init", Text: "initialize OK"})

	_ = d.write(ctx, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	})

	if err := d.write(ctx, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list",
	}); err != nil {
		return err
	}
	resp, err := d.readJSON(ctx)
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	d.push(Event{Kind: "tool-list"})

	tools, _ := resp["result"].(map[string]any)["tools"].([]any)
	names := []string{}
	for _, t := range tools {
		if m, ok := t.(map[string]any); ok {
			if name, _ := m["name"].(string); name != "" {
				names = append(names, name)
			}
		}
	}
	hasPosess := false
	for _, n := range names {
		if n == "radiant_possess" {
			hasPosess = true
		}
	}
	if !hasPosess {
		return fmt.Errorf("radiant_possess not in tools/list (have: %v)", names)
	}
	return nil
}

// runPossess invokes the radiant_possess tool, simulating a host that
// answers sampling/createMessage with a phase-correct canned response
// after the configured cold-start delay.
func (d *Driver) runPossess(ctx context.Context, c *Case) error {
	if err := d.write(ctx, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{
			"name": "radiant_possess",
			"arguments": map[string]any{
				"task":    c.UserPrompt,
				"workdir": c.Path,
				"profile": d.cfg.Profile,
			},
		},
	}); err != nil {
		return err
	}

	const maxIterations = 32 // belt-and-suspenders
	for i := 0; i < maxIterations; i++ {
		timeout := d.cfg.SamplingTO
		deadline, hasDeadline := ctx.Deadline()
		if hasDeadline && time.Until(deadline) < timeout {
			timeout = time.Until(deadline)
		}
		msg, err := d.readJSONWithTimeout(timeout)
		if err != nil {
			return fmt.Errorf("read json-rpc: %w", err)
		}
		method, _ := msg["method"].(string)
		if method != "sampling/createMessage" {
			if errObj, ok := msg["error"].(map[string]any); ok {
				code, _ := errObj["code"].(float64)
				mtext, _ := errObj["message"].(string)
				return fmt.Errorf("harness error (code=%v): %s", int(code), mtext)
			}
			resultText := extractResultText(msg)
			d.push(Event{Kind: "final", Text: resultText})
			return nil
		}
		mid := msg["id"]
		phase := detectPhaseFromSampling(msg)
		body := CannedResponse(phase, c)
		d.push(Event{
			Kind:  "sampling",
			Phase: phase,
			Text:  body,
		})
		sleepRand(d.cfg.ColdStartMs, d.cfg.JitterMs)
		if err := d.write(ctx, map[string]any{
			"jsonrpc": "2.0", "id": mid,
			"result": map[string]any{
				"role": "assistant",
				"content": map[string]any{"type": "text", "text": body},
				"model": "radiant-test-case-synth",
			},
		}); err != nil {
			return fmt.Errorf("write sampling response: %w", err)
		}
		d.push(Event{Kind: "phase-done", Phase: phase, Text: trim(body, 200)})
	}
	return fmt.Errorf("harness loop did not terminate after %d sampling rounds", maxIterations)
}

func (d *Driver) write(ctx context.Context, msg map[string]any) error {
	if d.stdin == nil {
		return fmt.Errorf("stdin closed")
	}
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := d.stdin.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (d *Driver) readJSON(ctx context.Context) (map[string]any, error) {
	return d.readJSONWithTimeout(d.cfg.SamplingTO)
}

func (d *Driver) readJSONWithTimeout(timeout time.Duration) (map[string]any, error) {
	if d.outCh == nil {
		return nil, fmt.Errorf("reader closed")
	}
	select {
	case r, ok := <-d.outCh:
		if !ok {
			return nil, fmt.Errorf("harness closed stream before sending a response")
		}
		if r.err != nil {
			return nil, r.err
		}
		var msg map[string]any
		if err := json.Unmarshal(r.line, &msg); err != nil {
			return nil, fmt.Errorf("malformed json line: %w (%s)", err, string(r.line))
		}
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("read json-rpc: timeout after %s", timeout)
	}
}

func (d *Driver) push(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	d.events = append(d.events, ev)
}

func sleepRand(meanMs, _ int) {
	if meanMs <= 0 {
		return
	}
	time.Sleep(time.Duration(meanMs) * time.Millisecond)
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func extractResultText(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	if errObj, ok := resp["error"].(map[string]any); ok {
		if m, _ := errObj["message"].(string); m != "" {
			return "error: " + m
		}
	}
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		return ""
	}
	content, _ := result["content"].([]any)
	for _, c := range content {
		if m, ok := c.(map[string]any); ok {
			if t, _ := m["type"].(string); t == "text" {
				if s, _ := m["text"].(string); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// detectPhaseFromSampling reads the `## radiant-phase: <name>` marker
// from the user message text and returns the matching Phase constant.
func detectPhaseFromSampling(msg map[string]any) Phase {
	params, _ := msg["params"].(map[string]any)
	msgs, _ := params["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		if mm == nil {
			continue
		}
		if role, _ := mm["role"].(string); role != "user" {
			continue
		}
		c, _ := mm["content"].(map[string]any)
		if c == nil {
			continue
		}
		text, _ := c["text"].(string)
		if !strings.Contains(text, "## radiant-phase: ") {
			continue
		}
		for _, ph := range []Phase{PhaseDiscover, PhasePlan, PhaseExecute, PhaseVerify} {
			marker := "## radiant-phase: " + string(ph) + "\n"
			if strings.Contains(text, marker) {
				return ph
			}
		}
	}
	// Fallback: not the harness's scheme (maybe sampling without the
	// marker). Caller treats missing marker as discover.
	return PhaseDiscover
}
