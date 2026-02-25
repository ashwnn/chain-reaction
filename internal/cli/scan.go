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
		Short: "Run one bounded attack-chain validation pass",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCLI(state.opts)
			result, err := agent.Run(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "tool: %s\n", result.ToolUsed)
			fmt.Fprintf(cmd.OutOrStdout(), "graph: %s\n", result.GraphPath)
			fmt.Fprintf(cmd.OutOrStdout(), "evidence: %s\n", result.EvidencePath)
			fmt.Fprintf(cmd.OutOrStdout(), "duration: %s\n", result.Duration)
			return nil
		},
	}
}
