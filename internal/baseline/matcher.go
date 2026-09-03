package baseline

import (
	"time"
)

// TraceEntry is the matcher's view of a single tool execution from the
// validation loop trace. The caller converts agent.ToolExecutionResult
// entries into this type before passing to MatchSteps.
type TraceEntry struct {
	// ToolName is the fully qualified tool name (e.g., "validation.check_token").
	ToolName string
	// Outcome is the validation taxonomy result: "validated", "failed", or "".
	Outcome string
	// Timestamp is when the tool execution started.
	Timestamp time.Time
	// Namespace is the namespace context of this tool execution, extracted from
	// Input["namespace"] by the caller. Empty string means no namespace was
	// specified or the tool doesn't operate in a namespace context.
	Namespace string
}

// MatcherInput carries all data needed for the post-hoc scenario matcher.
type MatcherInput struct {
	// TraceEntries are the tool execution results from the validation run, in
	// chronological order.
	TraceEntries []TraceEntry
	// AgentNamespace is the namespace the agent is running in (cfg.Namespace).
	// When empty, cross-namespace checks for KG-005 cannot be performed honestly
	// and KG-005's chain cannot validate.
	AgentNamespace string
	// RunStartedAt is the time the validation run began (for TTC computation).
	RunStartedAt time.Time
}

// MatcherOutput contains the results of matching trace entries against the
// step-chain catalog.
type MatcherOutput struct {
	// Families holds per-family chain-matching results.
	Families []ChainResult `json:"families"`
	// ValidatedChainCount is the number of families whose chains were fully validated.
	ValidatedChainCount int `json:"validated_chain_count"`
	// TotalFamilies is the locked SC denominator (always 5).
	TotalFamilies int `json:"total_families"`
	// ScenarioRate is ValidatedChainCount / TotalFamilies, or nil when
	// ValidatedChainCount is 0. This is the paper's SC metric.
	ScenarioRate *float64 `json:"scenario_rate"`
	// ValidatedSteps is the total count of individually validated steps across all
	// families.
	ValidatedSteps int `json:"validated_steps"`
	// TotalSteps is the total count of catalog steps across all families.
	TotalSteps int `json:"total_steps"`
	// CatalogStepCoverage is ValidatedSteps / TotalSteps, or nil when TotalSteps
	// is 0. This is the headline catalog-step coverage metric.
	CatalogStepCoverage *float64 `json:"catalog_step_coverage"`
	// AttemptedSteps is the total count of steps that had at least one trace entry
	// with a matching tool (regardless of outcome).
	AttemptedSteps int `json:"attempted_steps"`
	// AttemptedStepSuccessRate is ValidatedSteps / AttemptedSteps, or nil when
	// AttemptedSteps is 0. This is an execution-efficiency metric, not catalog
	// coverage.
	AttemptedStepSuccessRate *float64 `json:"attempted_step_success_rate"`
	// TimeToFirstChain is the duration from RunStartedAt to the timestamp of the
	// trace entry that completed the first fully-validated chain. Nil when no
	// chains validated. This is the paper's TTC metric.
	TimeToFirstChain *time.Duration `json:"time_to_first_chain"`
}

// ChainResult holds the matching outcome for one scenario family.
type ChainResult struct {
	// FamilyID is the catalog family identifier (e.g., "KG-001").
	FamilyID string `json:"family_id"`
	// FamilyName is the human-readable name.
	FamilyName string `json:"family_name"`
	// TotalSteps is the number of steps in this family's minimum chain.
	TotalSteps int `json:"total_steps"`
	// ValidatedSteps is the number of steps that matched successfully.
	ValidatedSteps int `json:"validated_steps"`
	// ChainValidated is true when every step in the chain matched in sequence.
	ChainValidated bool `json:"chain_validated"`
	// Steps holds per-step matching results.
	Steps []StepResult `json:"steps"`
	// CompletionTime is the duration from RunStartedAt to the timestamp of the
	// trace entry that completed this chain's final step. Nil when the chain
	// did not fully validate.
	CompletionTime *time.Duration `json:"completion_time,omitempty"`
}

// StepResult holds the matching outcome for one step within a family.
type StepResult struct {
	// StepID is the catalog step identifier (e.g., "KG-001-S1").
	StepID string `json:"step_id"`
	// Attempted is true when at least one trace entry targeted this step
	// (tool name matched, regardless of outcome).
	Attempted bool `json:"attempted"`
	// Matched is true when a validated trace entry satisfied this step's
	// criteria including prerequisites.
	Matched bool `json:"matched"`
	// MatchIndex is the trace entry index that matched, or -1.
	MatchIndex int `json:"match_index"`
	// FailReason describes why the step did not match, when attempted but not
	// matched. Empty when matched or not attempted.
	FailReason string `json:"fail_reason,omitempty"`
}

