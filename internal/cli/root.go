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
		Short:         "Validate Kubernetes attack chains from inside the cluster",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().StringVar(&state.opts.Kubeconfig, "kubeconfig", "", "Absolute path to kubeconfig (defaults to in-cluster, then local config)")
	cmd.PersistentFlags().StringVar(&state.opts.Namespace, "namespace", "", "Target namespace scope")
	cmd.PersistentFlags().StringVar(&state.opts.OutputPath, "output", "artifacts", "Output directory for graph and evidence")
	cmd.PersistentFlags().StringVar(&state.opts.OutputFormat, "format", "json", "Output format (json)")
	cmd.PersistentFlags().DurationVar(&state.opts.TimeBudget, "time-budget", 10*time.Minute, "Maximum runtime (example: 10m)")
	cmd.PersistentFlags().IntVar(&state.opts.MaxSteps, "max-steps", 20, "Maximum planning steps")
	cmd.PersistentFlags().StringSliceVar(&state.opts.AllowListNamespaces, "allow-namespace", nil, "Allow-list namespace (repeatable)")
	cmd.PersistentFlags().Float32Var(&state.opts.QPS, "k8s-qps", 20, "Kubernetes API QPS")
	cmd.PersistentFlags().IntVar(&state.opts.Burst, "k8s-burst", 30, "Kubernetes API burst")
	cmd.PersistentFlags().StringVar(&state.opts.OpenAIModel, "openai-model", "gpt-5-mini", "OpenAI model for planning")
	cmd.PersistentFlags().StringVar(&state.opts.OpenAIAPIKey, "openai-api-key", "", "OpenAI API key (optional, otherwise OPENAI_API_KEY env var)")

	cmd.AddCommand(newScanCmd(state))
	cmd.AddCommand(newVersionCmd(state))

	return cmd
}
