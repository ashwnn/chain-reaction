package validation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

const (
	defaultProbeTimeoutSeconds = 5
	defaultProbeRetries        = 1
	maxProbeRetries            = 5

	probeTypeTCP  = "tcp"
	probeTypeHTTP = "http"
	probeTypeDNS  = "dns"
)

type ProbeNetworkTool struct {
	resolver   *net.Resolver
	dialer     *net.Dialer
	httpClient *http.Client
	enforcer   *guardrails.Enforcer
	collector  *evidence.Collector
}

func NewProbeNetworkTool(enforcer *guardrails.Enforcer, collector *evidence.Collector) *ProbeNetworkTool {
	return &ProbeNetworkTool{
		resolver:  net.DefaultResolver,
		dialer:    &net.Dialer{},
		enforcer:  enforcer,
		collector: collector,
		httpClient: &http.Client{
			Timeout: defaultProbeTimeoutSeconds * time.Second,
			// Do not follow redirects — capture the raw response status.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (t *ProbeNetworkTool) Name() string {
	return "validation.probe_network"
}

func (t *ProbeNetworkTool) Description() string {
	return "Tests network reachability via TCP connect, HTTP GET, or DNS resolution probe"
}

func (t *ProbeNetworkTool) ParameterSchema() tools.Schema {
	return tools.Schema{
		Type:        "object",
		Description: "Probe-specific inputs for bounded TCP, HTTP, or DNS validation. Conditional requirements are described per field rather than enforced with JSON Schema conditionals.",
		Properties: map[string]tools.Schema{
			"probe": {
				Type:        "string",
				Description: "Probe type to run. Use tcp for host:port reachability, http for a single GET request, or dns for hostname resolution.",
				Enum:        []string{probeTypeTCP, probeTypeHTTP, probeTypeDNS},
				Default:     probeTypeTCP,
			},
			"target": {
				Type:        "string",
				Description: "Hostname or IP address to probe. Required for tcp and dns probes; ignored for http probes.",
			},
			"url": {
				Type:        "string",
				Description: "HTTP or HTTPS URL to request. Required for http probes; ignored for tcp and dns probes.",
			},
			"port": {
				Type:        "integer",
				Description: "TCP destination port in the range 1-65535. Required for tcp probes; ignored for http and dns probes.",
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: "Per-probe timeout in seconds. Mixed numeric and numeric-string inputs are accepted at runtime; values below 1 fall back to the default.",
				Default:     defaultProbeTimeoutSeconds,
			},
			"retries": {
				Type:        "integer",
				Description: "Number of TCP connection attempts. Mixed numeric and numeric-string inputs are accepted at runtime; values below 1 fall back to the default and values above the runtime cap are clamped. Ignored for http and dns probes.",
				Default:     defaultProbeRetries,
			},
		},
	}
}

func (t *ProbeNetworkTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	probeType := probeTypeTCP
	target := ""
	url := ""
	port := 0
	timeoutSeconds := defaultProbeTimeoutSeconds
	retries := defaultProbeRetries

	if input != nil {
		if v, ok := input["probe"].(string); ok && strings.TrimSpace(v) != "" {
			probeType = strings.TrimSpace(strings.ToLower(v))
		}
		if v, ok := input["target"].(string); ok {
			target = strings.TrimSpace(v)
		}
		if v, ok := input["url"].(string); ok {
			url = strings.TrimSpace(v)
		}

		parsedPort, err := intFromInput(input["port"])
		if err != nil {
			return nil, fmt.Errorf("parse port: %w", err)
		}
		if parsedPort != 0 {
			port = parsedPort
		}

		parsedTimeout, err := intFromInput(input["timeout_seconds"])
		if err != nil {
			return nil, fmt.Errorf("parse timeout_seconds: %w", err)
		}
		if parsedTimeout > 0 {
			timeoutSeconds = parsedTimeout
		}

		parsedRetries, err := intFromInput(input["retries"])
		if err != nil {
			return nil, fmt.Errorf("parse retries: %w", err)
		}
		if parsedRetries > 0 {
			retries = parsedRetries
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if retries < 1 {
		retries = defaultProbeRetries
	}
	if retries > maxProbeRetries {
		retries = maxProbeRetries
	}
	if timeoutSeconds < 1 {
		timeoutSeconds = defaultProbeTimeoutSeconds
	}

	switch probeType {
	case probeTypeHTTP:
		if url == "" {
			return nil, errors.New("url is required for http probe")
		}
		return t.runHTTPProbe(ctx, url, timeoutSeconds)
	case probeTypeDNS:
		if target == "" {
			return nil, errors.New("target is required for dns probe")
		}
		return t.runDNSProbe(ctx, target, timeoutSeconds)
	case probeTypeTCP:
		return t.runTCPProbe(ctx, target, port, timeoutSeconds, retries)
	default:
		return nil, fmt.Errorf("unsupported probe type %q; use %q, %q, or %q", probeType, probeTypeTCP, probeTypeHTTP, probeTypeDNS)
	}
}

// runTCPProbe performs TCP connection attempts and records the final result.
func (t *ProbeNetworkTool) runTCPProbe(ctx context.Context, target string, port, timeoutSeconds, retries int) (map[string]any, error) {
	if target == "" {
		return nil, errors.New("target is required")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}

	if t.enforcer != nil {
		if ns := extractNamespaceFromTarget(target); ns != "" {
			if err := t.enforcer.CheckNamespace(ns); err != nil {
				return map[string]any{
					"probe":          probeTypeTCP,
					"target":         target,
					"port":           port,
					"status":         string(StepFailed),
					"failure_reason": string(FailureGuardrailBlocked),
					"error":          err.Error(),
				}, nil
			}
		} else if ns == "" && t.enforcer.HasAllowList() {
			// If we can't determine the namespace but an allow-list is present,
			// block the request as it might be trying to reach an external
			// or cluster-level service not in the allow-list.
			return map[string]any{
				"probe":          probeTypeTCP,
				"target":         target,
				"port":           port,
				"status":         string(StepFailed),
				"failure_reason": string(FailureGuardrailBlocked),
				"error":          fmt.Sprintf("target %q is outside allowed scopes", target),
			}, nil
		}
	}

	result := map[string]any{
		"probe":             probeTypeTCP,
		"target":            target,
		"port":              port,
		"timeout_seconds":   timeoutSeconds,
		"retries":           retries,
		"reachable":         false,
		"latency_ms":        nil,
		"resolved_ips":      []string{},
		"resolved_ip_count": 0,
		"attempts":          make([]map[string]any, 0, retries),
		"result":            string(StepFailed), // default; updated below on success
		"failure_reason":    string(FailureNetworkUnreachable),
	}

	for attempt := 1; attempt <= retries; attempt++ {
		attemptResult, err := t.probeAttempt(ctx, target, port, timeoutSeconds, attempt)
		result["attempts"] = append(result["attempts"].([]map[string]any), attemptResult)

		resolvedIPs := attemptResult["resolved_ips"].([]string)
		if len(resolvedIPs) > 0 && result["resolved_ip_count"].(int) == 0 {
			result["resolved_ips"] = resolvedIPs
			result["resolved_ip_count"] = len(resolvedIPs)
		}

		if err == nil {
			result["reachable"] = true
			result["latency_ms"] = attemptResult["latency_ms"]
			result["result"] = string(StepValidated)
			delete(result, "failure_reason")
			t.recordEvidence(result)
			return result, nil
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() != nil {
				t.recordEvidence(result)
				return result, err
			}
		}
	}

	t.recordEvidence(result)
	return result, nil
}

// runHTTPProbe performs a single HTTP GET to url. It does not follow redirects.
// The timeout is applied at the request level via the context.
func (t *ProbeNetworkTool) runHTTPProbe(ctx context.Context, url string, timeoutSeconds int) (map[string]any, error) {
	if t.enforcer != nil {
		if ns := extractNamespaceFromURL(url); ns != "" {
			if err := t.enforcer.CheckNamespace(ns); err != nil {
				return map[string]any{
					"probe":          probeTypeHTTP,
					"url":            url,
					"status":         string(StepFailed),
					"failure_reason": string(FailureGuardrailBlocked),
					"error":          err.Error(),
				}, nil
			}
		} else if ns == "" && t.enforcer.HasAllowList() {
			return map[string]any{
				"probe":          probeTypeHTTP,
				"url":            url,
				"status":         string(StepFailed),
				"failure_reason": string(FailureGuardrailBlocked),
				"error":          fmt.Sprintf("URL %q is outside allowed scopes", url),
			}, nil
		}
	}

	result := map[string]any{
		"probe":           probeTypeHTTP,
		"url":             url,
		"timeout_seconds": timeoutSeconds,
		"reachable":       false,
		"status_code":     nil,
		"latency_ms":      nil,
		"error":           "",
		"result":          string(StepFailed), // default; updated below on success
		"failure_reason":  string(FailureNetworkUnreachable),
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}

	if t.enforcer != nil {
		if err := t.enforcer.Acquire(reqCtx); err != nil {
			result["error"] = fmt.Sprintf("rate limit wait failed: %v", err)
			t.recordEvidence(result)
			return result, err
		}
	}

	started := time.Now()
	resp, err := t.httpClient.Do(req)
	latency := float64(time.Since(started)) / float64(time.Millisecond)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() != nil {
				t.recordEvidence(result)
				return result, err
			}
		}
		result["error"] = err.Error()
		t.recordEvidence(result)
		return result, nil
	}
	_ = resp.Body.Close()

	result["reachable"] = true
	result["status_code"] = resp.StatusCode
	result["latency_ms"] = latency
	result["result"] = string(StepValidated)
	delete(result, "failure_reason")
	t.recordEvidence(result)
	return result, nil
}

func (t *ProbeNetworkTool) probeAttempt(ctx context.Context, target string, port, timeoutSeconds, attempt int) (map[string]any, error) {
	attemptResult := map[string]any{
		"attempt":      attempt,
		"success":      false,
		"latency_ms":   nil,
		"error":        "",
		"address":      "",
		"resolved_ips": []string{},
	}

	attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	ipAddrs, err := t.resolver.LookupIPAddr(attemptCtx, target)
	if err != nil {
		attemptResult["error"] = err.Error()
		return attemptResult, err
	}

	resolvedIPs := make([]string, 0, len(ipAddrs))
	for _, ipAddr := range ipAddrs {
		resolvedIPs = append(resolvedIPs, ipAddr.IP.String())
	}
	attemptResult["resolved_ips"] = resolvedIPs

	if len(resolvedIPs) == 0 {
		err = errors.New("dns resolution returned no IPs")
		attemptResult["error"] = err.Error()
		return attemptResult, err
	}

	var dialErr error
	for _, resolvedIP := range resolvedIPs {
		if err := attemptCtx.Err(); err != nil {
			attemptResult["error"] = err.Error()
			return attemptResult, err
		}

		address := net.JoinHostPort(resolvedIP, strconv.Itoa(port))
		attemptResult["address"] = address

		if t.enforcer != nil {
			if err := t.enforcer.Acquire(attemptCtx); err != nil {
				attemptResult["error"] = fmt.Sprintf("rate limit wait failed: %v", err)
				return attemptResult, err
			}
		}

		startedAt := time.Now()
		conn, err := t.dialer.DialContext(attemptCtx, "tcp", address)
		if err != nil {
			dialErr = err
			attemptResult["error"] = err.Error()
			continue
		}

		latency := time.Since(startedAt)
		_ = conn.Close()

		attemptResult["success"] = true
		attemptResult["error"] = ""
		attemptResult["latency_ms"] = float64(latency) / float64(time.Millisecond)
		return attemptResult, nil
	}

	if dialErr == nil {
		dialErr = errors.New("tcp probe failed")
	}

	return attemptResult, dialErr
}

// runDNSProbe resolves target using net.DefaultResolver.LookupHost and returns
// the resolved addresses. Returns an error if resolution fails (e.g. NXDOMAIN).
func (t *ProbeNetworkTool) runDNSProbe(ctx context.Context, target string, timeoutSeconds int) (map[string]any, error) {
	if t.enforcer != nil {
		if ns := extractNamespaceFromTarget(target); ns != "" {
			if err := t.enforcer.CheckNamespace(ns); err != nil {
				return map[string]any{
					"probe":          probeTypeDNS,
					"target":         target,
					"status":         string(StepFailed),
					"failure_reason": string(FailureGuardrailBlocked),
					"error":          err.Error(),
				}, nil
			}
		} else if ns == "" && t.enforcer.HasAllowList() {
			return map[string]any{
				"probe":          probeTypeDNS,
				"target":         target,
				"status":         string(StepFailed),
				"failure_reason": string(FailureGuardrailBlocked),
				"error":          fmt.Sprintf("target %q is outside allowed scopes", target),
			}, nil
		}
	}

	result := map[string]any{
		"probe":           probeTypeDNS,
		"target":          target,
		"timeout_seconds": timeoutSeconds,
		"resolved":        false,
		"addresses":       []string{},
		"address_count":   0,
		"error":           "",
		"result":          string(StepFailed), // default; updated below on success
		"failure_reason":  string(FailureNetworkUnreachable),
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if t.enforcer != nil {
		if err := t.enforcer.Acquire(reqCtx); err != nil {
			result["error"] = fmt.Sprintf("rate limit wait failed: %v", err)
			t.recordEvidence(result)
			return result, err
		}
	}

	addrs, err := t.resolver.LookupHost(reqCtx, target)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() != nil {
				t.recordEvidence(result)
				return result, err
			}
		}
		result["error"] = err.Error()
		t.recordEvidence(result)
		return result, nil
	}

	result["resolved"] = true
	result["addresses"] = addrs
	result["address_count"] = len(addrs)
	result["result"] = string(StepValidated)
	delete(result, "failure_reason")
	t.recordEvidence(result)
	return result, nil
}

func (t *ProbeNetworkTool) recordEvidence(result map[string]any) {
	if t.collector != nil {
		// Log failures implicitly via err != nil logic upstream, wait, no, just record
		// whatever 'result' map has been built.
		_ = t.collector.Record("network_probe", result)
	}
}

func extractNamespaceFromTarget(target string) string {
	host := target
	if colon := strings.LastIndex(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 4 && parts[len(parts)-3] == "svc" && parts[len(parts)-2] == "cluster" && parts[len(parts)-1] == "local" {
		return parts[len(parts)-4]
	}
	return ""
}

func extractNamespaceFromURL(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		return extractNamespaceFromTarget(rawURL)
	}
	parts := strings.SplitN(rawURL, "://", 2)
	if len(parts) < 2 {
		return ""
	}
	host := parts[1]
	if slash := strings.Index(host, "/"); slash >= 0 {
		host = host[:slash]
	}
	return extractNamespaceFromTarget(host)
}

func intFromInput(value any) (int, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q", v)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported integer type %T", value)
	}
}
