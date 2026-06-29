
// MCP runtime. runMCPServe here is the sole MCP server entry point.
// It dispatches every tool call through `callMCPTool`, which routes
// inference via MCP sampling/createMessage back to the host agent.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/loop"
)

// MCP server. Single tool: radiant_run (host-agent driven via
// sampling/createMessage). No HTTP LLM, no API keys.
func runMCPServe(in io.Reader, out io.Writer, samplingMode bool, samplingTimeout time.Duration, modelHint string) error {
	d := &mcpDispatcher{}
	if samplingMode {
		d.sampling = llm.NewSamplingBackend(llm.SamplingOptions{
			ModelHint: modelHint,
			MaxTokens: 8192,
			Timeout:   samplingTimeout,
			Out:       out,
		})
	}

	tools := []mcpTool{
		{Name: "radiant_run", Description: "Run the full harness loop for a goal. Blocks until complete. Returns the full execution trace.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"goal":     {Type: "string", Description: "The goal to achieve (required)"},
				"profile":  {Type: "string", Description: "Execution profile: lean | standard | thorough (default: standard)"},
				"max_iter": {Type: "number", Description: "Max iterations (default 20)"},
				"max_cost": {Type: "string", Description: "Dollar cap, e.g. '2.00'"},
				"max_time": {Type: "string", Description: "Wall-clock cap, e.g. '10m'"},
			},
			Required: []string{"goal"},
		}},
	}

	var encMu sync.Mutex
	enc := json.NewEncoder(out)
	encode := func(v mcpResponse) {
		encMu.Lock()
		_ = enc.Encode(v)
		encMu.Unlock()
	}

	if samplingMode {
		d.sampling.SetWriteMu(&encMu)
	}

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if samplingMode && llm.IsSamplingResponse(line) {
			d.sampling.Dispatch(line)
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			encode(mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32700, Message: "parse error"}})
			continue
		}
		if samplingMode && req.Method == "tools/call" {
			go func(req mcpRequest) {
				encode(handleMCPRequestLight(req, tools, d))
			}(req)
			continue
		}
		encode(handleMCPRequestLight(req, tools, d))
	}
	return scanner.Err()
}

func handleMCPRequestLight(req mcpRequest, tools []mcpTool, d *mcpDispatcher) mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "radiant-harness",
					"version": version,
				},
				"capabilities": map[string]interface{}{"tools": map[string]interface{}{}},
			},
		}
	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32602, Message: "invalid params"}}
		}
		result := callMCPToolLight(params.Name, params.Arguments, d)
		result.ID = req.ID
		return result
	default:
		return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

func callMCPToolLight(name string, args json.RawMessage, d *mcpDispatcher) mcpResponse {
	if name != "radiant_run" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "unknown tool: " + name}}
	}
	return mcpRunWithBackendLight(args, d.backend())
}

func mcpRunWithBackendLight(args json.RawMessage, backend llm.Backend) mcpResponse {
	var a struct {
		Goal    string `json:"goal"`
		Profile string `json:"profile"`
		Model   string `json:"model"`
		MaxIter int    `json:"max_iter"`
		MaxCost string `json:"max_cost"`
		MaxTime string `json:"max_time"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Goal == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "radiant_run: goal is required"}}
	}
	if a.Profile == "" {
		a.Profile = "standard"
	}
	modelID := a.Model
	if modelID == "" {
		modelID = os.Getenv("RADIANT_MODEL")
	}
	if modelID == "" {
		modelID = "mcp-sampling"
	}
	maxIter := a.MaxIter
	var maxDuration time.Duration
	if a.MaxTime != "" {
		maxDuration, _ = time.ParseDuration(a.MaxTime)
	}
	runID := "run-" + strconv.FormatInt(time.Now().Unix(), 10)
	runCfg := loop.RunConfig{
		Backend: backend,
		Budget: loop.BudgetConfig{
			MaxIter:     maxIter,
			Profile:     loop.BudgetProfile(a.Profile),
			MaxDuration: maxDuration,
		},
	}
	result, err := loop.Run(context.Background(), "", runID, a.Goal, runCfg)
	var b []byte
	b = append(b, []byte(fmt.Sprintf("Run ID: %s\nGoal: %s\nModel: %s (sampling)\n\n", runID, a.Goal, modelID))...)
	if err != nil {
		b = append(b, []byte(fmt.Sprintf("Loop failed: %v\n", err))...)
	} else {
		b = append(b, []byte(fmt.Sprintf("Exit: %s\nIterations: %d\nElapsed: %s\nTokens: %d\n",
			result.ExitReason, result.Iterations,
			result.Elapsed.Round(time.Second), result.TokensUsed))...)
		if result.CostUSD > 0 {
			b = append(b, []byte(fmt.Sprintf("Cost: $%.4f\n", result.CostUSD))...)
		}
	}
	isErr := err != nil
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(b)}},
			"isError": isErr,
		},
	}
}
