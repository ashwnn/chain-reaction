package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func validationErrors(cfg Config) []string {
	err := Validate(cfg)
	if err == nil {
		return nil
	}
	if ve, ok := err.(*validationError); ok {
		var out []string
		for _, e := range ve.Unwrap() {
			out = append(out, e.Error())
		}
		return out
	}
	return []string{err.Error()}
}

func cfgWith(ms int, tb time.Duration, qps float32, burst int) Config {
	return Config{
		MaxSteps:   ms,
		TimeBudget: tb,
		QPS:        qps,
		Burst:      burst,
	}
}

// ---------------------------------------------------------------------------
// Validate: valid configs
// ---------------------------------------------------------------------------

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Fatalf("Default() should pass validation: %v", err)
	}
}

func TestValidate_ValidBoundaryValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "min valid max_steps", cfg: cfgWith(1, 5*time.Minute, 10, 20)},
		{name: "max valid max_steps", cfg: cfgWith(10000, 5*time.Minute, 10, 20)},
		{name: "min valid time_budget", cfg: cfgWith(20, 1*time.Second, 10, 20)},
		{name: "max valid time_budget", cfg: cfgWith(20, 24*time.Hour, 10, 20)},
		{name: "min valid QPS", cfg: cfgWith(20, 5*time.Minute, 0.1, 20)},
		{name: "max valid QPS", cfg: cfgWith(20, 5*time.Minute, 10000, 10000)},
		{name: "burst equals QPS", cfg: cfgWith(20, 5*time.Minute, 10, 10)},
		{name: "all optional fields nil", cfg: Config{MaxSteps: 20, TimeBudget: 5 * time.Minute, QPS: 10, Burst: 20}},
		{name: "temperature at boundaries", cfg: func() Config {
			cfg := Default()
			t0 := 0.0
			cfg.LLMTemperature = &t0
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.cfg); err != nil {
				t.Fatalf("expected valid config: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Validate: MaxSteps bounds
// ---------------------------------------------------------------------------

func TestValidate_MaxStepsZero(t *testing.T) {
	cfg := cfgWith(0, 5*time.Minute, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for max_steps=0")
	}
	if !strings.Contains(errs[0], "max_steps") {
		t.Fatalf("error should mention max_steps: %s", errs[0])
	}
}

func TestValidate_MaxStepsNegative(t *testing.T) {
	cfg := cfgWith(-1, 5*time.Minute, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for max_steps=-1")
	}
}

func TestValidate_MaxStepsTooLarge(t *testing.T) {
	cfg := cfgWith(10001, 5*time.Minute, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for max_steps=10001")
	}
}

// ---------------------------------------------------------------------------
// Validate: TimeBudget bounds
// ---------------------------------------------------------------------------

func TestValidate_TimeBudgetZero(t *testing.T) {
	cfg := cfgWith(20, 0, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for time_budget=0")
	}
	if !strings.Contains(errs[0], "time_budget") {
		t.Fatalf("error should mention time_budget: %s", errs[0])
	}
}

func TestValidate_TimeBudgetTooLarge(t *testing.T) {
	cfg := cfgWith(20, 25*time.Hour, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for time_budget=25h")
	}
}

func TestValidate_TimeBudgetJustUnderMin(t *testing.T) {
	cfg := cfgWith(20, 999*time.Millisecond, 10, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for time_budget=999ms")
	}
}

// ---------------------------------------------------------------------------
// Validate: QPS bounds
// ---------------------------------------------------------------------------

func TestValidate_QPSZero(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, 0, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for qps=0")
	}
	if !strings.Contains(errs[0], "k8s_qps") {
		t.Fatalf("error should mention k8s_qps: %s", errs[0])
	}
}

func TestValidate_QPSNegative(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, -1, 20)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for qps=-1")
	}
}

func TestValidate_QPSTooLarge(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, 10001, 10001)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for qps=10001")
	}
}

// ---------------------------------------------------------------------------
// Validate: Burst bounds and relationship to QPS
// ---------------------------------------------------------------------------

func TestValidate_BurstZero(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, 10, 0)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for burst=0")
	}
	if !strings.Contains(errs[0], "k8s_burst") {
		t.Fatalf("error should mention k8s_burst: %s", errs[0])
	}
}

func TestValidate_BurstTooLarge(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, 10, 100001)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for burst=100001")
	}
}

func TestValidate_BurstLessThanQPS(t *testing.T) {
	cfg := cfgWith(20, 5*time.Minute, 10, 5)
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for burst < qps")
	}
	if !strings.Contains(errs[0], "k8s_burst") || !strings.Contains(errs[0], "k8s_qps") {
		t.Fatalf("error should mention both burst and qps: %s", errs[0])
	}
}

// ---------------------------------------------------------------------------
// Validate: LLMTemperature bounds
// ---------------------------------------------------------------------------

func TestValidate_TemperatureNegative(t *testing.T) {
	cfg := Default()
	v := -0.1
	cfg.LLMTemperature = &v
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for temperature=-0.1")
	}
	if !strings.Contains(errs[0], "llm_temperature") {
		t.Fatalf("error should mention llm_temperature: %s", errs[0])
	}
}

