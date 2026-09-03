package validation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"

	"github.com/ashwnn/chain-reaction/internal/k8s"
)

// Test helper: creates a fake k8s client with a ServiceAccount.
func newFakeSATokenClient(t *testing.T, ns, saName string) *k8s.Client {
	t.Helper()
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            saName,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		AutomountServiceAccountToken: boolPtr(true),
	}
	return &k8s.Client{
		Config:    &rest.Config{Host: "https://example.invalid"},
		Clientset: fake.NewSimpleClientset(sa),
	}
}

func boolPtr(b bool) *bool { return &b }

func TestCheckTokenParameterSchema(t *testing.T) {
	tool := NewCheckTokenTool(newFakeSATokenClient(t, "default", "default-sa"))
	schema := tool.ParameterSchema()

	if schema.Type != "object" {
		t.Fatalf("expected object schema, got %q", schema.Type)
	}
	if got := schema.Map()["additionalProperties"]; got != false {
		t.Fatalf("expected closed object schema, got %#v", got)
	}
	if len(schema.Required) != 0 {
		t.Fatalf("expected optional name field, got %#v", schema.Required)
	}

	nameProp, ok := schema.Properties["name"]
	if !ok || nameProp.Type != "string" {
		t.Fatalf("expected string name property, got %#v", nameProp)
	}

	nsProp, ok := schema.Properties["namespace"]
	if !ok || nsProp.Type != "string" {
		t.Fatalf("expected string namespace property, got %#v", nsProp.Type)
	}
	if nsProp.Default != "default" {
		t.Fatalf("expected default namespace 'default', got %#v", nsProp.Default)
	}
}

func TestCheckTokenValidatedSA(t *testing.T) {
	// Create a temp file with a valid (non-expired) JWT so the tool can read the token.
	tmpFile := writeValidToken(t, "team-a", "my-sa", "abc123", 4102444800)
	defer os.Remove(tmpFile)

	// Override the token path for this test.
	origPath := mountedTokenPath
	mountedTokenPath = tmpFile
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "my-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "my-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "validated" {
		t.Fatalf("expected validated status, got %#v", result["status"])
	}
	if result["reason"] != "service_account_inspected" {
		t.Fatalf("expected service_account_inspected reason, got %#v", result["reason"])
	}
	sa, ok := result["service_account"].(k8s.ServiceAccountDetail)
	if !ok {
		t.Fatalf("expected service_account detail, got %T", result["service_account"])
	}
	if sa.Name != "my-sa" {
		t.Fatalf("expected SA name my-sa, got %s", sa.Name)
	}
	if sa.Namespace != "team-a" {
		t.Fatalf("expected SA namespace team-a, got %s", sa.Namespace)
	}
	if sa.AutomountToken != true {
		t.Fatalf("expected automount_token=true, got %v", sa.AutomountToken)
	}
	// token_claims is present when token file is readable
	claims, ok := result["token_claims"].(k8s.MountedTokenMetadata)
	if !ok {
		t.Fatalf("expected token_claims, got %T", result["token_claims"])
	}
	if claims.ServiceAccountName != "my-sa" {
		t.Fatalf("expected token_claims.service_account_name my-sa, got %s", claims.ServiceAccountName)
	}
	if claims.Namespace != "team-a" {
		t.Fatalf("expected token_claims.namespace team-a, got %s", claims.Namespace)
	}
	if claims.ServiceAccountUID != "abc123" {
		t.Fatalf("expected token_claims.service_account_uid abc123, got %s", claims.ServiceAccountUID)
	}
	// effective_permissions is absent when SelfSubjectRulesReview fails in fake client
	if _, ok := result["effective_permissions"]; ok {
		t.Log("effective_permissions present (may succeed in some fake client configs)")
	}
}

func TestCheckTokenSAMissing(t *testing.T) {
	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "my-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "missing-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status, got %#v", result["status"])
	}
	if result["reason"] != "missing_prerequisite" {
		t.Fatalf("expected missing_prerequisite, got %#v", result["reason"])
	}
}

