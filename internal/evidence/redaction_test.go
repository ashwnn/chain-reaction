package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactWritersRedactNestedValuesAndErrors(t *testing.T) {
	const token = "eyJhbGciOiJSUzI1NiJ9.payload.signature"
	const password = "do-not-persist-password"
	payload := map[string]any{
		"nested": map[string]any{
			"api_key": password,
			"headers": []any{map[string]any{"Authorization": "Bearer " + token}},
		},
		"error": "request failed: Authorization: Bearer " + token,
	}

	dir := t.TempDir()
	collector, err := NewCollector(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.Record("test", payload); err != nil {
		t.Fatal(err)
	}
	if err := collector.Close(); err != nil {
		t.Fatal(err)
	}
	writer := NewSnapshotWriter(dir)
	path, err := writer.WriteSnapshot("test.tool", payload)
	if err != nil {
		t.Fatal(err)
	}

	for _, artifact := range []string{filepath.Join(dir, "evidence.jsonl"), path} {
		body, err := os.ReadFile(artifact)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), token) || strings.Contains(string(body), password) {
			t.Fatalf("artifact %q leaked a credential: %s", artifact, body)
		}
		if !strings.Contains(string(body), redactedValue) {
			t.Fatalf("artifact %q did not contain a redaction marker: %s", artifact, body)
		}
	}
}

func TestRedactValueDoesNotMutateCallerData(t *testing.T) {
	payload := map[string]any{"token": "original"}
	redacted := RedactValue(payload).(map[string]any)
	if redacted["token"] != redactedValue {
		t.Fatalf("expected redacted copy, got %#v", redacted)
	}
	if payload["token"] != "original" {
		t.Fatalf("redaction mutated caller data: %#v", payload)
	}
}
