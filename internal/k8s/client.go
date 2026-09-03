package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Config    *rest.Config
	Clientset kubernetes.Interface
}

func NewClient(kubeconfig string, qps float32, burst int) (*Client, error) {
	cfg, err := buildRESTConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	if qps > 0 {
		cfg.QPS = qps
	}
	if burst > 0 {
		cfg.Burst = burst
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %w", err)
	}

	return &Client{Config: cfg, Clientset: clientset}, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	list, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]NamespaceInfo, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, NamespaceInfo{
			Name:            item.Name,
			Labels:          item.Labels,
			Status:          string(item.Status.Phase),
			ResourceVersion: item.ResourceVersion,
			Created:         item.CreationTimestamp,
		})
	}
	return result, nil
}

// PodInfo represents a normalized pod with metadata for attack chain discovery
type PodInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels,omitempty"`
	ServiceAccount  string            `json:"service_account"`
	NodeName        string            `json:"node_name,omitempty"`
	PodIP           string            `json:"pod_ip,omitempty"`
	HostIP          string            `json:"host_ip,omitempty"`
	Status          string            `json:"status"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// NamespaceInfo represents a normalized namespace with metadata
type NamespaceInfo struct {
	Name            string            `json:"name"`
	Labels          map[string]string `json:"labels,omitempty"`
	Status          string            `json:"status"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ListPods returns normalized pod information for a namespace
func (c *Client) ListPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]PodInfo, 0, len(pods.Items))
	for _, p := range pods.Items {
		sa := "default"
		if p.Spec.ServiceAccountName != "" {
			sa = p.Spec.ServiceAccountName
		}
		result = append(result, PodInfo{
			Name:            p.Name,
			Namespace:       p.Namespace,
			Labels:          p.Labels,
			ServiceAccount:  sa,
			NodeName:        p.Spec.NodeName,
			PodIP:           p.Status.PodIP,
			HostIP:          p.Status.HostIP,
			Status:          string(p.Status.Phase),
			ResourceVersion: p.ResourceVersion,
			Created:         p.CreationTimestamp,
		})
	}
	return result, nil
}

// ServiceInfo represents a normalized service
type ServiceInfo struct {
	Name            string               `json:"name"`
	Namespace       string               `json:"namespace"`
	Type            string               `json:"type"`
	ClusterIP       string               `json:"cluster_ip"`
	ExternalIPs     []string             `json:"external_ips,omitempty"`
	Ports           []corev1.ServicePort `json:"ports,omitempty"`
	Selector        map[string]string    `json:"selector,omitempty"`
	ResourceVersion string               `json:"resource_version"`
	Created         metav1.Time          `json:"created"`
}

// ListServices returns normalized service information for a namespace
func (c *Client) ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	svcs, err := c.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ServiceInfo, 0, len(svcs.Items))
	for _, s := range svcs.Items {
		result = append(result, ServiceInfo{
			Name:            s.Name,
			Namespace:       s.Namespace,
			Type:            string(s.Spec.Type),
			ClusterIP:       s.Spec.ClusterIP,
			ExternalIPs:     s.Spec.ExternalIPs,
			Ports:           s.Spec.Ports,
			Selector:        s.Spec.Selector,
			ResourceVersion: s.ResourceVersion,
			Created:         s.CreationTimestamp,
		})
	}
	return result, nil
}

// EndpointInfo represents normalized endpoint information
type EndpointInfo struct {
	Name            string                `json:"name"`
	Namespace       string                `json:"namespace"`
	Addresses       []string              `json:"addresses"`
	Ports           []corev1.EndpointPort `json:"ports,omitempty"`
	ResourceVersion string                `json:"resource_version"`
	Created         metav1.Time           `json:"created"`
}

