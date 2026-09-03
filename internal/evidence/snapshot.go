package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type SnapshotWriter struct {
	dir string
}

type Snapshot struct {
	CollectedAt time.Time   `json:"collected_at"`
	ToolName    string      `json:"tool_name"`
	Namespace   string      `json:"namespace,omitempty"`
	Data        interface{} `json:"data"`
}

type SnapshotEnvelope struct {
	CollectedAt time.Time   `json:"collected_at"`
	ToolName    string      `json:"tool_name"`
	Namespace   string      `json:"namespace,omitempty"`
	ItemCount   int         `json:"item_count"`
	Items       interface{} `json:"items"`
}

type SnapshotIndexEntry struct {
	Path        string    `json:"path"`
	CollectedAt time.Time `json:"collected_at"`
	ToolName    string    `json:"tool_name"`
	Namespace   string    `json:"namespace,omitempty"`
}

type SnapshotIndex struct {
	StartTime     time.Time            `json:"start_time"`
	EndTime       time.Time            `json:"end_time"`
	Namespaces    []string             `json:"namespaces"`
	ToolsExecuted []string             `json:"tools_executed"`
	Snapshots     []SnapshotIndexEntry `json:"snapshots"`
}

func NewSnapshotWriter(evidenceDir string) *SnapshotWriter {
	return &SnapshotWriter{dir: evidenceDir}
}

func (w *SnapshotWriter) WriteSnapshot(toolName string, data interface{}) (string, error) {
	now := time.Now().UTC()
	normalizedTool := normalizeToolSegment(toolName)
	snapshotDir := filepath.Join(w.dir, "snapshots", normalizedTool)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}

	snapshot := SnapshotEnvelope{
		CollectedAt: now,
		ToolName:    toolName,
		Namespace:   extractNamespace(data),
		ItemCount:   computeItemCount(data),
		Items:       data,
	}

	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}

	filename := now.Format("20060102T150405.000000000Z") + ".json"
	path := filepath.Join(snapshotDir, filename)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write snapshot file: %w", err)
	}

	return path, nil
}

func (w *SnapshotWriter) Write(tool string, payload interface{}) (string, error) {
	return w.WriteSnapshot(tool, payload)
}

func (w *SnapshotWriter) WriteIndex(index SnapshotIndex) (string, error) {
	path := filepath.Join(w.dir, "index.json")
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal snapshot index: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write snapshot index: %w", err)
	}
	return path, nil
}

func extractNamespace(data interface{}) string {
	m, ok := data.(map[string]interface{})
	if !ok {
		return ""
	}

	v, ok := m["namespace"]
	if !ok {
		return ""
	}

	namespace, ok := v.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(namespace)
}

func normalizeToolSegment(tool string) string {
	trimmed := strings.TrimSpace(strings.ToLower(tool))
	if trimmed == "" {
		return "unknown"
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}

	normalized := strings.Trim(b.String(), "._-")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func computeItemCount(items interface{}) int {
	if items == nil {
		return 0
	}

	if m, ok := items.(map[string]interface{}); ok {
		if nested, exists := m["items"]; exists {
			return computeItemCount(nested)
		}
	}

	v := reflect.ValueOf(items)
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len()
	default:
		return 1
	}
}
