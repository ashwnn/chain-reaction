package benchmark

import (
	"encoding/json"
	"strings"
	"testing"
)

const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validScenario() ScenarioManifest {
	return ScenarioManifest{
		Version:        ScenarioVersion,
		InstanceID:     "instance-001",
		Archetype:      ArchetypeSecretAccess,
		Split:          SplitHidden,
		Variant:        VariantPositive,
		SeedCommitment: testDigest,
		NetworkPort:    30001,
		Attacker:       Actor{Namespace: "range-a", Name: "agent", Profile: IdentityLeastPrivilege},
		Resources:      []ObjectRef{{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "object-a"}},
		AllowedActions: []ProofAction{{ID: "read", Kind: "kubernetes_get", TimeoutSecs: 30}},
		OracleRef:      OracleReference{Version: OracleVersion, Digest: testDigest},
		Control:        CounterfactualControl{Kind: "rbac_binding", Enabled: true},
		Lifecycle:      Lifecycle{RunNamespace: "range-a", CleanupOwner: "controller", TTLSeconds: 300},
	}
}

func TestScenarioManifestValidation(t *testing.T) {
	manifest := validScenario()
	if err := manifest.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	manifest.AllowedActions = append(manifest.AllowedActions, manifest.AllowedActions[0])
	if err := manifest.Validate(); err == nil {
		t.Fatal("duplicate action ID accepted")
	}
}

func TestStrictScenarioDecoderRejectsUnknownAndTrailingValues(t *testing.T) {
	manifest := validScenario()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if _, err := DecodeScenarioManifest(append(body, []byte(" {}")...)); err == nil {
		t.Fatal("trailing JSON value accepted")
	}
	unknown := strings.Replace(string(body), "{", `{"unexpected":true,`, 1)
	if _, err := DecodeScenarioManifest([]byte(unknown)); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestCanonicalDigestIsStableAndRejectsAmbiguousValues(t *testing.T) {
	first, err := Digest(validScenario())
	if err != nil {
		t.Fatalf("digest first manifest: %v", err)
	}
	second, err := Digest(validScenario())
	if err != nil {
		t.Fatalf("digest second manifest: %v", err)
	}
	if first != second {
		t.Fatalf("digest mismatch: %q != %q", first, second)
	}
	if _, err := CanonicalJSON(map[string]string{"ambiguous": "map"}); err == nil {
		t.Fatal("map accepted by canonical JSON")
	}
	if _, err := CanonicalJSON(struct{ Value float64 }{Value: 1}); err == nil {
		t.Fatal("float accepted by canonical JSON")
	}
}

func TestCommitmentManifestCannotExposeHiddenMaterial(t *testing.T) {
	manifest := CommitmentManifest{
		Version:        CommitmentVersion,
		ProtocolDigest: testDigest,
		Commitments: []PublicCommitment{{
			InstanceID:     "instance-001",
			SeedCommitment: testDigest,
			ScenarioDigest: testDigest,
			OracleDigest:   testDigest,
		}},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("valid commitment rejected: %v", err)
	}
	body, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatalf("canonical commitment: %v", err)
	}
	for _, forbidden := range []string{"seed", "predicate", "target", "attacker", "variant"} {
		if strings.Contains(string(body), `"`+forbidden+`"`) {
			t.Fatalf("public commitment contains forbidden field %q: %s", forbidden, body)
		}
	}
}

func TestSeedDerivationIsStableAndDoesNotEchoInputs(t *testing.T) {
	seed := []byte("private-seed-material-for-test-only")
	commitment, err := CommitSeed(seed, "hidden/a01")
	if err != nil {
		t.Fatalf("commit seed: %v", err)
	}
	if strings.Contains(commitment, string(seed)) {
		t.Fatal("commitment leaks seed")
	}
	name, err := DeriveDNSName(seed, "hidden/a01", 20)
	if err != nil {
		t.Fatalf("derive name: %v", err)
	}
	if !strings.HasPrefix(name, "cr-") || strings.Contains(name, "hidden") || strings.Contains(name, "a01") {
		t.Fatalf("derived name is not neutral: %q", name)
	}
	portA, err := DerivePort(seed, "hidden/a01", 30000, 30100)
	if err != nil {
		t.Fatalf("derive port: %v", err)
	}
	portB, err := DerivePort(seed, "hidden/a01", 30000, 30100)
	if err != nil {
		t.Fatalf("derive port again: %v", err)
	}
	if portA != portB || portA < 30000 || portA > 30100 {
		t.Fatalf("invalid deterministic port %d and %d", portA, portB)
	}
}

func TestOracleRejectsUnknownPredecessor(t *testing.T) {
	oracle := OracleContract{
		Version:        OracleVersion,
		ScenarioDigest: testDigest,
		Predicates: []Predicate{{
			ID:             "step-1",
			Actor:          Actor{Namespace: "range-a", Name: "agent", Profile: IdentityLeastPrivilege},
			ActionID:       "read",
			Target:         ObjectRef{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "object-a"},
			ExpectedEffect: "read",
			Predecessors:   []string{"missing"},
		}},
	}
	if err := oracle.Validate(); err == nil {
		t.Fatal("unknown predecessor accepted")
	}
}