// ListEndpoints returns normalized endpoint information for a namespace
func (c *Client) ListEndpoints(ctx context.Context, namespace string) ([]EndpointInfo, error) {
	eps, err := c.Clientset.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]EndpointInfo, 0, len(eps.Items))
	for _, e := range eps.Items {
		var addresses []string
		var ports []corev1.EndpointPort
		for _, subset := range e.Subsets {
			for _, addr := range subset.Addresses {
				addresses = append(addresses, addr.IP)
			}
			ports = append(ports, subset.Ports...)
		}
		result = append(result, EndpointInfo{
			Name:            e.Name,
			Namespace:       e.Namespace,
			Addresses:       addresses,
			Ports:           ports,
			ResourceVersion: e.ResourceVersion,
			Created:         e.CreationTimestamp,
		})
	}
	return result, nil
}

// ServiceAccountInfo represents a normalized service account
type ServiceAccountInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels,omitempty"`
	Secrets         []string          `json:"secrets,omitempty"`
	AutomountToken  bool              `json:"automount_token"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ListServiceAccounts returns normalized service account information for a namespace
func (c *Client) ListServiceAccounts(ctx context.Context, namespace string) ([]ServiceAccountInfo, error) {
	sas, err := c.Clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ServiceAccountInfo, 0, len(sas.Items))
	for _, sa := range sas.Items {
		automountToken := true
		if sa.AutomountServiceAccountToken != nil {
			automountToken = *sa.AutomountServiceAccountToken
		}

		secrets := make([]string, 0, len(sa.Secrets))
		for _, s := range sa.Secrets {
			secrets = append(secrets, s.Name)
		}
		result = append(result, ServiceAccountInfo{
			Name:            sa.Name,
			Namespace:       sa.Namespace,
			Labels:          sa.Labels,
			Secrets:         secrets,
			AutomountToken:  automountToken,
			ResourceVersion: sa.ResourceVersion,
			Created:         sa.CreationTimestamp,
		})
	}
	return result, nil
}

// GetServiceAccountWithTokenSecrets returns full SA metadata including the annotations
// and metadata of any mounted token secrets. It never reads or emits secret.Data bytes.
// This is the Token Projector's primary API surface: cluster-confirmed identity metadata
// without exposing the actual ServiceAccount token.
func (c *Client) GetServiceAccountWithTokenSecrets(ctx context.Context, namespace, name string) (ServiceAccountDetail, error) {
	sa, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ServiceAccountDetail{}, err
	}

	automountToken := true
	if sa.AutomountServiceAccountToken != nil {
		automountToken = *sa.AutomountServiceAccountToken
	}

	detail := ServiceAccountDetail{
		Name:            sa.Name,
		Namespace:       sa.Namespace,
		Labels:          sa.Labels,
		Annotations:     sa.Annotations,
		AutomountToken:  automountToken,
		ResourceVersion: sa.ResourceVersion,
		Created:         sa.CreationTimestamp,
	}

	// Read metadata for any service-account token secrets.
	// Filter by annotation to distinguish token secrets from other secret types.
	for _, ref := range sa.Secrets {
		secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			// Skip secrets we can't read — likely a permissions issue.
			continue
		}
		// Only include secrets that are explicitly service-account tokens.
		if secret.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		// Extract annotation-only metadata; never touch secret.Data.
		detail.TokenSecrets = append(detail.TokenSecrets, TokenSecretMetadata{
			Name:            secret.Name,
			Namespace:       secret.Namespace,
			Annotations:     secret.Annotations,
			ResourceVersion: secret.ResourceVersion,
			Created:         secret.CreationTimestamp,
		})
	}

	return detail, nil
}

type ConfigMapInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels,omitempty"`
	Keys            []string          `json:"keys"`
	BinaryKeys      []string          `json:"binary_keys,omitempty"`
	Immutable       bool              `json:"immutable"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

func (c *Client) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMapInfo, error) {
	configMaps, err := c.Clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ConfigMapInfo, 0, len(configMaps.Items))
	for _, cm := range configMaps.Items {
		keys := make([]string, 0, len(cm.Data))
		for key := range cm.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		binaryKeys := make([]string, 0, len(cm.BinaryData))
		for key := range cm.BinaryData {
			binaryKeys = append(binaryKeys, key)
		}
		sort.Strings(binaryKeys)

		immutable := false
		if cm.Immutable != nil {
			immutable = *cm.Immutable
		}

		result = append(result, ConfigMapInfo{
			Name:            cm.Name,
			Namespace:       cm.Namespace,
			Labels:          cm.Labels,
			Keys:            keys,
			BinaryKeys:      binaryKeys,
			Immutable:       immutable,
			ResourceVersion: cm.ResourceVersion,
			Created:         cm.CreationTimestamp,
		})
	}
	return result, nil
}

type EffectivePermissions struct {
	Namespace        string                            `json:"namespace"`
	ResourceRules    []authorizationv1.ResourceRule    `json:"resource_rules"`
	NonResourceRules []authorizationv1.NonResourceRule `json:"non_resource_rules,omitempty"`
	Incomplete       bool                              `json:"incomplete"`
	EvaluationError  string                            `json:"evaluation_error,omitempty"`
}

func (c *Client) GetEffectivePermissions(ctx context.Context, namespace string) (EffectivePermissions, error) {
	review, err := c.Clientset.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{Namespace: namespace},
	}, metav1.CreateOptions{})
	if err != nil {
		return EffectivePermissions{}, err
	}

	return EffectivePermissions{
		Namespace:        namespace,
		ResourceRules:    review.Status.ResourceRules,
		NonResourceRules: review.Status.NonResourceRules,
		Incomplete:       review.Status.Incomplete,
		EvaluationError:  review.Status.EvaluationError,
	}, nil
}

// EffectivePermissionsSummary is a compact summary of effective permissions.
// This is the safe output for auth-scope validation: rule counts and evaluation
// metadata, without the full rule lists that would bloat the output.
type EffectivePermissionsSummary struct {
	Namespace            string `json:"namespace"`
	ResourceRuleCount    int    `json:"resource_rule_count"`
	NonResourceRuleCount int    `json:"non_resource_rule_count"`
	Incomplete           bool   `json:"incomplete"`
	EvaluationError      string `json:"evaluation_error,omitempty"`
}

// GetEffectivePermissionsSummary returns a compact summary of the current identity's
// effective permissions in the given namespace. Unlike GetEffectivePermissions which
// returns full rule lists, this returns only counts and evaluation metadata.
// This is the Token Projector's auth-scope validation output: cluster-confirmed scope
// via the SelfSubjectRulesReview API, not token-claim inference.
func (c *Client) GetEffectivePermissionsSummary(ctx context.Context, namespace string) (EffectivePermissionsSummary, error) {
	full, err := c.GetEffectivePermissions(ctx, namespace)
	if err != nil {
		return EffectivePermissionsSummary{}, err
	}
	return EffectivePermissionsSummary{
		Namespace:            full.Namespace,
		ResourceRuleCount:    len(full.ResourceRules),
		NonResourceRuleCount: len(full.NonResourceRules),
		Incomplete:           full.Incomplete,
		EvaluationError:      full.EvaluationError,
	}, nil
}

type AccessCheck struct {
	Namespace       string `json:"namespace"`
	Verb            string `json:"verb"`
	Resource        string `json:"resource"`
	APIGroup        string `json:"api_group,omitempty"`
	Subresource     string `json:"subresource,omitempty"`
	Name            string `json:"name,omitempty"`
	Allowed         bool   `json:"allowed"`
	Denied          bool   `json:"denied"`
	Reason          string `json:"reason,omitempty"`
	EvaluationError string `json:"evaluation_error,omitempty"`
}

func (c *Client) CanI(ctx context.Context, namespace, verb, resource, apiGroup, subresource, name string) (AccessCheck, error) {
	review, err := c.Clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   namespace,
				Verb:        verb,
				Group:       apiGroup,
				Resource:    resource,
				Subresource: subresource,
				Name:        name,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return AccessCheck{}, err
	}

	return AccessCheck{
		Namespace:       namespace,
		Verb:            verb,
		Resource:        resource,
		APIGroup:        apiGroup,
		Subresource:     subresource,
		Name:            name,
		Allowed:         review.Status.Allowed,
		Denied:          review.Status.Denied,
		Reason:          review.Status.Reason,
		EvaluationError: review.Status.EvaluationError,
	}, nil
}

// RoleInfo represents a normalized RBAC role
type RoleInfo struct {
	Name            string              `json:"name"`
	Namespace       string              `json:"namespace"`
	Labels          map[string]string   `json:"labels,omitempty"`
	Rules           []rbacv1.PolicyRule `json:"rules"`
	ResourceVersion string              `json:"resource_version"`
	Created         metav1.Time         `json:"created"`
}

// ListRoles returns normalized role information for a namespace
func (c *Client) ListRoles(ctx context.Context, namespace string) ([]RoleInfo, error) {
	roles, err := c.Clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]RoleInfo, 0, len(roles.Items))
	for _, r := range roles.Items {
		result = append(result, RoleInfo{
			Name:            r.Name,
			Namespace:       r.Namespace,
			Labels:          r.Labels,
			Rules:           r.Rules,
			ResourceVersion: r.ResourceVersion,
			Created:         r.CreationTimestamp,
		})
	}
	return result, nil
}

// ClusterRoleInfo represents a normalized cluster role
type ClusterRoleInfo struct {
	Name            string              `json:"name"`
	Labels          map[string]string   `json:"labels,omitempty"`
	Rules           []rbacv1.PolicyRule `json:"rules"`
	ResourceVersion string              `json:"resource_version"`
	Created         metav1.Time         `json:"created"`
}

// ListClusterRoles returns normalized cluster role information
func (c *Client) ListClusterRoles(ctx context.Context) ([]ClusterRoleInfo, error) {
	roles, err := c.Clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ClusterRoleInfo, 0, len(roles.Items))
	for _, r := range roles.Items {
		result = append(result, ClusterRoleInfo{
			Name:            r.Name,
			Labels:          r.Labels,
			Rules:           r.Rules,
			ResourceVersion: r.ResourceVersion,
			Created:         r.CreationTimestamp,
		})
	}
	return result, nil
}

// RoleBindingInfo represents a normalized role binding
type RoleBindingInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels,omitempty"`
	Subjects        []rbacv1.Subject  `json:"subjects"`
	RoleRef         rbacv1.RoleRef    `json:"role_ref"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ListRoleBindings returns normalized role binding information for a namespace
func (c *Client) ListRoleBindings(ctx context.Context, namespace string) ([]RoleBindingInfo, error) {
	bindings, err := c.Clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]RoleBindingInfo, 0, len(bindings.Items))
	for _, b := range bindings.Items {
		result = append(result, RoleBindingInfo{
			Name:            b.Name,
			Namespace:       b.Namespace,
			Labels:          b.Labels,
			Subjects:        b.Subjects,
			RoleRef:         b.RoleRef,
			ResourceVersion: b.ResourceVersion,
			Created:         b.CreationTimestamp,
		})
	}
	return result, nil
}

// ClusterRoleBindingInfo represents a normalized cluster role binding
type ClusterRoleBindingInfo struct {
	Name            string            `json:"name"`
	Labels          map[string]string `json:"labels,omitempty"`
	Subjects        []rbacv1.Subject  `json:"subjects"`
	RoleRef         rbacv1.RoleRef    `json:"role_ref"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ListClusterRoleBindings returns normalized cluster role binding information
