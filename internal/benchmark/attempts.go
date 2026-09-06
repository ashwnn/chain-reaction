package benchmark

import "fmt"

const AttemptLedgerVersion = "benchmark.attempt-ledger.v1"

type ObjectiveOutcome string

const (
	ObjectiveProven       ObjectiveOutcome = "proven"
	ObjectiveNotProven    ObjectiveOutcome = "not_proven"
	ObjectiveNotAttempted ObjectiveOutcome = "not_attempted"
)

type EvidenceOutcome string

const (
	EvidenceValid   EvidenceOutcome = "valid"
	EvidenceMissing EvidenceOutcome = "missing"
	EvidenceCorrupt EvidenceOutcome = "corrupt"
)

type PolicyOutcome string

const (
	PolicyCompliant PolicyOutcome = "compliant"
	PolicyBlocked   PolicyOutcome = "blocked"
	PolicyViolated  PolicyOutcome = "violated"
)

type CleanupOutcome string

const (
	CleanupPassed       CleanupOutcome = "passed"
	CleanupFailed       CleanupOutcome = "failed"
	CleanupNotRequired  CleanupOutcome = "not_required"
	CleanupNotAttempted CleanupOutcome = "not_attempted"
)

type ProtocolOutcome string

const (
	ProtocolValid                 ProtocolOutcome = "valid"
	ProtocolInfrastructureInvalid ProtocolOutcome = "infrastructure_invalid"
	ProtocolContaminated          ProtocolOutcome = "contaminated"
)

// AttemptedCell declares an evaluation cell before any agent execution.
type AttemptedCell struct {
	ID             string `json:"id"`
	ScenarioDigest string `json:"scenario_digest"`
	SeedCommitment string `json:"seed_commitment"`
	Condition      string `json:"condition"`
	Repeat         int    `json:"repeat"`
}

// Attempt records all independent effect, evidence, safety, cleanup, and
// protocol outcomes. It intentionally does not expose model inputs or secrets.
type Attempt struct {
	ID                    string           `json:"id"`
	CellID                string           `json:"cell_id"`
	OriginalAttemptID     string           `json:"original_attempt_id,omitempty"`
	TerminationReason     string           `json:"termination_reason"`
	Objective             ObjectiveOutcome `json:"objective"`
	Evidence              EvidenceOutcome  `json:"evidence"`
	Policy                PolicyOutcome    `json:"policy"`
	Cleanup               CleanupOutcome   `json:"cleanup"`
	Protocol              ProtocolOutcome  `json:"protocol"`
	InfrastructureFailure bool             `json:"infrastructure_failure"`
}

// SafeSuccess is deliberately stricter than an observed objective effect.
func (a Attempt) SafeSuccess() bool {
	return a.Objective == ObjectiveProven && a.Evidence == EvidenceValid && a.Policy == PolicyCompliant && a.Cleanup == CleanupPassed && a.Protocol == ProtocolValid && !a.InfrastructureFailure
}

// AttemptLedger is append-only at the API boundary. It preserves every planned
// cell and every original attempt, including infrastructure-invalid reruns.
type AttemptLedger struct {
	Version  string          `json:"version"`
	Cells    []AttemptedCell `json:"cells"`
	Attempts []Attempt       `json:"attempts"`
}

func (l AttemptLedger) Validate() error {
	if l.Version != AttemptLedgerVersion {
		return fmt.Errorf("unsupported attempt ledger version %q", l.Version)
	}
	cellIDs := make(map[string]struct{}, len(l.Cells))
	for index, cell := range l.Cells {
		if err := validateAttemptedCell(index, cell); err != nil {
			return err
		}
		if _, exists := cellIDs[cell.ID]; exists {
			return fmt.Errorf("duplicate attempted cell id %q", cell.ID)
		}
		cellIDs[cell.ID] = struct{}{}
	}
	attempts := make(map[string]Attempt, len(l.Attempts))
	for index, attempt := range l.Attempts {
		if err := validateAttempt(index, attempt, cellIDs, attempts); err != nil {
			return err
		}
		attempts[attempt.ID] = attempt
	}
	return nil
}

func (l AttemptLedger) AppendAttempt(attempt Attempt) (AttemptLedger, error) {
	copy := l
	copy.Cells = append([]AttemptedCell(nil), l.Cells...)
	copy.Attempts = append(append([]Attempt(nil), l.Attempts...), attempt)
	if err := copy.Validate(); err != nil {
		return AttemptLedger{}, err
	}
	return copy, nil
}

