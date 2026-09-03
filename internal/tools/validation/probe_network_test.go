package validation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeNetworkHTTPTimeoutReturnsFailedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewProbeNetworkTool(nil, nil)

	result, err := tool.Run(context.Background(), map[string]any{
		"probe":           "http",
		"url":             server.URL,
		"timeout_seconds": 1,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result["result"] != string(StepFailed) {
		t.Fatalf("expected failed result, got %#v", result["result"])
	}
	if result["failure_reason"] != string(FailureNetworkUnreachable) {
		t.Fatalf("expected network_unreachable, got %#v", result["failure_reason"])
	}
	if errMsg, ok := result["error"].(string); !ok || errMsg == "" {
		t.Fatalf("expected non-empty error string, got %#v", result["error"])
	}
}
