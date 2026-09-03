package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/llm"
)

const validationDebugLogName = "validate-debug.log"

type validationEventLogger struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	stream bool
}

func newValidationEventLogger(outputPath string, stream bool) (*validationEventLogger, error) {
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, fmt.Errorf("create validation output directory: %w", err)
	}

	path := filepath.Join(outputPath, validationDebugLogName)
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create validation debug log: %w", err)
	}

	return &validationEventLogger{
		path:   path,
		file:   file,
		stream: stream,
	}, nil
}

func (l *validationEventLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *validationEventLogger) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *validationEventLogger) Log(event string, fields map[string]any) {
	if l == nil || l.file == nil {
		return
	}

	line := formatValidationDebugLine(event, fields)

	l.mu.Lock()
	defer l.mu.Unlock()

	_, _ = fmt.Fprintln(l.file, line)
	if l.stream {
		_, _ = fmt.Fprintln(os.Stderr, line)
	}
}

func formatValidationDebugLine(event string, fields map[string]any) string {
	parts := []string{
		time.Now().UTC().Format(time.RFC3339Nano),
		"event=" + event,
	}

	if len(fields) == 0 {
		return strings.Join(parts, " ")
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		parts = append(parts, key+"="+formatValidationDebugValue(fields[key]))
	}

	return strings.Join(parts, " ")
}

func formatValidationDebugValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		if v == "" {
			return `""`
		}
		if strings.ContainsAny(v, " \t\n\r\"=") {
			return strconv.Quote(v)
		}
		return v
	case time.Duration:
		return v.String()
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case fmt.Stringer:
		return formatValidationDebugValue(v.String())
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		payload, err := json.Marshal(v)
		if err != nil {
			return strconv.Quote(fmt.Sprintf("%v", v))
		}
		return string(payload)
	}
}

func logValidationPreflightStart(logger *validationEventLogger, cfg config.Config, start time.Time) {
	if logger == nil {
		return
	}
	logger.Log("run_start", map[string]any{
		"llm_model":    cfg.LLMModel,
		"llm_provider": cfg.LLMProvider,
		"max_steps":    cfg.MaxSteps,
		"output_path":  cfg.OutputPath,
		"started_at":   start.UTC(),
		"time_budget":  cfg.TimeBudget,
	})
}

func logValidationPreflightError(logger *validationEventLogger, stage string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Log("preflight_error", map[string]any{
		"error": err.Error(),
		"stage": stage,
	})
}

func logValidationLoopStart(logger *validationEventLogger, cfg config.Config, availableTools []string) {
	if logger == nil {
		return
	}
	logger.Log("validation_loop_start", map[string]any{
		"available_tools": len(availableTools),
		"debug_stream":    cfg.Debug,
		"max_steps":       cfg.MaxSteps,
		"time_budget":     cfg.TimeBudget,
	})
}

func logPlannerRequestStart(logger *validationEventLogger, state *state, checker terminationChecker) {
	if logger == nil || state == nil {
		return
	}
	logger.Log("planner_request_start", map[string]any{
		"elapsed":         elapsedSince(state),
		"history_entries": len(state.History),
		"iteration":       state.Iteration + 1,
		"remaining":       remainingBudget(state, checker.Timeout),
	})
}

