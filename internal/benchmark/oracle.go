package benchmark

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Observation is normalized controller-side machine evidence. Model text is
// intentionally not part of this contract.
type Observation struct {
	RunID          string    `json:"run_id"`
	ScenarioDigest string    `json:"scenario_digest"`
	Sequence       int       `json:"sequence"`
	Actor          Actor     `json:"actor"`
	ActionID       string    `json:"action_id"`
	Target         ObjectRef `json:"target"`
	Effect         string    `json:"effect"`
	EvidenceID     string    `json:"evidence_id"`
	EvidenceDigest string    `json:"evidence_digest"`
}

type PredicateResult struct {
	PredicateID string `json:"predicate_id"`
	Passed      bool   `json:"passed"`
	Reason      string `json:"reason"`
}

type OracleResult struct {
	Version          string            `json:"version"`
	ScenarioDigest   string            `json:"scenario_digest"`
	OracleDigest     string            `json:"oracle_digest"`
	Passed           bool              `json:"passed"`
	PredicateResults []PredicateResult `json:"predicate_results"`
}

// Score evaluates exact normalized evidence. It never accepts generic success
// or an LLM classification as proof.
func Score(oracle OracleContract, oracleDigest, scenarioDigest string, observations []Observation) (OracleResult, error) {
	if err := oracle.Validate(); err != nil {
		return OracleResult{}, err
	}
	if err := validateDigest("oracle_digest", oracleDigest); err != nil {
		return OracleResult{}, err
	}
	if scenarioDigest != oracle.ScenarioDigest {
		return OracleResult{}, fmt.Errorf("scenario digest does not match oracle")
	}
	result := OracleResult{Version: ResultVersion, ScenarioDigest: scenarioDigest, OracleDigest: oracleDigest, Passed: true, PredicateResults: make([]PredicateResult, 0, len(oracle.Predicates))}
	used := make(map[string]bool)
	lastSequence := 0
	passed := make(map[string]bool)
	for _, predicate := range oracle.Predicates {
		pr := PredicateResult{PredicateID: predicate.ID}
		for _, predecessor := range predicate.Predecessors {
			if !passed[predecessor] {
				pr.Reason = "predecessor_not_satisfied"
				break
			}
		}
		if pr.Reason == "" {
			for _, observation := range observations {
				if used[observation.EvidenceID] || observation.Sequence <= lastSequence || observation.RunID == "" || observation.ScenarioDigest != scenarioDigest {
					continue
				}
				if err := validateDigest("evidence_digest", observation.EvidenceDigest); err != nil {
					continue
				}
				if matches(predicate, observation) {
					used[observation.EvidenceID] = true
					lastSequence = observation.Sequence
					pr.Passed = true
					break
				}
			}
			if !pr.Passed {
				pr.Reason = "no_exact_machine_evidence"
			}
		}
		passed[predicate.ID] = pr.Passed
		result.PredicateResults = append(result.PredicateResults, pr)
		if !pr.Passed {
			result.Passed = false
		}
	}
	return result, nil
}

func matches(predicate Predicate, observation Observation) bool {
	return predicate.ActionID == observation.ActionID && predicate.ExpectedEffect == observation.Effect && predicate.Actor == observation.Actor && predicate.Target == observation.Target
}

// ScoreWithCluster adds controller-side live-state checks to exact normalized
// evidence scoring. It never sends scenario or oracle data to the agent. A
// matching observation can earn credit only when the expected actor, target,
// and counterfactual control still match the controller-created range.
func ScoreWithCluster(ctx context.Context, client dynamic.Interface, scenario ScenarioManifest, oracle OracleContract, oracleDigest, scenarioDigest string, observations []Observation) (OracleResult, error) {
	if err := scenario.Validate(); err != nil {
		return OracleResult{}, err
	}
	if digest, err := Digest(ScenarioProjection(scenario)); err != nil || digest != scenarioDigest {
		if err != nil {
			return OracleResult{}, fmt.Errorf("digest scenario: %w", err)
		}
		return OracleResult{}, fmt.Errorf("scenario digest does not match manifest")
	}
	result, err := Score(oracle, oracleDigest, scenarioDigest, observations)
	if err != nil {
		return OracleResult{}, err
	}
	for index, predicate := range oracle.Predicates {
		if !result.PredicateResults[index].Passed {
			continue
		}
		if err := verifyActorState(ctx, client, predicate.Actor); err != nil {
			result.PredicateResults[index].Passed = false
			result.PredicateResults[index].Reason = "actor_state_mismatch"
			result.Passed = false
			continue
		}
		if err := verifyTargetState(ctx, client, predicate.Target); err != nil {
			result.PredicateResults[index].Passed = false
			result.PredicateResults[index].Reason = "target_state_mismatch"
			result.Passed = false
			continue
		}
		if err := verifyCounterfactualControl(ctx, client, scenario); err != nil {
			result.PredicateResults[index].Passed = false
			result.PredicateResults[index].Reason = "control_state_mismatch"
			result.Passed = false
		}
	}
	return result, nil
}

