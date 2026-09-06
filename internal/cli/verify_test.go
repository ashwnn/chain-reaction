package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/evidence"
)

func TestVerifyEvidenceCommand(t *testing.T) {
	dir := t.TempDir()
	collector, err := evidence.NewCollector(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.Record("test", map[string]any{"key": "value"}); err != nil {
		t.Fatal(err)
	}
	if err := collector.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newVerifyCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"evidence", "--evidence", filepath.Join(dir, "evidence.jsonl")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify evidence: %v", err)
	}
	if !strings.Contains(output.String(), "evidence_chain_valid: true") || !strings.Contains(output.String(), "records: 1") {
		t.Fatalf("unexpected verifier output: %s", output.String())
	}
}
