// Package baseline provides the step-chain catalog and post-hoc scenario matcher
// for computing validation metrics against the baseline scenario families (KG-001
// through KG-005). The catalog is derived from built-in catalog
//
// This package must not import internal/metrics to avoid circular dependencies.
package baseline

const totalFamilies = 5

// Catalog represents the full step-chain catalog with all in-scope families.
type Catalog struct {
	Families []Family `json:"families"`
}

// Family represents one scenario family (e.g., KG-001 RBAC Over-Provisioning).
type Family struct {
	ID          string `json:"id"`          // e.g., "KG-001"
	Name        string `json:"name"`        // e.g., "RBAC Over-Provisioning"
	Description string `json:"description"` // human-readable intent
	Steps       []Step `json:"steps"`       // ordered by SeqNum
}

// Step represents one validation step within a family's minimum chain.
type Step struct {
	StepID        string   `json:"step_id"`        // e.g., "KG-001-S1"
	SeqNum        int      `json:"seq_num"`        // 1-based position in chain
	Description   string   `json:"description"`    // what this step validates
	ExpectedTools []string `json:"expected_tools"` // tool names that can satisfy this step
	Prerequisites []string `json:"prerequisites"`  // StepIDs that must be validated first
}

// DefaultCatalog returns the v1 step-chain catalog derived from
// built-in catalog The catalog is immutable after
// creation; callers should treat it as read-only.
func DefaultCatalog() Catalog {
	return Catalog{
		Families: []Family{
			{
				ID:          "KG-001",
				Name:        "RBAC Over-Provisioning",
				Description: "Confirm that the pod's SA has more permissions than required for its function",
				Steps: []Step{
					{
						StepID:        "KG-001-S1",
						SeqNum:        1,
						Description:   "Identify current SA identity and token context",
						ExpectedTools: []string{"validation.check_token"},
						Prerequisites: nil,
					},
					{
						StepID:        "KG-001-S2",
						SeqNum:        2,
						Description:   "Enumerate effective permissions; confirm sensitive access",
						ExpectedTools: []string{"validation.check_permissions"},
						Prerequisites: []string{"KG-001-S1"},
					},
					{
						StepID:        "KG-001-S3",
						SeqNum:        3,
						Description:   "Exercise the over-provisioned permission to prove exploitability",
						ExpectedTools: []string{"validation.read_secret", "validation.check_permissions"},
						Prerequisites: []string{"KG-001-S2"},
					},
				},
			},
			{
				ID:          "KG-002",
				Name:        "Secret or ConfigMap Data Access",
				Description: "Confirm that the pod's SA can read sensitive data from a Secret or ConfigMap",
				Steps: []Step{
					{
						StepID:        "KG-002-S1",
						SeqNum:        1,
						Description:   "Confirm permission to read secrets in target namespace",
						ExpectedTools: []string{"validation.check_permissions"},
						Prerequisites: nil,
					},
					{
						StepID:        "KG-002-S2",
						SeqNum:        2,
						Description:   "Read the target secret object",
						ExpectedTools: []string{"validation.read_secret"},
						Prerequisites: []string{"KG-002-S1"},
					},
				},
			},
			{
				ID:          "KG-003",
				Name:        "ServiceAccount Token Abuse",
				Description: "Confirm that the mounted SA token represents an identity with exploitable permissions",
				Steps: []Step{
					{
						StepID:        "KG-003-S1",
						SeqNum:        1,
						Description:   "Inspect mounted token and SA identity context",
						ExpectedTools: []string{"validation.check_token"},
						Prerequisites: nil,
					},
					{
						StepID:        "KG-003-S2",
						SeqNum:        2,
						Description:   "Confirm permissions enabled by the token's SA identity",
						ExpectedTools: []string{"validation.check_permissions"},
						Prerequisites: []string{"KG-003-S1"},
					},
				},
			},
			{
				ID:          "KG-004",
				Name:        "Network Reachability Pivot",
				Description: "Confirm that the pod can reach a network target that supports lateral movement",
				Steps: []Step{
					{
						StepID:        "KG-004-S1",
						SeqNum:        1,
						Description:   "Resolve and confirm reachability to a service endpoint",
						ExpectedTools: []string{"validation.probe_network"},
						Prerequisites: nil,
					},
					{
						StepID:        "KG-004-S2",
						SeqNum:        2,
						Description:   "Confirm connectivity to a secondary target or cross-namespace endpoint",
						ExpectedTools: []string{"validation.probe_network"},
						Prerequisites: []string{"KG-004-S1"},
					},
				},
			},
			{
				ID:          "KG-005",
				Name:        "Namespace Bypass or Cross-Namespace Access",
				Description: "Confirm that the pod can access resources or services in a namespace other than its own",
				Steps: []Step{
					{
						StepID:        "KG-005-S1",
						SeqNum:        1,
						Description:   "Enumerate namespaces beyond the pod's own",
						ExpectedTools: []string{"validation.check_permissions", "discovery.list_namespaces"},
						Prerequisites: nil,
					},
					{
						StepID:        "KG-005-S2",
						SeqNum:        2,
						Description:   "Confirm cross-namespace service reachability",
						ExpectedTools: []string{"validation.probe_network"},
						Prerequisites: []string{"KG-005-S1"},
					},
					{
						StepID:        "KG-005-S3",
						SeqNum:        3,
						Description:   "Confirm cross-namespace API access to a sensitive resource",
						ExpectedTools: []string{"validation.check_permissions", "validation.read_secret"},
						Prerequisites: []string{"KG-005-S2"},
					},
				},
			},
		},
	}
}
