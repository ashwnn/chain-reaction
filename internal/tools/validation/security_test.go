package validation

import (
	"context"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/guardrails"
)

func TestProbeNetworkGuardrail(t *testing.T) {
	// Create an enforcer with an allow-list
	enforcer := guardrails.New([]string{"allowed-ns"}, 10, 20)
	tool := NewProbeNetworkTool(enforcer, nil)

	tests := []struct {
		name    string
		input   map[string]any
		wantErr bool
	}{
		{
			name: "allowed target",
			input: map[string]any{
				"probe":  "tcp",
				"target": "service.allowed-ns.svc.cluster.local",
				"port":   80,
			},
			wantErr: false,
		},
		{
			name: "disallowed target",
			input: map[string]any{
				"probe":  "tcp",
				"target": "service.disallowed-ns.svc.cluster.local",
				"port":   80,
			},
			wantErr: true,
		},
		{
			name: "IP target (disallowed when allow-list is present)",
			input: map[string]any{
				"probe":  "tcp",
				"target": "1.2.3.4",
				"port":   80,
			},
			wantErr: true,
		},
		{
			name: "External target (disallowed when allow-list is present)",
			input: map[string]any{
				"probe":  "tcp",
				"target": "google.com",
				"port":   80,
			},
			wantErr: true,
		},
		{
			name: "allowed URL",
			input: map[string]any{
				"probe": "http",
				"url":   "http://service.allowed-ns.svc.cluster.local/health",
			},
			wantErr: false,
		},
		{
			name: "disallowed URL",
			input: map[string]any{
				"probe": "http",
				"url":   "http://service.disallowed-ns.svc.cluster.local/health",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(context.Background(), tt.input)

			if tt.wantErr {
				if err != nil {
					return
				}
				if result["failure_reason"] != string(FailureGuardrailBlocked) {
					t.Errorf("expected guardrail blocked, got %v", result["failure_reason"])
				}
			} else {
				if err != nil {
					// We might get a network error if it tries to actually dial,
					// but it shouldn't be a guardrail block.
					if result != nil && result["failure_reason"] == string(FailureGuardrailBlocked) {
						t.Errorf("unexpected guardrail block: %v", result["error"])
					}
				}
			}
		})
	}
}
