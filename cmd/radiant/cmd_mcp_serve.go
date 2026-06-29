package main

// `radiant mcp serve` boots the MCP server that any host agent (Claude Code,
// Cursor, Hermes, Codex, OpenCode, etc.) can connect to and drive the harness
// via JSON-RPC + sampling/createMessage. Inference comes exclusively from
// the host agent — radiant never opens an HTTP connection to an LLM provider.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Project-root markers, in priority order. The first one found when walking
// up from the current working directory wins.
var projectRootMarkers = []string{
	"rad.yaml",     // radiant-harness project config (highest priority)
	".git",         // any git repo
	"go.mod",       // Go module
	"package.json", // Node/npm
	"Cargo.toml",   // Rust
	"pyproject.toml", // Python (modern)
	"setup.py",     // Python (legacy)
	"pom.xml",      // Java (Maven)
	"build.gradle", // Java/Gradle
	"Gemfile",      // Ruby
	"composer.json", // PHP
}

// detectProjectRoot walks up from `start` looking for any of the
// projectRootMarkers. Returns the directory containing the first marker
// found, or `start` unchanged if nothing is found within maxLevels.
func detectProjectRoot(start string, maxLevels int) string {
	if start == "" {
		if cwd, err := os.Getwd(); err == nil {
			start = cwd
		} else {
			return ""
		}
	}
	dir := start
	for i := 0; i < maxLevels; i++ {
		for _, marker := range projectRootMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

// isCharDevice returns true if fd is a character device (TTY).
// Used to warn when `radiant mcp serve` is invoked from a terminal
// instead of from an MCP host.
func isCharDevice(fd *os.File) bool {
	info, err := fd.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func registerMCPServeCmd(root *cobra.Command) {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	var (
		flagCwd            string
		flagSamplingTimeout time.Duration
		flagModelHint      string
	)

	mcpServeCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server on stdio (sampling/createMessage to host agent)",
		Long: `Start the MCP server on stdio. Every LLM call is routed back
to the calling agent via MCP sampling/createMessage — Claude Code,
Hermes, Cursor, etc. The host agent pays for inference; radiant never
needs an API key.

Wire it into your agent with 'radiant setup-mcp', restart the agent,
and any prompt that calls 'radiant_run' will drive the loop.

Flags:

  --cwd=<path>           Set the working directory before booting the
                         loop. Used when the host MCP config cannot
                         express 'cwd' (Hermes, Xcode, …) or you want
                         a different project root than the agent's
                         own CWD. If empty, radiant auto-detects:
                         walks up from $PWD looking for rad.yaml,
                         .git, go.mod, package.json, Cargo.toml, etc.

  --sampling-timeout=<dur>  Per-call sampling timeout. Accepts Go
                         duration syntax (90s, 2m, 1500ms). Default
                         120 s when an MCP host is wired (so cold-
                         start calls complete); 5 s when there is no
                         wired host. Override via RADIANT_SAMPLING_TIMEOUT.

  --model-hint=<name>    MCP modelPreferences hint (suggestion only;
                         the host may ignore). Empty by default.
                         Environment RADIANT_MODEL has the same effect.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Sanity-check: if stdin is a TTY, the operator probably
			// ran this from a terminal by accident. Warn but don't
			// refuse — the MCP server can be useful for debugging.
			if isCharDevice(os.Stdin) {
				fmt.Fprintln(os.Stderr,
					"warning: radiant mcp serve is intended to be invoked "+
						"by an MCP host (e.g. Claude Code). Running from a "+
						"terminal with a TTY stdin won't receive any JSON-RPC "+
						"requests and will exit immediately.")
			}

			// Resolve cwd.
			cwd := strings.TrimSpace(flagCwd)
			if cwd == "" {
				if pwd := os.Getenv("RADIANT_CWD"); pwd != "" {
					cwd = pwd
				} else if pwd := os.Getenv("PWD"); pwd != "" {
					cwd = pwd
				}
			}
			if cwd == "" {
				cwd, _ = os.Getwd()
			} else if info, err := os.Stat(cwd); err == nil && info.IsDir() {
				// user-provided, exists
			} else {
				fmt.Fprintf(os.Stderr, "warning: --cwd %q does not exist; auto-detecting\n", cwd)
				cwd, _ = os.Getwd()
			}
			projectRoot := detectProjectRoot(cwd, 32)
			if projectRoot != cwd {
				if err := os.Chdir(projectRoot); err == nil {
					fmt.Fprintf(os.Stderr, "radiant: project root auto-detected → %s\n", projectRoot)
				}
			} else if cwd != "" {
				if err := os.Chdir(cwd); err == nil {
					fmt.Fprintf(os.Stderr, "radiant: cwd set to %s\n", cwd)
				}
			}

			// Resolve sampling timeout. Precedence:
			//   --sampling-timeout flag  >  $RADIANT_SAMPLING_TIMEOUT  >  default 120 s
			timeout := flagSamplingTimeout
			if timeout == 0 {
				if env := os.Getenv("RADIANT_SAMPLING_TIMEOUT"); env != "" {
					if d, err := time.ParseDuration(env); err == nil {
						timeout = d
					}
				}
			}
			if timeout == 0 {
				timeout = 120 * time.Second
			}

			// Resolve model hint: --model-hint flag, then $RADIANT_MODEL.
			modelHint := flagModelHint
			if modelHint == "" {
				modelHint = os.Getenv("RADIANT_MODEL")
			}

			fmt.Fprintf(os.Stderr, "radiant mcp serve: sampling_timeout=%s, model_hint=%q, cwd=%s\n",
				timeout, modelHint, mustGetwd())

			return runMCPServe(os.Stdin, os.Stdout, true, timeout, modelHint)
		},
	}

	mcpServeCmd.Flags().StringVar(&flagCwd, "cwd", "",
		"Set the working directory before booting the loop. Empty = auto-detect project root.")
	mcpServeCmd.Flags().DurationVar(&flagSamplingTimeout, "sampling-timeout", 0,
		"Per-call timeout for sampling/createMessage (e.g. 120s, 2m). "+
			"Default 120s; override with $RADIANT_SAMPLING_TIMEOUT.")
	mcpServeCmd.Flags().StringVar(&flagModelHint, "model-hint", "",
		"Optional model hint suggested to the host (MCP modelPreferences.hint.name). "+
			"Equivalent to $RADIANT_MODEL.")

	mcpCmd.AddCommand(mcpServeCmd)
	root.AddCommand(mcpCmd)
}

func mustGetwd() string {
	if d, err := os.Getwd(); err == nil {
		return d
	}
	return "<unknown>"
}
