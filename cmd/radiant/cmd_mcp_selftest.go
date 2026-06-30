package main

// `radiant mcp self-test` boots a child `radiant mcp serve` process, sends a
// real JSON-RPC `initialize` + `tools/list` flow, and reports whether the
// wire-up is healthy. Used as a smoke test for regressions in the MCP
// server state machine without needing a wired host agent.
//
// We deliberately do NOT call `tools/call radiant_possess` in self-test mode,
// because the harness loop would try to round-trip via sampling/createMessage
// back to the host (us) and deadlock waiting for a sampling response we
// never produce. Run a full MCP possession test (Python host) to exercise
// the loop path.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func registerMCPSelfTestCmd(parent *cobra.Command) {
	var flagTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "self-test",
		Short: "Boot a child radiant mcp serve, send init + tools/list, report PASS/FAIL",
		Long: `Boots a child 'radiant mcp serve' process, sends the JSON-RPC
discovery sequence (initialize → tools/list) over stdio, and reports
whether the wire-up is healthy.

Self-test does NOT need a wired host agent — it just verifies that
the binary itself is invokable through the MCP stdio wire format and
that the server responds to the basic MCP discovery sequence with the
expected toolset.

For a full loop run, see the Python possession host (tests/mcp_possession_test.py
or similar). Use 'radiant doctor --mcp' to validate the host agent's
MCP config (Hermes config.yaml, Claude's mcp.json, etc.).`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if flagTimeout == 0 {
				flagTimeout = 15 * time.Second
			}
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate radiant binary: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), flagTimeout)
			defer cancel()

			report := runMCPSelfTest(ctx, exe, os.Stdout, os.Stderr)
			if !report.OK {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 15*time.Second,
		"Total wall-clock timeout for the self-test.")

	parent.AddCommand(cmd)
}

// selfTestReport is the structured result of one self-test invocation.
type selfTestReport struct {
	OK            bool
	InitializeMs  int64
	ToolsListMs   int64
	TotalMs       int64
	ServerName    string
	ServerVersion string
	ToolNames     []string
	ErrorMessage  string
	Hint          string
}

// runMCPSelfTest is the testable body of `radiant mcp self-test`.
// It spawns the binary at `exe`, sends the JSON-RPC discovery sequence
// over stdio, and returns a structured report. Side effects: writes
// human-readable output to stdout / stderr.
//
// We deliberately stop after tools/list. Calling tools/call radiant_possess
// from a self-test would deadlock (the harness waits for a sampling
// response we never produce).
func runMCPSelfTest(ctx context.Context, exe string, stdout, stderr io.Writer) selfTestReport {
	start := time.Now()
	report := selfTestReport{}

	cmd := exec.CommandContext(ctx, exe, "mcp", "serve")
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		report.ErrorMessage = "create stdin pipe: " + err.Error()
		report.Hint = "self-test could not allocate pipes; check /dev/fd availability"
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	cmd.Stdin = pipeR
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		report.ErrorMessage = "create stdout pipe: " + err.Error()
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	cmd.Stdout = stdoutW
	// stderr: pass through so the operator sees the child's stderr too
	// (radiant mcp serve prints cwd / sampling-timeout hints there).
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		report.ErrorMessage = "start child radiant mcp serve: " + err.Error()
		report.Hint = "verify the radiant binary at " + exe + " is executable"
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	// We own the write ends of both pipes; closing pipeW makes the child's
	// scanner return EOF as soon as it has flushed any pending output.
	defer func() {
		_ = pipeW.Close()
		_ = stdoutW.Close()
	}()

	// 1. initialize
	initializeStart := time.Now()
	initResp, err := sendJSONRPC(pipeW, stdoutR, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]string{"name": "radiant-self-test", "version": "1.0"},
		},
	}, 5*time.Second)
	report.InitializeMs = time.Since(initializeStart).Milliseconds()
	if err != nil {
		report.ErrorMessage = "initialize failed: " + err.Error()
		report.Hint = "if the binary exited immediately, run with stderr visible to inspect startup"
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	serverInfo, ok := initResp["result"].(map[string]interface{})["serverInfo"].(map[string]interface{})
	if !ok {
		report.ErrorMessage = "initialize response missing serverInfo"
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	report.ServerName, _ = serverInfo["name"].(string)
	report.ServerVersion, _ = serverInfo["version"].(string)

	// 2. tools/list
	tlStart := time.Now()
	tlResp, err := sendJSONRPC(pipeW, stdoutR, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}, 5*time.Second)
	report.ToolsListMs = time.Since(tlStart).Milliseconds()
	if err != nil {
		report.ErrorMessage = "tools/list failed: " + err.Error()
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}
	toolsRaw, _ := tlResp["result"].(map[string]interface{})["tools"].([]interface{})
	for _, t := range toolsRaw {
		if m, ok := t.(map[string]interface{}); ok {
			if name, _ := m["name"].(string); name != "" {
				report.ToolNames = append(report.ToolNames, name)
			}
		}
	}
	if len(report.ToolNames) == 0 {
		report.ErrorMessage = "tools/list returned empty toolset"
		report.Hint = "the binary should register at least one tool (radiant_possess)"
		writeSelfTestReport(stdout, report, time.Since(start))
		return report
	}

	report.OK = true
	report.TotalMs = time.Since(start).Milliseconds()
	writeSelfTestReport(stdout, report, time.Since(start))
	// Close stdin so the child sees EOF and exits cleanly.
	_ = pipeW.Close()
	_ = cmd.Wait()
	return report
}

