package main

import (
	"fmt"
	"os"

	"github.com/quant-risk/radiant-harness/internal/semantic"
	"github.com/spf13/cobra"
)

func registerSemanticCmd(root *cobra.Command) {
	semCmd := &cobra.Command{
		Use:   "semantic",
		Short: "Resolve business terms against the semantic model",
		Long: `The semantic model maps business terms (PD, LGD, EAD, RWA, ...)
to formulas with regulation references (CMN 4.966, IFRS 9, Basileia).

When the loop runs in a project whose detected domain matches a
semantic-model domain, the runner automatically injects the resolved
metric definitions into the executor system prompt. The LLM resolves
queries like "RWA for Corporate exposure" against the curated YAML
instead of forgetting rules between turns.

This subcommand exposes the same model for direct inspection:
  radiant semantic list                    — list all domains
  radiant semantic show <domain>            — full markdown of one domain
  radiant semantic resolve <domain> <name>  — one metric, formula + regulation
  radiant semantic search <domain> <query>  — fuzzy search across the domain`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all known semantic-model domains",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Println("Available domains:")
			for _, d := range semantic.AllDomains() {
				l := semantic.NewLoader("")
				m, err := l.LoadDomain(d)
				if err != nil {
					fmt.Printf("  %-22s (no model embedded)\n", d)
					continue
				}
				fmt.Printf("  %-22s %s (%d metrics, v%s)\n", d, m.Title, len(m.Metrics), m.Version)
			}
			return nil
		},
	}
	semCmd.AddCommand(listCmd)

	showCmd := &cobra.Command{
		Use:   "show <domain>",
		Short: "Show the full semantic model for a domain (markdown)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l := semantic.NewLoader("")
			m, err := l.LoadDomain(semantic.Domain(args[0]))
			if err != nil {
				return err
			}
			fmt.Print(m.RenderMarkdown())
			return nil
		},
	}
	semCmd.AddCommand(showCmd)

	resolveCmd := &cobra.Command{
		Use:   "resolve <domain> <metric>",
		Short: "Resolve a single metric — formula, regulation, scopes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l := semantic.NewLoader("")
			m, err := l.LoadDomain(semantic.Domain(args[0]))
			if err != nil {
				return err
			}
			mt, err := m.Resolve(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("# %s (%s)\n\n", mt.Name, mt.Unit)
			fmt.Println(mt.Description)
			if mt.Regulation != "" {
				fmt.Printf("\n**Regulation:** %s\n", mt.Regulation)
			}
			if len(mt.Tags) > 0 {
				fmt.Printf("\n**Tags:** %v\n", mt.Tags)
			}
			if len(mt.Scopes) > 0 {
				fmt.Println("\n**Scopes:**")
				for _, s := range mt.Scopes {
					fmt.Printf("  - %s ∈ {%s}\n", s.Field, joinStrings(s.Values, ", "))
				}
			}
			fmt.Println("\n**Formula:**")
			fmt.Println("```")
			fmt.Println(string(mt.Formula))
			fmt.Println("```")
			return nil
		},
	}
	semCmd.AddCommand(resolveCmd)

	searchCmd := &cobra.Command{
		Use:   "search <domain> <query>",
		Short: "Fuzzy-search a domain for matching metrics",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l := semantic.NewLoader("")
			m, err := l.LoadDomain(semantic.Domain(args[0]))
			if err != nil {
				return err
			}
			hits := m.Search(args[1])
			if len(hits) == 0 {
				fmt.Printf("No metrics matching %q in %s\n", args[1], args[0])
				return nil
			}
			fmt.Printf("Matches for %q in %s:\n", args[1], args[0])
			for _, mt := range hits {
				fmt.Printf("  - %s (%s) — %s\n", mt.Name, mt.Unit, firstLine(mt.Description))
			}
			return nil
		},
	}
	semCmd.AddCommand(searchCmd)

	root.AddCommand(semCmd)
}

func joinStrings(s []string, sep string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// ensure os is used (other commands import it; some go vet configs complain otherwise)
var _ = os.Getenv