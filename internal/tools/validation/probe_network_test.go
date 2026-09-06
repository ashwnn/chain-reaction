package validation

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProbeNetworkHTTPTimeoutReturnsFailedResult(t *testing.T) {
	tool := NewProbeNetworkTool(nil, nil)
	tool.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}

	result, err := tool.Run(context.Background(), map[string]any{
		"probe":           "http",
		"url":             "https://example.test/slow",
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

func TestProbeNetworkBlocksUnsafeDestinationsBeforeIO(t *testing.T) {
	tool := NewProbeNetworkTool(nil, nil)

	tests := []struct {
		name  string
		input map[string]any
	}{
		{
			name:  "tcp loopback IPv4",
			input: map[string]any{"probe": "tcp", "target": "127.0.0.1", "port": 80},
		},
		{
			name:  "tcp link local metadata",
			input: map[string]any{"probe": "tcp", "target": "169.254.169.254", "port": 80},
		},
		{
			name:  "dns control plane service",
			input: map[string]any{"probe": "dns", "target": "kubernetes.default.svc"},
		},
		{
			name:  "HTTP IPv6 loopback",
			input: map[string]any{"probe": "http", "url": "http://[::1]/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if result["failure_reason"] != string(FailureGuardrailBlocked) {
				t.Fatalf("expected guardrail block, got %#v", result)
			}
		})
	}
}

func TestValidateResolvedIPsRejectsMixedAnswers(t *testing.T) {
	err := validateResolvedIPs([]net.IPAddr{
		{IP: net.ParseIP("10.0.0.10")},
		{IP: net.ParseIP("127.0.0.1")},
	})
	if err == nil {
		t.Fatal("expected mixed safe and unsafe DNS answers to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected a policy error, got %v", err)
	}
}