func TestOracleCapabilityContractAllowsIndependentPrerequisites(t *testing.T) {
	actor := Actor{Namespace: "range-a", Name: "agent", Profile: IdentityLeastPrivilege}
	executionContext := ExecutionContext{
		Version:             ExecutionContextVersion,
		ID:                  "agent-pod",
		InitialActor:        actor,
		CurrentActor:        actor,
		Pod:                 ObjectRef{APIVersion: "v1", Kind: "Pod", Namespace: "range-a", Name: "agent-pod", UID: "pod-uid"},
		Container:           "agent",
		IdentityAttestation: ContextAttested,
		NetworkAttestation:  ContextObserved,
	}
	oracle := OracleContract{
		Version:           OracleVersion,
		ScenarioDigest:    testDigest,
		ExecutionContexts: []ExecutionContext{executionContext},
		InitialCapabilities: []Capability{{
			ID: "initial-session", Kind: CapabilityAuthenticatedSession, Actor: actor, ExecutionContextID: executionContext.ID,
		}},
		Predicates: []Predicate{
			{
				ID: "independent-a", Actor: actor, ActionID: "read-a", Target: ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "a"}, ExpectedEffect: "read", ExecutionContextID: executionContext.ID,
				RequiresAll: []string{"initial-session"}, Produces: []Capability{{ID: "cap-a", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: executionContext.ID}},
			},
			{
				ID: "independent-b", Actor: actor, ActionID: "read-b", Target: ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "b"}, ExpectedEffect: "read", ExecutionContextID: executionContext.ID,
				RequiresAll: []string{"initial-session"}, Produces: []Capability{{ID: "cap-b", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: executionContext.ID}},
			},
			{
				ID: "goal", Actor: actor, ActionID: "read-goal", Target: ObjectRef{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "goal"}, ExpectedEffect: "read", ExecutionContextID: executionContext.ID,
				RequiresAll: []string{"cap-a", "cap-b"},
			},
		},
	}
	if err := oracle.Validate(); err != nil {
		t.Fatalf("valid capability contract rejected: %v", err)
	}
}

func TestOracleCapabilityContractRejectsCyclesAndWrongContext(t *testing.T) {
	actor := Actor{Namespace: "range-a", Name: "agent", Profile: IdentityLeastPrivilege}
	oracle := OracleContract{
		Version:        OracleVersion,
		ScenarioDigest: testDigest,
		ExecutionContexts: []ExecutionContext{{
			Version: ExecutionContextVersion, ID: "agent-pod", InitialActor: actor, CurrentActor: actor,
			Pod: ObjectRef{APIVersion: "v1", Kind: "Pod", Namespace: "range-a", Name: "agent-pod", UID: "pod-uid"}, Container: "agent",
			IdentityAttestation: ContextAttested, NetworkAttestation: ContextObserved,
		}},
		Predicates: []Predicate{
			{
				ID: "a", Actor: actor, ActionID: "a", Target: ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "a"}, ExpectedEffect: "read", ExecutionContextID: "agent-pod",
				RequiresAll: []string{"cap-b"}, Produces: []Capability{{ID: "cap-a", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: "agent-pod"}},
			},
			{
				ID: "b", Actor: actor, ActionID: "b", Target: ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "range-a", Name: "b"}, ExpectedEffect: "read", ExecutionContextID: "agent-pod",
				RequiresAll: []string{"cap-a"}, Produces: []Capability{{ID: "cap-b", Kind: CapabilityAuthorizedOperation, Actor: actor, ExecutionContextID: "agent-pod"}},
			},
		},
	}
	if err := oracle.Validate(); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("cyclic capability contract accepted: %v", err)
	}
	oracle.Predicates[0].RequiresAll = nil
	oracle.Predicates[1].RequiresAll = nil
	oracle.Predicates[1].ExecutionContextID = "wrong-pod"
	if err := oracle.Validate(); err == nil || !strings.Contains(err.Error(), "unknown execution context") {
		t.Fatalf("wrong execution context accepted: %v", err)
	}
}

func TestFinalizeScenarioOracleBindsDocumentedProjection(t *testing.T) {
	manifest := validScenario()
	oracle := OracleContract{
		Version: OracleVersion,
		Predicates: []Predicate{{
			ID:             "step-1",
			Actor:          manifest.Attacker,
			ActionID:       "read",
			Target:         manifest.Resources[0],
			ExpectedEffect: "read",
		}},
	}
	finalManifest, finalOracle, err := FinalizeScenarioOracle(manifest, oracle)
	if err != nil {
		t.Fatalf("finalize scenario/oracle: %v", err)
	}
	projectionDigest, err := Digest(ScenarioProjection(finalManifest))
	if err != nil {
		t.Fatalf("digest projection: %v", err)
	}
	if finalOracle.ScenarioDigest != projectionDigest {
		t.Fatalf("oracle scenario digest = %q, want %q", finalOracle.ScenarioDigest, projectionDigest)
	}
	oracleDigest, err := Digest(finalOracle)
	if err != nil {
		t.Fatalf("digest oracle: %v", err)
	}
	if finalManifest.OracleRef.Digest != oracleDigest {
		t.Fatalf("scenario oracle digest = %q, want %q", finalManifest.OracleRef.Digest, oracleDigest)
	}
}
