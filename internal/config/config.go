package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type PlannerMode string

const (
	PlannerModeBlind          PlannerMode = "blind"
	PlannerModeGoatHinted     PlannerMode = "goat_hinted"
	PlannerModeScriptedOracle PlannerMode = "scripted_oracle"
)

type Config struct {
	Namespace            string
	Kubeconfig           string
	OutputPath           string
	OutputFormat         string
	Debug                bool
	TimeBudget           time.Duration
	MaxSteps             int
	AllowListNamespaces  []string
	QPS                  float32
	Burst                int
	LLMProvider          string
	PlannerMode          PlannerMode
	LLMAPIKey            string
	LLMBaseURL           string
	LLMModel             string
	LLMTemperature       *float64
	LLMMaxTokens         *int
	RepeatedActionLimit  int
	StepOutcomeEvaluator bool
}

type CLIOptions struct {
	ConfigPath           string
	Namespace            string
	Kubeconfig           string
	OutputPath           string
	OutputFormat         string
	Debug                bool
	TimeBudget           time.Duration
	MaxSteps             int
	AllowListNamespaces  []string
	QPS                  float32
	Burst                int
	LLMProvider          string
	PlannerMode          PlannerMode
	LLMBaseURL           string
	LLMModel             string
	LLMTemperature       float64 // Use -1 to indicate unset
	LLMMaxTokens         int     // Use 0 to indicate unset
	StepOutcomeEvaluator bool
}

type fileOptions struct {
	Namespace            string      `yaml:"namespace"`
	Kubeconfig           string      `yaml:"kubeconfig"`
	OutputPath           string      `yaml:"output"`
	OutputFormat         string      `yaml:"format"`
	Debug                *bool       `yaml:"debug"`
	TimeBudget           string      `yaml:"time_budget"`
	MaxSteps             *int        `yaml:"max_steps"`
	AllowListNamespaces  []string    `yaml:"allow_namespaces"`
	QPS                  *float32    `yaml:"k8s_qps"`
	Burst                *int        `yaml:"k8s_burst"`
	LLMProvider          string      `yaml:"llm_provider"`
	PlannerMode          PlannerMode `yaml:"planner_mode"`
	LLMBaseURL           string      `yaml:"llm_base_url"`
	LLMModel             string      `yaml:"llm_model"`
	LLMTemperature       *float64    `yaml:"llm_temperature"`
	LLMMaxTokens         *int        `yaml:"llm_max_tokens"`
	RepeatedActionLimit  *int        `yaml:"repeated_action_limit"`
	StepOutcomeEvaluator *bool       `yaml:"step_outcome_evaluator"`
}

func Default() Config {
	return Config{
		Namespace:            "",
		OutputPath:           "artifacts",
		OutputFormat:         "json",
		TimeBudget:           5 * time.Minute,
		MaxSteps:             20,
		AllowListNamespaces:  nil,
		QPS:                  10,
		Burst:                20,
		LLMProvider:          "openai",
		PlannerMode:          PlannerModeBlind,
		StepOutcomeEvaluator: false,
	}
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var options fileOptions
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&options); err != nil {
		return Config{}, fmt.Errorf("decode config file %q: %w", path, err)
	}

	cfg := Default()
	if err := applyFileOptions(&cfg, options); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}
	return cfg, nil
}

func FromCLI(opts CLIOptions) (Config, error) {
	cfg := Default()

	if opts.ConfigPath != "" {
		fileCfg, err := LoadFile(opts.ConfigPath)
		if err != nil {
			return Config{}, err
		}
		cfg = fileCfg
	}

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
	cfg.Debug = opts.Debug || cfg.Debug
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
	if opts.LLMProvider != "" {
		cfg.LLMProvider = strings.ToLower(opts.LLMProvider)
	}
	if opts.PlannerMode != "" {
		cfg.PlannerMode = opts.PlannerMode
	}
	if opts.LLMBaseURL != "" {
		cfg.LLMBaseURL = opts.LLMBaseURL
	}
	if opts.LLMModel != "" {
		cfg.LLMModel = opts.LLMModel
	}
	if opts.LLMTemperature >= 0 {
		temp := opts.LLMTemperature
		cfg.LLMTemperature = &temp
	}
	if opts.LLMMaxTokens > 0 {
		tokens := opts.LLMMaxTokens
		cfg.LLMMaxTokens = &tokens
	}

	cfg.LLMAPIKey = resolveLLMAPIKeyFromEnv(cfg.LLMProvider)

	// CLI flag takes explicit precedence; file-config opt-in is preserved when CLI flag is absent.
	cfg.StepOutcomeEvaluator = opts.StepOutcomeEvaluator || cfg.StepOutcomeEvaluator

	// Validate the fully resolved config before returning.
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func applyFileOptions(cfg *Config, options fileOptions) error {
	if cfg == nil {
		return nil
	}

	if options.Namespace != "" {
		cfg.Namespace = options.Namespace
	}
	if options.Kubeconfig != "" {
		cfg.Kubeconfig = options.Kubeconfig
	}
	if options.OutputPath != "" {
		cfg.OutputPath = options.OutputPath
	}
	if options.OutputFormat != "" {
		cfg.OutputFormat = strings.ToLower(options.OutputFormat)
	}
	if options.Debug != nil {
		cfg.Debug = *options.Debug
	}
	if options.TimeBudget != "" {
		duration, err := time.ParseDuration(options.TimeBudget)
		if err != nil {
			return fmt.Errorf("time_budget: %w", err)
		}
		cfg.TimeBudget = duration
	}
	if options.MaxSteps != nil {
		cfg.MaxSteps = *options.MaxSteps
	}
	if options.AllowListNamespaces != nil {
		cfg.AllowListNamespaces = options.AllowListNamespaces
	}
	if options.QPS != nil {
		cfg.QPS = *options.QPS
	}
	if options.Burst != nil {
		cfg.Burst = *options.Burst
	}
	if options.LLMProvider != "" {
		cfg.LLMProvider = strings.ToLower(options.LLMProvider)
	}
	if options.PlannerMode != "" {
		cfg.PlannerMode = options.PlannerMode
	}
	if options.LLMBaseURL != "" {
		cfg.LLMBaseURL = options.LLMBaseURL
	}
	if options.LLMModel != "" {
		cfg.LLMModel = options.LLMModel
	}
	if options.LLMTemperature != nil {
		temp := *options.LLMTemperature
		cfg.LLMTemperature = &temp
	}
	if options.LLMMaxTokens != nil {
		tokens := *options.LLMMaxTokens
		cfg.LLMMaxTokens = &tokens
	}
	if options.RepeatedActionLimit != nil {
		cfg.RepeatedActionLimit = *options.RepeatedActionLimit
	}
	if options.StepOutcomeEvaluator != nil {
		cfg.StepOutcomeEvaluator = *options.StepOutcomeEvaluator
	}

	return nil
}

func resolveLLMAPIKeyFromEnv(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}
	case "groq":
		if key := os.Getenv("GROQ_API_KEY"); key != "" {
			return key
		}
	default:
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return key
		}
	}
	return os.Getenv("LLM_API_KEY")
}