func (c *Client) ListClusterRoleBindings(ctx context.Context) ([]ClusterRoleBindingInfo, error) {
	bindings, err := c.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]ClusterRoleBindingInfo, 0, len(bindings.Items))
	for _, b := range bindings.Items {
		result = append(result, ClusterRoleBindingInfo{
			Name:            b.Name,
			Labels:          b.Labels,
			Subjects:        b.Subjects,
			RoleRef:         b.RoleRef,
			ResourceVersion: b.ResourceVersion,
			Created:         b.CreationTimestamp,
		})
	}
	return result, nil
}

// TokenSecretMetadata represents token secret metadata without the actual token bytes.
// This is the safe output of the Token Projector: it exposes annotation-based token
// metadata (expiry, audience, UID) but never emits the secret Data field.
type TokenSecretMetadata struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ServiceAccountDetail represents full ServiceAccount metadata including
// automount settings and associated token secret metadata. This is the
// Token Projector's primary output type.
type ServiceAccountDetail struct {
	Name            string                `json:"name"`
	Namespace       string                `json:"namespace"`
	Labels          map[string]string     `json:"labels,omitempty"`
	Annotations     map[string]string     `json:"annotations,omitempty"`
	AutomountToken  bool                  `json:"automount_token"`
	TokenSecrets    []TokenSecretMetadata `json:"token_secrets,omitempty"`
	ResourceVersion string                `json:"resource_version"`
	Created         metav1.Time           `json:"created"`
}