func TestValidate_TemperatureTooHigh(t *testing.T) {
	cfg := Default()
	v := 2.1
	cfg.LLMTemperature = &v
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for temperature=2.1")
	}
}

func TestValidate_TemperatureNilIsOK(t *testing.T) {
	cfg := Default()
	cfg.LLMTemperature = nil
	if err := Validate(cfg); err != nil {
		t.Fatalf("nil temperature should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate: LLMMaxTokens bounds
// ---------------------------------------------------------------------------

func TestValidate_MaxTokensZeroIsOK(t *testing.T) {
	cfg := Default()
	v := 0
	cfg.LLMMaxTokens = &v
	if err := Validate(cfg); err != nil {
		t.Fatalf("max_tokens=0 should be valid as unset: %v", err)
	}
}

func TestValidate_MaxTokensNegative(t *testing.T) {
	cfg := Default()
	v := -1
	cfg.LLMMaxTokens = &v
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for max_tokens=-1")
	}
}

func TestValidate_MaxTokensTooLarge(t *testing.T) {
	cfg := Default()
	v := 100001
	cfg.LLMMaxTokens = &v
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for max_tokens=100001")
	}
}

func TestValidate_MaxTokensNilIsOK(t *testing.T) {
	cfg := Default()
	cfg.LLMMaxTokens = nil
	if err := Validate(cfg); err != nil {
		t.Fatalf("nil max_tokens should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate: RepeatedActionLimit bounds
// ---------------------------------------------------------------------------

func TestValidate_RepeatedActionLimitNegative(t *testing.T) {
	cfg := Default()
	cfg.RepeatedActionLimit = -1
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for repeated_action_limit=-1")
	}
	if !strings.Contains(errs[0], "repeated_action_limit") {
		t.Fatalf("error should mention repeated_action_limit: %s", errs[0])
	}
}

func TestValidate_RepeatedActionLimitZeroIsOK(t *testing.T) {
	cfg := Default()
	cfg.RepeatedActionLimit = 0
	if err := Validate(cfg); err != nil {
		t.Fatalf("repeated_action_limit=0 should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate: LLMProvider format
// ---------------------------------------------------------------------------

func TestValidate_LLMProviderUnknown(t *testing.T) {
	cfg := Default()
	cfg.LLMProvider = "unknown-provider"
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown provider")
	}
	if !strings.Contains(errs[0], "llm_provider") {
		t.Fatalf("error should mention llm_provider: %s", errs[0])
	}
}

func TestValidate_LLMProviderKnown(t *testing.T) {
	for _, p := range []string{"openai", "anthropic", "groq", "OPENAI", "Anthropic", "GROQ"} {
		t.Run(p, func(t *testing.T) {
			cfg := Default()
			cfg.LLMProvider = p
			if err := Validate(cfg); err != nil {
				t.Fatalf("provider %q should be valid: %v", p, err)
			}
		})
	}
}

func TestValidate_LLMProviderEmptyIsOK(t *testing.T) {
	cfg := Default()
	cfg.LLMProvider = ""
	if err := Validate(cfg); err != nil {
		t.Fatalf("empty provider should be valid (uses default): %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate: OutputFormat format
// ---------------------------------------------------------------------------

func TestValidate_OutputFormatUnknown(t *testing.T) {
	cfg := Default()
	cfg.OutputFormat = "xml"
	errs := validationErrors(cfg)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown format")
	}
	if !strings.Contains(errs[0], "format") {
		t.Fatalf("error should mention format: %s", errs[0])
	}
}

func TestValidate_OutputFormatJSONIsOK(t *testing.T) {
	cfg := Default()
	cfg.OutputFormat = "json"
	if err := Validate(cfg); err != nil {
		t.Fatalf("json format should be valid: %v", err)
	}
}

func TestValidate_OutputFormatEmptyIsOK(t *testing.T) {
	cfg := Default()
	cfg.OutputFormat = ""
	if err := Validate(cfg); err != nil {
		t.Fatalf("empty format should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate: error aggregation
// ---------------------------------------------------------------------------

func TestValidate_MultipleErrorsAggregated(t *testing.T) {
	cfg := Config{
		MaxSteps:            0,
		TimeBudget:          0,
		QPS:                 0,
		Burst:               0,
		LLMTemperature:      ptr(-0.5),
		LLMMaxTokens:        ptr(-1),
		RepeatedActionLimit: -1,
		LLMProvider:         "bad",
		OutputFormat:        "xml",
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	ve, ok := err.(*validationError)
	if !ok {
		t.Fatalf("expected *validationError, got %T", err)
	}
	unwrapped := ve.Unwrap()
	if len(unwrapped) < 5 {
		t.Fatalf("expected multiple errors aggregated, got %d: %v", len(unwrapped), unwrapped)
	}
}

// ---------------------------------------------------------------------------
// ValidateForMode: mode-specific requirements
// ---------------------------------------------------------------------------

func TestValidateForMode_ValidateRequiresAPIKey(t *testing.T) {
	cfg := Default()
	cfg.LLMAPIKey = ""
	cfg.LLMModel = "gpt-4o-mini"
	err := ValidateForMode(cfg, "validate")
	if err == nil {
		t.Fatal("expected error for missing API key in validate mode")
	}
	if !strings.Contains(err.Error(), "llm_api_key") {
		t.Fatalf("error should mention llm_api_key: %v", err)
	}
}

func TestValidateForMode_ValidateRequiresModel(t *testing.T) {
	cfg := Default()
	cfg.LLMAPIKey = "sk-test-key"
	cfg.LLMModel = ""
	err := ValidateForMode(cfg, "validate")
	if err == nil {
		t.Fatal("expected error for missing model in validate mode")
	}
	if !strings.Contains(err.Error(), "llm_model") {
		t.Fatalf("error should mention llm_model: %v", err)
	}
}

func TestValidateForMode_ValidateWithBothSetIsOK(t *testing.T) {
	cfg := Default()
	cfg.LLMAPIKey = "sk-test-key"
	cfg.LLMModel = "gpt-4o-mini"
	if err := ValidateForMode(cfg, "validate"); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
}

func TestValidateForMode_ScanDoesNotRequireLLM(t *testing.T) {
	cfg := Default()
	cfg.LLMAPIKey = ""
	cfg.LLMModel = ""
	if err := ValidateForMode(cfg, "scan"); err != nil {
		t.Fatalf("scan mode should not require LLM: %v", err)
	}
}

func TestValidateForMode_TheoryDoesNotRequireLLM(t *testing.T) {
	cfg := Default()
	cfg.LLMAPIKey = ""
	cfg.LLMModel = ""
	if err := ValidateForMode(cfg, "theory"); err != nil {
		t.Fatalf("theory mode should not require LLM: %v", err)
	}
}

func TestValidateForMode_EmptyModeIsOK(t *testing.T) {
	cfg := Default()
	if err := ValidateForMode(cfg, ""); err != nil {
		t.Fatalf("empty mode should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidationError type
// ---------------------------------------------------------------------------

func TestValidationError_Error(t *testing.T) {
	ve := newValidationError(
		errors.New("error one"),
		errors.New("error two"),
		errors.New("error three"),
	)
	msg := ve.Error()
	if !strings.Contains(msg, "error one") {
		t.Fatal("Error() should contain first error")
	}
	if !strings.Contains(msg, "error three") {
		t.Fatal("Error() should contain third error")
	}
	if strings.Count(msg, "\n") != 2 {
		t.Fatalf("Error() should join with newlines, got: %q", msg)
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	e1 := errors.New("e1")
	e2 := errors.New("e2")
	ve := newValidationError(e1, e2)
	unwrapped := ve.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(unwrapped))
	}
	if unwrapped[0] != e1 || unwrapped[1] != e2 {
		t.Fatal("Unwrap should return original errors")
	}
}

func TestValidationError_Nil(t *testing.T) {
	var ve *validationError
	if ve.Error() != "" {
		t.Fatal("nil Error() should return empty string")
	}
	if ve.Unwrap() != nil {
		t.Fatal("nil Unwrap() should return nil")
	}
	if newValidationError() != nil {
		t.Fatal("newValidationError with no errors should return nil")
	}
	if newValidationError(nil, nil) != nil {
		t.Fatal("newValidationError with only nil args should return nil")
	}
}

func TestValidationError_Is(t *testing.T) {
	e1 := errors.New("e1")
	ve := newValidationError(e1)
	if !errors.Is(ve, e1) {
		t.Fatal("errors.Is(ve, e1) should return true")
	}
	if errors.Is(ve, errors.New("other")) {
		t.Fatal("errors.Is(ve, other) should return false")
	}
}

// ---------------------------------------------------------------------------
// FromCLI integration
// ---------------------------------------------------------------------------

func TestFromCLI_InvalidConfigFailsEarly(t *testing.T) {
	path := writeConfigFixture(t, `
max_steps: 0
time_budget: 0
k8s_qps: 0
k8s_burst: 0
`)
	_, err := FromCLI(CLIOptions{ConfigPath: path})
	if err == nil {
		t.Fatal("expected validation error from FromCLI")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Fatalf("error should mention config validation: %v", err)
	}
}

func TestFromCLI_ValidConfigSucceeds(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	path := writeConfigFixture(t, `
namespace: test-ns
output: /tmp/out
format: json
time_budget: 5m
max_steps: 20
k8s_qps: 10
k8s_burst: 20
llm_provider: openai
llm_model: gpt-4o-mini
llm_temperature: 0
`)
	cfg, err := FromCLI(CLIOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("FromCLI should succeed with valid config: %v", err)
	}
	if cfg.MaxSteps != 20 {
		t.Fatalf("expected max_steps=20, got %d", cfg.MaxSteps)
	}
}

// ---------------------------------------------------------------------------
// ptr helper (replicated here for test independence)
// ---------------------------------------------------------------------------

func ptr[T any](v T) *T {
	return &v
}
