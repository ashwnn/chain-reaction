package benchmark

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

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

func TestScoreWithClusterRequiresLiveActorTargetAndControl(t *testing.T) {
	instance, err := Generate(GenerationRequest{InstanceID: "score-live-a", Archetype: ArchetypeSecretAccess, Split: SplitHidden, Variant: VariantPositive, Profile: IdentityLeastPrivilege, Seed: []byte("oracle-live-controller-private-seed")})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderKubernetes(instance)
	if err != nil {
		t.Fatal(err)
	}
	objects := make([]runtime.Object, 0, len(rendered.Objects))
	for index := range rendered.Objects {
		objects = append(objects, rendered.Objects[index].DeepCopy())
	}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), objects...)
	target := instance.Oracle.Predicates[0].Target
	observation := Observation{RunID: "run-a", ScenarioDigest: instance.Oracle.ScenarioDigest, Sequence: 1, Actor: instance.Scenario.Attacker, ActionID: "proof", Target: target, Effect: "proof_succeeded", EvidenceID: "evidence-a", EvidenceDigest: testDigest}
	result, err := ScoreWithCluster(context.Background(), client, instance.Scenario, instance.Oracle, instance.Scenario.OracleRef.Digest, instance.Oracle.ScenarioDigest, []Observation{observation})
	if err != nil || !result.Passed {
		t.Fatalf("live range evidence rejected: %+v %v", result, err)
	}
	resource, namespaced, err := resourceForRef(target)
	if err != nil || !namespaced {
		t.Fatal(err)
	}
	if err := client.Resource(resource).Namespace(target.Namespace).Delete(context.Background(), target.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err = ScoreWithCluster(context.Background(), client, instance.Scenario, instance.Oracle, instance.Scenario.OracleRef.Digest, instance.Oracle.ScenarioDigest, []Observation{observation})
	if err != nil || result.Passed || result.PredicateResults[0].Reason != "target_state_mismatch" {
		t.Fatalf("missing target accepted: %+v %v", result, err)
	}
}
