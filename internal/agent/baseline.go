package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
	"github.com/ashwnn/chain-reaction/internal/tools/discovery"
	"github.com/ashwnn/chain-reaction/internal/tools/introspection"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

var baselineNamespacedTools = []string{
	"discovery.list_pods",
	"discovery.list_services",
	"discovery.list_endpoints",
	"discovery.list_serviceaccounts",
	"discovery.list_configmaps",
	"discovery.list_roles",
	"discovery.list_rolebindings",
	"discovery.list_secrets",
	"discovery.list_networkpolicies",
}

var baselineClusterScopedTools = []string{
	"discovery.list_clusterroles",
	"discovery.list_clusterrolebindings",
}

func runBaselineDiscoveryScan(
	ctx context.Context,
	cfg config.Config,
	start time.Time,
	k8sClient *k8s.Client,
	enforcer *guardrails.Enforcer,
	collector *evidence.Collector,
	snapshotWriter *evidence.SnapshotWriter,
) (RunResult, error) {
	registry, err := newBaselineToolRegistry(k8sClient, enforcer, collector)
	if err != nil {
		return RunResult{}, err
	}

	ag := graph.New()
	ag.AddNode(graph.Node{ID: "pod:current", Phase: "foothold", Kind: "pod"})

	toolCalls := 0
	snapshotEntries := make([]evidence.SnapshotIndexEntry, 0, 32)
	toolSet := map[string]struct{}{}
	namespaceSet := map[string]struct{}{}

	executeTool := func(toolName string, input map[string]any, phase string) (map[string]any, error) {
		t, ok := registry.Get(toolName)
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", toolName)
		}

		if err := enforcer.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("guardrail rate-limit wait failed: %w", err)
		}

		output, err := t.Run(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("tool %s failed: %w", t.Name(), err)
		}

		namespace := ""
		if input != nil {
			if ns, ok := input["namespace"].(string); ok {
				namespace = ns
			}
		}

		snapshotPayload := any(output)
		if namespace != "" {
			snapshotPayload = map[string]any{
				"namespace": namespace,
				"items":     output,
			}
		}

		snapshotPath, err := snapshotWriter.WriteSnapshot(toolName, snapshotPayload)
		if err != nil {
			return nil, fmt.Errorf("write snapshot for %s: %w", toolName, err)
		}

		if err := collector.Record("tool_execution", map[string]any{
			"tool":          t.Name(),
			"input":         input,
			"output":        output,
			"snapshot_path": snapshotPath,
		}); err != nil {
			return nil, fmt.Errorf("record evidence: %w", err)
		}

		toolCalls++
		nodeID := fmt.Sprintf("discovery:%s:%d", t.Name(), toolCalls)
		nodeMeta := map[string]any{"tool": t.Name()}
		if namespace != "" {
			nodeMeta["namespace"] = namespace
		}
		// Attach the evidence snapshot reference so discovery graph nodes/edges carry
		// a direct link to the raw API snapshot that underpins each theoretical edge.
		nodeMeta["snapshot"] = snapshotPath
		ag.AddNode(graph.Node{ID: nodeID, Phase: phase, Kind: "api_call", Meta: nodeMeta})
		ag.AddEdge(graph.Edge{From: "pod:current", To: nodeID, Status: graph.EdgeTheoretical, Type: graph.ToolToEdgeType(t.Name()), EvidenceRef: snapshotPath, Meta: nodeMeta})

		snapshotEntries = append(snapshotEntries, evidence.SnapshotIndexEntry{
			Path:        snapshotPath,
			CollectedAt: time.Now().UTC(),
			ToolName:    t.Name(),
			Namespace:   namespace,
		})
		toolSet[t.Name()] = struct{}{}
		if namespace != "" {
			namespaceSet[namespace] = struct{}{}
		}

		return output, nil
	}

	namespacesOutput, err := executeTool("discovery.list_namespaces", nil, "discovery")
	if err != nil {
		return RunResult{}, err
	}

	targetNamespaces := extractNamespaceNames(namespacesOutput)
	if cfg.Namespace != "" {
		targetNamespaces = []string{cfg.Namespace}
	}

	filteredNamespaces := make([]string, 0, len(targetNamespaces))
	for _, namespace := range targetNamespaces {
		if namespace == "" {
			continue
		}
		if err := enforcer.CheckNamespace(namespace); err != nil {
			continue
		}
		filteredNamespaces = append(filteredNamespaces, namespace)
	}
	targetNamespaces = uniqueStrings(filteredNamespaces)

	for _, namespace := range targetNamespaces {
		for _, toolName := range baselineNamespacedTools {
			if _, err := executeTool(toolName, map[string]any{"namespace": namespace}, "discovery"); err != nil {
				return RunResult{}, err
			}
		}
	}

	for _, toolName := range baselineClusterScopedTools {
		if _, err := executeTool(toolName, nil, "discovery"); err != nil {
			return RunResult{}, err
		}
	}

	namespacesForIndex := make([]string, 0, len(namespaceSet))
	for namespace := range namespaceSet {
		namespacesForIndex = append(namespacesForIndex, namespace)
	}
	sort.Strings(namespacesForIndex)

	toolsForIndex := make([]string, 0, len(toolSet))
	for toolName := range toolSet {
		toolsForIndex = append(toolsForIndex, toolName)
	}
	sort.Strings(toolsForIndex)

	if _, err := snapshotWriter.WriteIndex(evidence.SnapshotIndex{
		StartTime:     start.UTC(),
		EndTime:       time.Now().UTC(),
		Namespaces:    namespacesForIndex,
		ToolsExecuted: toolsForIndex,
		Snapshots:     snapshotEntries,
	}); err != nil {
		return RunResult{}, fmt.Errorf("write snapshot index: %w", err)
	}

	graphDir := filepath.Join(cfg.OutputPath, "graph")
	graphPath, err := ag.WriteJSON(graphDir)
	if err != nil {
		return RunResult{}, fmt.Errorf("write graph output: %w", err)
	}

	if _, err := ag.WriteDOT(graphDir); err != nil {
		return RunResult{}, fmt.Errorf("write graph dot: %w", err)
	}

	summaryPath, err := writeBaselineSummary(
		cfg.OutputPath,
		cfg,
		start,
		time.Now().UTC(),
		namespacesForIndex,
		toolsForIndex,
		toolCalls,
		graphPath,
		collector.Dir(),
	)
	if err != nil {
		return RunResult{}, fmt.Errorf("write baseline summary: %w", err)
	}

	comparisonPath, err := writeDiscoveryComparisonBaseline(
		cfg.OutputPath,
		summaryPath,
		graphPath,
		collector.Dir(),
		toolsForIndex,
		time.Now().UTC(),
	)
	if err != nil {
		return RunResult{}, fmt.Errorf("write comparison baseline: %w", err)
	}

	return RunResult{
		GraphPath:      graphPath,
		EvidencePath:   collector.Dir(),
		SummaryPath:    summaryPath,
		ComparisonPath: comparisonPath,
		RunMode:        "baseline.discovery_full_pass",
		Duration:       time.Since(start),
	}, nil
}

func newBaselineToolRegistry(k8sClient *k8s.Client, enforcer *guardrails.Enforcer, collector *evidence.Collector) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	if err := registry.Register(discovery.NewListNamespacesTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListPodsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListServicesTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListEndpointsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListServiceAccountsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListConfigMapsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListRolesTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListClusterRolesTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListRoleBindingsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListClusterRoleBindingsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListSecretsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(discovery.NewListNetworkPoliciesTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(validation.NewCheckPermissionsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(validation.NewReadSecretTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(validation.NewProbeNetworkTool(enforcer, collector)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(validation.NewCheckTokenTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	if err := registry.Register(introspection.NewGetEffectivePermissionsTool(k8sClient)); err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}
	return registry, nil
}
