package benchmark

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RenderedRange is controller-only in-memory Kubernetes input. It must not be
// written below public fixture paths for hidden instances.
type RenderedRange struct {
	Objects []unstructured.Unstructured
}

// RenderKubernetes deterministically renders scenario objects without creating
// cluster resources or requiring a Kubernetes client.
func RenderKubernetes(instance GeneratedInstance) (RenderedRange, error) {
	if err := instance.Scenario.Validate(); err != nil {
		return RenderedRange{}, err
	}
	if err := instance.Oracle.Validate(); err != nil {
		return RenderedRange{}, err
	}
	s := instance.Scenario
	ns := s.Lifecycle.RunNamespace
	objects := []unstructured.Unstructured{
		resource("v1", "Namespace", "", ns, map[string]any{"labels": map[string]any{"app.kubernetes.io/managed-by": "chain-reaction-controller"}}),
		resource("v1", "ServiceAccount", ns, s.Attacker.Name, nil),
		resource("rbac.authorization.k8s.io/v1", "Role", ns, "range-reader", map[string]any{"rules": []any{map[string]any{"apiGroups": []any{""}, "resources": []any{"secrets", "configmaps", "services"}, "verbs": []any{"get"}}}}),
		resource("v1", "Pod", ns, "workload", map[string]any{"labels": map[string]any{"app": "range-workload"}, "spec": map[string]any{"serviceAccountName": s.Attacker.Name, "containers": []any{map[string]any{"name": "workload", "image": "registry.k8s.io/pause:3.10"}}}}),
		resource("v1", "Secret", ns, "proof-material", map[string]any{"type": "Opaque"}),
		resource("v1", "Service", ns, "proof-service", map[string]any{"spec": map[string]any{"selector": map[string]any{"app": "range-workload"}, "ports": []any{map[string]any{"port": int64(s.NetworkPort), "targetPort": int64(s.NetworkPort)}}}}),
		resource("v1", "ConfigMap", ns, "decoy", map[string]any{"data": map[string]any{"notice": "not proof material"}}),
	}
	if s.Control.Kind == "rbac_binding" && s.Control.Enabled {
		objects = append(objects, resource("rbac.authorization.k8s.io/v1", "RoleBinding", ns, "range-reader", map[string]any{"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": s.Attacker.Name, "namespace": ns}}, "roleRef": map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": "range-reader"}}))
	}
	if s.Control.Kind == "network_policy" {
		egress := []any{}
		if s.Control.Enabled {
			egress = []any{map[string]any{}}
		}
		objects = append(objects, resource("networking.k8s.io/v1", "NetworkPolicy", ns, "proof-egress", map[string]any{"spec": map[string]any{"podSelector": map[string]any{}, "policyTypes": []any{"Egress"}, "egress": egress}}))
	}
	for _, target := range s.Resources {
		if target.Name == "" {
			return RenderedRange{}, fmt.Errorf("scenario resource has no name")
		}
		if !hasObject(objects, target.Kind, target.Name) {
			objects = append(objects, resource(target.APIVersion, target.Kind, target.Namespace, target.Name, nil))
		}
	}
	return RenderedRange{Objects: objects}, nil
}

func resource(apiVersion, kind, namespace, name string, fields map[string]any) unstructured.Unstructured {
	metadata := map[string]any{"name": name}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	object := map[string]any{"apiVersion": apiVersion, "kind": kind, "metadata": metadata}
	for key, value := range fields {
		if key == "labels" {
			metadata["labels"] = value
			continue
		}
		object[key] = value
	}
	return unstructured.Unstructured{Object: object}
}

func hasObject(objects []unstructured.Unstructured, kind, name string) bool {
	for _, object := range objects {
		if strings.EqualFold(object.GetKind(), kind) && object.GetName() == name {
			return true
		}
	}
	return false
}