func logPlannerRequestComplete(logger *validationEventLogger, state *state, checker terminationChecker, action plannerAction, duration time.Duration) {
	if logger == nil || state == nil {
		return
	}

	fields := map[string]any{
		"action_type": action.ActionType,
		"duration_ms": duration.Milliseconds(),
		"elapsed":     elapsedSince(state),
		"iteration":   state.Iteration + 1,
		"remaining":   remainingBudget(state, checker.Timeout),
		"tool_name":   action.ToolName,
	}
	if len(action.Parameters) > 0 {
		fields["parameters"] = action.Parameters
	}
	if action.FinalAnswer != "" {
		fields["final_answer"] = action.FinalAnswer
	}
	if action.Thought != "" {
		fields["planner_rationale"] = compactValidationDebugText(action.Thought, 240)
	}
	for k, v := range formatLLMUsageFields(action.Usage, "llm_") {
		fields[k] = v
	}
	logger.Log("planner_request_complete", fields)

	decisionFields := map[string]any{
		"action_type": action.ActionType,
		"tool_name":   action.ToolName,
	}
	if len(action.Parameters) > 0 {
		decisionFields["parameters"] = action.Parameters
	}
	if action.FinalAnswer != "" {
		decisionFields["final_answer"] = compactValidationDebugText(action.FinalAnswer, 160)
	}
	if action.Thought != "" {
		decisionFields["planner_rationale"] = compactValidationDebugText(action.Thought, 160)
	}
	if action.Usage != nil && action.Usage.TotalTokens > 0 {
		decisionFields["llm_total_tokens"] = action.Usage.TotalTokens
	}
	logger.Log("planner_decision", decisionFields)
}

