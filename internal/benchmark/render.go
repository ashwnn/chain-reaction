package benchmark

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RenderedRange is controller-only in-memory Kubernetes input. It must not be
// written below public fixture paths for hidden instances.
type RenderedRange struct {
	Objects []unstructured.Unstructured
}

// RenderKubernetes deterministically renders each generated resource. Every
// Kubernetes name derives from the controller seed retained in the scenario.
func RenderKubernetes(instance GeneratedInstance) (RenderedRange, error) {
	if err := instance.Scenario.Validate(); err != nil {
		return RenderedRange{}, err
	}
	if err := instance.Oracle.Validate(); err != nil {
		return RenderedRange{}, err
	}
	s := instance.Scenario
	attacker := ObjectRef{APIVersion: "v1", Kind: "ServiceAccount", Namespace: s.Attacker.Namespace, Name: s.Attacker.Name}
	if !containsResource(s.Resources, attacker) {
		return RenderedRange{}, fmt.Errorf("scenario does not include attacker ServiceAccount")
	}
	role, err := requireSingleResource(s.Resources, "Role")
	if err != nil {
		return RenderedRange{}, err
	}
	binding, err := requireSingleResource(s.Resources, "RoleBinding")
	if err != nil {
		return RenderedRange{}, err
	}
	pod, err := requireSingleResource(s.Resources, "Pod")
	if err != nil {
		return RenderedRange{}, err
	}
	secret, err := requireSingleResource(s.Resources, "Secret")
	if err != nil {
		return RenderedRange{}, err
	}
	service, err := requireSingleResource(s.Resources, "Service")
	if err != nil {
		return RenderedRange{}, err
	}
	decoy, err := requireSingleResource(s.Resources, "ConfigMap")
	if err != nil {
		return RenderedRange{}, err
	}
	policy, err := requireSingleResource(s.Resources, "NetworkPolicy")
	if err != nil {
		return RenderedRange{}, err
	}

	objects := make([]unstructured.Unstructured, 0, len(s.Resources))
	namespaces := resourcesByKind(s.Resources, "Namespace")
	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Name < namespaces[j].Name })
	for _, namespace := range namespaces {
		objects = append(objects, resource(namespace, map[string]any{"labels": map[string]any{"app.kubernetes.io/managed-by": "chain-reaction-controller"}}))
	}
	objects = append(objects,
		resource(attacker, nil),
		resource(role, map[string]any{"rules": []any{map[string]any{"apiGroups": []any{""}, "resources": []any{"secrets", "configmaps", "services"}, "verbs": []any{"get"}}}}),
		resource(pod, podFields(s, attacker)),
		resource(secret, map[string]any{"type": "Opaque"}),
		resource(service, map[string]any{"spec": map[string]any{"selector": map[string]any{"app": "range-workload"}, "ports": []any{map[string]any{"port": int64(s.NetworkPort), "targetPort": int64(s.NetworkPort)}}}}),
		resource(decoy, map[string]any{"data": map[string]any{"notice": "not proof material"}}),
	)
	if s.Control.Kind == "rbac_binding" && s.Control.Enabled {
		objects = append(objects, resource(binding, map[string]any{"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": s.Attacker.Name, "namespace": s.Attacker.Namespace}}, "roleRef": map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": role.Name}}))
	}
	if s.Control.Kind == "network_policy" {
		egress := []any{}
		if s.Control.Enabled {
			egress = []any{map[string]any{}}
		}
		objects = append(objects, resource(policy, map[string]any{"spec": map[string]any{"podSelector": map[string]any{}, "policyTypes": []any{"Egress"}, "egress": egress}}))
	}
	return RenderedRange{Objects: objects}, nil
}

func podFields(s ScenarioManifest, attacker ObjectRef) map[string]any {
	fields := map[string]any{"labels": map[string]any{"app": "range-workload"}, "spec": map[string]any{"serviceAccountName": attacker.Name, "containers": []any{map[string]any{"name": "workload", "image": "registry.k8s.io/pause:3.10"}}}}
	if s.Control.Kind == "token_audience" {
		audience := "benchmark-proof"
		if !s.Control.Enabled {
			audience = "benchmark-blocked"
		}
		fields["spec"].(map[string]any)["volumes"] = []any{map[string]any{
			"name": "identity",
			"projected": map[string]any{"sources": []any{map[string]any{
				"serviceAccountToken": map[string]any{
					"path":              "token",
					"audience":          audience,
					"expirationSeconds": int64(600),
				},
			}}},
		}}
	}
	return fields
}

func resource(ref ObjectRef, fields map[string]any) unstructured.Unstructured {
	metadata := map[string]any{"name": ref.Name}
	if ref.Namespace != "" {
		metadata["namespace"] = ref.Namespace
	}
	object := map[string]any{"apiVersion": ref.APIVersion, "kind": ref.Kind, "metadata": metadata}
	for key, value := range fields {
		if key == "labels" {
			metadata["labels"] = value
			continue
		}
		object[key] = value
	}
	return unstructured.Unstructured{Object: object}
}

func containsResource(refs []ObjectRef, expected ObjectRef) bool {
	for _, ref := range refs {
		if ref == expected {
			return true
		}
	}
	return false
}

func resourcesByKind(refs []ObjectRef, kind string) []ObjectRef {
	var result []ObjectRef
	for _, ref := range refs {
		if ref.Kind == kind {
			result = append(result, ref)
		}
	}
	return result
}

func requireSingleResource(refs []ObjectRef, kind string) (ObjectRef, error) {
	resources := resourcesByKind(refs, kind)
	if len(resources) != 1 {
		return ObjectRef{}, fmt.Errorf("scenario requires exactly one %s, got %d", kind, len(resources))
	}
	return resources[0], nil
}

func hasObject(objects []unstructured.Unstructured, kind, name string) bool {
	for _, object := range objects {
		if object.GetKind() == kind && object.GetName() == name {
			return true
		}
	}
	return false
}
