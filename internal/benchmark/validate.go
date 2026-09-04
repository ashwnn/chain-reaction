package benchmark

import (
	"fmt"
	"sort"
)

func (m ScenarioManifest) Validate() error {
	if m.Version != ScenarioVersion {
		return fmt.Errorf("unsupported scenario version %q", m.Version)
	}
	if err := validateIdentifier("instance_id", m.InstanceID); err != nil {
		return err
	}
	if !m.Split.valid() || !m.Variant.valid() {
		return fmt.Errorf("scenario split or variant is invalid")
	}
	if err := validateDigest("seed_commitment", m.SeedCommitment); err != nil {
		return err
	}
	if m.NetworkPort < 1024 || m.NetworkPort > 65535 {
		return fmt.Errorf("network_port must be between 1024 and 65535")
	}
	if err := validateActor("attacker", m.Attacker); err != nil {
		return err
	}
	if len(m.Resources) == 0 || len(m.AllowedActions) == 0 {
		return fmt.Errorf("scenario requires resources and allowed actions")
	}
	seenResources := make(map[string]struct{}, len(m.Resources))
	for index, ref := range m.Resources {
		if err := validateObjectRef(fmt.Sprintf("resources[%d]", index), ref); err != nil {
			return err
		}
		key := ref.APIVersion + "|" + ref.Kind + "|" + ref.Namespace + "|" + ref.Name
		if _, ok := seenResources[key]; ok {
			return fmt.Errorf("resources contains duplicate object %q", key)
		}
		seenResources[key] = struct{}{}
	}
	seenActions := make(map[string]struct{}, len(m.AllowedActions))
	for index, action := range m.AllowedActions {
		if err := validateIdentifier(fmt.Sprintf("allowed_actions[%d].id", index), action.ID); err != nil {
			return err
		}
		if err := validateIdentifier(fmt.Sprintf("allowed_actions[%d].kind", index), action.Kind); err != nil {
			return err
		}
		if action.TimeoutSecs < 1 || action.TimeoutSecs > 300 {
			return fmt.Errorf("allowed_actions[%d].timeout_seconds must be between 1 and 300", index)
		}
		if _, ok := seenActions[action.ID]; ok {
			return fmt.Errorf("allowed_actions contains duplicate id %q", action.ID)
		}
		seenActions[action.ID] = struct{}{}
	}
	if m.OracleRef.Version != OracleVersion {
		return fmt.Errorf("unsupported oracle reference version %q", m.OracleRef.Version)
	}
	if err := validateDigest("oracle_ref.digest", m.OracleRef.Digest); err != nil {
		return err
	}
	if err := validateIdentifier("control.kind", m.Control.Kind); err != nil {
		return err
	}
	if (m.Variant == VariantPositive) != m.Control.Enabled {
		return fmt.Errorf("scenario variant and control.enabled disagree")
	}
	if err := validateIdentifier("lifecycle.run_namespace", m.Lifecycle.RunNamespace); err != nil {
		return err
	}
	if err := validateIdentifier("lifecycle.cleanup_owner", m.Lifecycle.CleanupOwner); err != nil {
		return err
	}
	if m.Lifecycle.TTLSeconds < 1 || m.Lifecycle.TTLSeconds > 86400 {
		return fmt.Errorf("lifecycle.ttl_seconds must be between 1 and 86400")
	}
	return nil
}

func (o OracleContract) Validate() error {
	if o.Version != OracleVersion {
		return fmt.Errorf("unsupported oracle version %q", o.Version)
	}
	if err := validateDigest("scenario_digest", o.ScenarioDigest); err != nil {
		return err
	}
	if len(o.Predicates) == 0 {
		return fmt.Errorf("oracle requires predicates")
	}
	seen := make(map[string]struct{}, len(o.Predicates))
	for index, predicate := range o.Predicates {
		if err := validateIdentifier(fmt.Sprintf("predicates[%d].id", index), predicate.ID); err != nil {
			return err
		}
		if _, ok := seen[predicate.ID]; ok {
			return fmt.Errorf("oracle contains duplicate predicate id %q", predicate.ID)
		}
		seen[predicate.ID] = struct{}{}
		if err := validateActor(fmt.Sprintf("predicates[%d].actor", index), predicate.Actor); err != nil {
			return err
		}
		if err := validateIdentifier(fmt.Sprintf("predicates[%d].action_id", index), predicate.ActionID); err != nil {
			return err
		}
		if err := validateObjectRef(fmt.Sprintf("predicates[%d].target", index), predicate.Target); err != nil {
			return err
		}
		if err := validateIdentifier(fmt.Sprintf("predicates[%d].expected_effect", index), predicate.ExpectedEffect); err != nil {
			return err
		}
	}
	for index, predicate := range o.Predicates {
		for _, predecessor := range predicate.Predecessors {
			if predecessor == predicate.ID {
				return fmt.Errorf("predicates[%d] cannot depend on itself", index)
			}
			if _, ok := seen[predecessor]; !ok {
				return fmt.Errorf("predicates[%d] references unknown predecessor %q", index, predecessor)
			}
		}
	}
	return nil
}

func (m CommitmentManifest) Validate() error {
	if m.Version != CommitmentVersion {
		return fmt.Errorf("unsupported commitment version %q", m.Version)
	}
	if err := validateDigest("protocol_digest", m.ProtocolDigest); err != nil {
		return err
	}
	if len(m.Commitments) == 0 {
		return fmt.Errorf("commitment manifest requires commitments")
	}
	seen := make(map[string]struct{}, len(m.Commitments))
	for index, commitment := range m.Commitments {
		if err := validateIdentifier(fmt.Sprintf("commitments[%d].instance_id", index), commitment.InstanceID); err != nil {
			return err
		}
		if _, ok := seen[commitment.InstanceID]; ok {
			return fmt.Errorf("commitments contains duplicate instance id %q", commitment.InstanceID)
		}
		seen[commitment.InstanceID] = struct{}{}
		for label, digest := range map[string]string{
			"seed_commitment": commitment.SeedCommitment,
			"scenario_digest": commitment.ScenarioDigest,
			"oracle_digest":   commitment.OracleDigest,
		} {
			if err := validateDigest(fmt.Sprintf("commitments[%d].%s", index, label), digest); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m CommitmentManifest) Canonicalized() CommitmentManifest {
	copy := m
	copy.Commitments = append([]PublicCommitment(nil), m.Commitments...)
	sort.Slice(copy.Commitments, func(i, j int) bool {
		return copy.Commitments[i].InstanceID < copy.Commitments[j].InstanceID
	})
	return copy
}

func (m RunManifest) Validate() error {
	if m.Version != RunVersion {
		return fmt.Errorf("unsupported run version %q", m.Version)
	}
	for label, value := range map[string]string{
		"run_id":             m.RunID,
		"git_sha":            m.GitSHA,
		"image_digest":       m.ImageDigest,
		"scenario_digest":    m.ScenarioDigest,
		"oracle_digest":      m.OracleDigest,
		"prompt_digest":      m.PromptDigest,
		"tool_policy_digest": m.ToolPolicyDigest,
		"seed_commitment":    m.SeedCommitment,
	} {
		if label == "run_id" || label == "git_sha" || label == "image_digest" {
			if err := validateIdentifier(label, value); err != nil {
				return err
			}
			continue
		}
		if err := validateDigest(label, value); err != nil {
			return err
		}
	}
	return nil
}