// SecretInfo represents normalized secret information (metadata only, not values)
type SecretInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Type            string            `json:"type"`
	Labels          map[string]string `json:"labels,omitempty"`
	Keys            []string          `json:"keys"`
	ResourceVersion string            `json:"resource_version"`
	Created         metav1.Time       `json:"created"`
}

// ListSecrets returns normalized secret information for a namespace (metadata only)
func (c *Client) ListSecrets(ctx context.Context, namespace string) ([]SecretInfo, error) {
	secrets, err := c.Clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]SecretInfo, 0, len(secrets.Items))
	for _, s := range secrets.Items {
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		result = append(result, SecretInfo{
			Name:            s.Name,
			Namespace:       s.Namespace,
			Type:            string(s.Type),
			Labels:          s.Labels,
			Keys:            keys,
			ResourceVersion: s.ResourceVersion,
			Created:         s.CreationTimestamp,
		})
	}
	return result, nil
}

type SecretKeySummary struct {
	Name      string `json:"name"`
	ByteCount int    `json:"byte_count"`
}

type SecretReadSummary struct {
	Name            string             `json:"name"`
	Namespace       string             `json:"namespace"`
	Type            string             `json:"type"`
	Labels          map[string]string  `json:"labels,omitempty"`
	Keys            []SecretKeySummary `json:"keys"`
	ResourceVersion string             `json:"resource_version"`
	Created         metav1.Time        `json:"created"`
}

