package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

func TestRunUsesBaselinePath(t *testing.T) {
	tmpDir := t.TempDir()
	client := newFakeK8sClient(t)

	originalNewClient := newK8sClient
	originalBaselineRunFn := baselineRunFn
	t.Cleanup(func() {
		newK8sClient = originalNewClient
		baselineRunFn = originalBaselineRunFn
	})

	baselineCalled := false
	newK8sClient = func(_ string, _ float32, _ int) (*k8s.Client, error) {
		return client, nil
	}
	baselineRunFn = func(ctx context.Context, cfg config.Config, start time.Time, k8sClient *k8s.Client, enforcer *guardrails.Enforcer, collector *evidence.Collector, snapshotWriter *evidence.SnapshotWriter) (RunResult, error) {
		baselineCalled = true
		if ctx == nil {
			t.Fatal("expected context to be passed to baseline runner")
		}
		if k8sClient != client {
			t.Fatal("expected Run to pass the created k8s client to baseline runner")
		}
		if collector == nil || snapshotWriter == nil || enforcer == nil {
			t.Fatal("expected Run to initialize evidence and guardrail dependencies before baseline runner")
		}
		if cfg.OutputPath != tmpDir {
			t.Fatalf("expected output path %q, got %q", tmpDir, cfg.OutputPath)
		}
		return RunResult{RunMode: "baseline.discovery_full_pass"}, nil
	}

	result, err := Run(context.Background(), config.Config{OutputPath: tmpDir, TimeBudget: time.Minute, QPS: 1, Burst: 1})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !baselineCalled {
		t.Fatal("expected Run to dispatch to baseline runner")
	}
	if result.RunMode != "baseline.discovery_full_pass" {
		t.Fatalf("expected baseline run mode, got %q", result.RunMode)
	}
}

func TestBaselineScanRemainsPromptFree(t *testing.T) {
	tmpDir := t.TempDir()
	client := newFakeK8sClient(t)

	originalNewClient := newK8sClient
	originalBaselineRunFn := baselineRunFn
	t.Cleanup(func() {
		newK8sClient = originalNewClient
		baselineRunFn = originalBaselineRunFn
	})

	baselineCalled := false
	newK8sClient = func(_ string, _ float32, _ int) (*k8s.Client, error) {
		return client, nil
	}
	baselineRunFn = func(ctx context.Context, cfg config.Config, start time.Time, k8sClient *k8s.Client, enforcer *guardrails.Enforcer, collector *evidence.Collector, snapshotWriter *evidence.SnapshotWriter) (RunResult, error) {
		baselineCalled = true
		if cfg.LLMProvider != "groq" {
			t.Fatalf("expected baseline path to receive original config unchanged, got provider %q", cfg.LLMProvider)
		}
		return RunResult{RunMode: "baseline.discovery_full_pass"}, nil
	}

	result, err := Run(context.Background(), config.Config{
		OutputPath:  tmpDir,
		TimeBudget:  time.Minute,
		QPS:         1,
		Burst:       1,
		LLMProvider: "groq",
		LLMAPIKey:   "ignored-for-scan",
		LLMModel:    "meta-llama/llama-4-scout-17b-16e-instruct",
		LLMBaseURL:  "https://api.groq.com/openai/v1",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !baselineCalled {
		t.Fatal("expected Run to remain on the baseline path")
	}
	if result.RunMode != "baseline.discovery_full_pass" {
		t.Fatalf("expected baseline run mode, got %q", result.RunMode)
	}
}

func TestBaselineRunModeStable(t *testing.T) {
	result, _ := runBaselineForTest(t)
	if result.RunMode != "baseline.discovery_full_pass" {
		t.Fatalf("expected baseline run mode, got %q", result.RunMode)
	}
}

func TestBaselineArtifactPathsStable(t *testing.T) {
	tmpDir := t.TempDir()
	result, evidenceDir := runBaselineForTest(t, withOutputPath(tmpDir))

	expectedGraphPath := filepath.Join(tmpDir, "graph", "attack-graph.json")
	if result.GraphPath != expectedGraphPath {
		t.Fatalf("expected graph path %q, got %q", expectedGraphPath, result.GraphPath)
	}
	if result.EvidencePath != evidenceDir {
		t.Fatalf("expected evidence dir %q, got %q", evidenceDir, result.EvidencePath)
	}
	expectedSummaryPath := filepath.Join(tmpDir, "baseline-summary.json")
	if result.SummaryPath != expectedSummaryPath {
		t.Fatalf("expected summary path %q, got %q", expectedSummaryPath, result.SummaryPath)
	}
	expectedComparisonPath := filepath.Join(tmpDir, "comparison-baseline.json")
	if result.ComparisonPath != expectedComparisonPath {
		t.Fatalf("expected comparison path %q, got %q", expectedComparisonPath, result.ComparisonPath)
	}

	requiredPaths := []string{
		result.SummaryPath,
		result.ComparisonPath,
		filepath.Join(evidenceDir, "evidence.jsonl"),
		filepath.Join(evidenceDir, "index.json"),
		filepath.Join(evidenceDir, "snapshots", "discovery.list_namespaces"),
		filepath.Join(evidenceDir, "snapshots", "discovery.list_configmaps"),
		result.GraphPath,
	}
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact at %q: %v", path, err)
		}
	}
}