func TestCheckTokenRBACDenied(t *testing.T) {
	client := newFakeSATokenClient(t, "team-a", "my-sa")
	// Make the SA Get call return Forbidden.
	client.Clientset.(*fake.Clientset).PrependReactor("get", "serviceaccounts", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "serviceaccounts"}, "my-sa", nil)
	})
	tool := NewCheckTokenTool(client)
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "my-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status, got %#v", result["status"])
	}
	if result["reason"] != "rbac_denied" {
		t.Fatalf("expected rbac_denied, got %#v", result["reason"])
	}
}

func TestCheckTokenRequiresNameWhenMountedIdentityUnavailable(t *testing.T) {
	tool := NewCheckTokenTool(newFakeSATokenClient(t, "default", "default-sa"))
	_, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a"})
	if err == nil {
		t.Fatal("expected missing name error")
	}
}

func TestCheckTokenInfersNameAndNamespaceFromMountedToken(t *testing.T) {
	tmpFile := writeValidToken(t, "chain-reaction", "chain-reaction", "abc123", 4102444800)
	defer os.Remove(tmpFile)

	origPath := mountedTokenPath
	mountedTokenPath = tmpFile
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "chain-reaction", "chain-reaction"))
	result, err := tool.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "validated" {
		t.Fatalf("expected validated status, got %#v", result["status"])
	}
	if result["name"] != "chain-reaction" {
		t.Fatalf("expected inferred service account name chain-reaction, got %#v", result["name"])
	}
	if result["namespace"] != "chain-reaction" {
		t.Fatalf("expected inferred namespace chain-reaction, got %#v", result["namespace"])
	}
}

func TestCheckTokenMissingTokenFile(t *testing.T) {
	// Point to a token file that does not exist.
	origPath := mountedTokenPath
	mountedTokenPath = "/nonexistent/nonexistent/nonexistent"
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "my-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "my-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status for missing token file, got %#v", result["status"])
	}
	if result["reason"] != string(FailureAuthFailed) {
		t.Fatalf("expected auth_failed for missing token file, got %#v", result["reason"])
	}
	// Error message must not be "<nil>"
	errMsg, ok := result["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected non-empty error message, got %T: %#v", result["error"], result["error"])
	}
	if errMsg == "<nil>" {
		t.Fatal("error message must not be the literal string '<nil>'")
	}
}

func TestCheckTokenExpiredToken(t *testing.T) {
	// Create a temp file with an expired token.
	tmpFile := writeExpiredToken(t, "team-a", "my-sa", "abc123")
	defer os.Remove(tmpFile)

	origPath := mountedTokenPath
	mountedTokenPath = tmpFile
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "my-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "my-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status for expired token, got %#v", result["status"])
	}
	if result["reason"] != string(FailureTokenExpired) {
		t.Fatalf("expected token_expired, got %#v", result["reason"])
	}
}

func TestCheckTokenRequestedSAMismatchFails(t *testing.T) {
	tmpFile := writeValidToken(t, "team-a", "mounted-sa", "abc123", 4102444800)
	defer os.Remove(tmpFile)

	origPath := mountedTokenPath
	mountedTokenPath = tmpFile
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "requested-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "requested-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status for mismatched token, got %#v", result["status"])
	}
	if result["reason"] != string(FailureMissingPrerequisite) {
		t.Fatalf("expected missing_prerequisite, got %#v", result["reason"])
	}
}