// sendJSONRPC writes one JSON-RPC request and reads one matching response
// (matched by `id`). The HTTP-style request/response format used here is
// line-delimited NDJSON — the same format `radiant mcp serve` writes and reads.
func sendJSONRPC(w io.Writer, r io.Reader, msg map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	id, _ := msg["id"].(json.Number)
	if id == "" {
		// numeric or string both ok; just stringify once
		idJSON, _ := json.Marshal(msg["id"])
		id = json.Number(string(idJSON))
	}
	// marshal and write
	line, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	if _, err := w.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// read lines until we find one whose "id" matches
	deadline := time.Now().Add(timeout)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for response id=%s", id)
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("read: %w", err)
			}
			return nil, fmt.Errorf("server closed stream before sending id=%s", id)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue // skip non-JSON lines, scan.go warnings, etc.
		}
		// Match id (accept either JSON Number form or pre-stringified).
		if matchesID(resp["id"], msg["id"]) {
			return resp, nil
		}
	}
}

// matchesID tolerates JSON decoding variations for ID. JSON numbers and
// strings are commonly returned by different hosts, and the server itself
// does not enforce a type. Returns true when the two values are equal under
// a best-effort comparison.
func matchesID(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func isWireDown(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "server closed stream") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "i/o on closed pipe")
}

// extractResultText pulls the inner tool result text from a tools/call response.
func extractResultText(resp map[string]interface{}) string {
	if resp == nil {
		return ""
	}
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		if m, _ := errObj["message"].(string); m != "" {
			return "error: " + m
		}
	}
	result, _ := resp["result"].(map[string]interface{})
	if result == nil {
		return ""
	}
	content, _ := result["content"].([]interface{})
	for _, c := range content {
		if m, ok := c.(map[string]interface{}); ok {
			if t, _ := m["type"].(string); t == "text" {
				if s, _ := m["text"].(string); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// extractIsError reads the isError flag from a tools/call response.
func extractIsError(resp map[string]interface{}) bool {
	if resp == nil {
		return false
	}
	result, _ := resp["result"].(map[string]interface{})
	if result == nil {
		return false
	}
	v, _ := result["isError"].(bool)
	return v
}

// writeSelfTestReport prints the structured self-test report in a
// human-readable format. Failed reports print first, then a Suggested fix
// hint pointing at `radiant doctor --mcp` for further diagnosis.
func writeSelfTestReport(w io.Writer, r selfTestReport, elapsed time.Duration) {
	verdict := "PASS"
	if !r.OK {
		verdict = "FAIL"
	}
	fmt.Fprintf(w, "radiant mcp self-test: %s\n", verdict)
	fmt.Fprintf(w, "  server         : %s %s\n", r.ServerName, r.ServerVersion)
	fmt.Fprintf(w, "  tools          : %s\n", strings.Join(r.ToolNames, ", "))
	fmt.Fprintf(w, "  initialize     : %d ms\n", r.InitializeMs)
	fmt.Fprintf(w, "  tools/list     : %d ms\n", r.ToolsListMs)
	fmt.Fprintf(w, "  total          : %s\n", elapsed.Round(time.Millisecond))
	if !r.OK {
		fmt.Fprintf(w, "  error          : %s\n", r.ErrorMessage)
		if r.Hint != "" {
			fmt.Fprintf(w, "  hint           : %s\n", r.Hint)
		}
		fmt.Fprintf(w, "  next           : run `radiant doctor --mcp` for host config diagnosis\n")
	}
}