func compactValidationDebugText(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

func logPlannerRequestError(logger *validationEventLogger, state *state, checker terminationChecker, err error) {
	if logger == nil || state == nil || err == nil {
		return
	}
	logger.Log("planner_request_error", map[string]any{
		"elapsed":    elapsedSince(state),
		"error":      err.Error(),
		"error_type": classifyErrorType(err),
		"iteration":  state.Iteration + 1,
		"remaining":  remainingBudget(state, checker.Timeout),
	})
}

// classifyErrorType categorises an error returned from the planner call so that
// the debug log clearly distinguishes context-expiry (deadline exceeded), explicit
// cancellation (ctx cancelled, e.g. guardrail), from other provider errors.
func classifyErrorType(err error) string {
	if err == nil {
		return "unknown"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "context_cancelled"
	}
	// Surface common retry-layer error patterns.
	if strings.Contains(err.Error(), "retry cancelled") {
		return "context_cancelled"
	}
	if strings.Contains(err.Error(), "retry deadline expired") {
		return "context_deadline_exceeded"
	}
	if strings.Contains(err.Error(), "max retries exceeded") {
		return "provider_max_retries"
	}
	return "provider_error"
}

func logTimeBudgetWarning(logger *validationEventLogger, state *state, checker terminationChecker) {
	if logger == nil || state == nil || checker.Timeout <= 0 {
		return
	}
	if flagFromContext(state, "time_budget_warning_emitted") {
		return
	}

	remaining := remainingBudget(state, checker.Timeout)
	if remaining <= 0 {
		return
	}

	threshold := checker.Timeout / 10
	if threshold > time.Minute {
		threshold = time.Minute
	}
	if threshold < 15*time.Second {
		threshold = 15 * time.Second
	}
	if remaining > threshold {
		return
	}

	logger.Log("time_budget_warning", map[string]any{
		"elapsed":   elapsedSince(state),
		"iteration": state.Iteration + 1,
		"remaining": remaining,
		"threshold": threshold,
	})
	state.Context["time_budget_warning_emitted"] = true
}

func logToolStart(logger *validationEventLogger, state *state, checker terminationChecker, action plannerAction) {
	if logger == nil || state == nil {
		return
	}

	fields := map[string]any{
		"elapsed":   elapsedSince(state),
		"iteration": state.Iteration + 1,
		"remaining": remainingBudget(state, checker.Timeout),
		"tool_name": action.ToolName,
	}
	if namespace, ok := action.Parameters["namespace"].(string); ok && strings.TrimSpace(namespace) != "" {
		fields["namespace"] = strings.TrimSpace(namespace)
	}
	if len(action.Parameters) > 0 {
		fields["parameters"] = action.Parameters
	}
	logger.Log("tool_start", fields)
}

func logToolComplete(logger *validationEventLogger, state *state, checker terminationChecker, action plannerAction, result toolExecutionResult) {
	if logger == nil || state == nil {
		return
	}

	fields := map[string]any{
		"duration_ms": result.DurationMS,
		"elapsed":     elapsedSince(state),
		"iteration":   state.Iteration + 1,
		"outcome":     result.Outcome,
		"remaining":   remainingBudget(state, checker.Timeout),
		"tool_name":   action.ToolName,
	}
	if result.FailureReason != "" {
		fields["failure_reason"] = result.FailureReason
	}
	if namespace := extractNamespaceForMatcher(action.ToolName, action.Parameters, result.Output); namespace != "" {
		fields["namespace"] = namespace
	}
	for k, v := range formatLLMUsageFields(result.EvaluatorUsage, "evaluator_") {
		fields[k] = v
	}
	logger.Log("tool_complete", fields)
}

func logToolError(logger *validationEventLogger, state *state, checker terminationChecker, action plannerAction, err error, duration time.Duration) {
	if logger == nil || state == nil || err == nil {
		return
	}
	logger.Log("tool_error", map[string]any{
		"duration_ms": duration.Milliseconds(),
		"elapsed":     elapsedSince(state),
		"error":       err.Error(),
		"iteration":   state.Iteration + 1,
		"remaining":   remainingBudget(state, checker.Timeout),
		"tool_name":   action.ToolName,
	})
}

func logValidationTermination(logger *validationEventLogger, state *state, checker terminationChecker, reason stopReason) {
	if logger == nil || state == nil {
		return
	}

	fields := map[string]any{
		"elapsed":             elapsedSince(state),
		"iteration":           state.Iteration,
		"last_attempted_tool": stringValueFromContext(state, "last_attempted_tool"),
		"last_completed_tool": stringValueFromContext(state, "last_tool"),
		"reason":              reason,
		"remaining":           remainingBudget(state, checker.Timeout),
	}
	if activePhase := stringValueFromContext(state, "active_phase"); activePhase != "" {
		fields["active_phase"] = activePhase
	}
	logger.Log("termination", fields)
}

func elapsedSince(state *state) time.Duration {
	if state == nil || state.StartedAt.IsZero() {
		return 0
	}
	return time.Since(state.StartedAt.UTC()).Round(time.Millisecond)
}

func remainingBudget(state *state, timeout time.Duration) time.Duration {
	if state == nil || state.StartedAt.IsZero() || timeout <= 0 {
		return 0
	}
	remaining := timeout - time.Since(state.StartedAt.UTC())
	if remaining < 0 {
		return 0
	}
	return remaining.Round(time.Millisecond)
}

func stringValueFromContext(state *state, key string) string {
	if state == nil || state.Context == nil {
		return ""
	}
	value, _ := state.Context[key].(string)
	return strings.TrimSpace(value)
}

// formatLLMUsageFields returns a map of debug-friendly LLM usage fields for the
// given prefix. Fields are included only when non-zero/non-nil so the debug log
// degrades cleanly when usage or pricing data is absent.
func formatLLMUsageFields(u *llm.UsageMetadata, prefix string) map[string]any {
	if u == nil {
		return nil
	}
	fields := make(map[string]any)
	if u.InputTokens > 0 {
		fields[prefix+"input_tokens"] = u.InputTokens
	}
	if u.OutputTokens > 0 {
		fields[prefix+"output_tokens"] = u.OutputTokens
	}
	if u.TotalTokens > 0 {
		fields[prefix+"total_tokens"] = u.TotalTokens
	}
	if u.CacheRead > 0 {
		fields[prefix+"cache_read_tokens"] = u.CacheRead
	}
	if u.CacheWrite > 0 {
		fields[prefix+"cache_write_tokens"] = u.CacheWrite
	}
	if u.EstimatedCostUSD != nil {
		fields[prefix+"estimated_cost_usd"] = *u.EstimatedCostUSD
	}
	return fields
}