func (c *Client) ReadSecretSummary(ctx context.Context, namespace, name string) (SecretReadSummary, error) {
	secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return SecretReadSummary{}, err
	}

	keys := make([]SecretKeySummary, 0, len(secret.Data))
	for key, value := range secret.Data {
		keys = append(keys, SecretKeySummary{Name: key, ByteCount: len(value)})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})

	return SecretReadSummary{
		Name:            secret.Name,
		Namespace:       secret.Namespace,
		Type:            string(secret.Type),
		Labels:          secret.Labels,
		Keys:            keys,
		ResourceVersion: secret.ResourceVersion,
		Created:         secret.CreationTimestamp,
	}, nil
}

func IsSecretAccessForbidden(err error) bool {
	return apierrors.IsForbidden(err)
}

func IsSecretMissing(err error) bool {
	return apierrors.IsNotFound(err)
}

// NetworkPolicyInfo represents normalized network policy information
type NetworkPolicyInfo struct {
	Name            string                                  `json:"name"`
	Namespace       string                                  `json:"namespace"`
	Labels          map[string]string                       `json:"labels,omitempty"`
	PodSelector     metav1.LabelSelector                    `json:"pod_selector"`
	Ingress         []networkingv1.NetworkPolicyIngressRule `json:"ingress,omitempty"`
	Egress          []networkingv1.NetworkPolicyEgressRule  `json:"egress,omitempty"`
	ResourceVersion string                                  `json:"resource_version"`
	Created         metav1.Time                             `json:"created"`
}

