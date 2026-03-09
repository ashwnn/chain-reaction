# Guardrails Specification (Draft)

## Objective

Constrain Chain Reaction to safe, bounded, evidence-producing behavior suitable for lab evaluation in Kubernetes Goat environments.

## Core Principles

Based on Kubernetes security best practices and audit logging research:

- Prefer read-only actions by default.
- Require explicit allow-list checks before any operation.
- Enforce bounded time, rate, and retry behavior.
- Record enough evidence to audit every action decision.
- Fail safely: deny by default, require explicit authorization.

## A. Allowed Actions

### Always Allowed (Read-Oriented)

API operations that are safe for evidence collection:

- `get`, `list`, `watch` on non-sensitive resources (Pods, Services, ConfigMaps, Namespaces)
- `get`, `list` on RBAC objects (Roles, RoleBindings, ClusterRoles, ClusterRoleBindings) for permission analysis
- Non-invasive reachability probes:
  - DNS resolution (A/AAAA records only, no zone transfer)
  - TCP connect checks with strict timeouts (≤5 seconds)
  - HTTP HEAD/GET probes with bounded response size

### Conditionally Allowed (Requires Rationale)

Write-like validation actions only when:

1. Explicitly required by scenario validation
2. Within configured policy scope
3. Pre-approved action type in guardrails config
4. Includes documented rationale, scope, and expected evidence

Examples:
- Creating ephemeral test resources (with auto-cleanup)
- Reading Secrets (only when RBAC allows and explicitly required for chain validation)

### Denied by Default

- Any destructive operation (delete, patch with destructive changes)
- Privilege escalation attempts (create privileged Pod, modify RBAC to grant admin)
- Container escape techniques (hostPath mounts, hostNetwork, privileged containers)
- Unbounded recursive exploration or high-fanout probing
- Operations outside configured namespace allow-list
- Actions exceeding configured QPS/burst limits

## B. Stop Conditions

Execution stops immediately when ANY condition is met:

1. **Time budget exceeded** - Total runtime exceeds configured limit (default: 2 minutes)
2. **Step count exceeded** - Maximum number of tool executions reached (default: 50)
3. **Loop detected** - Repeated pattern of same tool with same inputs
4. **Error threshold** - Consecutive errors exceed limit (default: 5)
5. **Guardrail deny** - Action explicitly denied by allow-list
6. **RBAC escalation blocked** - Attempt to escalate privileges detected
7. **Resource exhaustion** - Memory/CPU limits approaching

## C. Rate and Resource Limits

### API Rate Limiting

Based on Kubernetes API server best practices:

- **QPS limit:** Configurable, default 10 requests/second
- **Burst limit:** Configurable, default 20 requests
- **Per-resource cooldown:** 1 second between identical resource reads
- **Global API budget:** Maximum 600 API calls per run (configurable)

### Network Probe Limits

- **DNS timeout:** 3 seconds per query
- **TCP connect timeout:** 5 seconds per attempt
- **Max retries:** 2 attempts per target
- **Concurrent probes:** Maximum 5 parallel network operations
- **Probe cooldown:** 500ms between probes to same target

### Resource Constraints

- **Memory:** Maximum 512MB per run
- **CPU:** Respect container limits (do not spin busy loops)
- **Disk:** Evidence files limited to 100MB total

## D. Evidence Requirements Per Action

Following Kubernetes audit logging best practices, each action must record:

### Required Fields

1. **Timestamp** - UTC timestamp with millisecond precision
2. **Tool name** - Fully qualified tool identifier (e.g., `discovery.list_namespaces`)
3. **Input parameters** - Normalized, sanitized input map
4. **API method** - HTTP method and API endpoint (e.g., `GET /api/v1/namespaces`)
5. **Response summary** - Status code, resource count, error message (if any)
6. **Result classification** - `validated` | `theoretical` | `failed`
7. **Failure reason** - From taxonomy (if result is `failed`)
8. **Evidence reference** - Path to full response artifact in evidence bundle

### Sensitive Data Handling

- Redact Secret values (show only metadata: name, namespace, type)
- Mask ServiceAccount tokens (show only first/last 8 characters)
- Exclude cloud provider credentials entirely
- Hash any collected tokens for correlation without exposure

## E. Error Handling Policy

### RBAC Denial

- **Action:** Classify step as `failed`
- **Evidence:** Capture denial response with verb/resource attempted
- **Continue:** Yes, if alternative paths exist
- **Report:** Include in failure taxonomy

### Network/API Failures

- **Transient failures** (timeout, connection reset):
  - Retry up to 2 times with exponential backoff
  - If still failing: classify as `failed` with reason
- **Permanent failures** (404 Not Found, 403 Forbidden):
  - No retry, classify immediately

### Tool Execution Errors

- Capture full error context
- Preserve stack trace in debug logs (not evidence bundle)
- Classify step as `failed`
- Continue only if safe (no partial state corruption)

## F. Reporting Requirements

### Attack Graph Output

- Every edge must have `status` field: `validated` | `theoretical` | `failed`
- Include evidence reference for `validated` edges
- Include failure reason for `failed` edges
- Phase labels per node (foothold, discovery, exploitation, impact)

### Evidence Bundle Structure

```
evidence/
├── manifest.json          # Run metadata, checksums
├── evidence.jsonl         # NDJSON line-delimited events
├── responses/             # Full API responses (compressed)
│   ├── 001_list_namespaces.json
│   ├── 002_get_serviceaccount.json
│   └── ...
└── summary.json           # Run statistics, stop reason
```

### Final Report Requirements

- Stop reason (success, timeout, guardrail, error threshold)
- Run statistics (steps executed, API calls, duration)
- Coverage summary (scenarios attempted/validated/failed)
- List of denied actions (if any guardrail triggers)

## G. Alignment with Project Goals

This guardrails specification aligns with:

- **Assumed-breach execution** - Agent runs as standard Pod with real ServiceAccount
- **Evidence-backed validation** - Every action produces auditable evidence
- **Kubernetes Goat evaluation** - Safe for intentionally vulnerable environments
- **Academic rigor** - Reproducible, measurable constraints

## H. Audit and Compliance

### Evidence Integrity

- Evidence files are append-only during run
- Checksums (SHA-256) recorded in manifest
- Timestamps from monotonic clock + UTC wall clock
- Immutable after write (no modifications post-capture)

### Reviewability

- All actions reconstructible from evidence bundle
- Decision chain traceable (LLM plan → tool execution → observation)
- Guardrail decisions logged with rationale
- Stop conditions clearly documented

## References

Based on research from:
- Kubernetes SIG Security Admission Control Threat Model
- OWASP Kubernetes Security Cheat Sheet
- Kubernetes Goat scenarios (RBAC, SSRF, namespace bypass)
- Kubernetes audit logging best practices (Datadog, Palo Alto Networks)
- KubeHound attack path methodology

## I. Future Enhancements

- Integration with Kubernetes Audit Policy for native audit log correlation
- Admission Controller integration for pre-execution validation
- Integration with Falco/Tetragon for runtime detection validation
