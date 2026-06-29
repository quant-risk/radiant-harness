package main

// `radiant host-info` — print which agent (if any) is currently
// invoking radiant-harness. Useful in either context:
//
//   - Inside an agent (Claude Code, Cursor, Hermes, ...):
//     `host-info` confirms which one detected us.
//   - From a shell with no agent: `host-info` reports "unknown"
//     so the operator knows possession isn't happening.
//
// The subcommand is implemented on top of the internal/hostdetect
// package. When a host supports MCP sampling/createMessage, the
// harness automatically routes inference back to that host.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quant-risk/radiant-harness/internal/hostdetect"
)

func registerHostInfoCmd(root *cobra.Command) {
	var jsonFlag bool
	var verboseFlag bool

	cmd := &cobra.Command{
		Use:   "host-info",
		Short: "Detect and print which agent (if any) is currently invoking radiant",
		Long: `Detect which agent host — Claude Code, Cursor, Hermes, Kimi CLI,
OpenClaw, Codex, Cline, OpenCode, VS Code Copilot, or none — is currently
driving this radiant-harness invocation.

The detection is two-layered:
  1. Env-var fingerprint (high confidence)
  2. Parent process name fallback (medium confidence)

The output is designed for humans by default and JSON when --json
is passed. Both modes include the agent ID, confidence score, and
which env-var fingerprint matched.

Use this to verify possession: if SupportsSampling=true, the harness
can drive the host agent's LLM via MCP sampling/createMessage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d := hostdetect.New()
			info := d.Detect()

			if jsonFlag {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			renderHostInfo(os.Stdout, info, verboseFlag)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON instead of human-readable output")
	cmd.Flags().BoolVar(&verboseFlag, "verbose", false, "show internal detection details (which env vars matched, etc.)")
	root.AddCommand(cmd)
}

// renderHostInfo writes info to w. Pretty-prints the most relevant
// fields; --verbose shows the full breakdown.
func renderHostInfo(w *os.File, info hostdetect.HostInfo, verbose bool) {
	// Header.
	confLabel := confidenceLabel(info.Confidence)
	fmt.Fprintf(w, "Detected host agent:  %s (%s)\n", info.Agent.String(), confLabel)
	fmt.Fprintf(w, "Sampling supported:  %s\n", boolWord(info.SupportsSampling))
	fmt.Fprintf(w, "Detection source:    %s\n", info.DetectionSource)
	fmt.Fprintf(w, "PID:                  %d  PPID: %d\n", info.PID, info.PPID)
	if info.ParentCmd != "" {
		fmt.Fprintf(w, "Parent cmd:           %s\n", info.ParentCmd)
	}

	if verbose && len(info.SampleEnvVars) > 0 {
		fmt.Fprintln(w, "\nMatched env vars:")
		// Stable ordering for human readability.
		sorted := append([]string(nil), info.SampleEnvVars...)
		sort.Strings(sorted)
		for _, k := range sorted {
			fmt.Fprintf(w, "  - %s\n", k)
		}
	}

	fmt.Fprintln(w)
	switch {
	case info.Agent == hostdetect.AgentUnknown:
		fmt.Fprintln(w, "No agent host detected. radiant-harness is running standalone.")
		fmt.Fprintln(w, "Run `radiant setup-mcp` from inside your agent to wire it in.")
	case !info.SupportsSampling:
		fmt.Fprintf(w, "Host %q does not support MCP sampling/createMessage.\n", info.Agent)
		fmt.Fprintln(w, "Possession via sampling is not possible from this host.")
	default:
		fmt.Fprintf(w, "Host %q supports MCP sampling — possession is possible.\n", info.Agent)
		if info.Confidence >= 75 {
			fmt.Fprintln(w, "radiant can drive this agent's LLM via MCP sampling.")
		} else {
			fmt.Fprintln(w, "(Medium/Low confidence — heuristic; may be wrong.)")
		}
	}
}

// confidenceLabel maps a 0-100 confidence to a human label.
func confidenceLabel(c int) string {
	switch {
	case c >= 90:
		return "Certain"
	case c >= 75:
		return "High confidence"
	case c >= 50:
		return "Medium confidence"
	case c > 0:
		return "Low confidence"
	default:
		return "no signal"
	}
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// silence unused imports if any pair becomes stale.
var _ = strings.TrimSpace