// ListNetworkPolicies returns normalized network policy information for a namespace
func (c *Client) ListNetworkPolicies(ctx context.Context, namespace string) ([]NetworkPolicyInfo, error) {
	policies, err := c.Clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]NetworkPolicyInfo, 0, len(policies.Items))
	for _, p := range policies.Items {
		result = append(result, NetworkPolicyInfo{
			Name:            p.Name,
			Namespace:       p.Namespace,
			Labels:          p.Labels,
			PodSelector:     p.Spec.PodSelector,
			Ingress:         p.Spec.Ingress,
			Egress:          p.Spec.Egress,
			ResourceVersion: p.ResourceVersion,
			Created:         p.CreationTimestamp,
		})
	}
	return result, nil
}

func buildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		if inCluster, err := rest.InClusterConfig(); err == nil {
			return inCluster, nil
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		if kubeconfig == "" {
			return nil, fmt.Errorf("load kubeconfig: %w (set --kubeconfig or run inside a cluster)", err)
		}
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	return restCfg, nil
}

// MountedTokenMetadata represents safe claim metadata extracted from a mounted
// ServiceAccount JWT token file. Only scalar claim fields are extracted; raw token
// bytes, base64 segments, and hashes are never included.
//
// IMPORTANT: These are claim assertions parsed from the token, not cluster-confirmed
// facts. The token is parsed without signature verification — the claims reflect what
// the token issuer asserted, not what the cluster has verified. For cluster-confirmed
// identity scope, use GetEffectivePermissions or GetEffectivePermissionsSummary.
type MountedTokenMetadata struct {
	// Issuer is the URI of the token issuer (iss claim).
	Issuer string `json:"issuer,omitempty"`
	// Subject is the ServiceAccount identifier (sub claim).
	Subject string `json:"subject,omitempty"`
	// Audience is the intended recipient(s) (aud claim), joined by comma if multiple.
	Audience string `json:"audience,omitempty"`
	// Namespace is the kubernetes.io/serviceaccount/namespace claim value.
	Namespace string `json:"namespace,omitempty"`
	// ServiceAccountName is the kubernetes.io/serviceaccount/name claim value.
	ServiceAccountName string `json:"service_account_name,omitempty"`
	// ServiceAccountUID is the kubernetes.io/serviceaccount/uid claim value.
	ServiceAccountUID string `json:"service_account_uid,omitempty"`
	// IssuedAt is the Unix timestamp when the token was issued (iat claim).
	IssuedAt int64 `json:"issued_at,omitempty"`
	// Expiry is the Unix timestamp when the token expires (exp claim).
	Expiry int64 `json:"expiry,omitempty"`
}

// ReadMountedTokenMetadata reads and parses a mounted ServiceAccount token file,
// extracting safe claim metadata without exposing the raw token. The token is
// parsed without signature verification — the returned metadata reflects token
// claims only, not cluster-confirmed identity facts.
//
// If the token file cannot be read (e.g., not running in-cluster), returns
// zero value and false. If the file is read but the token cannot be parsed,
// returns zero value and an error.
func ReadMountedTokenMetadata(tokenPath string) (MountedTokenMetadata, bool, error) {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return MountedTokenMetadata{}, false, nil
	}

	meta, err := parseMountedTokenJWT(data)
	if err != nil {
		return MountedTokenMetadata{}, false, err
	}
	return meta, true, nil
}

