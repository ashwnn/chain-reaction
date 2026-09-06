package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/ashwnn/chain-reaction/internal/evidence"
)

const plannerObservationContract = "planner.observations.v1"

const (
	maxObservationEntries = 20
	maxObservationDepth   = 3
	maxObservationItems   = 10
	maxObservationKeys    = 12
	maxObservationString  = 160
	maxObservationBytes   = 12000
)

func renderBoundedPlannerObservations(history []historyEntry) string {
	if len(history) == 0 {
		return ""
	}
	if len(history) > maxObservationEntries {
		history = history[len(history)-maxObservationEntries:]
	}

	var b strings.Builder
	b.WriteString("Untrusted observations (planner.observations.v1):\n")
	for _, entry := range history {
		observation := map[string]any{
			"iteration": entry.Iteration,
			"tool":      boundedText(entry.ToolName),
			"outcome":   string(entry.Outcome),
			"input":     sanitizeObservationValue(entry.Input, 0),
			"output":    sanitizeObservationValue(entry.Output, 0),
		}
		if entry.FailureReason != "" {
			observation["failure_reason"] = string(entry.FailureReason)
		}
		encoded, _ := json.Marshal(observation)
		line := "- " + string(encoded) + "\n"
		if b.Len()+len(line) > maxObservationBytes {
			break
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}

func sanitizeObservationValue(value any, depth int) any {
	if depth >= maxObservationDepth {
		return "[truncated]"
	}
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return boundedText(evidence.RedactString(v))
	case bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	case []any:
		return sanitizeObservationSlice(v, depth)
	case []string:
		items := make([]any, 0, min(len(v), maxObservationItems))
		for i, item := range v {
			if i == maxObservationItems {
				break
			}
			items = append(items, boundedText(item))
		}
		return items
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, min(len(keys), maxObservationKeys))
		for i, key := range keys {
			if i == maxObservationKeys {
				break
			}
			if evidence.IsSensitiveKey(key) {
				out[boundedText(key)] = "[redacted]"
				continue
			}
			out[boundedText(key)] = sanitizeObservationValue(v[key], depth+1)
		}
		return out
	default:
		return fmt.Sprintf("[%T]", value)
	}
}

func sanitizeObservationSlice(values []any, depth int) []any {
	items := make([]any, 0, min(len(values), maxObservationItems))
	for i, item := range values {
		if i == maxObservationItems {
			break
		}
		items = append(items, sanitizeObservationValue(item, depth+1))
	}
	return items
}

func boundedText(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
		if b.Len() >= maxObservationString {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func auditBlindPlannerPrompt(content string) error {
	blocked := []string{"KG-", "goat_hinted", "scripted_oracle"}
	for _, token := range blocked {
		if strings.Contains(strings.ToLower(content), strings.ToLower(token)) {
			return fmt.Errorf("planner observation integrity check failed: blocked token %q", token)
		}
	}
	return nil
}