func TestBaselineDoesNotEmitValidationStatus(t *testing.T) {
	result, _ := runBaselineForTest(t)

	contents, err := os.ReadFile(result.GraphPath)
	if err != nil {
		t.Fatalf("read graph output: %v", err)
	}

	var attackGraph graph.AttackGraph
	if err := json.Unmarshal(contents, &attackGraph); err != nil {
		t.Fatalf("unmarshal graph output: %v", err)
	}
	if len(attackGraph.Edges) == 0 {
		t.Fatal("expected baseline scan to emit graph edges")
	}
	for _, edge := range attackGraph.Edges {
		if edge.Status != graph.EdgeTheoretical {
			t.Fatalf("expected theoretical discovery edge status, got %q", edge.Status)
		}
	}
}

func TestBaselineSummaryContract(t *testing.T) {
	tmpDir := t.TempDir()
	result, _ := runBaselineForTest(t, withOutputPath(tmpDir))

	contents, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatalf("read baseline summary: %v", err)
	}

	var summary map[string]any
	if err := json.Unmarshal(contents, &summary); err != nil {
		t.Fatalf("unmarshal baseline summary: %v", err)
	}
	if got := summary["contract_version"]; got != "baseline.discovery.v1" {
		t.Fatalf("expected baseline discovery contract version, got %#v", got)
	}
	if got := summary["run_mode"]; got != "baseline.discovery_full_pass" {
		t.Fatalf("expected baseline run mode in summary, got %#v", got)
	}
	if got := summary["execution_model"]; got != "deterministic_discovery_only" {
		t.Fatalf("expected deterministic discovery execution model, got %#v", got)
	}
	artifacts, ok := summary["artifacts"].(map[string]any)
	if !ok {
		t.Fatalf("expected artifacts object in summary, got %#v", summary["artifacts"])
	}
	if got := artifacts["summary_path"]; got != result.SummaryPath {
		t.Fatalf("expected summary path %#v, got %#v", result.SummaryPath, got)
	}
}

func TestBaselineComparisonContract(t *testing.T) {
	tmpDir := t.TempDir()
	result, _ := runBaselineForTest(t, withOutputPath(tmpDir))

	contents, err := os.ReadFile(result.ComparisonPath)
	if err != nil {
		t.Fatalf("read comparison baseline: %v", err)
	}

	var artifact comparisonBaselineArtifact
	if err := json.Unmarshal(contents, &artifact); err != nil {
		t.Fatalf("unmarshal comparison baseline: %v", err)
	}
	if artifact.ContractVersion != "baseline.comparison.v1" {
		t.Fatalf("expected comparison contract version, got %q", artifact.ContractVersion)
	}
	if artifact.BaselineKind != "discovery" {
		t.Fatalf("expected discovery baseline kind, got %q", artifact.BaselineKind)
	}

	var kg005 comparisonBaselineFamily
	for _, family := range artifact.Families {
		if family.FamilyID == "KG-005" {
			kg005 = family
			break
		}
	}
	if kg005.FamilyID == "" {
		t.Fatal("expected KG-005 family in comparison artifact")
	}
	if len(kg005.Steps) == 0 || kg005.Steps[0].Status != "observed" {
		t.Fatalf("expected KG-005-S1 to be observed, got %#v", kg005.Steps)
	}

	var kg001 comparisonBaselineFamily
	for _, family := range artifact.Families {
		if family.FamilyID == "KG-001" {
			kg001 = family
			break
		}
	}
	if kg001.FamilyID == "" {
		t.Fatal("expected KG-001 family in comparison artifact")
	}
	if len(kg001.Steps) == 0 || kg001.Steps[0].Status != "not_attempted" {
		t.Fatalf("expected KG-001-S1 to remain not_attempted, got %#v", kg001.Steps)
	}
}

