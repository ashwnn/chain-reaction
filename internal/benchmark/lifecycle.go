package benchmark

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// AppliedRange records only controller-side object references. It contains no
// raw seed or oracle predicate data and is safe to retain with run metadata.
type AppliedRange struct {
	Objects []ObjectRef `json:"objects"`
}

// ApplyRenderedRange creates a range in renderer order. It fails if any object
// already exists, preventing one run from silently reusing another run's state.
func ApplyRenderedRange(ctx context.Context, client dynamic.Interface, rendered RenderedRange) (AppliedRange, error) {
	applied := AppliedRange{Objects: make([]ObjectRef, 0, len(rendered.Objects))}
	for _, object := range rendered.Objects {
		resource, namespaced, err := resourceFor(object)
		if err != nil {
			return AppliedRange{}, err
		}
		var created *unstructured.Unstructured
		if namespaced {
			created, err = client.Resource(resource).Namespace(object.GetNamespace()).Create(ctx, object.DeepCopy(), metav1.CreateOptions{})
		} else {
			created, err = client.Resource(resource).Create(ctx, object.DeepCopy(), metav1.CreateOptions{})
		}
		if err != nil {
			_ = CleanupAppliedRange(ctx, client, applied)
			return AppliedRange{}, fmt.Errorf("create %s/%s: %w", object.GetKind(), object.GetName(), err)
		}
		applied.Objects = append(applied.Objects, ObjectRef{APIVersion: created.GetAPIVersion(), Kind: created.GetKind(), Namespace: created.GetNamespace(), Name: created.GetName(), UID: string(created.GetUID())})
	}
	return applied, nil
}

// CleanupAppliedRange deletes children before their namespace. NotFound is
// accepted so failed or partially cleaned runs can be retried safely.
func CleanupAppliedRange(ctx context.Context, client dynamic.Interface, applied AppliedRange) error {
	for index := len(applied.Objects) - 1; index >= 0; index-- {
		object := applied.Objects[index]
		resource, namespaced, err := resourceForRef(object)
		if err != nil {
			return err
		}
		if namespaced {
			err = client.Resource(resource).Namespace(object.Namespace).Delete(ctx, object.Name, metav1.DeleteOptions{})
		} else {
			err = client.Resource(resource).Delete(ctx, object.Name, metav1.DeleteOptions{})
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s/%s: %w", object.Kind, object.Name, err)
		}
	}
	return nil
}

func resourceFor(object unstructured.Unstructured) (schema.GroupVersionResource, bool, error) {
	return resourceForRef(ObjectRef{APIVersion: object.GetAPIVersion(), Kind: object.GetKind()})
}

func resourceForRef(ref ObjectRef) (schema.GroupVersionResource, bool, error) {
	key := ref.APIVersion + "/" + ref.Kind
	resources := map[string]struct {
		resource   schema.GroupVersionResource
		namespaced bool
	}{
		"v1/Namespace":                      {schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, false},
		"v1/ServiceAccount":                 {schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, true},
		"v1/Pod":                            {schema.GroupVersionResource{Version: "v1", Resource: "pods"}, true},
		"v1/Secret":                         {schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, true},
		"v1/Service":                        {schema.GroupVersionResource{Version: "v1", Resource: "services"}, true},
		"v1/Endpoints":                      {schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}, true},
		"v1/ConfigMap":                      {schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, true},
		"rbac.authorization.k8s.io/v1/Role": {schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, true},
		"rbac.authorization.k8s.io/v1/RoleBinding": {schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}, true},
		"networking.k8s.io/v1/NetworkPolicy":       {schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, true},
	}
	value, ok := resources[key]
	if !ok {
		return schema.GroupVersionResource{}, false, fmt.Errorf("unsupported rendered resource %s", key)
	}
	return value.resource, value.namespaced, nil
}
