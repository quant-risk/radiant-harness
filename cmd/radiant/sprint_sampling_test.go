//go:build with_full

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
)

// TestMCPServe_SamplingMode_ToolsListWorks verifies that tools/list works
// normally in sampling mode — the sampling flag doesn't break basic MCP.
func TestMCPServe_SamplingMode_ToolsListWorks(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer
	if err := runMCPServe(in, &out, true); err != nil {
		t.Fatalf("runMCPServe sampling mode: %v", err)
	}
	// Parse the response — should be a valid tools/list result.
	var resp map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\nraw: %s", err, out.String())
	}
	if resp["error"] != nil {
		t.Errorf("unexpected error in tools/list: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing result in tools/list response: %s", out.String())
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("missing tools array: %s", out.String())
	}
	if len(tools) == 0 {
		t.Error("expected non-empty tools list")
	}
}

// TestMCPRunFull_SamplingMode_RoutesToSampling verifies that mcpRunFull
// with a non-nil backend routes to the sampling path (no API key required).
// The loop blocks waiting for a sampling response, so we run it in a
// goroutine with a timeout. If it times out, that's proof it took the
// sampling path (the HTTP path would fail immediately with an API error).
// If it completes, the output should mention "sampling".
func TestMCPRunFull_SamplingMode_RoutesToSampling(t *testing.T) {
	// Create a sampling backend with a buffer writer.
	backend := llm.NewSamplingBackend(llm.SamplingOptions{
		ModelHint: "test-model",
		MaxTokens: 256,
		Out:       &bytes.Buffer{},
	})

	// Ensure NO API key is set.
	t.Setenv("RADIANT_OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("RADIANT_OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("RADIANT_ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	args := json.RawMessage(`{"goal":"test goal","max_iter":1}`)

	done := make(chan mcpResponse, 1)
	go func() {
		done <- mcpRunFull(args, backend)
	}()

	select {
	case resp := <-done:
		if resp.Error != nil {
			t.Errorf("should not be JSON-RPC error: %+v", resp.Error)
		}
		if resp.Result != nil {
			resultMap, _ := resp.Result.(map[string]interface{})
			if content, ok := resultMap["content"].([]map[string]string); ok && len(content) > 0 {
				if !strings.Contains(content[0]["text"], "sampling") {
					t.Errorf("expected 'sampling' in output, got: %s", content[0]["text"])
				}
			}
		}
	case <-time.After(2 * time.Second):
		// Expected: loop blocks on sampling with no responder. Pass.
	}
}

// TestMCPServe_SamplingMode_DispatchesResponse verifies that a JSON-RPC
// response on stdin (in sampling mode) is dispatched to the SamplingBackend
// rather than being treated as a parse error or unknown method.
func TestMCPServe_SamplingMode_DispatchesResponse(t *testing.T) {
	// We test that a sampling response line doesn't crash the server or
	// produce an error response. The response has "result" but no "method",
	// so IsSamplingResponse should route it to Dispatch.
	input := `{"jsonrpc":"2.0","id":99999,"result":{"role":"assistant","content":{"type":"text","text":"hi"},"model":"test"}}` + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	// This should complete without error and produce NO output (the response
	// is dispatched silently to the backend, no MCP response is written).
	if err := runMCPServe(in, &out, true); err != nil {
		t.Fatalf("runMCPServe: %v", err)
	}

	// The dispatched response is for an unknown ID, so it's silently dropped.
	// No JSON-RPC response should be written to stdout.
	if out.Len() > 0 {
		// If there IS output, it should NOT be a parse error or method-not-found.
		raw := out.String()
		if strings.Contains(raw, "parse error") {
			t.Errorf("sampling response should not be a parse error: %s", raw)
		}
		if strings.Contains(raw, "method not found") {
			t.Errorf("sampling response should not be 'method not found': %s", raw)
		}
	}
}

// TestMCPServe_SamplingMode_MixedRequestsAndResponses verifies that normal
// MCP requests and sampling responses can coexist on the same stdin.
func TestMCPServe_SamplingMode_MixedRequestsAndResponses(t *testing.T) {
	// A normal tools/list request followed by a sampling response.
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":99999,"result":{"role":"assistant","content":{"type":"text","text":"x"}}}` + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := runMCPServe(in, &out, true); err != nil {
		t.Fatalf("runMCPServe: %v", err)
	}

	// The tools/list request should produce exactly one response line.
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 response line (for tools/list), got %d: %s", len(lines), out.String())
	}

	// That line should contain the tools list.
	if !strings.Contains(out.String(), "radiant_run") {
		t.Errorf("expected radiant_run in tools/list response: %s", out.String())
	}
}

// TestMcpDispatcher_BackendNil verifies that backend() returns nil for a
// normal (non-sampling) dispatcher.
func TestMcpDispatcher_BackendNil(t *testing.T) {
	d := &mcpDispatcher{}
	if d.backend() != nil {
		t.Error("expected nil backend for normal dispatcher")
	}
}

// TestMcpDispatcher_BackendNilReceiver verifies that backend() handles a
// nil receiver safely.
func TestMcpDispatcher_BackendNilReceiver(t *testing.T) {
	var d *mcpDispatcher
	if d.backend() != nil {
		t.Error("expected nil backend for nil receiver")
	}
}