func TestCheckTokenMalformedToken(t *testing.T) {
	// Create a temp file with content that is not a valid JWT.
	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	if _, err := tmpFile.WriteString("not-a-jwt-at-all"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	origPath := mountedTokenPath
	mountedTokenPath = tmpFile.Name()
	defer func() { mountedTokenPath = origPath }()

	tool := NewCheckTokenTool(newFakeSATokenClient(t, "team-a", "my-sa"))
	result, err := tool.Run(context.Background(), map[string]any{"namespace": "team-a", "name": "my-sa"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["status"] != "failed" {
		t.Fatalf("expected failed status for malformed token, got %#v", result["status"])
	}
	if result["reason"] != string(FailureAuthFailed) {
		t.Fatalf("expected auth_failed for malformed token, got %#v", result["reason"])
	}
}

// --- Mounted token JWT parsing tests ---

func TestReadMountedTokenMetadataMissingFile(t *testing.T) {
	meta, ok, err := k8s.ReadMountedTokenMetadata("/nonexistent/path/to/token")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing file")
	}
	if meta.Issuer != "" {
		t.Fatalf("expected zero metadata, got %+v", meta)
	}
}

func TestReadMountedTokenMetadataValidJWT(t *testing.T) {
	// Create a temp file with a real JWT-like payload.
	// JWT payload for {"iss":"https://issuer.example.com","sub":"system:serviceaccount:default:my-sa","aud":["kubernetes.default.svc"],"iat":1700000000,"exp":17000003600,"kubernetes.io/serviceaccount/namespace":"default","kubernetes.io/serviceaccount/name":"my-sa","kubernetes.io/serviceaccount/uid":"abc123"}
	// Base64url-encoded payload: eyJpc3MiOiJodHRwczovL2lzc3Vlci5leGFtcGxlLmNvbSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0Om15LXNhIiwiYXVkIjpbImt1YmVybmV0ZXMuZGVmYXVsdC5zdmMiXSwiaWF0IjoxNzAwMDAwMDAwLCJleHAiOjE3MDAwMDAzNjAwLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L25hbWVzcGFjZSI6ImRlZmF1bHQiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L25hbWUiOiJteS1zYSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvdWlkIjoiYWJjMTIzIn0
	// Base64url encoding is identical to standard base64 for this payload (no +/ chars).
	payloadB64 := "eyJpc3MiOiJodHRwczovL2lzc3Vlci5leGFtcGxlLmNvbSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0Om15LXNhIiwiYXVkIjpbImt1YmVybmV0ZXMuZGVmYXVsdC5zdmMiXSwiaWF0IjoxNzAwMDAwMDAwLCJleHAiOjE3MDAwMDAzNjAwLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L25hbWVzcGFjZSI6ImRlZmF1bHQiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L25hbWUiOiJteS1zYSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvdWlkIjoiYWJjMTIzIn0"
	// Build a minimal JWT: header.payload.signature (all fake base64)
	jwt := "eyJhbGciOiJSUzI1NiJ9." + payloadB64 + ".fakesignature"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for valid JWT")
	}

	if meta.Issuer != "https://issuer.example.com" {
		t.Errorf("expected issuer https://issuer.example.com, got %q", meta.Issuer)
	}
	if meta.Subject != "system:serviceaccount:default:my-sa" {
		t.Errorf("expected subject system:serviceaccount:default:my-sa, got %q", meta.Subject)
	}
	if meta.Audience != "kubernetes.default.svc" {
		t.Errorf("expected audience kubernetes.default.svc, got %q", meta.Audience)
	}
	if meta.Namespace != "default" {
		t.Errorf("expected namespace default, got %q", meta.Namespace)
	}
	if meta.ServiceAccountName != "my-sa" {
		t.Errorf("expected service_account_name my-sa, got %q", meta.ServiceAccountName)
	}
	if meta.ServiceAccountUID != "abc123" {
		t.Errorf("expected service_account_uid abc123, got %q", meta.ServiceAccountUID)
	}
	if meta.IssuedAt != 1700000000 {
		t.Errorf("expected issued_at 1700000000, got %d", meta.IssuedAt)
	}
	if meta.Expiry != 17000003600 {
		t.Errorf("expected expiry 17000003600, got %d", meta.Expiry)
	}
}

func TestReadMountedTokenMetadataMultipleAudience(t *testing.T) {
	// Test that multiple audience values are joined by comma.
	// Payload: {"aud":["kubernetes.default.svc","https://kubernetes.default.svc"]}
	payload, _ := json.Marshal(map[string]any{"aud": []any{"kubernetes.default.svc", "https://kubernetes.default.svc"}})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	jwt := "header." + payloadB64 + ".sig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	expected := "kubernetes.default.svc,https://kubernetes.default.svc"
	if meta.Audience != expected {
		t.Errorf("expected audience %q, got %q", expected, meta.Audience)
	}
}

func TestReadMountedTokenMetadataInvalidJWT(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write something that is not a JWT (no dots)
	if _, err := tmpFile.WriteString("not-a-jwt"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err == nil {
		t.Fatal("expected error for invalid JWT")
	}
	if ok {
		t.Fatal("expected ok=false for invalid JWT")
	}
	_ = meta // zero value expected
}

func TestReadMountedTokenMetadataValidAudienceSingle(t *testing.T) {
	// Test single string audience (some token issuers use string instead of array).
	payload, _ := json.Marshal(map[string]any{"aud": "kubernetes.default.svc"})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	jwt := "header." + payloadB64 + ".sig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Audience != "kubernetes.default.svc" {
		t.Errorf("expected audience kubernetes.default.svc, got %q", meta.Audience)
	}
}

func TestReadMountedTokenMetadataEmptyPayload(t *testing.T) {
	// Build a valid JWT with an empty JSON payload {}.
	payload, _ := json.Marshal(map[string]any{})
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	jwt := "header." + payloadB64 + ".sig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for valid JWT with empty payload")
	}
	if meta.Issuer != "" || meta.Subject != "" || meta.Expiry != 0 {
		t.Fatalf("expected all zero values, got %+v", meta)
	}
}

