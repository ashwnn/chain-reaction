package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EdgeStatus string

const (
	EdgeValidated EdgeStatus = "validated"
	// EdgeObserved records a completed tool action whose result was not classified
	// as validated, theoretical, or failed. It is retained for auditability but
	// never contributes validation credit.
	EdgeObserved    EdgeStatus = "observed"
	EdgeTheoretical EdgeStatus = "theoretical"
	EdgeFailedRBAC  EdgeStatus = "failed_rbac"

	// EdgeFailed is the generic failure status for a StepFailed outcome that is not
	// covered by a more specific edge status (e.g., EdgeFailedRBAC).
	EdgeFailed EdgeStatus = "failed"
)

// EdgeType classifies the security relationship represented by a graph edge.
// This is a first-class alternative to inferring the relationship from Meta["tool"].
type EdgeType string

const (
	// EdgeTypeDiscovery represents a theoretical enumeration edge from baseline scanning.
	edgeTypeDiscovery EdgeType = "discovery"

	// EdgeTypeSecretAccess represents a secret-read validation step.
	EdgeTypeSecretAccess EdgeType = "secret_access"

	// EdgeTypePermissionCheck represents an RBAC access review validation step.
	EdgeTypePermissionCheck EdgeType = "permission_check"

	// EdgeTypeNetworkProbe represents a TCP/HTTP/DNS connectivity validation step.
	EdgeTypeNetworkProbe EdgeType = "network_probe"

	// EdgeTypeTokenReview represents a ServiceAccount token inspection step.
	EdgeTypeTokenReview EdgeType = "token_review"
)

// ToolToEdgeType maps a fully-qualified tool name to its canonical EdgeType.
// Tools outside the known set receive EdgeTypeDiscovery as a safe default.
func ToolToEdgeType(toolName string) EdgeType {
	switch {
	case strings.HasPrefix(toolName, "discovery.") || strings.HasPrefix(toolName, "introspection."):
		return edgeTypeDiscovery
	case toolName == "validation.read_secret":
		return EdgeTypeSecretAccess
	case toolName == "validation.check_permissions":
		return EdgeTypePermissionCheck
	case toolName == "validation.probe_network":
		return EdgeTypeNetworkProbe
	case toolName == "validation.check_token":
		return EdgeTypeTokenReview
	default:
		return edgeTypeDiscovery
	}
}

