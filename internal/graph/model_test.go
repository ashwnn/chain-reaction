package graph

import (
	"strings"
	"testing"
)

func TestRenderDOT(t *testing.T) {
	tests := []struct {
		name  string
		graph *AttackGraph
		check func(t *testing.T, got string)
	}{
		{
			name:  "empty graph produces valid digraph",
			graph: New(),
			check: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, "digraph attack_graph {") {
					t.Error("missing digraph header")
				}
				if !strings.Contains(got, "rankdir=LR;") {
					t.Error("missing rankdir")
				}
				if !strings.HasSuffix(got, "}\n") {
					t.Errorf("expected trailing newline after closing brace, got %q", got[len(got)-5:])
				}
				if strings.Contains(got, "->") {
					t.Error("empty graph should have no edges")
				}
			},
		},
		{
			name: "single node no edges",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, `"pod:current"`) || !strings.Contains(got, `Assumed-breach Pod`) {
					t.Errorf("node label not found in output:\n%s", got)
				}
			},
		},
		{
			name: "structured node ID shows tool name",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g.AddNode(Node{ID: "discovery:list_pods:1", Phase: "discovery", Kind: "api_call"})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, `"discovery:list_pods:1"`) || !strings.Contains(got, `list_pods`) {
					t.Errorf("structured node label not found in output:\n%s", got)
				}
			},
		},
		{
			name: "all four edge statuses render correctly",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g.AddNode(Node{ID: "validation:read_secret:1", Phase: "validation", Kind: "api_call"})
				g.AddNode(Node{ID: "discovery:list_pods:1", Phase: "discovery", Kind: "api_call"})
				g.AddNode(Node{ID: "validation:check_permissions:1", Phase: "validation", Kind: "api_call"})
				g.AddNode(Node{ID: "validation:probe_network:1", Phase: "validation", Kind: "api_call"})
				g.AddEdge(Edge{From: "pod:current", To: "validation:read_secret:1", Status: EdgeValidated, Type: EdgeTypeSecretAccess})
				g.AddEdge(Edge{From: "pod:current", To: "discovery:list_pods:1", Status: EdgeTheoretical, Type: edgeTypeDiscovery})
				g.AddEdge(Edge{From: "pod:current", To: "validation:check_permissions:1", Status: EdgeFailedRBAC, Type: EdgeTypePermissionCheck})
				g.AddEdge(Edge{From: "pod:current", To: "validation:probe_network:1", Status: EdgeFailed, Type: EdgeTypeNetworkProbe})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, `color="#0072B2", style=solid`) {
					t.Error("validated edge style not found")
				}
				if !strings.Contains(got, `color="#999999", style=dashed`) {
					t.Error("theoretical edge style not found")
				}
				if !strings.Contains(got, `color="#D55E00", style=dashed`) {
					t.Error("failed_rbac edge style not found")
				}
				if !strings.Contains(got, `color="#D55E00", style=solid`) {
					t.Error("failed edge style not found")
				}
			},
		},
		{
			name: "edge with failure reason includes it in label",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g.AddNode(Node{ID: "validation:check_permissions:1", Phase: "validation", Kind: "api_call"})
				g.AddEdge(Edge{
					From:          "pod:current",
					To:            "validation:check_permissions:1",
					Status:        EdgeFailedRBAC,
					Type:          EdgeTypePermissionCheck,
					FailureReason: "rbac_denied",
				})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				if !strings.Contains(got, `\n(reason: rbac_denied)`) {
					t.Errorf("failure reason not in edge label:\n%s", got)
				}
			},
		},
		{
			name: "edge without failure reason has clean label",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g.AddNode(Node{ID: "validation:read_secret:1", Phase: "validation", Kind: "api_call"})
				g.AddEdge(Edge{
					From:   "pod:current",
					To:     "validation:read_secret:1",
					Status: EdgeValidated,
					Type:   EdgeTypeSecretAccess,
				})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				// The label should be just the type, no reason suffix
				if strings.Contains(got, "reason:") {
					t.Errorf("validated edge should not contain reason:\n%s", got)
				}
				if !strings.Contains(got, `label="secret access"`) {
					t.Errorf("edge label should be just the type:\n%s", got)
				}
			},
		},
		{
			name: "deterministic output across multiple calls",
			graph: func() *AttackGraph {
				g := New()
				g.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g.AddNode(Node{ID: "validation:read_secret:1", Phase: "validation", Kind: "api_call"})
				g.AddNode(Node{ID: "discovery:list_pods:1", Phase: "discovery", Kind: "api_call"})
				g.AddEdge(Edge{From: "pod:current", To: "discovery:list_pods:1", Status: EdgeTheoretical, Type: edgeTypeDiscovery})
				g.AddEdge(Edge{From: "pod:current", To: "validation:read_secret:1", Status: EdgeValidated, Type: EdgeTypeSecretAccess})
				return g
			}(),
			check: func(t *testing.T, got string) {
				t.Helper()
				// Call RenderDOT again and verify identical output.
				g2 := New()
				g2.AddNode(Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
				g2.AddNode(Node{ID: "validation:read_secret:1", Phase: "validation", Kind: "api_call"})
				g2.AddNode(Node{ID: "discovery:list_pods:1", Phase: "discovery", Kind: "api_call"})
				g2.AddEdge(Edge{From: "pod:current", To: "discovery:list_pods:1", Status: EdgeTheoretical, Type: edgeTypeDiscovery})
				g2.AddEdge(Edge{From: "pod:current", To: "validation:read_secret:1", Status: EdgeValidated, Type: EdgeTypeSecretAccess})
				got2 := g2.RenderDOT()
				if got != got2 {
					t.Errorf("non-deterministic output:\n--- first\n%s\n--- second\n%s", got, got2)
				}

				if !strings.Contains(got, "cluster_foothold") || !strings.Contains(got, "cluster_discovery") {
					t.Errorf("expected phase clusters:\n%s", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.graph.RenderDOT()
			tc.check(t, got)
		})
	}
}

func TestDotNodeLabel(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "structured discovery ID",
			node: Node{ID: "discovery:list_pods:3", Phase: "discovery", Kind: "api_call"},
			want: "list_pods\\nDiscovery",
		},
		{
			name: "structured validation ID",
			node: Node{ID: "validation:read_secret:1", Phase: "validation", Kind: "api_call"},
			want: "read_secret\\nValidation",
		},
		{
			name: "simple ID with phase and kind",
			node: Node{ID: "pod:current", Phase: "foothold", Kind: "pod"},
			want: "Assumed-breach Pod\\nFoothold",
		},
		{
			name: "simple ID with phase only",
			node: Node{ID: "target", Phase: "lateral", Kind: ""},
			want: "target",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dotNodeLabel(tc.node)
			if got != tc.want {
				t.Errorf("dotNodeLabel(%+v) = %q, want %q", tc.node, got, tc.want)
			}
		})
	}
}

func TestDotStyle(t *testing.T) {
	tests := []struct {
		status    EdgeStatus
		wantColor string
		wantStyle string
	}{
		{EdgeValidated, "#0072B2", "solid"},
		{EdgeTheoretical, "#999999", "dashed"},
		{EdgeFailedRBAC, "#D55E00", "dashed"},
		{EdgeFailed, "#D55E00", "solid"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			color, style := dotStyle(tc.status)
			if color != tc.wantColor {
				t.Errorf("color = %q, want %q", color, tc.wantColor)
			}
			if style != tc.wantStyle {
				t.Errorf("style = %q, want %q", style, tc.wantStyle)
			}
		})
	}
}