// TestBaselineGraphSnapshotLinkage is a focused regression test for evidence
// snapshot references are attached to discovery graph node/edge metadata. It verifies
// that every discovery node and edge carries a "snapshot" meta field pointing to an
// existing snapshot file on disk.
func TestBaselineGraphSnapshotLinkage(t *testing.T) {
	result, evidenceDir := runBaselineForTest(t)

	graphData, err := os.ReadFile(result.GraphPath)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}

	var ag graph.AttackGraph
	if err := json.Unmarshal(graphData, &ag); err != nil {
		t.Fatalf("unmarshal graph: %v", err)
	}

	// Expect foothold node + at least one discovery node.
	if len(ag.Nodes) < 2 {
		t.Fatalf("expected foothold + discovery nodes, got %d nodes", len(ag.Nodes))
	}

	for i, node := range ag.Nodes {
		// Foothold node has no snapshot — only API-call nodes do.
		if node.Kind == "pod" {
			continue
		}
		snapshotVal := node.Meta["snapshot"]
		if snapshotVal == nil || snapshotVal == "" {
			t.Fatalf("node[%d] %s: expected Meta[snapshot] to be set, got nil/empty: %v", i, node.ID, node.Meta)
		}
		snapshotPath, ok := snapshotVal.(string)
		if !ok {
			t.Fatalf("node[%d] %s: expected Meta[snapshot] to be string, got %T", i, node.ID, snapshotVal)
		}
		if _, err := os.Stat(snapshotPath); err != nil {
			t.Fatalf("node[%d] %s: expected snapshot file at %q to exist: %v", i, node.ID, snapshotPath, err)
		}
	}

	for i, edge := range ag.Edges {
		snapshotVal := edge.Meta["snapshot"]
		if snapshotVal == nil || snapshotVal == "" {
			t.Fatalf("edge[%d]: expected Meta[snapshot] to be set, got nil/empty: %v", i, edge.Meta)
		}
		snapshotPath, ok := snapshotVal.(string)
		if !ok {
			t.Fatalf("edge[%d]: expected Meta[snapshot] to be string, got %T", i, snapshotVal)
		}
		if _, err := os.Stat(snapshotPath); err != nil {
			t.Fatalf("edge[%d]: expected snapshot file at %q to exist: %v", i, snapshotPath, err)
		}
	}

	// Also verify the evidence index and snapshot files exist alongside the graph linkage.
	indexPath := filepath.Join(evidenceDir, "index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected evidence index at %q: %v", indexPath, err)
	}
}

func TestBaselineRegistryIncludesProbeNetwork(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	names := registry.Names()
	if !slices.Contains(names, "validation.probe_network") {
		t.Fatalf("expected validation.probe_network in registry names, got %v", names)
	}
}

func TestBaselineToolSchemas_ProbeNetworkExportsExplicitSchema(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	definitions, err := registry.Definitions([]string{"validation.probe_network"})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	parameters := definitions[0].Parameters
	if got := parameters["type"]; got != "object" {
		t.Fatalf("expected object schema, got %#v", got)
	}
	properties, ok := parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %#v", parameters["properties"])
	}

	for _, name := range []string{"probe", "target", "url", "port", "timeout_seconds", "retries"} {
		if _, ok := properties[name]; !ok {
			t.Fatalf("expected probe_network property %q, got %#v", name, properties)
		}
	}

	probe, ok := properties["probe"].(map[string]any)
	if !ok {
		t.Fatalf("expected probe schema map, got %#v", properties["probe"])
	}
	if got := probe["default"]; got != "tcp" {
		t.Fatalf("expected probe default tcp, got %#v", got)
	}
	if got := properties["timeout_seconds"].(map[string]any)["default"]; got != 5 {
		t.Fatalf("expected timeout_seconds default 5, got %#v", got)
	}
	if got := properties["retries"].(map[string]any)["default"]; got != 1 {
		t.Fatalf("expected retries default 1, got %#v", got)
	}

	if reflect.DeepEqual(parameters, tools.EmptyObjectSchema().Map()) {
		t.Fatalf("expected explicit schema richer than empty object, got %#v", parameters)
	}
}