func TestReadMountedTokenMetadataBase64URLVariant(t *testing.T) {
	// Kubernetes tokens use base64url encoding. Test that base64url-encoded values
	// with + and / characters are decoded correctly (e.g., "a+b/c" base64url-encodes
	// with - for + and _ for /).
	payload, _ := json.Marshal(map[string]any{"kubernetes.io/serviceaccount/namespace": "a+b/c"})
	payloadB64URL := base64.RawURLEncoding.EncodeToString(payload)
	jwt := "header." + payloadB64URL + ".sig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Namespace != "a+b/c" {
		t.Errorf("expected namespace a+b/c, got %q", meta.Namespace)
	}
}

func TestReadMountedTokenMetadataProjectedServiceAccountClaims(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"iss": "https://kubernetes.default.svc",
		"sub": "system:serviceaccount:chain-reaction:chain-reaction",
		"aud": []string{"https://kubernetes.default.svc"},
		"kubernetes.io": map[string]any{
			"namespace": "chain-reaction",
			"serviceaccount": map[string]any{
				"name": "chain-reaction",
				"uid":  "uid-123",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	jwt := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(jwt); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	meta, ok, err := k8s.ReadMountedTokenMetadata(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadMountedTokenMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Namespace != "chain-reaction" {
		t.Fatalf("expected namespace chain-reaction, got %q", meta.Namespace)
	}
	if meta.ServiceAccountName != "chain-reaction" {
		t.Fatalf("expected service account chain-reaction, got %q", meta.ServiceAccountName)
	}
	if meta.ServiceAccountUID != "uid-123" {
		t.Fatalf("expected service account uid uid-123, got %q", meta.ServiceAccountUID)
	}
}

// writeValidToken creates a temp file with a valid (non-expired) ServiceAccount JWT token
// and sets mountedTokenPath to point to it. Returns the temp file path so the caller
// can remove it after the test. The expiry is set to year 2099 by default.
func writeValidToken(t *testing.T, ns, saName, saUID string, expiry int64) string {
	t.Helper()
	// Build JWT payload with the given claims and a valid expiry in the future.
	payload := map[string]any{
		"iss":                                    "https://kubernetes.default.svc",
		"sub":                                    "system:serviceaccount:" + ns + ":" + saName,
		"aud":                                    "kubernetes.default.svc",
		"iat":                                    1700000000,
		"exp":                                    expiry,
		"kubernetes.io/serviceaccount/namespace": ns,
		"kubernetes.io/serviceaccount/name":      saName,
		"kubernetes.io/serviceaccount/uid":       saUID,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	// JWT with minimal header and a fake signature segment.
	jwt := "eyJhbGciOiJSUzI1NiJ9." + payloadB64 + ".fakesig"

	tmpFile, err := os.CreateTemp("", "token-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(jwt); err != nil {
		os.Remove(tmpFile.Name())
		tmpFile.Close()
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()
	path := tmpFile.Name()
	mountedTokenPath = path // Override for this test.
	return path
}

// writeExpiredToken creates a temp file with an expired ServiceAccount JWT token.
// Expiry is set to Jan 1, 2020 (definitely in the past).
func writeExpiredToken(t *testing.T, ns, saName, saUID string) string {
	t.Helper()
	return writeValidToken(t, ns, saName, saUID, 1577836800)
}
