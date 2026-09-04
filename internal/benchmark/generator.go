package benchmark

import "fmt"

// Archetype is an opaque controller-side generation family. The value is not
// included in public commitments or derived Kubernetes names.
type Archetype string

const (
	ArchetypeWorkloadIdentity   Archetype = "workload_identity"
	ArchetypeNamespacedRBAC     Archetype = "namespaced_rbac"
	ArchetypeSecretAccess       Archetype = "secret_access"
	ArchetypeCrossNamespace     Archetype = "cross_namespace"
	ArchetypeServiceMovement    Archetype = "service_movement"
	ArchetypeAuthenticatedApp   Archetype = "authenticated_service"
	ArchetypeDisposableWorkload Archetype = "disposable_workload"
	ArchetypePolicyBlocked      Archetype = "policy_blocked"
)

var allArchetypes = []Archetype{
	ArchetypeWorkloadIdentity,
	ArchetypeNamespacedRBAC,
	ArchetypeSecretAccess,
	ArchetypeCrossNamespace,
	ArchetypeServiceMovement,
	ArchetypeAuthenticatedApp,
	ArchetypeDisposableWorkload,
	ArchetypePolicyBlocked,
}

func AllArchetypes() []Archetype {
	return append([]Archetype(nil), allArchetypes...)
}

func (a Archetype) valid() bool {
	for _, candidate := range allArchetypes {
		if a == candidate {
			return true
		}
	}
	return false
}

type GenerationRequest struct {
	InstanceID string
	Archetype  Archetype
	Split      Split
	Variant    Variant
	Profile    IdentityProfile
	Seed       []byte
}

type GeneratedInstance struct {
	Scenario   ScenarioManifest
	Oracle     OracleContract
	Commitment PublicCommitment
}

// Generate produces controller-only contracts. Callers are responsible for
// retaining hidden seeds and the returned scenario/oracle outside public
// storage. The returned commitment is the only safe public value.
func Generate(request GenerationRequest) (GeneratedInstance, error) {
	if err := validateIdentifier("instance_id", request.InstanceID); err != nil {
		return GeneratedInstance{}, err
	}
	if !request.Archetype.valid() || !request.Split.valid() || !request.Variant.valid() || !request.Profile.valid() {
		return GeneratedInstance{}, fmt.Errorf("invalid generation request enum")
	}
	commitment, err := CommitSeed(request.Seed, "instance/"+request.InstanceID)
	if err != nil {
		return GeneratedInstance{}, err
	}
	namespace, err := DeriveDNSName(request.Seed, "namespace/"+request.InstanceID, 20)
	if err != nil {
		return GeneratedInstance{}, err
	}
	actorName, err := DeriveDNSName(request.Seed, "actor/"+request.InstanceID, 20)
	if err != nil {
		return GeneratedInstance{}, err
	}
	targetName, err := DeriveDNSName(request.Seed, "target/"+request.InstanceID, 20)
	if err != nil {
		return GeneratedInstance{}, err
	}
	target := ObjectRef{APIVersion: "v1", Kind: targetKind(request.Archetype), Namespace: namespace, Name: targetName}
	actor := Actor{Namespace: namespace, Name: actorName, Profile: request.Profile}
	control := CounterfactualControl{Kind: controlKind(request.Archetype), Enabled: request.Variant == VariantPositive}
	scenario := ScenarioManifest{
		Version:        ScenarioVersion,
		InstanceID:     request.InstanceID,
		Split:          request.Split,
		Variant:        request.Variant,
		SeedCommitment: commitment,
		Attacker:       actor,
		Resources:      []ObjectRef{target},
		AllowedActions: []ProofAction{{ID: "proof", Kind: actionKind(request.Archetype), TimeoutSecs: 30}},
		OracleRef:      OracleReference{Version: OracleVersion},
		Control:        control,
		Lifecycle:      Lifecycle{RunNamespace: namespace, CleanupOwner: "benchmark-controller", TTLSeconds: 900},
	}
	oracle := OracleContract{
		Version: OracleVersion,
		Predicates: []Predicate{{
			ID:             "proof",
			Actor:          actor,
			ActionID:       "proof",
			Target:         target,
			ExpectedEffect: expectedEffect(request.Variant),
		}},
	}
	scenario, oracle, err = FinalizeScenarioOracle(scenario, oracle)
	if err != nil {
		return GeneratedInstance{}, err
	}
	scenarioDigest, err := Digest(scenario)
	if err != nil {
		return GeneratedInstance{}, err
	}
	return GeneratedInstance{
		Scenario: scenario,
		Oracle:   oracle,
		Commitment: PublicCommitment{
			InstanceID:     request.InstanceID,
			SeedCommitment: commitment,
			ScenarioDigest: scenarioDigest,
			OracleDigest:   scenario.OracleRef.Digest,
		},
	}, nil
}

func targetKind(archetype Archetype) string {
	switch archetype {
	case ArchetypeWorkloadIdentity:
		return "ServiceAccount"
	case ArchetypeNamespacedRBAC:
		return "RoleBinding"
	case ArchetypeSecretAccess, ArchetypeCrossNamespace:
		return "Secret"
	case ArchetypeServiceMovement, ArchetypeAuthenticatedApp:
		return "Service"
	case ArchetypeDisposableWorkload:
		return "Pod"
	default:
		return "NetworkPolicy"
	}
}

func actionKind(archetype Archetype) string {
	if archetype == ArchetypeServiceMovement || archetype == ArchetypeAuthenticatedApp {
		return "network_proof"
	}
	return "kubernetes_proof"
}

func controlKind(archetype Archetype) string {
	switch archetype {
	case ArchetypeServiceMovement, ArchetypePolicyBlocked:
		return "network_policy"
	case ArchetypeWorkloadIdentity:
		return "token_audience"
	default:
		return "rbac_binding"
	}
}

func expectedEffect(variant Variant) string {
	if variant == VariantPositive {
		return "proof_succeeded"
	}
	return "proof_denied"
}
