package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

// tokenPath is the path to the mounted ServiceAccount token. It defaults to the standard
// in-cluster path but can be overridden by tests via tokenPath().
var mountedTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// tokenPath returns the configured token path. Exists as a function so tests can
// inject a temp file path without modifying the package-level variable directly.
func tokenPath() string {
	return mountedTokenPath
}

type CheckTokenTool struct {
	client *k8s.Client
}

func NewCheckTokenTool(client *k8s.Client) *CheckTokenTool {
	return &CheckTokenTool{client: client}
}

func (t *CheckTokenTool) Name() string {
	return "validation.check_token"
}

func (t *CheckTokenTool) Description() string {
	return "Reads a ServiceAccount, its mounted token-secret metadata, and the in-cluster mounted token JWT claims. Includes cluster-confirmed effective permissions summary. Never exposes raw token bytes or base64 segments."
}

func (t *CheckTokenTool) ParameterSchema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Namespace containing the ServiceAccount. Defaults to the mounted token namespace when omitted.",
				Default:     "default",
			},
			"name": {
				Type:        "string",
				Description: "ServiceAccount name to inspect. Defaults to the mounted token identity when omitted.",
			},
		},
	}
}

func (t *CheckTokenTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := "default"
	namespaceProvided := false
	name := ""

	if input != nil {
		if v, ok := input["namespace"].(string); ok && v != "" {
			namespace = v
			namespaceProvided = true
		}
		if v, ok := input["name"].(string); ok && v != "" {
			name = v
		}
	}

	// Attempt to read and parse the mounted ServiceAccount token for safe claim metadata.
	// Claims are parsed without signature verification — they are metadata assertions,
	// not cluster-confirmed identity facts.
	//
	// Semantic honesty: the token-inspection tool fails if the token file is missing,
	// unreadable, malformed, or expired. A successful token read is required for
	// StepValidated. This is distinct from SA metadata read (which may succeed even if
	// the pod has no token) and effective permissions (which are informative only).
	tokenMeta, tokenOK, tokenErr := k8s.ReadMountedTokenMetadata(tokenPath())

	if name == "" && tokenOK && tokenMeta.ServiceAccountName != "" {
		name = tokenMeta.ServiceAccountName
	}
	if !namespaceProvided && tokenOK && tokenMeta.Namespace != "" {
		namespace = tokenMeta.Namespace
	}

	if name == "" {
		return nil, fmt.Errorf("name is required when mounted token identity is unavailable")
	}

	detail, err := t.client.GetServiceAccountWithTokenSecrets(ctx, namespace, name)
	if err != nil {
		reason := FailureToolOrRuntimeError
		if k8s.IsSecretAccessForbidden(err) {
			reason = FailureRBACDenied
		} else if k8s.IsSecretMissing(err) {
			// ServiceAccount not found is semantically a missing prerequisite.
			reason = FailureMissingPrerequisite
		}
		return map[string]any{
			"namespace": namespace,
			"name":      name,
			"status":    string(StepFailed),
			"reason":    string(reason),
			"error":     err.Error(),
		}, nil
	}

	if !tokenOK {
		// Token file missing or unreadable. This is an auth failure, not an RBAC denial.
		// The tool cannot validate the token without reading it.
		errMsg := "token file not found or not readable"
		if tokenErr != nil {
			errMsg = tokenErr.Error()
		}
		return map[string]any{
			"namespace": namespace,
			"name":      name,
			"status":    string(StepFailed),
			"reason":    string(FailureAuthFailed),
			"error":     errMsg,
		}, nil
	}

	if tokenMeta.Expiry > 0 {
		now := time.Now().Unix()
		if tokenMeta.Expiry < now {
			// Token has expired. Distinct from "missing token" — the file was readable
			// but the token is no longer valid for authentication.
			return map[string]any{
				"namespace": namespace,
				"name":      name,
				"status":    string(StepFailed),
				"reason":    string(FailureTokenExpired),
				"error":     fmt.Sprintf("mounted token expired at %d (current: %d)", tokenMeta.Expiry, now),
			}, nil
		}
	}

	if tokenMeta.ServiceAccountName != "" && tokenMeta.ServiceAccountName != name {
		return map[string]any{
			"namespace": namespace,
			"name":      name,
			"status":    string(StepFailed),
			"reason":    string(FailureMissingPrerequisite),
			"error":     fmt.Sprintf("mounted token belongs to service account %s, not requested %s", tokenMeta.ServiceAccountName, name),
		}, nil
	}
	if tokenMeta.Namespace != "" && tokenMeta.Namespace != namespace {
		return map[string]any{
			"namespace": namespace,
			"name":      name,
			"status":    string(StepFailed),
			"reason":    string(FailureMissingPrerequisite),
			"error":     fmt.Sprintf("mounted token belongs to namespace %s, not requested %s", tokenMeta.Namespace, namespace),
		}, nil
	}

	// Token is readable and not expired. Retrieve effective permissions and build output.
	permsSummary, permsErr := t.client.GetEffectivePermissionsSummary(ctx, namespace)

	output := map[string]any{
		"namespace":         namespace,
		"name":              name,
		"status":            string(StepValidated),
		"reason":            "service_account_inspected",
		"service_account":   detail,
		"has_token_secrets": len(detail.TokenSecrets) > 0,
		"token_claims":      tokenMeta,
	}
	if permsErr == nil {
		output["effective_permissions"] = permsSummary
	}
	// If permsErr != nil, omit the field. The token was successfully validated.

	return output, nil
}
