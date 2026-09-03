package cli

import (
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/metrics"
)

func TestValidateCommandIncludesDebugFlag(t *testing.T) {
	cmd := NewRootCmd("v", "c", "d")
	validateCmd, _, err := cmd.Find([]string{"validate"})
	if err != nil {
		t.Fatalf("expected validate command to be registered: %v", err)
	}
	if validateCmd.Flags().Lookup("debug") == nil {
		t.Fatal("expected validate command to expose --debug")
	}
}

func TestWriteLLMUsageSummary_CostUnavailable(t *testing.T) {
	cacheWrite := 10
	efficiency := 0.2
	usage := &metrics.MetricsLLMUsage{
		InputTokens:      100,
		OutputTokens:     50,
		TotalTokens:      150,
		CacheReadTokens:  20,
		CacheWriteTokens: &cacheWrite,
		CacheEfficiency:  &efficiency,
		EstimatedCostUSD: nil,
	}
	var buf strings.Builder
	writeLLMUsageSummary(&buf, usage)

	out := buf.String()

	if !strings.Contains(out, "llm_usage:\n") {
		t.Fatalf("expected llm_usage header, got:\n%s", out)
	}
	if !strings.Contains(out, "input_tokens: 100\n") {
		t.Fatalf("expected input_tokens line, got:\n%s", out)
	}
	if !strings.Contains(out, "cost_note: pricing unavailable") {
		t.Fatalf("expected cost_note line when EstimatedCostUSD is nil, got:\n%s", out)
	}
	if strings.Contains(out, "estimated_cost_usd") {
		t.Fatalf("estimated_cost_usd should not appear when EstimatedCostUSD is nil, got:\n%s", out)
	}
}

func TestWriteLLMUsageSummary_CostAvailable(t *testing.T) {
	cost := 0.0015
	usage := &metrics.MetricsLLMUsage{
		InputTokens:      100,
		OutputTokens:     50,
		TotalTokens:      150,
		EstimatedCostUSD: &cost,
	}
	var buf strings.Builder
	writeLLMUsageSummary(&buf, usage)

	out := buf.String()

	if !strings.Contains(out, "estimated_cost_usd: $0.001500\n") {
		t.Fatalf("expected estimated_cost_usd line, got:\n%s", out)
	}
	if strings.Contains(out, "cost_note") {
		t.Fatalf("cost_note should not appear when EstimatedCostUSD is set, got:\n%s", out)
	}
}

func TestWriteLLMUsageSummary_NilUsage(t *testing.T) {
	var buf strings.Builder
	writeLLMUsageSummary(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil usage, got:\n%s", buf.String())
	}
}

func TestWriteLLMUsageSummary_CacheFieldsOmittedWhenZero(t *testing.T) {
	usage := &metrics.MetricsLLMUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}
	var buf strings.Builder
	writeLLMUsageSummary(&buf, usage)

	out := buf.String()

	if strings.Contains(out, "cache_read_tokens") {
		t.Fatalf("cache_read_tokens should be omitted when zero, got:\n%s", out)
	}
	if strings.Contains(out, "cache_write_tokens") {
		t.Fatalf("cache_write_tokens should be omitted when nil, got:\n%s", out)
	}
	if strings.Contains(out, "cache_efficiency") {
		t.Fatalf("cache_efficiency should be omitted when nil, got:\n%s", out)
	}
}
