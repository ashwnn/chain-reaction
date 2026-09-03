package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/agent"
	"github.com/ashwnn/chain-reaction/internal/config"
)

func newTheoryCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "theory",
		Short: "Export the static theoretical baseline artifact",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromCLI(state.opts)
			if err != nil {
				return err
			}
			result, err := agent.ExportTheoreticalBaseline(cfg)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "run_mode: %s\n", result.RunMode)
			fmt.Fprintf(cmd.OutOrStdout(), "artifact: %s\n", result.Path)
			if result.ComparisonPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "comparison: %s\n", result.ComparisonPath)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "duration: %s\n", result.Duration)
			return nil
		},
	}
}
