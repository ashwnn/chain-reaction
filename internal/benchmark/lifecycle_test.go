package benchmark

import (
	"context"
	"fmt"
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
	for index, archetype := range AllArchetypes() {
		for _, variant := range []Variant{VariantPositive, VariantBlocked} {
			t.Run(fmt.Sprintf("%s/%s", archetype, variant), func(t *testing.T) {
				instance, err := Generate(GenerationRequest{InstanceID: fmt.Sprintf("kind-lifecycle-%02d", index), Archetype: archetype, Split: SplitHidden, Variant: variant, Profile: IdentityLeastPrivilege, Seed: []byte("kind-lifecycle-controller-private-seed")})
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
				service, err := requireSingleResource(instance.Scenario.Resources, "Service")
				if err != nil {
					t.Fatal(err)
				}
				endpoints := schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}
				if _, err := client.Resource(endpoints).Namespace(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{}); err != nil {
					t.Fatalf("service endpoints missing: %v", err)
				}
				if err := CleanupAppliedRange(ctx, client, applied); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
