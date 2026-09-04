package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/config"
)

type appState struct {
	version string
	commit  string
	date    string
	opts    config.CLIOptions
}

func NewRootCmd(version, commit, date string) *cobra.Command {
	state := &appState{version: version, commit: commit, date: date}

	cmd := &cobra.Command{
		Use:           "chain-reaction",
		Short:         "Run bounded Kubernetes validation workflows",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().StringVar(&state.opts.Kubeconfig, "kubeconfig", "", "Absolute path to kubeconfig (defaults to in-cluster, then local config)")
	cmd.PersistentFlags().StringVar(&state.opts.ConfigPath, "config", "", "Path to evaluation config YAML")
	cmd.PersistentFlags().StringVar(&state.opts.Namespace, "namespace", "", "Target namespace scope")
	cmd.PersistentFlags().StringVar(&state.opts.OutputPath, "output", "artifacts", "Output directory for graph and evidence")
	cmd.PersistentFlags().StringVar(&state.opts.OutputFormat, "format", "json", "Output format (json)")
	cmd.PersistentFlags().DurationVar(&state.opts.TimeBudget, "time-budget", 5*time.Minute, "Maximum runtime (example: 5m)")
	cmd.PersistentFlags().StringSliceVar(&state.opts.AllowListNamespaces, "allow-namespace", nil, "Allow-list namespace (repeatable)")
	cmd.PersistentFlags().Float32Var(&state.opts.QPS, "k8s-qps", 10, "Kubernetes API QPS")
	cmd.PersistentFlags().IntVar(&state.opts.Burst, "k8s-burst", 20, "Kubernetes API burst")
	cmd.PersistentFlags().StringVar(&state.opts.LLMProvider, "llm-provider", "", "LLM provider for validation planning (scan remains deterministic)")
	cmd.PersistentFlags().StringVar((*string)(&state.opts.PlannerMode), "planner-mode", string(config.PlannerModeBlind), "Planner mode: blind, goat_hinted, or scripted_oracle")
	cmd.PersistentFlags().StringVar(&state.opts.LLMAPIKey, "llm-api-key", "", "LLM API key for validation planning (not used by scan)")
	cmd.PersistentFlags().StringVar(&state.opts.LLMBaseURL, "llm-base-url", "", "LLM base URL for validation planning (not used by scan)")
	cmd.PersistentFlags().StringVar(&state.opts.LLMModel, "llm-model", "", "LLM model for validation planning (not used by scan)")
	cmd.PersistentFlags().Float64Var(&state.opts.LLMTemperature, "llm-temperature", -1.0, "LLM temperature (0.0 to 1.0) (-1.0 to leave unset/provider default)")
	cmd.PersistentFlags().IntVar(&state.opts.LLMMaxTokens, "llm-max-tokens", 0, "LLM max tokens (0 to leave unset/provider default)")

	cmd.AddCommand(newScanCmd(state))
	cmd.AddCommand(newTheoryCmd(state))
	cmd.AddCommand(newValidateCmd(state))
	cmd.AddCommand(newAnalyzeCmd(state))
	cmd.AddCommand(newCompareCmd(state))
	cmd.AddCommand(newBenchmarkCmd())
	cmd.AddCommand(newVersionCmd(state))

	return cmd
}
