package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// validationError holds multiple validation errors.
type validationError struct {
	errs []error
}

// newValidationError creates a validationError from a list of errors.
// Empty errors are silently dropped. Returns nil if the resulting error list
// is empty, so Validate returns nil for a valid config.
func newValidationError(errs ...error) *validationError {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	// Use len() check: variadic evaluation may produce []error{} (non-nil, empty)
	// even when all inputs are nil. We must return nil for an empty error list.
	if len(filtered) == 0 {
		return nil
	}
	return &validationError{errs: filtered}
}

func (ve *validationError) Error() string {
	if ve == nil {
		return ""
	}
	if len(ve.errs) == 0 {
		// Should not happen in practice (newValidationError returns nil instead),
		// but defensively return a non-empty string so err != nil is meaningful.
		return "validation error with no errors"
	}
	var b strings.Builder
	for i, err := range ve.errs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

// Unwrap returns the underlying errors for errors.Is/errors.Join compatibility.
func (ve *validationError) Unwrap() []error {
	if ve == nil {
		return nil
	}
	return ve.errs
}

// As finds the first error in ve's error list that matches target and,
// if one is found, sets target to that error value and returns true.
// It also handles the case where errors.As is called with &ve (where ve is
// *validationError) by checking if target is a **validationError and
// setting the caller's *validationError variable to ve. This implements
// errors.As behavior for multi-error types.
func (ve *validationError) As(target any) bool {
	// When errors.As(err, &ve) is called with var ve *validationError,
	// target is interface{} holding **validationError (&ve).
	// We need to set the caller's ve variable to this ve.
	if vep, ok := target.(**validationError); ok {
		// *vep is the caller's *validationError variable. Set it to this ve.
		*vep = ve
		return true
	}
	// Recurse into each underlying error for nested multi-errors.
	for _, err := range ve.errs {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// Validate checks that cfg contains valid values for all numeric bounds,
// format fields, and structural invariants. It returns nil if the config
// is valid, or a validationError listing all problems if it is not.
//
// Validate does NOT check mode-specific requirements (e.g., that LLMAPIKey
// is set for validate mode). Use ValidateForMode for that.
func Validate(cfg Config) error {
	// Accumulate errors into a nil slice so newValidationError receives nil
	// when all validators pass. This avoids the case where variadic evaluation
	// produces a non-nil empty []error{} that bypasses the nil check.
	var errs []error
	errs = append(errs, validateMaxSteps(cfg.MaxSteps))
	errs = append(errs, validateTimeBudget(cfg.TimeBudget))
	errs = append(errs, validateQPS(cfg.QPS))
	errs = append(errs, validateBurst(cfg.Burst, cfg.QPS))
	errs = append(errs, validateTemperature(cfg.LLMTemperature))
	errs = append(errs, validateMaxTokens(cfg.LLMMaxTokens))
	errs = append(errs, validateRepeatedActionLimit(cfg.RepeatedActionLimit))
	errs = append(errs, validateLLMProvider(cfg.LLMProvider))
	errs = append(errs, validatePlannerMode(cfg.PlannerMode))
	errs = append(errs, validateOutputFormat(cfg.OutputFormat))
	ve := newValidationError(errs...)
	if ve == nil {
		return nil
	}
	return ve
}

// ValidateForMode checks mode-specific requirements in addition to the
// generic validation performed by Validate. mode should be one of:
// "scan", "theory", "validate", or "" for no mode-specific checks.
//
// For the "validate" mode, this requires that LLMAPIKey and LLMModel are set,
// since the validation loop requires an LLM provider.
func ValidateForMode(cfg Config, mode string) error {
	var errs []error

	// Always run generic validation first.
	if err := Validate(cfg); err != nil {
		errs = append(errs, err)
	}

	// Mode-specific checks.
	switch strings.ToLower(mode) {
	case "validate":
		if cfg.LLMAPIKey == "" {
			errs = append(errs, errors.New("llm_api_key: is required for validate mode (set via --llm-api-key flag, config file, provider-specific environment variable, or LLM_API_KEY)"))
		}
		if cfg.LLMModel == "" {
			errs = append(errs, errors.New("llm_model: is required for validate mode (set via --llm-model flag or config file)"))
		}
	case "scan", "theory":
		// These modes do not require an LLM.
		// No additional mode-specific requirements.
	}

	ve := newValidationError(errs...)
	if ve == nil {
		return nil
	}
	return ve
}

// --- Individual field validators ---

func validateMaxSteps(steps int) error {
	if steps < 1 || steps > 10000 {
		return fmt.Errorf("max_steps: must be between 1 and 10000, got %d", steps)
	}
	return nil
}

func validateTimeBudget(tb time.Duration) error {
	minDuration := time.Second
	maxDuration := 24 * time.Hour
	if tb < minDuration || tb > maxDuration {
		return fmt.Errorf("time_budget: must be between 1s and 24h, got %s", tb)
	}
	return nil
}

func validateQPS(qps float32) error {
	if qps <= 0 || qps > 10000 {
		return fmt.Errorf("k8s_qps: must be between 0.0 (exclusive) and 10000, got %.2f", qps)
	}
	return nil
}

func validateBurst(burst int, qps float32) error {
	if burst < 1 || burst > 100000 {
		return fmt.Errorf("k8s_burst: must be between 1 and 100000, got %d", burst)
	}
	if float32(burst) < qps {
		return fmt.Errorf("k8s_burst (%d) must be >= k8s_qps (%.2f)", burst, qps)
	}
	return nil
}

func validateTemperature(temp *float64) error {
	if temp == nil {
		return nil
	}
	if *temp < 0 || *temp > 2.0 {
		return fmt.Errorf("llm_temperature: must be between 0.0 and 2.0, got %.2f", *temp)
	}
	return nil
}

func validateMaxTokens(tokens *int) error {
	if tokens == nil {
		return nil
	}
	if *tokens == 0 {
		return nil
	}
	if *tokens < 0 || *tokens > 100000 {
		return fmt.Errorf("llm_max_tokens: must be 0 (unset) or between 1 and 100000, got %d", *tokens)
	}
	return nil
}

func validateRepeatedActionLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("repeated_action_limit: must be non-negative, got %d", limit)
	}
	return nil
}

var knownProviders = []string{"openai", "anthropic", "groq"}

func validateLLMProvider(provider string) error {
	if provider == "" {
		return nil // Provider is optional; default is applied by Default()
	}
	if !slices.Contains(knownProviders, strings.ToLower(provider)) {
		return fmt.Errorf("llm_provider: unknown provider %q, expected one of: %s", provider, strings.Join(knownProviders, ", "))
	}
	return nil
}

func validatePlannerMode(mode PlannerMode) error {
	switch mode {
	case "", PlannerModeBlind, PlannerModeGoatHinted, PlannerModeScriptedOracle:
		return nil
	default:
		return fmt.Errorf("planner_mode: unknown mode %q, expected blind, goat_hinted, or scripted_oracle", mode)
	}
}
func validateOutputFormat(format string) error {
	if format == "" {
		return nil
	}
	if format != "json" {
		return fmt.Errorf("format: unknown format %q, expected json", format)
	}
	return nil
}
