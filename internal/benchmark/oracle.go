package benchmark

import "fmt"

// Observation is normalized controller-side machine evidence. Model text is
// intentionally not part of this contract.
type Observation struct {
	RunID          string    `json:"run_id"`
	ScenarioDigest string    `json:"scenario_digest"`
	Sequence       int       `json:"sequence"`
	Actor          Actor     `json:"actor"`
	ActionID       string    `json:"action_id"`
	Target         ObjectRef `json:"target"`
	Effect         string    `json:"effect"`
	EvidenceID     string    `json:"evidence_id"`
	EvidenceDigest string    `json:"evidence_digest"`
}

type PredicateResult struct {
	PredicateID string `json:"predicate_id"`
	Passed      bool   `json:"passed"`
	Reason      string `json:"reason"`
}

type OracleResult struct {
	Version          string            `json:"version"`
	ScenarioDigest   string            `json:"scenario_digest"`
	OracleDigest     string            `json:"oracle_digest"`
	Passed           bool              `json:"passed"`
	PredicateResults []PredicateResult `json:"predicate_results"`
}

// Score evaluates exact normalized evidence. It never accepts generic success
// or an LLM classification as proof.
func Score(oracle OracleContract, oracleDigest, scenarioDigest string, observations []Observation) (OracleResult, error) {
	if err := oracle.Validate(); err != nil {
		return OracleResult{}, err
	}
	if err := validateDigest("oracle_digest", oracleDigest); err != nil {
		return OracleResult{}, err
	}
	if scenarioDigest != oracle.ScenarioDigest {
		return OracleResult{}, fmt.Errorf("scenario digest does not match oracle")
	}
	result := OracleResult{Version: ResultVersion, ScenarioDigest: scenarioDigest, OracleDigest: oracleDigest, Passed: true, PredicateResults: make([]PredicateResult, 0, len(oracle.Predicates))}
	used := make(map[string]bool)
	lastSequence := 0
	passed := make(map[string]bool)
	for _, predicate := range oracle.Predicates {
		pr := PredicateResult{PredicateID: predicate.ID}
		for _, predecessor := range predicate.Predecessors {
			if !passed[predecessor] {
				pr.Reason = "predecessor_not_satisfied"
				break
			}
		}
		if pr.Reason == "" {
			for _, observation := range observations {
				if used[observation.EvidenceID] || observation.Sequence <= lastSequence || observation.RunID == "" || observation.ScenarioDigest != scenarioDigest {
					continue
				}
				if err := validateDigest("evidence_digest", observation.EvidenceDigest); err != nil {
					continue
				}
				if matches(predicate, observation) {
					used[observation.EvidenceID] = true
					lastSequence = observation.Sequence
					pr.Passed = true
					break
				}
			}
			if !pr.Passed {
				pr.Reason = "no_exact_machine_evidence"
			}
		}
		passed[predicate.ID] = pr.Passed
		result.PredicateResults = append(result.PredicateResults, pr)
		if !pr.Passed {
			result.Passed = false
		}
	}
	return result, nil
}

func matches(predicate Predicate, observation Observation) bool {
	return predicate.ActionID == observation.ActionID && predicate.ExpectedEffect == observation.Effect && predicate.Actor == observation.Actor && predicate.Target == observation.Target
}