func TestBaselineToolSchemas_ZeroInputToolsExportEmptyObject(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	names := []string{
		"discovery.list_namespaces",
		"discovery.list_clusterroles",
		"discovery.list_clusterrolebindings",
	}
	sort.Strings(names)

	definitions, err := registry.Definitions(names)
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != len(names) {
		t.Fatalf("expected %d definitions, got %d", len(names), len(definitions))
	}

	expectedSchema := tools.EmptyObjectSchema().Map()
	for i, definition := range definitions {
		if definition.Name != names[i] {
			t.Fatalf("expected definition %d to be %q, got %q", i, names[i], definition.Name)
		}
		if !reflect.DeepEqual(definition.Parameters, expectedSchema) {
			t.Fatalf("expected %q to export empty object schema %#v, got %#v", definition.Name, expectedSchema, definition.Parameters)
		}
	}
}

func TestBaselineToolSchemas_NamespaceScopedToolsExposeOptionalNamespace(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	names := []string{
		"discovery.list_configmaps",
		"discovery.list_endpoints",
		"discovery.list_networkpolicies",
		"discovery.list_pods",
		"discovery.list_rolebindings",
		"discovery.list_roles",
		"discovery.list_secrets",
		"discovery.list_serviceaccounts",
		"discovery.list_services",
		"introspection.get_effective_permissions",
	}
	sort.Strings(names)

	definitions, err := registry.Definitions(names)
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != len(names) {
		t.Fatalf("expected %d definitions, got %d", len(names), len(definitions))
	}

	expectedSchema := tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Kubernetes namespace to query",
				Default:     "default",
			},
		},
	}.Map()

	expectedIntrospectionSchema := tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Kubernetes namespace to evaluate",
				Default:     "default",
			},
		},
	}.Map()

	for i, definition := range definitions {
		if definition.Name != names[i] {
			t.Fatalf("expected definition %d to be %q, got %q", i, names[i], definition.Name)
		}

		expected := expectedSchema
		if definition.Name == "introspection.get_effective_permissions" {
			expected = expectedIntrospectionSchema
		}

		if !reflect.DeepEqual(definition.Parameters, expected) {
			t.Fatalf("expected %q to export namespace schema %#v, got %#v", definition.Name, expected, definition.Parameters)
		}

		if _, ok := definition.Parameters["required"]; ok {
			t.Fatalf("expected %q namespace schema to omit required fields, got %#v", definition.Name, definition.Parameters)
		}
	}
}

func TestBaselineToolSchemas_CheckPermissionsExportsExplicitOptionalSchema(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	definitions, err := registry.Definitions([]string{"validation.check_permissions"})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	expected := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"namespace": map[string]any{
				"type":        "string",
				"description": "Namespace to evaluate",
				"default":     "default",
			},
			"verb": map[string]any{
				"type":        "string",
				"description": "Kubernetes verb to check",
				"default":     "get",
			},
			"resource": map[string]any{
				"type":        "string",
				"description": "Kubernetes resource to check",
				"default":     "secrets",
			},
			"api_group": map[string]any{
				"type":        "string",
				"description": "Optional Kubernetes API group",
			},
			"subresource": map[string]any{
				"type":        "string",
				"description": "Optional Kubernetes subresource",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Optional resource name",
			},
		},
	}

	if !reflect.DeepEqual(definitions[0].Parameters, expected) {
		t.Fatalf("expected explicit check_permissions schema %#v, got %#v", expected, definitions[0].Parameters)
	}
	if _, ok := definitions[0].Parameters["required"]; ok {
		t.Fatalf("expected check_permissions schema to omit required fields, got %#v", definitions[0].Parameters["required"])
	}
}

