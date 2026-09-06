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

func TestScoreCapabilityDependenciesAllowIndependentReordering(t *testing.T) {
	actor := Actor{Namespace: "range-a", Name: "agent", Profile: IdentityLeastPrivilege}
	context := ExecutionContext{
		Version: ExecutionContextVersion, ID: "agent-pod", InitialActor: actor, CurrentActor: actor,
		Pod: ObjectRef{APIVersion: "v1", Kind: "Pod", Namespace: "range-a", Name: "agent", UID: "pod-uid"}, Container: "agent",
		IdentityAttestation: ContextAttested, NetworkAttestation: ContextObserved,
	}
	configMapA := ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "a"}
	configMapB := ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "b"}
	secret := ObjectRef{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "goal"}
	oracle := OracleContract{
		Version: OracleVersion, ScenarioDigest: testDigest, ExecutionContexts: []ExecutionContext{context},
		InitialCapabilities: []Capability{{ID: "session", Kind: CapabilityAuthenticatedSession, Actor: actor, ExecutionContextID: context.ID}},
		Predicates: []Predicate{
			{ID: "a", Actor: actor, ActionID: "read-a", Target: configMapA, ExpectedEffect: "read", ExecutionContextID: context.ID, RequiresAll: []string{"session"}, Produces: []Capability{{ID: "cap-a", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: context.ID}}},
			{ID: "b", Actor: actor, ActionID: "read-b", Target: configMapB, ExpectedEffect: "read", ExecutionContextID: context.ID, RequiresAll: []string{"session"}, Produces: []Capability{{ID: "cap-b", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: context.ID}}},
			{ID: "goal", Actor: actor, ActionID: "read-goal", Target: secret, ExpectedEffect: "read", ExecutionContextID: context.ID, RequiresAll: []string{"cap-a", "cap-b"}},
		},
	}
	observations := []Observation{
		{RunID: "run-a", ScenarioDigest: testDigest, Sequence: 1, Actor: actor, ActionID: "read-b", Target: configMapB, Effect: "read", EvidenceID: "evidence-b", EvidenceDigest: testDigest, ExecutionContextID: context.ID, ConsumedCapabilities: []string{"session"}, ProducedCapabilities: []string{"cap-b"}},
		{RunID: "run-a", ScenarioDigest: testDigest, Sequence: 2, Actor: actor, ActionID: "read-a", Target: configMapA, Effect: "read", EvidenceID: "evidence-a", EvidenceDigest: testDigest, ExecutionContextID: context.ID, ConsumedCapabilities: []string{"session"}, ProducedCapabilities: []string{"cap-a"}},
		{RunID: "run-a", ScenarioDigest: testDigest, Sequence: 3, Actor: actor, ActionID: "read-goal", Target: secret, Effect: "read", EvidenceID: "evidence-goal", EvidenceDigest: testDigest, ExecutionContextID: context.ID, ConsumedCapabilities: []string{"cap-a", "cap-b"}},
	}
	result, err := Score(oracle, testDigest, testDigest, observations)
	if err != nil || !result.Passed {
		t.Fatalf("independent prerequisite reorder rejected: %+v %v", result, err)
	}

	observations[2].ExecutionContextID = "wrong-pod"
	result, err = Score(oracle, testDigest, testDigest, observations)
	if err != nil || result.Passed || result.PredicateResults[2].Reason != "no_exact_machine_evidence" {
		t.Fatalf("wrong execution context received credit: %+v %v", result, err)
	}
}
