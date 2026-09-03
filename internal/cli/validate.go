package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/agent"
	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/llm"
	"github.com/ashwnn/chain-reaction/internal/metrics"
)

func writeLLMUsageSummary(w io.Writer, usage *metrics.MetricsLLMUsage) {
	if usage == nil {
		return
	}
	fmt.Fprintf(w, "llm_usage:\n")
	fmt.Fprintf(w, "  input_tokens: %d\n", usage.InputTokens)
	fmt.Fprintf(w, "  output_tokens: %d\n", usage.OutputTokens)
	fmt.Fprintf(w, "  total_tokens: %d\n", usage.TotalTokens)
	if usage.CacheReadTokens > 0 {
		fmt.Fprintf(w, "  cache_read_tokens: %d\n", usage.CacheReadTokens)
	}
	if usage.CacheWriteTokens != nil {
		fmt.Fprintf(w, "  cache_write_tokens: %d\n", *usage.CacheWriteTokens)
	}
	if usage.CacheEfficiency != nil {
		fmt.Fprintf(w, "  cache_efficiency: %.1f%%\n", *usage.CacheEfficiency*100)
	}
	if usage.EstimatedCostUSD != nil {
		fmt.Fprintf(w, "  estimated_cost_usd: $%.6f\n", *usage.EstimatedCostUSD)
	}
	if usage.EstimatedCostUSD == nil {
		fmt.Fprintf(w, "  cost_note: pricing unavailable for provider/model (see validation-metrics.json for token totals)\n")
	}
}

func newValidateCmd(state *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run an evidence-verifiable Kubernetes validation workflow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromCLI(state.opts)
			if err != nil {
				return err
			}

			// Validate mode-specific requirements (requires LLM API key and model).
			if err := config.ValidateForMode(cfg, "validate"); err != nil {
				return err
			}

			// Connect config parameters to the LLM provider to ensure valid setup
			providerCfg := llm.ProviderConfig{
				Provider:    cfg.LLMProvider,
				BaseURL:     cfg.LLMBaseURL,
				APIKey:      cfg.LLMAPIKey,
				Model:       cfg.LLMModel,
				Temperature: cfg.LLMTemperature,
				MaxTokens:   cfg.LLMMaxTokens,
			}
			if _, err := llm.NewProvider(providerCfg); err != nil {
				return fmt.Errorf("invalid llm configuration: %w", err)
			}

			result, err := agent.Validate(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "run_mode: %s\n", result.RunMode)
			fmt.Fprintf(cmd.OutOrStdout(), "termination_reason: %s\n", result.TerminationReason)
			fmt.Fprintf(cmd.OutOrStdout(), "steps: %d\n", result.Steps)
			if result.FinalAnswer != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "final_answer: %s\n", result.FinalAnswer)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "graph: %s\n", result.GraphPath)
			fmt.Fprintf(cmd.OutOrStdout(), "evidence: %s\n", result.EvidencePath)
			if result.DebugLogPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "debug_log: %s\n", result.DebugLogPath)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "duration: %s\n", result.Duration)

			writeLLMUsageSummary(cmd.OutOrStdout(), result.LLMUsage)
			return nil
		},
	}

	cmd.Flags().BoolVar(&state.opts.Debug, "debug", false, "Emit live validation progress logs and persist a debug log artifact (validate only)")
	cmd.Flags().IntVar(&state.opts.MaxSteps, "max-steps", config.Default().MaxSteps, "Maximum validation steps before stopping (validate only)")
	cmd.Flags().BoolVar(&state.opts.StepOutcomeEvaluator, "step-outcome-evaluator", false, "Enable the step-outcome evaluator (experimental; incurs additional LLM cost per validated step)")
	return cmd
}
