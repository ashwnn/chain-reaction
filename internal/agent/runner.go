package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/llm"
	"github.com/ashwnn/chain-reaction/internal/tools"
	"github.com/ashwnn/chain-reaction/internal/tools/discovery"
)

type RunResult struct {
	GraphPath    string
	EvidencePath string
	ToolUsed     string
	Duration     time.Duration
}

func Run(ctx context.Context, cfg config.Config) (RunResult, error) {
	start := time.Now()

	timedCtx, cancel := context.WithTimeout(ctx, cfg.TimeBudget)
	defer cancel()

	k8sClient, err := k8s.NewClient(cfg.Kubeconfig, cfg.QPS, cfg.Burst)
	if err != nil {
		return RunResult{}, fmt.Errorf("initialize k8s client: %w", err)
	}

	enforcer := guardrails.New(cfg.AllowListNamespaces, cfg.QPS, cfg.Burst)
	if cfg.Namespace != "" {
		if err := enforcer.CheckNamespace(cfg.Namespace); err != nil {
			return RunResult{}, err
		}
	}

	evidenceDir := filepath.Join(cfg.OutputPath, "evidence")
	collector, err := evidence.NewCollector(evidenceDir)
	if err != nil {
		return RunResult{}, fmt.Errorf("initialize evidence collector: %w", err)
	}
	defer collector.Close()

	registry := tools.NewRegistry()
	if err := registry.Register(discovery.NewListNamespacesTool(k8sClient)); err != nil {
		return RunResult{}, fmt.Errorf("register tool: %w", err)
	}

	planner := llm.NewPlanner(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	toolName, err := planner.SuggestNextTool(timedCtx, "bootstrap discovery")
	if err != nil {
		return RunResult{}, fmt.Errorf("planner failure: %w", err)
	}

	toolName = strings.TrimSpace(toolName)
	toolName = strings.Trim(toolName, "\"`")
	t, ok := registry.Get(toolName)
	if !ok {
		return RunResult{}, fmt.Errorf("planner returned unknown tool %q", toolName)
	}

	if err := enforcer.Acquire(timedCtx); err != nil {
		return RunResult{}, fmt.Errorf("guardrail rate-limit wait failed: %w", err)
	}

	output, err := t.Run(timedCtx, nil)
	if err != nil {
		return RunResult{}, fmt.Errorf("tool %s failed: %w", t.Name(), err)
	}

	if err := collector.Record("tool_execution", map[string]any{
		"tool":   t.Name(),
		"output": output,
	}); err != nil {
		return RunResult{}, fmt.Errorf("record evidence: %w", err)
	}

	ag := graph.New()
	ag.AddNode(graph.Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})
	ag.AddNode(graph.Node{ID: "discovery:namespaces", Phase: "discovery", Kind: "api_call"})
	ag.AddEdge(graph.Edge{
		From:   "pod:current",
		To:     "discovery:namespaces",
		Status: graph.EdgeValidated,
		Meta: map[string]any{
			"tool": t.Name(),
		},
	})

	graphDir := filepath.Join(cfg.OutputPath, "graph")
	graphPath, err := ag.WriteJSON(graphDir)
	if err != nil {
		return RunResult{}, fmt.Errorf("write graph output: %w", err)
	}

	return RunResult{
		GraphPath:    graphPath,
		EvidencePath: collector.Dir(),
		ToolUsed:     t.Name(),
		Duration:     time.Since(start),
	}, nil
}
