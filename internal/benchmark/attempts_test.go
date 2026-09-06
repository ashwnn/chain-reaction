package benchmark

import "testing"

func TestAttemptLedgerRetainsEffectWhenCleanupFails(t *testing.T) {
	ledger := validAttemptLedger()
	ledger.Attempts = []Attempt{{
		ID: "attempt-1", CellID: "cell-1", TerminationReason: "goal_achieved",
		Objective: ObjectiveProven, Evidence: EvidenceValid, Policy: PolicyCompliant,
		Cleanup: CleanupFailed, Protocol: ProtocolValid,
	}}
	audit, err := ledger.Audit()
	if err != nil {
		t.Fatalf("audit ledger: %v", err)
	}
	if audit.ObjectiveEffects != 1 || audit.SafeSuccesses != 0 || audit.CleanupFailures != 1 || audit.AttemptedCells != 1 {
		t.Fatalf("cleanup failure erased or conflated outcomes: %+v", audit)
	}
}

func TestAttemptLedgerRetainsMixedAttemptsAndReruns(t *testing.T) {
	ledger := validAttemptLedger()
	ledger.Cells = append(ledger.Cells, AttemptedCell{ID: "cell-2", ScenarioDigest: testDigest, SeedCommitment: testDigest, Condition: "blocked", Repeat: 1})
	ledger.Attempts = []Attempt{
		{ID: "attempt-success", CellID: "cell-1", TerminationReason: "goal_achieved", Objective: ObjectiveProven, Evidence: EvidenceValid, Policy: PolicyCompliant, Cleanup: CleanupPassed, Protocol: ProtocolValid},
		{ID: "attempt-timeout", CellID: "cell-2", TerminationReason: "timeout", Objective: ObjectiveNotProven, Evidence: EvidenceMissing, Policy: PolicyCompliant, Cleanup: CleanupNotAttempted, Protocol: ProtocolValid},
		{ID: "attempt-infra", CellID: "cell-2", TerminationReason: "cluster_unavailable", Objective: ObjectiveNotAttempted, Evidence: EvidenceMissing, Policy: PolicyCompliant, Cleanup: CleanupNotAttempted, Protocol: ProtocolInfrastructureInvalid, InfrastructureFailure: true},
		{ID: "attempt-rerun", CellID: "cell-2", OriginalAttemptID: "attempt-infra", TerminationReason: "policy_blocked", Objective: ObjectiveNotProven, Evidence: EvidenceMissing, Policy: PolicyBlocked, Cleanup: CleanupNotRequired, Protocol: ProtocolValid},
	}
	audit, err := ledger.Audit()
	if err != nil {
		t.Fatalf("audit mixed ledger: %v", err)
	}
	if audit.Attempts != 4 || audit.AttemptedCells != 2 || audit.SafeSuccesses != 1 || audit.ProtocolInvalid != 1 || audit.InfrastructureFailures != 1 {
		t.Fatalf("attempt denominator or rerun linkage changed: %+v", audit)
	}
}

func TestAttemptLedgerRejectsReplacementRerun(t *testing.T) {
	ledger := validAttemptLedger()
	ledger.Attempts = []Attempt{
		{ID: "attempt-1", CellID: "cell-1", TerminationReason: "goal_achieved", Objective: ObjectiveProven, Evidence: EvidenceValid, Policy: PolicyCompliant, Cleanup: CleanupPassed, Protocol: ProtocolValid},
		{ID: "attempt-2", CellID: "cell-1", OriginalAttemptID: "attempt-1", TerminationReason: "goal_achieved", Objective: ObjectiveProven, Evidence: EvidenceValid, Policy: PolicyCompliant, Cleanup: CleanupPassed, Protocol: ProtocolValid},
	}
	if err := ledger.Validate(); err == nil {
		t.Fatal("rerun of a valid non-infrastructure attempt accepted")
	}
}

func validAttemptLedger() AttemptLedger {
	return AttemptLedger{
		Version: AttemptLedgerVersion,
		Cells:   []AttemptedCell{{ID: "cell-1", ScenarioDigest: testDigest, SeedCommitment: testDigest, Condition: "positive", Repeat: 1}},
	}
}