// MatchSteps performs post-hoc matching of trace entries against the default
// step-chain catalog. It returns a MatcherOutput with per-family results and
// aggregate metrics (ScenarioRate, CatalogStepCoverage,
// AttemptedStepSuccessRate, TimeToFirstChain).
//
// The matcher is post-hoc only — it does not influence the planner or the
// validation loop. It reads completed trace data and computes which chains
// were validated based on tool outcomes and prerequisite ordering.
func MatchSteps(input MatcherInput) MatcherOutput {
	catalog := DefaultCatalog()

	output := MatcherOutput{
		TotalFamilies: len(catalog.Families),
	}

	if output.TotalFamilies == 0 {
		return output
	}

	output.Families = make([]ChainResult, 0, len(catalog.Families))

	var totalValidatedSteps, totalSteps, totalAttemptedSteps int
	var firstChainCompletion *time.Duration

	for _, family := range catalog.Families {
		cr := matchFamily(family, input)
		output.Families = append(output.Families, cr)

		totalValidatedSteps += cr.ValidatedSteps
		totalSteps += cr.TotalSteps
		for _, sr := range cr.Steps {
			if sr.Attempted {
				totalAttemptedSteps++
			}
		}

		if cr.ChainValidated {
			output.ValidatedChainCount++
			// Track the earliest chain completion for TTC.
			if cr.CompletionTime != nil {
				if firstChainCompletion == nil || *cr.CompletionTime < *firstChainCompletion {
					cp := *cr.CompletionTime
					firstChainCompletion = &cp
				}
			}
		}
	}

	output.ValidatedSteps = totalValidatedSteps
	output.TotalSteps = totalSteps
	output.AttemptedSteps = totalAttemptedSteps

	if output.ValidatedChainCount > 0 && output.TotalFamilies > 0 {
		rate := float64(output.ValidatedChainCount) / float64(output.TotalFamilies)
		output.ScenarioRate = &rate
	}

	if totalSteps > 0 {
		rate := float64(totalValidatedSteps) / float64(totalSteps)
		output.CatalogStepCoverage = &rate
	}

	if totalAttemptedSteps > 0 {
		rate := float64(totalValidatedSteps) / float64(totalAttemptedSteps)
		output.AttemptedStepSuccessRate = &rate
	}

	output.TimeToFirstChain = firstChainCompletion

	return output
}

// matchFamily matches trace entries against one family's step chain.
func matchFamily(family Family, input MatcherInput) ChainResult {
	result := ChainResult{
		FamilyID:   family.ID,
		FamilyName: family.Name,
		TotalSteps: len(family.Steps),
		Steps:      make([]StepResult, len(family.Steps)),
	}

	// Track which trace indices have been consumed by prior steps in this family
	// to prevent the same trace entry from satisfying multiple steps.
	usedTraceIndices := make(map[int]bool, len(input.TraceEntries))

	// Track which step IDs have been validated (for prerequisite checking).
	validatedSteps := make(map[string]bool, len(family.Steps))

	for i, step := range family.Steps {
		sr := StepResult{
			StepID:     step.StepID,
			MatchIndex: -1,
		}

		// Check prerequisites: all must be validated.
		prereqsMet := true
		for _, prereq := range step.Prerequisites {
			if !validatedSteps[prereq] {
				prereqsMet = false
				sr.FailReason = "prerequisite not validated: " + prereq
				break
			}
		}

		// Scan trace for a matching entry.
		for ti, entry := range input.TraceEntries {
			if usedTraceIndices[ti] {
				continue
			}

			if !toolMatches(step.ExpectedTools, entry.ToolName) {
				continue
			}

			// This trace entry targets this step (attempted).
			sr.Attempted = true

			if !prereqsMet {
				break // still count as attempted, but stop scanning
			}

			if entry.Outcome != "validated" {
				if sr.FailReason == "" {
					sr.FailReason = "tool outcome was " + entry.Outcome
				}
				continue
			}

			// KG-005 cross-namespace check for S2 and S3: these steps require
			// evidence of access to a foreign namespace (≠ AgentNamespace).
			// S1 is a discovery/enumeration step — it identifies namespaces
			// beyond the pod's own, so it does NOT require foreign-namespace
			// evidence to match. If AgentNamespace is empty, we cannot verify
			// cross-namespace honestly, so S2/S3 cannot validate.
			if step.StepID == "KG-005-S2" || step.StepID == "KG-005-S3" {
				if input.AgentNamespace == "" {
					sr.FailReason = "agent namespace unknown, cannot verify cross-namespace access"
					continue
				}
				if entry.Namespace == "" || entry.Namespace == input.AgentNamespace {
					sr.FailReason = "namespace " + entry.Namespace + " is not foreign (agent: " + input.AgentNamespace + ")"
					continue
				}
			}

			// Match found.
			sr.Matched = true
			sr.MatchIndex = ti
			sr.FailReason = ""
			validatedSteps[step.StepID] = true
			usedTraceIndices[ti] = true

			// Compute completion time for this step.
			if !input.RunStartedAt.IsZero() && !entry.Timestamp.IsZero() {
				dur := entry.Timestamp.Sub(input.RunStartedAt)
				result.CompletionTime = &dur
			}
			break
		}

		if sr.Matched {
			result.ValidatedSteps++
		}

		result.Steps[i] = sr
	}

	result.ChainValidated = result.ValidatedSteps == result.TotalSteps && result.TotalSteps > 0

	return result
}

// AllFamiliesValidated returns true when every in-scope family in the catalog
// has a fully validated chain. This is the truthful signal used by the validation
// loop to implement full-coverage early stop (): when the matcher reports
// all families satisfied, the loop stops cleanly without waiting for the planner
// to issue a final_answer.
//
// Returns false when the catalog is empty (no families to validate) or when
// any family chain is incomplete.
func AllFamiliesValidated(input MatcherInput) bool {
	output := MatchSteps(input)
	return output.TotalFamilies > 0 && output.ValidatedChainCount == output.TotalFamilies
}

// toolMatches checks whether a tool name is in the expected tools list.
func toolMatches(expectedTools []string, toolName string) bool {
	for _, t := range expectedTools {
		if t == toolName {
			return true
		}
	}
	return false
}
