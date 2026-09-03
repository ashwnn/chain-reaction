package validation

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type CheckPermissionsTool struct {
	client *k8s.Client
}

func NewCheckPermissionsTool(client *k8s.Client) *CheckPermissionsTool {
	return &CheckPermissionsTool{client: client}
}

func (t *CheckPermissionsTool) Name() string {
	return "validation.check_permissions"
}

func (t *CheckPermissionsTool) Description() string {
	return "Checks whether current identity can perform a specific Kubernetes API action"
}

func (t *CheckPermissionsTool) ParameterSchema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Namespace to evaluate",
				Default:     "default",
			},
			"verb": {
				Type:        "string",
				Description: "Kubernetes verb to check",
				Default:     "get",
			},
			"resource": {
				Type:        "string",
				Description: "Kubernetes resource to check",
				Default:     "secrets",
			},
			"api_group": {
				Type:        "string",
				Description: "Optional Kubernetes API group",
			},
			"subresource": {
				Type:        "string",
				Description: "Optional Kubernetes subresource",
			},
			"name": {
				Type:        "string",
				Description: "Optional resource name",
			},
		},
	}
}

func (t *CheckPermissionsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := "default"
	verb := "get"
	resource := "secrets"
	apiGroup := ""
	subresource := ""
	name := ""

	if input != nil {
		if v, ok := input["namespace"].(string); ok && v != "" {
			namespace = v
		}
		if v, ok := input["verb"].(string); ok && v != "" {
			verb = v
		}
		if v, ok := input["resource"].(string); ok && v != "" {
			resource = v
		}
		if v, ok := input["api_group"].(string); ok {
			apiGroup = v
		}
		if v, ok := input["subresource"].(string); ok {
			subresource = v
		}
		if v, ok := input["name"].(string); ok {
			name = v
		}
	}

	result, err := t.client.CanI(ctx, namespace, verb, resource, apiGroup, subresource, name)
	if err != nil {
		return nil, err
	}

	output := map[string]any{
		"namespace":        result.Namespace,
		"verb":             result.Verb,
		"resource":         result.Resource,
		"api_group":        result.APIGroup,
		"subresource":      result.Subresource,
		"name":             result.Name,
		"allowed":          result.Allowed,
		"denied":           result.Denied,
		"reason":           result.Reason,
		"evaluation_error": result.EvaluationError,
	}

	// Taxonomy-typed outcome fields. These express the same outcome using
	// StepResult/FailureReason vocabulary so the validation loop graph mapping
	// can consume them without needing to inspect raw RBAC fields.
	if result.Allowed {
		output["result"] = string(StepValidated)
	} else {
		output["result"] = string(StepFailed)
		// SelfSubjectAccessReview denial is always an RBAC denial.
		output["failure_reason"] = string(FailureRBACDenied)
	}

	return output, nil
}