func TestBaselineToolRegistry_GeneratedSchemasCoverAllRegisteredTools(t *testing.T) {
	registry, err := newBaselineToolRegistry(newFakeK8sClient(t), nil, nil)
	if err != nil {
		t.Fatalf("create baseline registry: %v", err)
	}

	registeredNames := registry.Names()
	sort.Strings(registeredNames)

	definitions, err := registry.Definitions(registeredNames)
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}
	if len(definitions) != len(registeredNames) {
		t.Fatalf("expected %d definitions, got %d", len(registeredNames), len(definitions))
	}

	definitionsByName := make(map[string]tools.Definition, len(definitions))
	for i, definition := range definitions {
		if definition.Name != registeredNames[i] {
			t.Fatalf("expected definition %d to be %q, got %q", i, registeredNames[i], definition.Name)
		}
		if _, exists := definitionsByName[definition.Name]; exists {
			t.Fatalf("expected %q to appear exactly once in exported definitions", definition.Name)
		}
		if definition.Parameters == nil {
			t.Fatalf("expected %q to export non-nil schema metadata", definition.Name)
		}
		if got := definition.Parameters["type"]; got != "object" {
			t.Fatalf("expected %q schema type object, got %#v", definition.Name, got)
		}
		properties, ok := definition.Parameters["properties"].(map[string]any)
		if !ok {
			t.Fatalf("expected %q schema properties map, got %#v", definition.Name, definition.Parameters["properties"])
		}
		if properties == nil {
			t.Fatalf("expected %q to export non-nil property metadata", definition.Name)
		}
		definitionsByName[definition.Name] = definition
	}
	if len(definitionsByName) != len(registeredNames) {
		t.Fatalf("expected %d unique exported definitions, got %d", len(registeredNames), len(definitionsByName))
	}

	zeroInput := definitionsByName["discovery.list_namespaces"].Parameters
	if !reflect.DeepEqual(zeroInput, tools.EmptyObjectSchema().Map()) {
		t.Fatalf("expected discovery.list_namespaces to export empty object schema %#v, got %#v", tools.EmptyObjectSchema().Map(), zeroInput)
	}

	namespaceOnly := definitionsByName["discovery.list_pods"].Parameters
	if _, ok := namespaceOnly["required"]; ok {
		t.Fatalf("expected discovery.list_pods namespace schema to omit required fields, got %#v", namespaceOnly["required"])
	}
	if got := namespaceOnly["properties"].(map[string]any)["namespace"]; !reflect.DeepEqual(got, map[string]any{
		"type":        "string",
		"description": "Kubernetes namespace to query",
		"default":     "default",
	}) {
		t.Fatalf("expected discovery.list_pods namespace property, got %#v", got)
	}

	checkPermissions := definitionsByName["validation.check_permissions"].Parameters
	if got := checkPermissions["additionalProperties"]; got != false {
		t.Fatalf("expected validation.check_permissions to reject undeclared keys, got %#v", got)
	}
	if _, ok := checkPermissions["required"]; ok {
		t.Fatalf("expected validation.check_permissions schema to omit required fields, got %#v", checkPermissions["required"])
	}
	checkPermissionsProperties := checkPermissions["properties"].(map[string]any)
	for property, expected := range map[string]map[string]any{
		"namespace": {
			"type":        "string",
			"description": "Namespace to evaluate",
			"default":     "default",
		},
		"verb": {
			"type":        "string",
			"description": "Kubernetes verb to check",
			"default":     "get",
		},
		"resource": {
			"type":        "string",
			"description": "Kubernetes resource to check",
			"default":     "secrets",
		},
		"api_group": {
			"type":        "string",
			"description": "Optional Kubernetes API group",
		},
		"subresource": {
			"type":        "string",
			"description": "Optional Kubernetes subresource",
		},
		"name": {
			"type":        "string",
			"description": "Optional resource name",
		},
	} {
		if got := checkPermissionsProperties[property]; !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected validation.check_permissions property %q to be %#v, got %#v", property, expected, got)
		}
	}

	readSecret := definitionsByName["validation.read_secret"].Parameters
	if got := readSecret["additionalProperties"]; got != false {
		t.Fatalf("expected validation.read_secret to reject undeclared keys, got %#v", got)
	}
	if got := readSecret["required"]; !reflect.DeepEqual(got, []string{"name"}) {
		t.Fatalf("expected validation.read_secret required fields [name], got %#v", got)
	}
	readSecretProperties := readSecret["properties"].(map[string]any)
	if got := readSecretProperties["name"]; !reflect.DeepEqual(got, map[string]any{
		"type":        "string",
		"description": "Secret name to read",
	}) {
		t.Fatalf("expected validation.read_secret name property, got %#v", got)
	}
	if got := readSecretProperties["namespace"]; !reflect.DeepEqual(got, map[string]any{
		"type":        "string",
		"description": "Namespace containing the Secret",
		"default":     "default",
	}) {
		t.Fatalf("expected validation.read_secret namespace property, got %#v", got)
	}
	if got := readSecretProperties["allow_namespaces"]; !reflect.DeepEqual(got, map[string]any{
		"type":        "array",
		"description": "Optional namespace allow-list guardrail",
		"items": map[string]any{
			"type": "string",
		},
	}) {
		t.Fatalf("expected validation.read_secret allow_namespaces property, got %#v", got)
	}

	probeNetwork := definitionsByName["validation.probe_network"].Parameters
	if got := probeNetwork["additionalProperties"]; got != false {
		t.Fatalf("expected validation.probe_network to reject undeclared keys, got %#v", got)
	}
	if got := probeNetwork["description"]; got != "Probe-specific inputs for bounded TCP, HTTP, or DNS validation. Conditional requirements are described per field rather than enforced with JSON Schema conditionals." {
		t.Fatalf("expected validation.probe_network top-level description, got %#v", got)
	}
	probeNetworkProperties := probeNetwork["properties"].(map[string]any)
	if got := probeNetworkProperties["probe"]; !reflect.DeepEqual(got, map[string]any{
		"type":        "string",
		"description": "Probe type to run. Use tcp for host:port reachability, http for a single GET request, or dns for hostname resolution.",
		"enum":        []string{"tcp", "http", "dns"},
		"default":     "tcp",
	}) {
		t.Fatalf("expected validation.probe_network probe property, got %#v", got)
	}
	for property, expectedDefault := range map[string]int{
		"timeout_seconds": 5,
		"retries":         1,
	} {
		propertySchema, ok := probeNetworkProperties[property].(map[string]any)
		if !ok {
			t.Fatalf("expected validation.probe_network %q property map, got %#v", property, probeNetworkProperties[property])
		}
		if got := propertySchema["default"]; got != expectedDefault {
			t.Fatalf("expected validation.probe_network %q default %d, got %#v", property, expectedDefault, got)
		}
	}
}

