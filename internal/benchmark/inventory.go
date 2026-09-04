package benchmark

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

const (
	privateSeedsPerArchetype = 3
	publicPairsPerArchetype  = 1
)

// CreateEvaluationInventory generates one public development pair and three
// private paired evaluation seeds for every archetype. Raw hidden seeds are
// written only below privateRoot. Returned commitments are safe to publish.
func CreateEvaluationInventory(privateRoot string, entropy io.Reader) (CommitmentManifest, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	instances := make([]GeneratedInstance, 0, len(AllArchetypes())*(2*publicPairsPerArchetype+2*privateSeedsPerArchetype))
	commitments := make([]PublicCommitment, 0, cap(instances))
	commitmentID := 0
	appendPair := func(archetype Archetype, split Split, rangeID string, seed []byte) error {
		for _, variant := range []Variant{VariantPositive, VariantBlocked} {
			instance, err := Generate(GenerationRequest{InstanceID: rangeID, Archetype: archetype, Split: split, Variant: variant, Profile: IdentityLeastPrivilege, Seed: seed})
			if err != nil {
				return err
			}
			instances = append(instances, instance)
			commitment := instance.Commitment
			commitmentID++
			commitment.InstanceID = fmt.Sprintf("instance-%03d", commitmentID)
			commitments = append(commitments, commitment)
		}
		return nil
	}
	for archetypeIndex, archetype := range AllArchetypes() {
		publicSeed := sha256.Sum256([]byte("chain-reaction-benchmark-v2-public-development/" + string(archetype)))
		if err := appendPair(archetype, SplitPublic, fmt.Sprintf("public-range-%02d", archetypeIndex+1), publicSeed[:]); err != nil {
			return CommitmentManifest{}, err
		}
		for replica := 0; replica < privateSeedsPerArchetype; replica++ {
			seed := make([]byte, 32)
			if _, err := io.ReadFull(entropy, seed); err != nil {
				return CommitmentManifest{}, fmt.Errorf("read private seed entropy: %w", err)
			}
			seedID := fmt.Sprintf("seed-%02d-%02d", archetypeIndex+1, replica+1)
			if _, err := WritePrivateSeed(privateRoot, seedID, seed); err != nil {
				return CommitmentManifest{}, err
			}
			if err := appendPair(archetype, SplitHidden, fmt.Sprintf("hidden-range-%02d-%02d", archetypeIndex+1, replica+1), seed); err != nil {
				return CommitmentManifest{}, err
			}
		}
	}
	protocolDigest, err := Digest(struct {
		Version string
		Format  string
	}{CommitmentVersion, "public-development-plus-private-paired-evaluation"})
	if err != nil {
		return CommitmentManifest{}, err
	}
	manifest := CommitmentManifest{Version: CommitmentVersion, ProtocolDigest: protocolDigest, Commitments: commitments}
	if err := manifest.Validate(); err != nil {
		return CommitmentManifest{}, err
	}
	return manifest.Canonicalized(), nil
}
