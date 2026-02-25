package validation

type StepResult string

const (
	StepValidated   StepResult = "validated"
	StepTheoretical StepResult = "theoretical"
	StepFailed      StepResult = "failed"
)
