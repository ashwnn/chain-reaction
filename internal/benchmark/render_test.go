package benchmark

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRenderKubernetesIsDeterministicAndKeepsPairIdentity(t *testing.T) {
	request := GenerationRequest{InstanceID: "render-pair", Archetype: ArchetypeSecretAccess, Split: SplitHidden, Variant: VariantPositive, Profile: IdentityLeastPrivilege, Seed: []byte("renderer-controller-seed-material")}
	positive, err := Generate(request)
	if err != nil {
		t.Fatal(err)
	}
	first, err := RenderKubernetes(positive)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderKubernetes(positive)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("renderer is not deterministic")
	}
	request.Variant = VariantBlocked
	blocked, err := Generate(request)
	if err != nil {
		t.Fatal(err)
	}
	blockedRange, err := RenderKubernetes(blocked)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := requireSingleResource(positive.Scenario.Resources, "RoleBinding")
	if err != nil {
		t.Fatal(err)
	}
	if hasObject(blockedRange.Objects, "RoleBinding", binding.Name) {
		t.Fatal("blocked RBAC pair includes binding")
	}
	if !hasObject(first.Objects, "RoleBinding", binding.Name) {
		t.Fatal("positive RBAC pair lacks binding")
	}
	body, err := json.Marshal(first.Objects)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Fatal("empty render")
	}
}
