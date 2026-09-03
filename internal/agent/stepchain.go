package agent

// stepChainCatalog maps tool names to the KG step IDs they can contribute to,
// derived from built-in catalog (v1, 2026-04-03).
//
// A single tool execution can be a candidate for multiple steps across different
// families because the same tool validates different security properties depending
// on context. The mapping is tool-name-derived only — it does NOT encode
// prerequisite ordering or outcome requirements.
//
// When the catalog is versioned (v2+), update this map and the tests accordingly.
var stepChainCatalog = map[string][]string{
	"validation.check_token":       {"KG-001-S1", "KG-003-S1"},
	"validation.check_permissions": {"KG-001-S2", "KG-001-S3", "KG-002-S1", "KG-003-S2", "KG-005-S1", "KG-005-S3"},
	"validation.read_secret":       {"KG-001-S3", "KG-002-S2", "KG-005-S3"},
	"validation.probe_network":     {"KG-004-S1", "KG-004-S2", "KG-005-S2"},
	"discovery.list_namespaces":    {"KG-005-S1"},
}

// toolToCandidateStepIDs returns the KG step IDs that a tool execution with the
// given tool name could contribute to. Returns nil when the tool is not part of
// any catalog step chain (e.g., introspection tools, unknown tools).
//
// The returned slice is a defensive copy — callers can mutate it without
// affecting the catalog map.
func toolToCandidateStepIDs(toolName string) []string {
	steps, ok := stepChainCatalog[toolName]
	if !ok {
		return nil
	}
	out := make([]string, len(steps))
	copy(out, steps)
	return out
}