type baselineTestOption func(*config.Config)

func withOutputPath(path string) baselineTestOption {
	return func(cfg *config.Config) {
		cfg.OutputPath = path
	}
}

func runBaselineForTest(t *testing.T, opts ...baselineTestOption) (RunResult, string) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := config.Config{
		OutputPath:          tmpDir,
		TimeBudget:          time.Minute,
		AllowListNamespaces: []string{"team-a"},
		QPS:                 1,
		Burst:               1,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	client := newFakeK8sClient(t)
	enforcer := guardrails.New(cfg.AllowListNamespaces, cfg.QPS, cfg.Burst)
	evidenceDir := filepath.Join(cfg.OutputPath, "evidence")
	collector, err := evidence.NewCollector(evidenceDir)
	if err != nil {
		t.Fatalf("create collector: %v", err)
	}
	t.Cleanup(func() {
		if err := collector.Close(); err != nil {
			t.Fatalf("close collector: %v", err)
		}
	})

	result, err := runBaselineDiscoveryScan(context.Background(), cfg, time.Now().UTC(), client, enforcer, collector, evidence.NewSnapshotWriter(evidenceDir))
	if err != nil {
		t.Fatalf("run baseline discovery scan: %v", err)
	}
	return result, evidenceDir
}

func newFakeK8sClient(t *testing.T) *k8s.Client {
	t.Helper()

	objects := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", ResourceVersion: "1", CreationTimestamp: metav1.NewTime(time.Unix(1, 0))}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "team-a", ResourceVersion: "2", CreationTimestamp: metav1.NewTime(time.Unix(2, 0))}, Spec: corev1.PodSpec{ServiceAccountName: "default"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "team-a", ResourceVersion: "3", CreationTimestamp: metav1.NewTime(time.Unix(3, 0))}, Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1", Type: corev1.ServiceTypeClusterIP}},
		&corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "team-a", ResourceVersion: "4", CreationTimestamp: metav1.NewTime(time.Unix(4, 0))}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "team-a", ResourceVersion: "5", CreationTimestamp: metav1.NewTime(time.Unix(5, 0))}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config-a", Namespace: "team-a", ResourceVersion: "5a", CreationTimestamp: metav1.NewTime(time.Unix(5, 500))}, Data: map[string]string{"config.yaml": "value"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret-a", Namespace: "team-a", ResourceVersion: "6", CreationTimestamp: metav1.NewTime(time.Unix(6, 0))}, Type: corev1.SecretTypeOpaque},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-a", Namespace: "team-a", ResourceVersion: "7", CreationTimestamp: metav1.NewTime(time.Unix(7, 0))}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb-a", Namespace: "team-a", ResourceVersion: "8", CreationTimestamp: metav1.NewTime(time.Unix(8, 0))}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "cr-a", ResourceVersion: "9", CreationTimestamp: metav1.NewTime(time.Unix(9, 0))}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb-a", ResourceVersion: "10", CreationTimestamp: metav1.NewTime(time.Unix(10, 0))}},
	}

	return &k8s.Client{
		Config:    &rest.Config{Host: "https://example.invalid"},
		Clientset: fake.NewSimpleClientset(objects...),
	}
}
