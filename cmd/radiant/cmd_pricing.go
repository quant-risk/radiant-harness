package main

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/pricing"
	"github.com/spf13/cobra"
)

func registerPricingCmd(root *cobra.Command) {
	pricingCmd := &cobra.Command{
		Use:   "pricing",
		Short: "Show, check, and refresh the LLM pricing table",
		Long: `The pricing table is embedded in the binary from
internal/pricing/data/pricing.yaml. It drives:
  - cost estimates in 'radiant loop status' / 'radiant loop export'
  - --max-cost budget enforcement
  - 'radiant eval' cost reporting

Commands:
  radiant pricing list   — show all known rates
  radiant pricing stale  — report whether the table needs refresh
  radiant pricing refresh — show how to refresh (data file path)`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all known model rates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := pricing.Default()
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PRESET\tPROVIDER\tMODEL\tINPUT/1K\tOUTPUT/1K\tMAX_TOKENS\tVERIFIED")
			for _, r := range c.List() {
				fmt.Fprintf(tw, "%s\t%s\t%s\t$%.5f\t$%.5f\t%d\t%s\n",
					r.Preset, r.Provider, r.Model, r.InputPer1K, r.OutputPer1K, r.MaxTokens, r.VerifiedAt)
			}
			return tw.Flush()
		},
	}
	pricingCmd.AddCommand(listCmd)

	staleCmd := &cobra.Command{
		Use:   "stale",
		Short: "Report whether the pricing table is out of date",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := pricing.Default()
			threshold := 90 * 24 * time.Hour
			stale := c.Stale(threshold)
			loaded := c.LoadedAt()
			fmt.Printf("Source:        %s\n", c.Source())
			fmt.Printf("Loaded at:     %s\n", loaded.Format(time.RFC3339))
			fmt.Printf("Stale (>%s old): %v\n", threshold, stale)
			if stale {
				fmt.Println()
				fmt.Println("⚠ Some rates are older than the 90-day threshold.")
				fmt.Println("  Edit internal/pricing/data/pricing.yaml and bump verified_at.")
				fmt.Println("  (See docs/PRICING.md for the full workflow.)")
				return fmt.Errorf("pricing table is stale")
			}
			return nil
		},
	}
	pricingCmd.AddCommand(staleCmd)

	refreshCmd := &cobra.Command{
		Use:   "refresh",
		Short: "Show how to refresh the pricing data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Println("To refresh the pricing table:")
			fmt.Println()
			fmt.Println("  1. Edit internal/pricing/data/pricing.yaml")
			fmt.Println("  2. Update the verified_at date for changed rows")
			fmt.Println("  3. Run: go test ./internal/pricing/  (round-trip parse check)")
			fmt.Println("  4. Commit and rebuild")
			fmt.Println()
			fmt.Println("The data file is embedded at build time via //go:embed.")
			fmt.Println("There is no runtime override path by design — pricing is")
			fmt.Println("a build-time concern, not a config-time one.")
			return nil
		},
	}
	pricingCmd.AddCommand(refreshCmd)

	root.AddCommand(pricingCmd)
}