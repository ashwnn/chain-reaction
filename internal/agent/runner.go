package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/k8s"
)

var (
	newK8sClient  = k8s.NewClient
	baselineRunFn = runBaselineDiscoveryScan
)

type RunResult struct {
	GraphPath      string
	EvidencePath   string
	SummaryPath    string
	ComparisonPath string
	RunMode        string
	Duration       time.Duration
}

func Run(ctx context.Context, cfg config.Config) (RunResult, error) {
	start := time.Now()

	timedCtx, cancel := context.WithTimeout(ctx, cfg.TimeBudget)
	defer cancel()

	k8sClient, err := newK8sClient(cfg.Kubeconfig, cfg.QPS, cfg.Burst)
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
	snapshotWriter := evidence.NewSnapshotWriter(evidenceDir)

	return baselineRunFn(timedCtx, cfg, start, k8sClient, enforcer, collector, snapshotWriter)
}

func extractNamespaceNames(output map[string]any) []string {
	if output == nil {
		return nil
	}

	rawNamespaces, ok := output["namespaces"]
	if !ok {
		return nil
	}

	namespaces := make([]string, 0)
	switch ns := rawNamespaces.(type) {
	case []k8s.NamespaceInfo:
		for _, item := range ns {
			if item.Name != "" {
				namespaces = append(namespaces, item.Name)
			}
		}
	case []any:
		for _, item := range ns {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, ok := m["name"].(string)
			if ok && name != "" {
				namespaces = append(namespaces, name)
			}
		}
	}

	return uniqueStrings(namespaces)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
