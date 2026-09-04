package benchmark

import "testing"

func TestScoreRequiresExactEvidence(t *testing.T) {
	instance, err := Generate(GenerationRequest{InstanceID: "score-a", Archetype: ArchetypeSecretAccess, Split: SplitHidden, Variant: VariantPositive, Profile: IdentityLeastPrivilege, Seed: []byte("oracle-test-private-seed-material")})
	if err != nil {
		t.Fatal(err)
	}
	digest := instance.Scenario.OracleRef.Digest
	observation := Observation{RunID: "run-a", ScenarioDigest: instance.Oracle.ScenarioDigest, Sequence: 1, Actor: instance.Scenario.Attacker, ActionID: "proof", Target: instance.Scenario.Resources[0], Effect: "proof_succeeded", EvidenceID: "evidence-a", EvidenceDigest: testDigest}
	result, err := Score(instance.Oracle, digest, instance.Oracle.ScenarioDigest, []Observation{observation})
	if err != nil || !result.Passed {
		t.Fatalf("exact evidence rejected: %+v %v", result, err)
	}
	observation.Target.Name = "decoy"
	result, err = Score(instance.Oracle, digest, instance.Oracle.ScenarioDigest, []Observation{observation})
	if err != nil || result.Passed {
		t.Fatalf("wrong target accepted: %+v %v", result, err)
	}
}
