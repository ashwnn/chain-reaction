package benchmark

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
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
			cleanupErr := CleanupAppliedRange(ctx, client, applied)
			if cleanupErr != nil {
				return AppliedRange{}, fmt.Errorf("create %s/%s: %w; cleanup failed: %v", object.GetKind(), object.GetName(), err, cleanupErr)
			}
			return AppliedRange{}, fmt.Errorf("create %s/%s: %w", object.GetKind(), object.GetName(), err)
		}
		applied.Objects = append(applied.Objects, ObjectRef{APIVersion: created.GetAPIVersion(), Kind: created.GetKind(), Namespace: created.GetNamespace(), Name: created.GetName(), UID: string(created.GetUID())})
	}
	if err := VerifyAppliedRange(ctx, client, applied); err != nil {
		cleanupErr := CleanupAppliedRange(ctx, client, applied)
		if cleanupErr != nil {
			return AppliedRange{}, fmt.Errorf("verify applied range: %w; cleanup failed: %v", err, cleanupErr)
		}
		return AppliedRange{}, fmt.Errorf("verify applied range: %w", err)
	}
	return applied, nil
}

// VerifyAppliedRange attests that every controller-created object still exists
// with the UID returned by the API server. A replacement object with the same
// name is not part of the range.
func VerifyAppliedRange(ctx context.Context, client dynamic.Interface, applied AppliedRange) error {
	for _, object := range applied.Objects {
		resource, namespaced, err := resourceForRef(object)
		if err != nil {
			return err
		}
		var current *unstructured.Unstructured
		if namespaced {
			current, err = client.Resource(resource).Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
		} else {
			current, err = client.Resource(resource).Get(ctx, object.Name, metav1.GetOptions{})
		}
		if err != nil {
			return fmt.Errorf("get %s/%s: %w", object.Kind, object.Name, err)
		}
		if object.UID == "" || string(current.GetUID()) != object.UID {
			return fmt.Errorf("identity mismatch for %s/%s", object.Kind, object.Name)
		}
	}
	return nil
}

// CleanupAppliedRange deletes children before their namespace and waits until
// no created object remains. UID preconditions prevent a retry from deleting a
// replacement object with the same name. NotFound is accepted because absence
// satisfies cleanup; a differently identified object is reported as residue.
func CleanupAppliedRange(ctx context.Context, client dynamic.Interface, applied AppliedRange) error {
	for index := len(applied.Objects) - 1; index >= 0; index-- {
		object := applied.Objects[index]
		resource, namespaced, err := resourceForRef(object)
		if err != nil {
			return err
		}
		var current *unstructured.Unstructured
		if namespaced {
			current, err = client.Resource(resource).Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
		} else {
			current, err = client.Resource(resource).Get(ctx, object.Name, metav1.GetOptions{})
		}
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("get %s/%s before delete: %w", object.Kind, object.Name, err)
		}
		if object.UID == "" || string(current.GetUID()) != object.UID {
			return fmt.Errorf("replacement object remains at %s/%s", object.Kind, object.Name)
		}
		uid := types.UID(object.UID)
		options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: ptr.To(uid)}}
		if namespaced {
			err = client.Resource(resource).Namespace(object.Namespace).Delete(ctx, object.Name, options)
		} else {
			err = client.Resource(resource).Delete(ctx, object.Name, options)
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s/%s: %w", object.Kind, object.Name, err)
		}
	}
	return WaitForNoResidue(ctx, client, applied)
}

// WaitForNoResidue waits for asynchronous namespace deletion and rejects an
// object recreated under a deleted range name. The caller owns the deadline.
func WaitForNoResidue(ctx context.Context, client dynamic.Interface, applied AppliedRange) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		residue, err := rangeResidue(ctx, client, applied)
		if err != nil {
			return err
		}
		if !residue {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("cleanup residue remains: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func rangeResidue(ctx context.Context, client dynamic.Interface, applied AppliedRange) (bool, error) {
	for _, object := range applied.Objects {
		resource, namespaced, err := resourceForRef(object)
		if err != nil {
			return false, err
		}
		var current *unstructured.Unstructured
		if namespaced {
			current, err = client.Resource(resource).Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
		} else {
			current, err = client.Resource(resource).Get(ctx, object.Name, metav1.GetOptions{})
		}
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("check cleanup residue %s/%s: %w", object.Kind, object.Name, err)
		}
		if string(current.GetUID()) != object.UID {
			return false, fmt.Errorf("replacement object remains at %s/%s", object.Kind, object.Name)
		}
		return true, nil
	}
	return false, nil
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
