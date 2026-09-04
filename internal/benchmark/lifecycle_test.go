package benchmark

import (
	"context"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func TestKindRenderedRangeLifecycle(t *testing.T) {
	if os.Getenv("CHAIN_REACTION_KIND") != "1" {
		t.Skip("set CHAIN_REACTION_KIND=1 to run against Kind")
	}
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		t.Fatal(err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := Generate(GenerationRequest{InstanceID: "kind-lifecycle-a", Archetype: ArchetypeSecretAccess, Split: SplitHidden, Variant: VariantPositive, Profile: IdentityLeastPrivilege, Seed: []byte("kind-lifecycle-controller-private-seed")})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderKubernetes(instance)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	applied, err := ApplyRenderedRange(ctx, client, rendered)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = CleanupAppliedRange(context.Background(), client, applied) }()
	ns := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	if _, err := client.Resource(ns).Get(ctx, instance.Scenario.Lifecycle.RunNamespace, metav1.GetOptions{}); err != nil {
		t.Fatalf("range namespace missing: %v", err)
	}
	endpoints := schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}
	if _, err := client.Resource(endpoints).Namespace(instance.Scenario.Lifecycle.RunNamespace).Get(ctx, "proof-service", metav1.GetOptions{}); err != nil {
		t.Fatalf("service endpoints missing: %v", err)
	}
	if err := CleanupAppliedRange(ctx, client, applied); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Resource(ns).Get(ctx, instance.Scenario.Lifecycle.RunNamespace, metav1.GetOptions{}); err != nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("range namespace remains after cleanup")
}
