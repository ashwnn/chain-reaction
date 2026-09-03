package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", state.version)
			fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", state.commit)
			fmt.Fprintf(cmd.OutOrStdout(), "date: %s\n", state.date)
		},
	}
}