type Node struct {
	ID    string         `json:"id"`
	Phase string         `json:"phase"`
	Kind  string         `json:"kind"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type Edge struct {
	From          string         `json:"from"`
	To            string         `json:"to"`
	Status        EdgeStatus     `json:"status"`
	Type          EdgeType       `json:"type,omitempty"`
	FailureReason string         `json:"failure_reason,omitempty"`
	EvidenceRef   string         `json:"evidence_ref,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type AttackGraph struct {
	GeneratedAt time.Time `json:"generated_at"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
}

func New() *AttackGraph {
	return &AttackGraph{GeneratedAt: time.Now().UTC()}
}

func (g *AttackGraph) AddNode(node Node) {
	g.Nodes = append(g.Nodes, node)
}

func (g *AttackGraph) AddEdge(edge Edge) {
	g.Edges = append(g.Edges, edge)
}

func (g *AttackGraph) WriteJSON(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create graph directory: %w", err)
	}

	path := filepath.Join(dir, "attack-graph.json")
	bytes, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal graph json: %w", err)
	}

	if err := os.WriteFile(path, bytes, 0o600); err != nil {
		return "", fmt.Errorf("write graph file: %w", err)
	}

	return path, nil
}

func dotStyle(s EdgeStatus) (color, style string) {
	switch s {
	case EdgeValidated:
		return "#0072B2", "solid"
	case EdgeObserved:
		return "#666666", "dotted"
	case EdgeTheoretical:
		return "#999999", "dashed"
	case EdgeFailedRBAC:
		return "#D55E00", "dashed"
	case EdgeFailed:
		return "#D55E00", "solid"
	default:
		return "#333333", "solid"
	}
}

func humanToolName(raw string) string {
	for _, prefix := range []string{"discovery.", "validation.", "introspection."} {
		raw = strings.TrimPrefix(raw, prefix)
	}
	return raw
}

func titlePhase(phase string) string {
	switch phase {
	case "foothold":
		return "Foothold"
	case "discovery":
		return "Discovery"
	case "validation":
		return "Validation"
	case "":
		return ""
	default:
		return strings.ToUpper(phase[:1]) + phase[1:]
	}
}

func dotNodeLabel(n Node) string {
	parts := strings.SplitN(n.ID, ":", 3)
	if len(parts) == 3 {
		name := humanToolName(parts[1])
		phase := titlePhase(n.Phase)
		if phase == "" {
			phase = titlePhase(parts[0])
		}
		if phase != "" {
			return fmt.Sprintf("%s\\n%s", name, phase)
		}
		return name
	}
	if n.ID == "pod:current" {
		if n.Kind != "" {
			return fmt.Sprintf("Assumed-breach Pod\\n%s", titlePhase(n.Phase))
		}
		return "Assumed-breach Pod"
	}
	if n.Phase != "" && n.Kind != "" {
		return fmt.Sprintf("%s\\n%s", n.ID, titlePhase(n.Phase))
	}
	return n.ID
}

func dotNodeFill(n Node) (fill, color, shape string) {
	switch n.Phase {
	case "foothold":
		return "#D55E00", "#7A3000", "hexagon"
	case "validation":
		return "#D6EAF8", "#0072B2", "box"
	case "discovery":
		return "#F2F2F2", "#666666", "box"
	default:
		return "#ECEFF1", "#455A64", "box"
	}
}

func (g *AttackGraph) RenderDOT() string {
	var b strings.Builder
	b.WriteString("digraph attack_graph {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  graph [bgcolor=\"#ffffff\", fontname=\"Helvetica\", fontsize=12, pad=\"0.4\", nodesep=\"0.45\", ranksep=\"0.7\", splines=true, label=\"Chain Reaction Attack Graph\", labelloc=t];\n")
	b.WriteString("  node [shape=box, style=\"rounded,filled\", fontname=\"Helvetica\", fontsize=10, color=\"#455A64\", fillcolor=\"#ECEFF1\", penwidth=1.15];\n")
	b.WriteString("  edge [fontname=\"Helvetica\", fontsize=9, penwidth=1.3, color=\"#333333\"];\n")

	sortedNodes := make([]Node, len(g.Nodes))
	copy(sortedNodes, g.Nodes)
	sort.Slice(sortedNodes, func(i, j int) bool {
		return sortedNodes[i].ID < sortedNodes[j].ID
	})

	byPhase := map[string][]Node{}
	phaseOrder := []string{"foothold", "discovery", "validation"}
	for _, n := range sortedNodes {
		phase := n.Phase
		if phase == "" {
			phase = "other"
		}
		byPhase[phase] = append(byPhase[phase], n)
	}
	if _, ok := byPhase["other"]; ok {
		phaseOrder = append(phaseOrder, "other")
	}

	b.WriteString("\n")
	for _, phase := range phaseOrder {
		nodes := byPhase[phase]
		if len(nodes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  subgraph cluster_%s {\n", phase)
		fmt.Fprintf(&b, "    label=%q;\n", titlePhase(phase))
		b.WriteString("    style=\"rounded,filled\";\n")
		b.WriteString("    color=\"#D7DCE2\";\n")
		b.WriteString("    fillcolor=\"#FBFCFD\";\n")
		b.WriteString("    fontsize=11;\n")
		for _, n := range nodes {
			fill, color, shape := dotNodeFill(n)
			fmt.Fprintf(&b, "    %q [label=%q, fillcolor=%q, color=%q, shape=%s];\n",
				n.ID, dotNodeLabel(n), fill, color, shape)
		}
		b.WriteString("  }\n")
	}

	sortedEdges := make([]Edge, len(g.Edges))
	copy(sortedEdges, g.Edges)
	sort.Slice(sortedEdges, func(i, j int) bool {
		if sortedEdges[i].From != sortedEdges[j].From {
			return sortedEdges[i].From < sortedEdges[j].From
		}
		if sortedEdges[i].To != sortedEdges[j].To {
			return sortedEdges[i].To < sortedEdges[j].To
		}
		return sortedEdges[i].Type < sortedEdges[j].Type
	})

	b.WriteString("\n")
	for _, e := range sortedEdges {
		color, style := dotStyle(e.Status)
		label := humanEdgeType(e.Type)
		if e.FailureReason != "" {
			label += fmt.Sprintf("\\n(reason: %s)", e.FailureReason)
		}
		penwidth := "1.3"
		if e.Status == EdgeValidated {
			penwidth = "1.8"
		}
		fmt.Fprintf(&b, "  %q -> %q [label=%q, color=%q, style=%s, penwidth=%s];\n",
			e.From, e.To, label, color, style, penwidth)
	}

	b.WriteString("\n")
	b.WriteString("  subgraph cluster_legend {\n")
	b.WriteString("    label=\"Legend\";\n")
	b.WriteString("    style=\"rounded,filled\";\n")
	b.WriteString("    color=\"#D7DCE2\";\n")
	b.WriteString("    fillcolor=\"#FFFFFF\";\n")
	b.WriteString("    fontsize=10;\n")
	b.WriteString("    rank=sink;\n")
	b.WriteString("    node [shape=plaintext, style=solid, fillcolor=\"#FFFFFF\"];\n")
	b.WriteString("    legend [label=<\n")
	b.WriteString("      <TABLE BORDER=\"0\" CELLBORDER=\"0\" CELLSPACING=\"4\">\n")
	b.WriteString("        <TR>\n")
	b.WriteString("          <TD><FONT COLOR=\"#0072B2\">━━</FONT> Validated</TD>\n")
	b.WriteString("          <TD><FONT COLOR=\"#999999\">- - </FONT> Theoretical</TD>\n")
	b.WriteString("          <TD><FONT COLOR=\"#D55E00\">- - </FONT> Failed / RBAC</TD>\n")
	b.WriteString("        </TR>\n")
	b.WriteString("      </TABLE>\n")
	b.WriteString("    >];\n")
	b.WriteString("  }\n")

	b.WriteString("}\n")
	return b.String()
}

func humanEdgeType(t EdgeType) string {
	switch t {
	case edgeTypeDiscovery:
		return "discovery"
	case EdgeTypeSecretAccess:
		return "secret access"
	case EdgeTypePermissionCheck:
		return "permission check"
	case EdgeTypeNetworkProbe:
		return "network probe"
	case EdgeTypeTokenReview:
		return "token review"
	default:
		if t == "" {
			return ""
		}
		return strings.ReplaceAll(string(t), "_", " ")
	}
}

// WriteDOT renders the graph as a DOT file and writes it to the given directory.
// Returns the path of the written file.
func (g *AttackGraph) WriteDOT(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create graph directory: %w", err)
	}

	path := filepath.Join(dir, "attack-graph.dot")
	if err := os.WriteFile(path, []byte(g.RenderDOT()), 0o600); err != nil {
		return "", fmt.Errorf("write dot file: %w", err)
	}

	return path, nil
}
