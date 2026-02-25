package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Namespace           string
	Kubeconfig          string
	OutputPath          string
	OutputFormat        string
	TimeBudget          time.Duration
	MaxSteps            int
	AllowListNamespaces []string
	QPS                 float32
	Burst               int
	OpenAIAPIKey        string
	OpenAIModel         string
}

type CLIOptions struct {
	Namespace           string
	Kubeconfig          string
	OutputPath          string
	OutputFormat        string
	TimeBudget          time.Duration
	MaxSteps            int
	AllowListNamespaces []string
	QPS                 float32
	Burst               int
	OpenAIAPIKey        string
	OpenAIModel         string
}

func Default() Config {
	return Config{
		Namespace:           "",
		OutputPath:          "artifacts",
		OutputFormat:        "json",
		TimeBudget:          10 * time.Minute,
		MaxSteps:            20,
		AllowListNamespaces: nil,
		QPS:                 20,
		Burst:               30,
		OpenAIModel:         "gpt-5-mini",
	}
}

func FromCLI(opts CLIOptions) Config {
	cfg := Default()

	if opts.Namespace != "" {
		cfg.Namespace = opts.Namespace
	}
	if opts.Kubeconfig != "" {
		cfg.Kubeconfig = opts.Kubeconfig
	}
	if opts.OutputPath != "" {
		cfg.OutputPath = opts.OutputPath
	}
	if opts.OutputFormat != "" {
		cfg.OutputFormat = strings.ToLower(opts.OutputFormat)
	}
	if opts.TimeBudget > 0 {
		cfg.TimeBudget = opts.TimeBudget
	}
	if opts.MaxSteps > 0 {
		cfg.MaxSteps = opts.MaxSteps
	}
	if len(opts.AllowListNamespaces) > 0 {
		cfg.AllowListNamespaces = opts.AllowListNamespaces
	}
	if opts.QPS > 0 {
		cfg.QPS = opts.QPS
	}
	if opts.Burst > 0 {
		cfg.Burst = opts.Burst
	}

	if opts.OpenAIAPIKey != "" {
		cfg.OpenAIAPIKey = opts.OpenAIAPIKey
	} else if env := os.Getenv("OPENAI_API_KEY"); env != "" {
		cfg.OpenAIAPIKey = env
	}

	if opts.OpenAIModel != "" {
		cfg.OpenAIModel = opts.OpenAIModel
	}

	return cfg
}
