# Threat Model (Draft)

## Purpose

Define the assumed-breach model for Chain Reaction so runtime validation stays realistic, measurable, and safe.

## Adversary Assumptions

- **Starting position:** attacker controls one application Pod in-cluster.
- **Identity:** attacker can use only the compromised Pod's ServiceAccount token.
- **Network position:** attacker can reach only destinations available from that Pod's network context.
- **No out-of-band access:** no node shell, cloud control-plane credentials, or host-level access.

## In Scope

Based on Kubernetes Goat scenarios and RBAC escalation research:

### RBAC-Derived Privilege Opportunities
- ServiceAccount token mounted in Pod (automatic by default)
- Role/ClusterRole bindings with overly permissive rules
- Wildcard permissions (`*` on verbs or resources)
- ClusterRoleBindings granting cluster-admin or equivalent
- Impersonation permissions (`impersonate` verb)
- Permission to create/update RoleBindings or ClusterRoleBindings

### Secret Access Paths
- Reading Secrets via RBAC-permitted API calls
- Environment variables containing sensitive data
- ConfigMaps with credentials
- ServiceAccount token exfiltration patterns

### Kubernetes Object Relationship Pivots
- Pod → ServiceAccount → RoleBinding → ClusterRoleBinding chain
- ServiceAccount → Secrets (token, imagePullSecrets)
- Pod → Service → Endpoints network relationships
- Namespace bypass via default flat networking

### Network Reachability
- Pod-to-Pod communication (default: all allowed)
- NodePort/LoadBalancer service exposure
- DNS-based service discovery
- NetworkPolicy enforcement gaps

### Relevant Kubernetes Goat Scenarios
From Kubernetes Goat (20+ scenarios mapped):
- RBAC least privilege misconfiguration
- Sensitive keys in codebases
- Service account token exploitation
- Namespace bypass
- NodePort exposed services
- Container escape (out of scope for validation, but related context)
- SSRF to cluster access

## Out of Scope

- Kernel/container escape exploitation development
- Destructive disruption actions in production-like environments
- Cloud provider account takeover outside Kubernetes identity context
- Host node compromise beyond Pod context

## Validation Semantics

A chain step is **validated** only when all conditions hold:

1. A concrete runtime action is executed from the in-cluster agent context.
2. The action result is captured as evidence (API output and/or probe result).
3. The result is sufficient to support the step claim.

Otherwise the step remains **theoretical** or **failed**, with explicit failure reason.

## Failure Taxonomy

Based on RBAC escalation research and Kubernetes Goat findings:

- **RBAC denial** - permission check failed, verb/resource not allowed
- **Missing prerequisite object** - RoleBinding, Secret, or target resource absent
- **Unreachable network target** - DNS resolution failure, TCP connect timeout
- **Guardrail stop/deny** - action outside allow-list or rate limit hit
- **Tool/runtime execution error** - API error, timeout, unexpected response
- **Authentication failure** - token invalid/expired (shouldn't happen in assumed-breach)

## Evaluation Mapping (Kubernetes Goat)

- Use Kubernetes Goat as baseline evaluation environment.
- Map each exercised scenario to: attempted chain, validated steps, failed steps, and evidence references.
- Target metric: runtime-validated chains for at least 80% of selected scenarios.

### Scenario Categories for Coverage
1. **RBAC Misconfig** - privilege escalation via binding
2. **Secret Access** - reading sensitive data
3. **Network Pivot** - lateral movement via services
4. **Namespace Bypass** - cross-namespace access
5. **Service Account Abuse** - token theft/impersonation

## Relationship to Existing Tools

### KubeHound (DataDog)
- Builds attack path graphs from cluster data
- Stores paths in Neo4j/JanusGraph for querying
- Identifies 25+ attack paths (container escape, lateral movement)
- **Difference:** KubeHound produces theoretical paths; Chain Reaction validates runtime

### Similar Projects Differentiation
From `docs/draft/similar-projects.md`:
- In-cluster assumed-breach execution (not just configuration scanning)
- Adaptive chaining with LLM-guided planning
- Evidence-backed validation (not just plausible paths)
- Phase-labeled graph output with explicit validated/theoretical status

## Success Criteria

- Threat model remains consistent with assumed-breach execution.
- Every reported validated edge has direct evidence linkage.
- Report differentiates validated vs theoretical attack paths.
- Coverage target: ≥80% of Kubernetes Goat scenarios have validated chains.
