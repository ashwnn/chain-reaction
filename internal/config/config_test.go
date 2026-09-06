package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFileParsesEvaluationConfig(t *testing.T) {
	path := writeConfigFixture(t, `
namespace: chain-reaction
output: artifacts/scenario-runs
format: json
debug: true
time_budget: 2m30s
max_steps: 17
allow_namespaces:
  - chain-reaction
  - default
k8s_qps: 7
k8s_burst: 14
llm_provider: openai
llm_model: gpt-4o-mini
llm_temperature: 0
llm_max_tokens: 256
repeated_action_limit: 3
step_outcome_evaluator: true
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	if cfg.Namespace != "chain-reaction" {
		t.Fatalf("expected namespace chain-reaction, got %q", cfg.Namespace)
	}
	if cfg.OutputPath != "artifacts/scenario-runs" {
		t.Fatalf("expected output path artifacts/scenario-runs, got %q", cfg.OutputPath)
	}
	if !cfg.Debug {
		t.Fatal("expected debug to be true")
	}
	if cfg.TimeBudget != 150*time.Second {
		t.Fatalf("expected time budget 2m30s, got %s", cfg.TimeBudget)
	}
	if cfg.MaxSteps != 17 {
		t.Fatalf("expected max steps 17, got %d", cfg.MaxSteps)
	}
	if len(cfg.AllowListNamespaces) != 2 || cfg.AllowListNamespaces[1] != "default" {
		t.Fatalf("unexpected allow namespaces %#v", cfg.AllowListNamespaces)
	}
	if cfg.QPS != 7 {
		t.Fatalf("expected qps 7, got %v", cfg.QPS)
	}
	if cfg.Burst != 14 {
		t.Fatalf("expected burst 14, got %d", cfg.Burst)
	}
	if cfg.LLMProvider != "openai" {
		t.Fatalf("expected llm provider openai, got %q", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Fatalf("expected llm model gpt-4o-mini, got %q", cfg.LLMModel)
	}
	if cfg.LLMTemperature == nil || *cfg.LLMTemperature != 0 {
		t.Fatalf("expected llm temperature 0, got %#v", cfg.LLMTemperature)
	}
	if cfg.LLMMaxTokens == nil || *cfg.LLMMaxTokens != 256 {
		t.Fatalf("expected llm max tokens 256, got %#v", cfg.LLMMaxTokens)
	}
	if cfg.RepeatedActionLimit != 3 {
		t.Fatalf("expected repeated action limit 3, got %d", cfg.RepeatedActionLimit)
	}
	if !cfg.StepOutcomeEvaluator {
		t.Fatal("expected step_outcome_evaluator to be true")
	}
}

func TestLoadFileAllowsZeroMaxTokensAsUnset(t *testing.T) {
	path := writeConfigFixture(t, `
llm_provider: openai
llm_model: gpt-4o-mini
llm_max_tokens: 0
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.LLMMaxTokens == nil || *cfg.LLMMaxTokens != 0 {
		t.Fatalf("expected llm max tokens 0, got %#v", cfg.LLMMaxTokens)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("loaded config with llm_max_tokens=0 should validate: %v", err)
	}
}

func TestLoadFileRejectsAPIKey(t *testing.T) {
	path := writeConfigFixture(t, "llm_api_key: unsafe\n")
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected llm_api_key to be rejected")
	}
}