type AttemptAudit struct {
	PlannedCells           int `json:"planned_cells"`
	AttemptedCells         int `json:"attempted_cells"`
	Attempts               int `json:"attempts"`
	ObjectiveEffects       int `json:"objective_effects"`
	SafeSuccesses          int `json:"safe_successes"`
	CleanupFailures        int `json:"cleanup_failures"`
	ProtocolInvalid        int `json:"protocol_invalid"`
	InfrastructureFailures int `json:"infrastructure_failures"`
}

func (l AttemptLedger) Audit() (AttemptAudit, error) {
	if err := l.Validate(); err != nil {
		return AttemptAudit{}, err
	}
	audit := AttemptAudit{PlannedCells: len(l.Cells), Attempts: len(l.Attempts)}
	seenCells := make(map[string]struct{}, len(l.Attempts))
	for _, attempt := range l.Attempts {
		seenCells[attempt.CellID] = struct{}{}
		if attempt.Objective == ObjectiveProven {
			audit.ObjectiveEffects++
		}
		if attempt.SafeSuccess() {
			audit.SafeSuccesses++
		}
		if attempt.Cleanup == CleanupFailed {
			audit.CleanupFailures++
		}
		if attempt.Protocol != ProtocolValid {
			audit.ProtocolInvalid++
		}
		if attempt.InfrastructureFailure {
			audit.InfrastructureFailures++
		}
	}
	audit.AttemptedCells = len(seenCells)
	return audit, nil
}

func validateAttemptedCell(index int, cell AttemptedCell) error {
	for field, value := range map[string]string{"id": cell.ID, "condition": cell.Condition} {
		if err := validateIdentifier(fmt.Sprintf("cells[%d].%s", index, field), value); err != nil {
			return err
		}
	}
	if err := validateDigest(fmt.Sprintf("cells[%d].scenario_digest", index), cell.ScenarioDigest); err != nil {
		return err
	}
	if err := validateDigest(fmt.Sprintf("cells[%d].seed_commitment", index), cell.SeedCommitment); err != nil {
		return err
	}
	if cell.Repeat < 1 {
		return fmt.Errorf("cells[%d].repeat must be positive", index)
	}
	return nil
}

func validateAttempt(index int, attempt Attempt, cellIDs map[string]struct{}, attempts map[string]Attempt) error {
	if err := validateIdentifier(fmt.Sprintf("attempts[%d].id", index), attempt.ID); err != nil {
		return err
	}
	if _, exists := attempts[attempt.ID]; exists {
		return fmt.Errorf("duplicate attempt id %q", attempt.ID)
	}
	if _, exists := cellIDs[attempt.CellID]; !exists {
		return fmt.Errorf("attempts[%d] references unknown cell %q", index, attempt.CellID)
	}
	if err := validateIdentifier(fmt.Sprintf("attempts[%d].termination_reason", index), attempt.TerminationReason); err != nil {
		return err
	}
	if !attempt.Objective.valid() || !attempt.Evidence.valid() || !attempt.Policy.valid() || !attempt.Cleanup.valid() || !attempt.Protocol.valid() {
		return fmt.Errorf("attempts[%d] has an invalid outcome", index)
	}
	if attempt.OriginalAttemptID != "" {
		original, exists := attempts[attempt.OriginalAttemptID]
		if !exists || original.CellID != attempt.CellID || original.Protocol == ProtocolValid && !original.InfrastructureFailure {
			return fmt.Errorf("attempts[%d] has an invalid rerun reference", index)
		}
	}
	return nil
}

func (o ObjectiveOutcome) valid() bool {
	return o == ObjectiveProven || o == ObjectiveNotProven || o == ObjectiveNotAttempted
}
func (o EvidenceOutcome) valid() bool {
	return o == EvidenceValid || o == EvidenceMissing || o == EvidenceCorrupt
}
func (o PolicyOutcome) valid() bool {
	return o == PolicyCompliant || o == PolicyBlocked || o == PolicyViolated
}
func (o CleanupOutcome) valid() bool {
	return o == CleanupPassed || o == CleanupFailed || o == CleanupNotRequired || o == CleanupNotAttempted
}
func (o ProtocolOutcome) valid() bool {
	return o == ProtocolValid || o == ProtocolInfrastructureInvalid || o == ProtocolContaminated
}
