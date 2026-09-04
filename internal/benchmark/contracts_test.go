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
		Split:          SplitHidden,
		Variant:        VariantPositive,
		SeedCommitment: testDigest,
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