func TestFromCLIRespectsConfigAndFlagPrecedence(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")

	path := writeConfigFixture(t, `
namespace: from-file
output: artifacts/from-file
debug: true
time_budget: 3m
max_steps: 11
llm_provider: openai
llm_model: from-file-model
`)

	cfg, err := FromCLI(CLIOptions{
		ConfigPath: path,
		Namespace:  "from-flag",
		OutputPath: "artifacts/from-flag",
		MaxSteps:   25,
		LLMModel:   "from-flag-model",
	})
	if err != nil {
		t.Fatalf("FromCLI returned error: %v", err)
	}

	if cfg.Namespace != "from-flag" {
		t.Fatalf("expected flag namespace override, got %q", cfg.Namespace)
	}
	if cfg.OutputPath != "artifacts/from-flag" {
		t.Fatalf("expected flag output override, got %q", cfg.OutputPath)
	}
	if cfg.MaxSteps != 25 {
		t.Fatalf("expected flag max steps override, got %d", cfg.MaxSteps)
	}
	if cfg.LLMModel != "from-flag-model" {
		t.Fatalf("expected flag llm model override, got %q", cfg.LLMModel)
	}
	if cfg.TimeBudget != 3*time.Minute {
		t.Fatalf("expected file time budget 3m, got %s", cfg.TimeBudget)
	}
	if !cfg.Debug {
		t.Fatal("expected debug true from file")
	}
	if cfg.LLMAPIKey != "env-openai-key" {
		t.Fatalf("expected env fallback key, got %q", cfg.LLMAPIKey)
	}
}

func TestFromCLIRejectsInvalidConfigDuration(t *testing.T) {
	path := writeConfigFixture(t, `
time_budget: definitely-not-a-duration
`)

	_, err := FromCLI(CLIOptions{ConfigPath: path})
	if err == nil {
		t.Fatal("expected invalid config error")
	}
}

func TestFromCLIUsesGenericLLMAPIKeyFallback(t *testing.T) {
	t.Setenv("LLM_API_KEY", "env-generic-key")

	path := writeConfigFixture(t, `
llm_provider: openai
llm_model: test-model
`)

	cfg, err := FromCLI(CLIOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("FromCLI returned error: %v", err)
	}
	if cfg.LLMAPIKey != "env-generic-key" {
		t.Fatalf("expected generic env fallback key, got %q", cfg.LLMAPIKey)
	}
}

func TestFromCLIProviderSpecificEnvBeatsGenericFallback(t *testing.T) {
	t.Setenv("LLM_API_KEY", "env-generic-key")
	t.Setenv("OPENAI_API_KEY", "env-openai-key")

	path := writeConfigFixture(t, `
llm_provider: openai
llm_model: test-model
`)

	cfg, err := FromCLI(CLIOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("FromCLI returned error: %v", err)
	}
	if cfg.LLMAPIKey != "env-openai-key" {
		t.Fatalf("expected provider-specific env key, got %q", cfg.LLMAPIKey)
	}
}

func TestStepOutcomeEvaluatorPrecedence(t *testing.T) {
	tests := []struct {
		name           string
		configValue    *bool // nil means omitted from config
		cliValue       bool
		expectedResult bool
	}{
		{
			name:           "file-config true + CLI absent => true",
			configValue:    boolPtr(true),
			cliValue:       false,
			expectedResult: true,
		},
		{
			name:           "file-config false + CLI enabled => true",
			configValue:    boolPtr(false),
			cliValue:       true,
			expectedResult: true,
		},
		{
			name:           "file-config false + CLI absent => false",
			configValue:    boolPtr(false),
			cliValue:       false,
			expectedResult: false,
		},
		{
			name:           "file-config true + CLI enabled => true",
			configValue:    boolPtr(true),
			cliValue:       true,
			expectedResult: true,
		},
		{
			name:           "config omitted + CLI absent => false (default)",
			configValue:    nil,
			cliValue:       false,
			expectedResult: false,
		},
		{
			name:           "config omitted + CLI enabled => true",
			configValue:    nil,
			cliValue:       true,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build config content dynamically
			configContent := "llm_provider: openai\nllm_model: test-model\n"
			if tt.configValue != nil {
				configContent += fmt.Sprintf("step_outcome_evaluator: %v\n", *tt.configValue)
			}

			path := writeConfigFixture(t, configContent)

			cfg, err := FromCLI(CLIOptions{
				ConfigPath:           path,
				StepOutcomeEvaluator: tt.cliValue,
			})
			if err != nil {
				t.Fatalf("FromCLI returned error: %v", err)
			}

			if cfg.StepOutcomeEvaluator != tt.expectedResult {
				t.Fatalf("expected StepOutcomeEvaluator=%v, got %v", tt.expectedResult, cfg.StepOutcomeEvaluator)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func writeConfigFixture(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "evaluation.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}
