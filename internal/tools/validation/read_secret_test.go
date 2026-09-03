package validation

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

func TestReadSecretParameterSchema(t *testing.T) {
	tool := NewReadSecretTool(newFakeSecretClient(t))

	schema := tool.ParameterSchema()
	if got := schema.Map()["additionalProperties"]; got != false {
		t.Fatalf("expected closed object schema, got %#v", got)
	}
	if !reflect.DeepEqual(schema.Required, []string{"name"}) {
		t.Fatalf("expected required name field, got %#v", schema.Required)
	}

	nameProperty, ok := schema.Properties["name"]
	if !ok || nameProperty.Type != "string" {
		t.Fatalf("expected string name property, got %#v", nameProperty)
	}

	namespaceProperty, ok := schema.Properties["namespace"]
	if !ok {
		t.Fatal("expected namespace property")
	}
	if namespaceProperty.Type != "string" {
		t.Fatalf("expected string namespace property, got %#v", namespaceProperty.Type)
	}
	if namespaceProperty.Default != "default" {
		t.Fatalf("expected default namespace 'default', got %#v", namespaceProperty.Default)
	}

	allowNamespacesProperty, ok := schema.Properties["allow_namespaces"]
	if !ok {
		t.Fatal("expected allow_namespaces property")
	}
	if allowNamespacesProperty.Type != "array" {
		t.Fatalf("expected array allow_namespaces property, got %#v", allowNamespacesProperty.Type)
	}
	if !reflect.DeepEqual(allowNamespacesProperty.Items, &tools.Schema{Type: "string"}) {
		t.Fatalf("expected string array items, got %#v", allowNamespacesProperty.Items)
	}
}

func TestReadSecretValidated(t *testing.T) {
	tool := NewReadSecretTool(newFakeSecretClient(t))

	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "secret-a"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "validated" {
		t.Fatalf("expected validated status, got %#v", result["status"])
	}
	secret, ok := result["secret"].(k8s.SecretReadSummary)
	if !ok {
		t.Fatalf("expected secret summary, got %T", result["secret"])
	}
	if len(secret.Keys) != 1 || secret.Keys[0].Name != "token" || secret.Keys[0].ByteCount != 3 {
		t.Fatalf("unexpected key summary: %#v", secret.Keys)
	}
}

func TestReadSecretForbidden(t *testing.T) {
	client := newFakeSecretClient(t)
	client.Clientset.(*fake.Clientset).PrependReactor("get", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "secret-a", nil)
	})
	tool := NewReadSecretTool(client)

	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "secret-a"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["reason"] != "rbac_denied" {
		t.Fatalf("expected rbac_denied, got %#v", result["reason"])
	}
}

func TestReadSecretMissing(t *testing.T) {
	tool := NewReadSecretTool(newFakeSecretClient(t))

	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "missing"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["reason"] != "secret_not_found" {
		t.Fatalf("expected secret_not_found, got %#v", result["reason"])
	}
}

func TestReadSecretGuardrailDenied(t *testing.T) {
	tool := NewReadSecretTool(newFakeSecretClient(t))

	result, err := tool.Run(context.Background(), map[string]any{
		"namespace":        "team-a",
		"name":             "secret-a",
		"allow_namespaces": []string{"team-b"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status, got %#v", result["status"])
	}
	if result["reason"] != "guardrail_blocked" {
		t.Fatalf("expected guardrail_blocked reason, got %#v", result["reason"])
	}
}

func TestReadSecretRequiresName(t *testing.T) {
	tool := NewReadSecretTool(newFakeSecretClient(t))

	_, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a"})
	if err == nil {
		t.Fatal("expected missing name error")
	}
}

func newFakeSecretClient(t *testing.T) *k8s.Client {
	t.Helper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "secret-a",
			Namespace:         "team-a",
			ResourceVersion:   "9",
			CreationTimestamp: metav1.NewTime(time.Unix(9, 0)),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"token": []byte("abc")},
	}
	return &k8s.Client{
		Config:    &rest.Config{Host: "https://example.invalid"},
		Clientset: fake.NewSimpleClientset(secret),
	}
}
