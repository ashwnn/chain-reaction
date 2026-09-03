package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

type hypothesisStatus string

const (
	hypothesisSupported hypothesisStatus = "supported"
	hypothesisBlocked   hypothesisStatus = "blocked"
	hypothesisOpen      hypothesisStatus = "open"
)

type evidenceHypothesis struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Status     hypothesisStatus `json:"status"`
	Confidence float64          `json:"confidence"`
	Evidence   []string         `json:"evidence"`
	ReasonCode string           `json:"reason_code,omitempty"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

func deriveEvidenceHypotheses(history []historyEntry) []evidenceHypothesis {
	byTool := make(map[string]evidenceHypothesis)
	for _, entry := range history {
		if entry.ToolName == "" {
			continue
		}
		status := hypothesisOpen
		confidence := 0.25
		switch entry.Outcome {
		case validation.StepValidated:
			status, confidence = hypothesisSupported, 0.75
		case validation.StepFailed:
			status, confidence = hypothesisBlocked, 0.75
		}
		kind := hypothesisKind(entry.ToolName)
		id := fmt.Sprintf("hyp-%x", sha256.Sum256([]byte(kind+":"+entry.ToolName)))
		h := byTool[id]
		if h.ID == "" || entry.Timestamp.After(h.UpdatedAt) {
			h = evidenceHypothesis{
				ID:         id,
				Kind:       kind,
				Status:     status,
				Confidence: confidence,
				Evidence:   []string{entry.ToolName},
				ReasonCode: string(entry.FailureReason),
				UpdatedAt:  entry.Timestamp.UTC(),
			}
		}
		byTool[id] = h
	}
	result := make([]evidenceHypothesis, 0, len(byTool))
	for _, hypothesis := range byTool {
		result = append(result, hypothesis)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func hypothesisKind(toolName string) string {
	switch {
	case strings.Contains(toolName, "permissions"):
		return "rbac_access"
	case strings.Contains(toolName, "secret"):
		return "secret_access"
	case strings.Contains(toolName, "network"):
		return "network_reachability"
	case strings.Contains(toolName, "token"):
		return "workload_identity"
	default:
		return "resource_discovery"
	}
}

func deriveTraceHypotheses(trace []toolExecutionResult) []evidenceHypothesis {
	history := make([]historyEntry, 0, len(trace))
	for index, entry := range trace {
		history = append(history, historyEntry{
			Iteration:     index + 1,
			ToolName:      entry.ToolName,
			Outcome:       entry.Outcome,
			FailureReason: entry.FailureReason,
			Timestamp:     entry.Timestamp,
		})
	}
	return deriveEvidenceHypotheses(history)
}
func writeHypothesisArtifact(outputDir string, hypotheses []evidenceHypothesis) (string, error) {
	path := filepath.Join(outputDir, "hypotheses.json")
	body, err := json.MarshalIndent(hypotheses, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal hypotheses: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write hypotheses: %w", err)
	}
	return path, nil
}
