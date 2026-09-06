package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/evidence"
)

func newVerifyCmd() *cobra.Command {
	var evidencePath string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify retained evidence artifacts",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "evidence",
		Short: "Verify an evidence JSONL hash chain",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if evidencePath == "" {
				return fmt.Errorf("--evidence is required")
			}
			path, err := filepath.Abs(evidencePath)
			if err != nil {
				return fmt.Errorf("resolve evidence path: %w", err)
			}
			last, err := evidence.VerifyEvidenceLog(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "evidence_chain_valid: true\nrecords: %d\nterminal_hash: %s\n", last.Sequence, last.Hash)
			return nil
		},
	})
	command := cmd.Commands()[0]
	command.Flags().StringVar(&evidencePath, "evidence", "", "Path to evidence.jsonl")
	return cmd
}
