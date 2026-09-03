package validation

// StepResult classifies the outcome of a validation step execution.
// These values align with the paper-aligned vocabulary in the baseline catalog.
type StepResult string

const (
	StepValidated   StepResult = "validated"
	StepTheoretical StepResult = "theoretical"
	StepFailed      StepResult = "failed"
)

// FailureReason classifies why a StepFailed outcome occurred.
// These values align with the six primary failure reasons defined in the
// baseline catalog: rbac_denied, missing_prerequisite, network_unreachable,
// guardrail_blocked, auth_failed, tool_or_runtime_error.
// Additional reasons cover tool-specific failure modes while remaining consistent
// with the taxonomy.
type FailureReason string

const (
	// FailureRBACDenied is returned when the identity lacks permission for the requested action.
	FailureRBACDenied FailureReason = "rbac_denied"

	// FailureMissingPrerequisite is returned when a required precondition for the step does not exist.
	FailureMissingPrerequisite FailureReason = "missing_prerequisite"

	// FailureNetworkUnreachable is returned when a network probe could not reach the target.
	FailureNetworkUnreachable FailureReason = "network_unreachable"

	// FailureGuardrailBlocked is returned when a guardrail (e.g., namespace allow-list) denied the action.
	FailureGuardrailBlocked FailureReason = "guardrail_blocked"

	// FailureAuthFailed is returned when authentication credentials are invalid or missing.
	FailureAuthFailed FailureReason = "auth_failed"

	// FailureToolOrRuntimeError is returned when a tool fails at runtime for a reason not covered
	// by the other specific failure reasons (e.g., I/O errors, timeouts, protocol errors).
	FailureToolOrRuntimeError FailureReason = "tool_or_runtime_error"

	// FailureUnknown is returned when the failure reason cannot be determined.
	FailureUnknown FailureReason = "unknown"

	// FailureSecretNotFound is returned when a requested Secret does not exist in the cluster.
	// Semantically equivalent to FailureMissingPrerequisite for secret-access steps.
	FailureSecretNotFound FailureReason = "secret_not_found"

	// FailureTokenExpired is returned when the mounted ServiceAccount token has expired.
	// This is distinct from FailureAuthFailed (missing/invalid token) and FailureRBACDenied
	// (permission denied): the token exists but is no longer valid for authentication.
	FailureTokenExpired FailureReason = "token_expired"
)
