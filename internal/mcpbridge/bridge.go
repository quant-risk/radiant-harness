package mcpbridge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/tools"
)

// LoadTools dials an MCP server and returns the list of tools
// converted to local tools.Tool instances. The caller is responsible
// for calling client.Close() when done — typically by deferring it
// in the same scope as the registry registration.
//
// Typical usage:
//
//	client, err := mcpbridge.LoadTools(ctx, "github", "npx",
//	    []string{"-y", "@modelcontextprotocol/server-github"})
//	if err != nil { return err }
//	defer client.Close()
//	for _, tool := range client.Tools() {
//	    registry.Register(tool)
//	}
//
// This function is the bridge entry point used by the engine boot
// path and the CLI --mcp-bridge flag.
func LoadTools(ctx context.Context, name, command string, args []string) (*Client, []tools.Tool, error) {
	client, err := Dial(ctx, name, command, args)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp_bridge: dial %s: %w", name, err)
	}

	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("mcp_bridge: list tools for %s: %w", name, err)
	}

	out := make([]tools.Tool, 0, len(mcpTools))
	for _, mt := range mcpTools {
		out = append(out, *mt.ToLocalTool(client))
	}
	return client, out, nil
}

// ParseSpec parses a CLI flag value of the form "name:command args..."
// into its components. Whitespace separates the prefix name from the
// command; the command and its args are space-delimited.
//
// Examples:
//
//	"github:npx -y @modelcontextprotocol/server-github"
//	=> "github", "npx", ["-y", "@modelcontextprotocol/server-github"]
//
//	"fs:./bin/my-mcp-server --port 8080"
//	=> "fs", "./bin/my-mcp-server", ["--port", "8080"]
//
// Quoted arguments are not supported — keep server commands simple.
// For complex specs, use a config file instead.
func ParseSpec(spec string) (name, command string, args []string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", nil, errors.New("mcp_bridge: empty spec")
	}
	colon := strings.Index(spec, ":")
	if colon < 0 {
		return "", "", nil, errors.New("mcp_bridge: spec must contain ':' separating name from command")
	}
	name = strings.TrimSpace(spec[:colon])
	rest := strings.TrimSpace(spec[colon+1:])
	if name == "" {
		return "", "", nil, errors.New("mcp_bridge: empty name in spec")
	}
	if rest == "" {
		return "", "", nil, errors.New("mcp_bridge: empty command in spec")
	}

	// Split rest into command + args using shell-like tokenisation.
	// Quotes are honoured; escapes are not (keep specs simple).
	parts := splitShellArgs(rest)
	if len(parts) == 0 {
		return "", "", nil, errors.New("mcp_bridge: failed to parse command")
	}
	command = parts[0]
	args = parts[1:]
	return name, command, args, nil
}

// splitShellArgs is a minimal shell-like tokeniser. Handles single
// and double quotes; does not handle escapes.
func splitShellArgs(s string) []string {
	var out []string
	var current strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return out
}