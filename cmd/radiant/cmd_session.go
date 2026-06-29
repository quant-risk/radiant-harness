//go:build !light_only

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func registerSessionCmds(root *cobra.Command) {
	// ── state + handoff (session continuity, see handoff skill) ──
	// `radiant state` shows the current resume point.
	// `radiant handoff` writes a new resume point before closing the
	// session. Both read/write `.radiant-harness/state.md`. Pure
	// file I/O — no LLM call, no network, sub-second.
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Show the current session state (resume point)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".radiant-harness", "state.md")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("  ✗ %s not found — run 'radiant init .' first\n", path)
					return fmt.Errorf("state not initialized")
				}
				return err
			}
			fmt.Printf("  %s\n", path)
			fmt.Println("  ---")
			fmt.Print(string(data))
			return nil
		},
	}
	handoffCmd := &cobra.Command{
		Use:   "handoff",
		Short: "Pause: write the current session state to .radiant-harness/state.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentFeature, _ := cmd.Flags().GetString("feature")
			tierFlag, _ := cmd.Flags().GetString("tier")
			note, _ := cmd.Flags().GetString("note")
			nextCmd, _ := cmd.Flags().GetString("next-command")

			path := filepath.Join(".radiant-harness", "state.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			var b strings.Builder
			b.WriteString("# State\n\n")
			b.WriteString("## Current position\n")
			fmt.Fprintf(&b, "- current_feature: %s\n", strOrEmpty(currentFeature))
			fmt.Fprintf(&b, "- tier: %s\n", strOrEmpty(tierFlag))
			fmt.Fprintf(&b, "- next_command: %s\n", strOrEmpty(nextCmd))
			if note != "" {
				fmt.Fprintf(&b, "- note: %s\n", note)
			}
			b.WriteString("- blockers: []\n")
			b.WriteString("- open_questions: []\n\n")
			fmt.Fprintf(&b, "## Last session\n")
			fmt.Fprintf(&b, "- last_updated: %s\n", time.Now().UTC().Format(time.RFC3339))
			fmt.Fprintf(&b, "- last_summary: %q\n", summaryFor(note, currentFeature))

			// Atomic write: temp + rename
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
				return err
			}
			if err := os.Rename(tmp, path); err != nil {
				os.Remove(tmp)
				return err
			}
			fmt.Printf("  ✓ handoff written to %s\n", path)
			if nextCmd != "" {
				fmt.Printf("  Resume with: %s\n", nextCmd)
			}
			return nil
		},
	}
	handoffCmd.Flags().String("feature", "", "current feature in flight (e.g. 0002-jwt-auth)")
	handoffCmd.Flags().String("tier", "", "tier: trivial | feature | architecture")
	handoffCmd.Flags().String("note", "", "one-line summary of the session")
	handoffCmd.Flags().String("next-command", "", "literal CLI command to resume (e.g. 'radiant run specs/0002-jwt-auth --continue')")
	root.AddCommand(stateCmd, handoffCmd)
}
