package benchmark

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestVerifyAppliedRangeRequiresMatchingUID(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), lifecycleObject("range-a", "proof-material", "uid-original"))
	applied := AppliedRange{Objects: []ObjectRef{{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "proof-material", UID: "uid-original"}}}
	if err := VerifyAppliedRange(context.Background(), client, applied); err != nil {
		t.Fatalf("matching object rejected: %v", err)
	}
	applied.Objects[0].UID = "uid-replacement"
	if err := VerifyAppliedRange(context.Background(), client, applied); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("mismatched UID accepted: %v", err)
	}
}

func TestCleanupAppliedRangeRejectsReplacementResidue(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), lifecycleObject("range-a", "proof-material", "uid-replacement"))
	applied := AppliedRange{Objects: []ObjectRef{{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "proof-material", UID: "uid-original"}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := CleanupAppliedRange(ctx, client, applied)
	if err == nil || !strings.Contains(err.Error(), "replacement object remains") {
		t.Fatalf("replacement object was not rejected: %v", err)
	}
}

func TestCleanupAppliedRangeRemovesMatchingObject(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), lifecycleObject("range-a", "proof-material", "uid-original"))
	applied := AppliedRange{Objects: []ObjectRef{{APIVersion: "v1", Kind: "Secret", Namespace: "range-a", Name: "proof-material", UID: "uid-original"}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := CleanupAppliedRange(ctx, client, applied); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	resource := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	if _, err := client.Resource(resource).Namespace("range-a").Get(ctx, "proof-material", metav1.GetOptions{}); err == nil {
		t.Fatal("object remains after cleanup")
	}
}

func lifecycleObject(namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"namespace": namespace,
			"name":      name,
			"uid":       uid,
		},
	}}
}
