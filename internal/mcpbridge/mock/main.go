// Mock MCP server for testing. Build with `go run` from the
// internal/mcpbridge/mock directory; reads JSON-RPC from stdin,
// writes to stdout.
//
// Usage in tests:
//
//	cmd := exec.Command("go", "run", "./internal/mcpbridge/mock", "-mock-server")
//	cmd.Env = append(os.Environ(), "MOCK_TOOLS=<json>")
//
// Or run standalone:
//
//	MOCK_TOOLS='[{"name":"foo"}]' go run ./internal/mcpbridge/mock -mock-server

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-mock-server" {
		runMockServer()
		return
	}
	fmt.Fprintln(os.Stderr, "mock_server: invoke with -mock-server")
	os.Exit(1)
}

func runMockServer() {
	toolsJSON := os.Getenv("MOCK_TOOLS")
	if toolsJSON == "" {
		toolsJSON = `[{"name":"echo","description":"Echoes the input","inputSchema":{"type":"object","properties":{"text":{"type":"string","description":"text to echo"}},"required":["text"]}}]`
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "mock_server: read: %v\n", err)
			}
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if req.JSONRPC != "2.0" {
			continue
		}
		if req.ID == nil {
			continue // notification
		}

		var result json.RawMessage
		switch req.Method {
		case "initialize":
			result = json.RawMessage(`{"protocolVersion":"2024-11-05","serverInfo":{"name":"mock","version":"0"},"capabilities":{}}`)
		case "tools/list":
			result = json.RawMessage(`{"tools":` + toolsJSON + `}`)
		case "tools/call":
			result = handleToolCall(req.Params)
		default:
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"error":   map[string]any{"code": -32601, "message": "method not found: " + req.Method},
			}
			writeJSON(os.Stdout, resp)
			continue
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      *req.ID,
			"result":  result,
		}
		writeJSON(os.Stdout, resp)
	}
}

func handleToolCall(params json.RawMessage) json.RawMessage {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(params, &p)

	if p.Name == "fail_tool" {
		return json.RawMessage(`{"isError":true,"content":[{"type":"text","text":"simulated failure"}]}`)
	}

	out := map[string]any{
		"isError": false,
		"content": []map[string]any{
			{"type": "text", "text": "echo: " + string(p.Arguments)},
		},
	}
	data, _ := json.Marshal(out)
	return data
}

func writeJSON(w io.Writer, v any) {
	data, _ := json.Marshal(v)
	w.Write(data)
	w.Write([]byte("\n"))
}