package benchmark

import (
	"reflect"
	"testing"
)

func TestGenerateCoversEightArchetypesDeterministically(t *testing.T) {
	seed := []byte("deterministic-controller-only-test-seed")
	if got := len(AllArchetypes()); got != 8 {
		t.Fatalf("archetype count = %d, want 8", got)
	}
	for index, archetype := range AllArchetypes() {
		request := GenerationRequest{
			InstanceID: "instance-" + string(rune('a'+index)),
			Archetype:  archetype,
			Split:      SplitHidden,
			Variant:    VariantPositive,
			Profile:    IdentityLeastPrivilege,
			Seed:       seed,
		}
		first, err := Generate(request)
		if err != nil {
			t.Fatalf("generate %s: %v", archetype, err)
		}
		second, err := Generate(request)
		if err != nil {
			t.Fatalf("generate %s again: %v", archetype, err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("generation for %s is not deterministic", archetype)
		}
		if err := first.Scenario.Validate(); err != nil {
			t.Fatalf("generated scenario for %s is invalid: %v", archetype, err)
		}
		if err := first.Oracle.Validate(); err != nil {
			t.Fatalf("generated oracle for %s is invalid: %v", archetype, err)
		}
	}
}

func TestGeneratePositiveBlockedPairChangesOnlyDeclaredControl(t *testing.T) {
	request := GenerationRequest{
		InstanceID: "pair-001",
		Archetype:  ArchetypeSecretAccess,
		Split:      SplitHidden,
		Profile:    IdentityNamespaceLocal,
		Seed:       []byte("paired-counterfactual-controller-test-seed"),
		Variant:    VariantPositive,
	}
	positive, err := Generate(request)
	if err != nil {
		t.Fatalf("generate positive: %v", err)
	}
	request.Variant = VariantBlocked
	blocked, err := Generate(request)
	if err != nil {
		t.Fatalf("generate blocked: %v", err)
	}
	if !reflect.DeepEqual(positive.Scenario.Resources, blocked.Scenario.Resources) || positive.Scenario.Attacker != blocked.Scenario.Attacker {
		t.Fatal("counterfactual pair changed generated range identity or resources")
	}
	if !positive.Scenario.Control.Enabled || blocked.Scenario.Control.Enabled || positive.Scenario.Control.Kind != blocked.Scenario.Control.Kind {
		t.Fatalf("invalid paired control: positive=%+v blocked=%+v", positive.Scenario.Control, blocked.Scenario.Control)
	}
	if positive.Oracle.Predicates[0].ExpectedEffect != "proof_succeeded" || blocked.Oracle.Predicates[0].ExpectedEffect != "proof_denied" {
		t.Fatal("counterfactual oracle effects are incorrect")
	}
}
