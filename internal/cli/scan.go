package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/agent"
	"github.com/ashwnn/chain-reaction/internal/config"
)

func newScanCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Run the current bounded baseline discovery pass",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromCLI(state.opts)
			if err != nil {
				return err
			}
			result, err := agent.Run(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "run_mode: %s\n", result.RunMode)
			fmt.Fprintf(cmd.OutOrStdout(), "graph: %s\n", result.GraphPath)
			fmt.Fprintf(cmd.OutOrStdout(), "evidence: %s\n", result.EvidencePath)
			fmt.Fprintf(cmd.OutOrStdout(), "summary: %s\n", result.SummaryPath)
			if result.ComparisonPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "comparison: %s\n", result.ComparisonPath)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "duration: %s\n", result.Duration)
			return nil
		},
	}
}
