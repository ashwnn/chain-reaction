package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/benchmark"
)

func newBenchmarkCmd() *cobra.Command {
	var privateRoot, commitmentsPath string
	cmd := &cobra.Command{Use: "benchmark", Short: "Manage controller-only benchmark v2 artifacts"}
	cmd.AddCommand(&cobra.Command{
		Use:   "initialize",
		Short: "Create private seeds and a public commitment inventory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !filepath.IsAbs(privateRoot) || !filepath.IsAbs(commitmentsPath) {
				return fmt.Errorf("--private-root and --commitments must be absolute paths")
			}
			manifest, err := benchmark.CreateEvaluationInventory(privateRoot, nil)
			if err != nil {
				return err
			}
			if err := benchmark.WritePublicCommitmentManifest(commitmentsPath, manifest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "commitments: %s\nprivate_seed_directory: %s\ninstances: %d\n", commitmentsPath, privateRoot, len(manifest.Commitments))
			return nil
		},
	})
	command := cmd.Commands()[0]
	command.Flags().StringVar(&privateRoot, "private-root", "", "Absolute controller-only directory for raw hidden seeds")
	command.Flags().StringVar(&commitmentsPath, "commitments", "", "Absolute path for new public commitment manifest")
	return cmd
}
