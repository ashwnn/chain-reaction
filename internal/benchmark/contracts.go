// Package benchmark defines controller-side contracts for benchmark v2.
//
// These contracts deliberately have no dependency on agent or planner code.
// The controller may use a ScenarioManifest and OracleContract to construct and
// score a range, while public artifacts use PublicCommitment only. Raw seeds,
// targets, predicates, and solution order must never be serialized into public
// commitments or planner-visible data.
package benchmark

import (
	"fmt"
	"strings"
)

const (
	ScenarioVersion    = "benchmark.scenario.v2"
	OracleVersion      = "benchmark.oracle.v2"
	RunVersion         = "benchmark.run.v2"
	CommitmentVersion  = "benchmark.commitments.v2"
	ResultVersion      = "benchmark.result.v2"
	DigestAlgorithmSHA = "sha256"
)

type Split string

const (
	SplitPublic Split = "public"
	SplitHidden Split = "hidden"
)

type Variant string

const (
	VariantPositive Variant = "positive"
	VariantBlocked  Variant = "blocked"
)

type IdentityProfile string

const (
	IdentityLeastPrivilege IdentityProfile = "least_privilege"
	IdentityNamespaceLocal IdentityProfile = "namespace_local"
	IdentityMisconfigured  IdentityProfile = "misconfigured"
	IdentityDenied         IdentityProfile = "denied"
)

// ObjectRef identifies a Kubernetes object without carrying its contents.
type ObjectRef struct {
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	UID        string `json:"uid,omitempty"`
}

// Actor identifies the ServiceAccount used to execute a scenario.
type Actor struct {
	Namespace string          `json:"namespace"`
	Name      string          `json:"name"`
	UID       string          `json:"uid,omitempty"`
	Profile   IdentityProfile `json:"profile"`
}

// ProofAction is an allow-listed, bounded controller-approved proof action.
// It is a capability description, not a planner instruction.
type ProofAction struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Mutating    bool   `json:"mutating"`
	TimeoutSecs int    `json:"timeout_seconds"`
}

// Lifecycle declares the range ownership and cleanup requirements.
type Lifecycle struct {
	RunNamespace string `json:"run_namespace"`
	CleanupOwner string `json:"cleanup_owner"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

// CounterfactualControl is the one declared condition changed between a
// positive and blocked scenario pair. It is controller-only metadata.
type CounterfactualControl struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// OracleReference binds a scenario to a controller-only oracle digest.
type OracleReference struct {
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// ScenarioManifest is controller-only. It may describe the deployed range but
// must not be passed to the agent or written to public fixtures for hidden runs.
type ScenarioManifest struct {
	Version        string                `json:"version"`
	InstanceID     string                `json:"instance_id"`
	Archetype      Archetype             `json:"archetype"`
	Split          Split                 `json:"split"`
	Variant        Variant               `json:"variant"`
	SeedCommitment string                `json:"seed_commitment"`
	NetworkPort    int                   `json:"network_port"`
	Attacker       Actor                 `json:"attacker"`
	Resources      []ObjectRef           `json:"resources"`
	AllowedActions []ProofAction         `json:"allowed_actions"`
	OracleRef      OracleReference       `json:"oracle_ref"`
	Control        CounterfactualControl `json:"control"`
	Lifecycle      Lifecycle             `json:"lifecycle"`
}

// Predicate is a controller-only machine predicate. Predicate details are not
// suitable for planner-visible output.
type Predicate struct {
	ID             string    `json:"id"`
	Actor          Actor     `json:"actor"`
	ActionID       string    `json:"action_id"`
	Target         ObjectRef `json:"target"`
	ExpectedEffect string    `json:"expected_effect"`
	Predecessors   []string  `json:"predecessors,omitempty"`
}

// OracleContract is controller-only and scored only after the run has ended.
type OracleContract struct {
	Version        string      `json:"version"`
	ScenarioDigest string      `json:"scenario_digest"`
	Predicates     []Predicate `json:"predicates"`
}

// PublicCommitment is safe to publish before evaluation. Its shape intentionally
// has no fields for raw seeds, resources, identities, predicates, or variants.
type PublicCommitment struct {
	InstanceID     string `json:"instance_id"`
	SeedCommitment string `json:"seed_commitment"`
	ScenarioDigest string `json:"scenario_digest"`
	OracleDigest   string `json:"oracle_digest"`
}

type CommitmentManifest struct {
	Version        string             `json:"version"`
	ProtocolDigest string             `json:"protocol_digest"`
	Commitments    []PublicCommitment `json:"commitments"`
}

// RunManifest is an immutable controller-produced record for one evaluation.
type RunManifest struct {
	Version          string `json:"version"`
	RunID            string `json:"run_id"`
	GitSHA           string `json:"git_sha"`
	ImageDigest      string `json:"image_digest"`
	ScenarioDigest   string `json:"scenario_digest"`
	OracleDigest     string `json:"oracle_digest"`
	PromptDigest     string `json:"prompt_digest"`
	ToolPolicyDigest string `json:"tool_policy_digest"`
	SeedCommitment   string `json:"seed_commitment"`
}

func (s Split) valid() bool {
	return s == SplitPublic || s == SplitHidden
}

func (v Variant) valid() bool {
	return v == VariantPositive || v == VariantBlocked
}

func (p IdentityProfile) valid() bool {
	return p == IdentityLeastPrivilege || p == IdentityNamespaceLocal || p == IdentityMisconfigured || p == IdentityDenied
}

func validateIdentifier(label, value string) error {
	if value = strings.TrimSpace(value); value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > 253 {
		return fmt.Errorf("%s exceeds 253 bytes", label)
	}
	return nil
}

func validateDigest(label, digest string) error {
	if len(digest) != 64 {
		return fmt.Errorf("%s must be a lowercase sha256 digest", label)
	}
	for _, r := range digest {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return fmt.Errorf("%s must be a lowercase sha256 digest", label)
		}
	}
	return nil
}

func validateObjectRef(label string, ref ObjectRef) error {
	for field, value := range map[string]string{
		"api_version": ref.APIVersion,
		"kind":        ref.Kind,
		"name":        ref.Name,
	} {
		if err := validateIdentifier(label+"."+field, value); err != nil {
			return err
		}
	}
	return nil
}

func validateActor(label string, actor Actor) error {
	if err := validateIdentifier(label+".namespace", actor.Namespace); err != nil {
		return err
	}
	if err := validateIdentifier(label+".name", actor.Name); err != nil {
		return err
	}
	if !actor.Profile.valid() {
		return fmt.Errorf("%s.profile is invalid", label)
	}
	return nil
}