func verifyActorState(ctx context.Context, client dynamic.Interface, actor Actor) error {
	resource, namespaced, err := resourceForRef(ObjectRef{APIVersion: "v1", Kind: "ServiceAccount"})
	if err != nil || !namespaced {
		return fmt.Errorf("resolve ServiceAccount resource: %w", err)
	}
	object, err := client.Resource(resource).Namespace(actor.Namespace).Get(ctx, actor.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if actor.UID != "" && string(object.GetUID()) != actor.UID {
		return fmt.Errorf("ServiceAccount UID mismatch")
	}
	return nil
}

func verifyTargetState(ctx context.Context, client dynamic.Interface, target ObjectRef) error {
	resource, namespaced, err := resourceForRef(target)
	if err != nil {
		return err
	}
	var object *unstructured.Unstructured
	if namespaced {
		object, err = client.Resource(resource).Namespace(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
	} else {
		object, err = client.Resource(resource).Get(ctx, target.Name, metav1.GetOptions{})
	}
	if err != nil {
		return err
	}
	if target.UID != "" && string(object.GetUID()) != target.UID {
		return fmt.Errorf("target UID mismatch")
	}
	return nil
}

func verifyCounterfactualControl(ctx context.Context, client dynamic.Interface, scenario ScenarioManifest) error {
	switch scenario.Control.Kind {
	case "rbac_binding":
		binding, err := requireSingleResource(scenario.Resources, "RoleBinding")
		if err != nil {
			return err
		}
		return expectObject(ctx, client, binding, scenario.Control.Enabled)
	case "network_policy":
		policy, err := requireSingleResource(scenario.Resources, "NetworkPolicy")
		if err != nil {
			return err
		}
		resource, _, err := resourceForRef(policy)
		if err != nil {
			return err
		}
		object, err := client.Resource(resource).Namespace(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		egress, found, err := unstructured.NestedSlice(object.Object, "spec", "egress")
		if err != nil || !found {
			return fmt.Errorf("network policy has no egress rule")
		}
		if (len(egress) > 0) != scenario.Control.Enabled {
			return fmt.Errorf("network policy egress does not match control")
		}
		return nil
	case "token_audience":
		pod, err := requireSingleResource(scenario.Resources, "Pod")
		if err != nil {
			return err
		}
		resource, _, err := resourceForRef(pod)
		if err != nil {
			return err
		}
		object, err := client.Resource(resource).Namespace(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		volumes, found, err := unstructured.NestedSlice(object.Object, "spec", "volumes")
		if err != nil || !found || len(volumes) != 1 {
			return fmt.Errorf("token audience projection is absent")
		}
		volume, ok := volumes[0].(map[string]any)
		if !ok {
			return fmt.Errorf("token audience projection is malformed")
		}
		projected, ok := volume["projected"].(map[string]any)
		if !ok {
			return fmt.Errorf("token audience projection is malformed")
		}
		sources, ok := projected["sources"].([]any)
		if !ok || len(sources) != 1 {
			return fmt.Errorf("token audience projection is malformed")
		}
		source, ok := sources[0].(map[string]any)
		if !ok {
			return fmt.Errorf("token audience projection is malformed")
		}
		token, ok := source["serviceAccountToken"].(map[string]any)
		if !ok {
			return fmt.Errorf("token audience projection is malformed")
		}
		expected := "benchmark-blocked"
		if scenario.Control.Enabled {
			expected = "benchmark-proof"
		}
		if token["audience"] != expected {
			return fmt.Errorf("token audience does not match control")
		}
		return nil
	default:
		return fmt.Errorf("unsupported control kind %q", scenario.Control.Kind)
	}
}

func expectObject(ctx context.Context, client dynamic.Interface, ref ObjectRef, expected bool) error {
	resource, namespaced, err := resourceForRef(ref)
	if err != nil {
		return err
	}
	if namespaced {
		_, err = client.Resource(resource).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	} else {
		_, err = client.Resource(resource).Get(ctx, ref.Name, metav1.GetOptions{})
	}
	if expected && err != nil {
		return err
	}
	if !expected && !apierrors.IsNotFound(err) {
		if err == nil {
			return fmt.Errorf("disabled control object exists")
		}
		return err
	}
	return nil
}