// parseMountedTokenJWT extracts safe claim metadata from raw JWT token bytes.
// The token is parsed without signature verification — only the payload segment
// is decoded and safe scalar fields are extracted.
func parseMountedTokenJWT(tokenBytes []byte) (MountedTokenMetadata, error) {
	// JWT format: header.payload.signature (all base64url-encoded).
	// We only decode the payload (segment 1) and extract safe fields.
	segments := 0
	payloadStart := -1
	segEnd := 0

	for j := 0; j < len(tokenBytes); j++ {
		if tokenBytes[j] == '.' {
			segments++
			if segments == 1 {
				payloadStart = j + 1
			}
			if segments >= 2 {
				// j is the dot; payload ends just before it.
				segEnd = j
				break
			}
		}
	}

	if payloadStart < 0 || segments < 1 {
		return MountedTokenMetadata{}, fmt.Errorf("invalid JWT format: no payload segment")
	}

	// segEnd == 0 means no second dot was found (loop ran to end).
	// In that case, payload spans from payloadStart to end of input.
	payloadEnd := segEnd
	if segments < 2 {
		payloadEnd = len(tokenBytes)
	}
	payloadEncoded := string(tokenBytes[payloadStart:payloadEnd])

	// Base64url decode. Go's RawURLEncoding handles base64url without padding.
	// It also correctly handles the base64url character set (- for +, _ for /).
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return MountedTokenMetadata{}, fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return MountedTokenMetadata{}, fmt.Errorf("parse JWT payload JSON: %w", err)
	}

	meta := MountedTokenMetadata{}

	if v, ok := claims["iss"].(string); ok {
		meta.Issuer = v
	}
	if v, ok := claims["sub"].(string); ok {
		meta.Subject = v
	}
	switch v := claims["aud"]; {
	case v == nil:
		// No audience
	case isinstanceString(v):
		meta.Audience = v.(string)
	case isinstanceStringSlice(v):
		meta.Audience = strings.Join(toStringSlice(v), ",")
	}
	if v, ok := claims["kubernetes.io/serviceaccount/namespace"].(string); ok {
		meta.Namespace = v
	}
	if v, ok := claims["kubernetes.io/serviceaccount/name"].(string); ok {
		meta.ServiceAccountName = v
	}
	if v, ok := claims["kubernetes.io/serviceaccount/uid"].(string); ok {
		meta.ServiceAccountUID = v
	}
	if kubeClaims, ok := claims["kubernetes.io"].(map[string]any); ok {
		if meta.Namespace == "" {
			if v, ok := kubeClaims["namespace"].(string); ok {
				meta.Namespace = v
			}
		}
		if saClaims, ok := kubeClaims["serviceaccount"].(map[string]any); ok {
			if meta.ServiceAccountName == "" {
				if v, ok := saClaims["name"].(string); ok {
					meta.ServiceAccountName = v
				}
			}
			if meta.ServiceAccountUID == "" {
				if v, ok := saClaims["uid"].(string); ok {
					meta.ServiceAccountUID = v
				}
			}
		}
	}
	if meta.Namespace == "" || meta.ServiceAccountName == "" {
		const prefix = "system:serviceaccount:"
		if strings.HasPrefix(meta.Subject, prefix) {
			parts := strings.SplitN(strings.TrimPrefix(meta.Subject, prefix), ":", 2)
			if len(parts) == 2 {
				if meta.Namespace == "" {
					meta.Namespace = parts[0]
				}
				if meta.ServiceAccountName == "" {
					meta.ServiceAccountName = parts[1]
				}
			}
		}
	}
	if v, ok := claims["iat"].(float64); ok {
		meta.IssuedAt = int64(v)
	}
	if v, ok := claims["exp"].(float64); ok {
		meta.Expiry = int64(v)
	}

	return meta, nil
}

func isinstanceString(v any) bool {
	_, ok := v.(string)
	return ok
}

func isinstanceStringSlice(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return false
	}
	for _, elem := range arr {
		if _, ok := elem.(string); !ok {
			return false
		}
	}
	return true
}

func toStringSlice(v any) []string {
	arr := v.([]any)
	result := make([]string, 0, len(arr))
	for _, elem := range arr {
		result = append(result, elem.(string))
	}
	return result
}
