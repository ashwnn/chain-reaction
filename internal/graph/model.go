package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type EdgeStatus string

const (
	EdgeValidated   EdgeStatus = "validated"
	EdgeTheoretical EdgeStatus = "theoretical"
)

type Node struct {
	ID    string         `json:"id"`
	Phase string         `json:"phase"`
	Kind  string         `json:"kind"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type Edge struct {
	From   string         `json:"from"`
	To     string         `json:"to"`
	Status EdgeStatus     `json:"status"`
	Meta   map[string]any `json:"meta,omitempty"`
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

	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return "", fmt.Errorf("write graph file: %w", err)
	}

	return path, nil
}
