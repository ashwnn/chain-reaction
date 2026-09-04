package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateSeedAndPublicCommitmentsRemainSeparate(t *testing.T) {
	seed := []byte("controller-only-private-seed-material")
	privateRoot := filepath.Join(t.TempDir(), privateSeedDirectory)
	seedPath, err := WritePrivateSeed(privateRoot, "instance-a", seed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WritePrivateSeed(privateRoot, "instance-a", seed); err == nil {
		t.Fatal("seed overwrite accepted")
	}
	stored, err := os.ReadFile(seedPath)
	if err != nil || string(stored) != string(seed) {
		t.Fatalf("private seed mismatch: %v", err)
	}
	instance, err := Generate(GenerationRequest{InstanceID: "instance-a", Archetype: ArchetypeSecretAccess, Split: SplitHidden, Variant: VariantPositive, Profile: IdentityLeastPrivilege, Seed: seed})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildCommitmentManifest(testDigest, []GeneratedInstance{instance})
	if err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(t.TempDir(), "commitments.json")
	if err := WritePublicCommitmentManifest(publicPath, manifest); err != nil {
		t.Fatal(err)
	}
	if err := WritePublicCommitmentManifest(publicPath, manifest); err == nil {
		t.Fatal("public commitment overwrite accepted")
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{string(seed), "target", "predicate", "attacker", "variant"} {
		if strings.Contains(string(public), forbidden) {
			t.Fatalf("public commitments leak %q", forbidden)
		}
	}
}
