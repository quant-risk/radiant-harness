package mcpbridge

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

// startMockServer prepares the mock MCP server command but does
// NOT start it. The caller is responsible for wiring pipes via
// cmd.StdinPipe()/cmd.StdoutPipe() before calling cmd.Start().
//
// Keeping the start out of this helper means dialViaCmd can own
// the full pipe lifecycle. The path to the mock is relative to
// the test's working directory (the mcpbridge package dir).
func startMockServer(t *testing.T, mockToolsJSON string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("go", "run", "./mock", "-mock-server")
	cmd.Env = append(os.Environ(), "MOCK_TOOLS="+mockToolsJSON)
	return cmd
}

// killMockServer terminates the subprocess and waits for it to exit.
func killMockServer(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func TestClient_Initialize(t *testing.T) {
	cmd := startMockServer(t, "")
	defer killMockServer(t, cmd)

	client, err := dialViaCmd(t, cmd, "")
	if err != nil {
		t.Fatalf("dialViaCmd: %v", err)
	}
	defer client.Close()

	if client.Name() != "test" {
		t.Errorf("Name: got %q want test", client.Name())
	}
}

func TestClient_ListTools(t *testing.T) {
	mockTools := `[{"name":"echo","description":"Echoes the input","inputSchema":{"type":"object","properties":{"text":{"type":"string","description":"text"}},"required":["text"]}}]`
	cmd := startMockServer(t, mockTools)
	defer killMockServer(t, cmd)

	client, err := dialViaCmd(t, cmd, mockTools)
	if err != nil {
		t.Fatalf("dialViaCmd: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools: got %d want 1", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name: got %q want echo", tools[0].Name)
	}
}

func TestClient_CallTool(t *testing.T) {
	cmd := startMockServer(t, "")
	defer killMockServer(t, cmd)

	client, err := dialViaCmd(t, cmd, "")
	if err != nil {
		t.Fatalf("dialViaCmd: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := client.CallTool(ctx, "echo",
		json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !contains(string(out), "echo:") {
		t.Errorf("output should contain 'echo:', got %q", out)
	}
}

func TestClient_CallToolError(t *testing.T) {
	cmd := startMockServer(t, "")
	defer killMockServer(t, cmd)

	client, err := dialViaCmd(t, cmd, "")
	if err != nil {
		t.Fatalf("dialViaCmd: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.CallTool(ctx, "fail_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for fail_tool, got nil")
	}
	if !contains(err.Error(), "isError=true") {
		t.Errorf("error should mention isError=true: %v", err)
	}
}

func TestBridge_LoadTools_FullRoundtrip(t *testing.T) {
	mockTools := `[{"name":"greet","description":"Greet someone","inputSchema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}]`
	cmd := startMockServer(t, mockTools)
	defer killMockServer(t, cmd)

	client, err := dialViaCmd(t, cmd, mockTools)
	if err != nil {
		t.Fatalf("dialViaCmd: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(mcpTools) != 1 {
		t.Fatalf("got %d tools, want 1", len(mcpTools))
	}

	// Convert to local tools.Tool.
	localTool := mcpTools[0].ToLocalTool(client)
	if localTool.Name != "test__greet" {
		t.Errorf("Name: got %q want test__greet", localTool.Name)
	}

	// Invoke through the local tool.
	out, err := localTool.Invoke(ctx,
		json.RawMessage(`{"name":"world"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !contains(string(out.(json.RawMessage)), "echo:") {
		t.Errorf("output should contain 'echo:', got %q", out)
	}
}

// dialViaCmd wires pipes, starts the cmd, and constructs a Client.
// Returns a Client ready for use. The caller is responsible for
// killing the cmd via killMockServer.
func dialViaCmd(t *testing.T, cmd *exec.Cmd, _ string) (*Client, error) {
	t.Helper()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	// Drain stderr in a background goroutine — the mock server
	// doesn't write to it, but real servers might. Discarding
	// keeps the pipe from blocking on full.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				os.Stderr.WriteString("mock_server stderr: " + string(buf[:n]) + "\n")
			}
			if err != nil {
				return
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, err
	}

	// Pass nil stderr to NewClientWithStdio so it doesn't spawn
	// a second drain goroutine that might race with ours.
	client := NewClientWithStdio("test", stdin, stdout, nil)

	// Perform handshake.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Handshake(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

func TestParseSpec(t *testing.T) {
	cases := []struct {
		spec        string
		wantName    string
		wantCommand string
		wantArgs    []string
		wantErr     bool
	}{
		{
			spec:        "github:npx -y @modelcontextprotocol/server-github",
			wantName:    "github",
			wantCommand: "npx",
			wantArgs:    []string{"-y", "@modelcontextprotocol/server-github"},
		},
		{
			spec:        "fs:./bin/my-mcp-server --port 8080",
			wantName:    "fs",
			wantCommand: "./bin/my-mcp-server",
			wantArgs:    []string{"--port", "8080"},
		},
		{
			spec:        `pg:/usr/local/bin/pg-server --db "my db"`,
			wantName:    "pg",
			wantCommand: "/usr/local/bin/pg-server",
			wantArgs:    []string{"--db", "my db"},
		},
		{
			spec:        "noseparator",
			wantErr:     true,
		},
		{
			spec:        ":missing-name",
			wantErr:     true,
		},
		{
			spec:        "name:",
			wantErr:     true,
		},
		{
			spec:        "",
			wantErr:     true,
		},
	}
	for _, c := range cases {
		name, command, args, err := ParseSpec(c.spec)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseSpec(%q): err = %v, wantErr = %v", c.spec, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if name != c.wantName {
			t.Errorf("ParseSpec(%q): name = %q, want %q", c.spec, name, c.wantName)
		}
		if command != c.wantCommand {
			t.Errorf("ParseSpec(%q): command = %q, want %q", c.spec, command, c.wantCommand)
		}
		if len(args) != len(c.wantArgs) {
			t.Errorf("ParseSpec(%q): args = %v, want %v", c.spec, args, c.wantArgs)
			continue
		}
		for i := range args {
			if args[i] != c.wantArgs[i] {
				t.Errorf("ParseSpec(%q): args[%d] = %q, want %q",
					c.spec, i, args[i], c.wantArgs[i])
			}
		}
	}
}

func TestMCPTool_ToLocalTool_StringParam(t *testing.T) {
	mt := MCPTool{
		Name:        "search",
		Description: "Search for stuff",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"the query"}},"required":["query"]}`),
	}
	client := &Client{name: "test"}
	tool := mt.ToLocalTool(client)

	if tool.Name != "test__search" {
		t.Errorf("Name: got %q want test__search (prefixed with bridge name)", tool.Name)
	}
	if !contains(tool.Description, "bridged from MCP server") {
		t.Errorf("Description should mention bridge source: %q", tool.Description)
	}
	if len(tool.Params) != 1 {
		t.Fatalf("Params: got %d want 1", len(tool.Params))
	}
	if tool.Params[0].Name != "query" || tool.Params[0].Type != "string" {
		t.Errorf("Param: got %+v want {query, string}", tool.Params[0])
	}
	if !tool.Params[0].Required {
		t.Error("query should be required")
	}
}

func TestMCPTool_ToLocalTool_MultipleTypes(t *testing.T) {
	mt := MCPTool{
		Name: "complex",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {"type": "string"},
				"count": {"type": "integer"},
				"verbose": {"type": "boolean"},
				"tags": {"type": "array"}
			},
			"required": ["text", "count"]
		}`),
	}
	client := &Client{name: "test"}
	tool := mt.ToLocalTool(client)
	if len(tool.Params) != 4 {
		t.Fatalf("Params: got %d want 4", len(tool.Params))
	}
	requiredCount := 0
	for _, p := range tool.Params {
		if p.Required {
			requiredCount++
		}
	}
	if requiredCount != 2 {
		t.Errorf("required: got %d want 2", requiredCount)
	}
}

func TestMCPTool_ToLocalTool_InvalidSchema_PassesThrough(t *testing.T) {
	mt := MCPTool{
		Name:        "weird",
		InputSchema: json.RawMessage(`not valid json`),
	}
	client := &Client{name: "test"}
	tool := mt.ToLocalTool(client)
	if len(tool.Params) != 1 || tool.Params[0].Type != "object" {
		t.Errorf("invalid schema should produce opaque 'object' param, got %+v", tool.Params)
	}
}

// contains is a tiny helper to avoid importing strings just for one check.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}