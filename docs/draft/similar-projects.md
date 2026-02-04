# Similar Projects and Adjacent Tools

## 1. Direct competitors

### Horizon3.ai NodeZero Kubernetes pentest (commercial)

**Primary purpose:** autonomous Kubernetes pentesting that runs **inside the cluster** to find and chain exploitable weaknesses.

**Why it matches (rubric mapping):**

- **In-cluster assumed-breach (2/2):** runs from inside the cluster and supports selecting a namespace and ServiceAccount to simulate a compromised Pod.
- **Multi-step chaining (2/2):** marketed as chaining context, exploitable vulnerabilities, misconfigurations, weak controls, and “loot” into prioritized attack paths.
- **Proof/validation (2/2):** marketed as producing proof and launching real-world attacks against live clusters.
- **Adaptivity (1/2):** strongly implied by autonomous chaining, but the internal decision logic is not publicly specified.
- **Output quality (1/2):** public materials emphasize outcomes and proof, but detailed evidence artifacts suitable for academic reproducibility are not clearly documented.

**What it likely does better than this project:**

- Production-grade packaging and workflows (for example an operator/runner model and continuous execution).
- Broader cross-environment chaining beyond Kubernetes.

**What it still lacks relative to the “evidence-backed validation” goal:**

- Evidence integrity guarantees (immutable step logs, signed artifacts, deterministic replay) are not clearly documented.
- Reasoning transparency (why a hypothesis was formed or pruned) is not described in an academically reviewable way.

**Rubric score:** 9/10 (high confidence on in-cluster + chaining + proof; conservative on evidence artifacts and reasoning transparency).

---

## 2. Partial overlaps

### A) Attack graphs and exposure modeling

**Open-source Kubernetes-focused graphing:**

- **KubeHound (Datadog Security Labs):** builds an attack-path graph for Kubernetes and supports path queries.
- **IceKube (WithSecure Labs):** enumerates cluster resources and stores them in Neo4j to query complex attack paths.
- **Konstellation (Praetorian):** enumerates resources into Neo4j for relationship mapping, including RBAC-oriented queries.

**Commercial graph + attack-path analysis (cloud-wide, Kubernetes-aware):**

- **Microsoft Defender for Cloud Attack Path Analysis:** cloud security graph with attack path computation and Kubernetes-relevant identity and lateral-movement modeling.
- **Wiz Security Graph / Attack Path Analysis:** graph-based contextual attack path analysis across cloud resources.
- **Prisma Cloud attack path policies / “Infinity Graph”:** graph views and policy constructs for attack path prioritization.

**Why this category does not fully solve the problem:**

- These tools produce **plausible paths** from posture, identities, and exposures, but typically do not execute step-by-step proof actions from an assumed-breach Pod context to confirm exploitability.

---

### B) Runtime sensing, detection, and enforcement (DAST-style overlaps)

**Open-source runtime detection/enforcement:**

- **Falco:** runtime threat detection with Kubernetes context.
- **Tetragon:** eBPF-based security observability and runtime policy/enforcement.
- **KubeArmor:** runtime hardening and policy enforcement using LSM/eBPF.

**Runtime sensing in posture platforms:**

- **Kubescape node-agent runtime threat detection** (eBPF-based).
- Prisma Cloud Compute runtime incident taxonomy for Kubernetes.

**Why this category does not fully solve the problem:**

- These are generally **defensive sensors/enforcers**. They are not built to autonomously plan, chain, and validate exploit paths from a compromised Pod while generating an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase  and supported by collected artifacts.

---

### C) Static posture, IaC scanning, and RBAC analysis

**Posture and misconfiguration scanning:**

- Kubescape, Trivy, Checkov, Polaris, kubeaudit.

**RBAC analysis helpers:**

- kubectl-who-can, rbac-tool, Krane, CyberArk kubernetes-rbac-audit.

**Why this category does not fully solve the problem:**

- These tools output findings (misconfigurations, risky permissions, violations) but rarely prove exploitability via controlled actions.
- They do not autonomously chain primitives into evidence-backed multi-stage compromise narratives.

**Note:** kube-hunter is commonly cited historically but is marked deprecated/unmaintained, so it is not a strong baseline for new work.

---

### D) Breach-and-attack simulation (BAS) and adversary emulation

- **Cymulate Exposure Validation:** empirical proof via simulations; not Kubernetes-native assumed-breach Pod reasoning.
- **Stratus Red Team (EKS/Kubernetes context):** technique execution; scenario-driven rather than discovery-driven.
- **Atomic Red Team (Kubernetes atomics):** technique unit tests, not autonomous multi-step chaining.
- **MITRE Caldera:** adversary emulation platform with deployable agents.

**Why this category does not fully solve the problem:**

- Most emulation frameworks are scenario-driven: you choose what to run.
- They typically do not discover unknown cluster-specific attack chains from a compromised Pod and then generate an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase  and supported by collected artifacts.

---

## 3. The “white space”

**Unmet capability (crisp definition):**

- A Kubernetes-native agent that starts from an assumed-breach Pod identity and network position, then autonomously discovers and validates multi-step attack chains across RBAC, ServiceAccounts, Secrets, and workload pivots. We generate an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase  and supported by collected artifacts.

**Minimum differentiators to credibly claim novelty:**

1. Assumed-breach execution model: runs as a normal Pod with a real ServiceAccount token and real cluster networking.
2. Adaptive chaining: re-plans based on discovered objects, permissions, and runtime constraints.
3. Safe proof actions: controlled validation (read-only where possible; bounded where necessary) with explicit guardrails.
4. Evidence-grade output: step logs, raw API responses, object snapshots, timestamps, and replayable runs.
5. Typed edge mapping: edges like “RBAC allows get Secrets” → “Secret yields credential” → “credential authenticates as X,” backed by observed artifacts.

**Positioning statement:**

- Chain Reaction is an assumed-breach, in-cluster Kubernetes agent that does not just flag misconfigurations. It autonomously validates which findings are truly exploitable. We generate an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase  and supported by collected artifacts.

---

## 4. Evidence table (high-level comparison)

| Tool | Category | Rubric score (0–10) | In-cluster? | Autonomous reasoning? | Validates exploitability? | Output artifacts | Notes |
|---|---:|---:|---|---|---|---|---|
| Horizon3 NodeZero Kubernetes pentest | Autonomous pentest | 9 | Yes | Yes | Yes | Attack paths + proof (vendor claim) | In-cluster chaining and proof emphasis; details vary by deployment and license. |
| KubeHound | Attack graph | 5 | Yes (possible) | No | No | Graph queries/paths | Strong path discovery; does not execute proof actions. |
| IceKube | Attack graph | 5 | Yes (possible) | No | No | Neo4j graph | Graph discovery; avoids Secret extraction by design. |
| Konstellation | Graph framework | 4 | Yes (possible) | No | No | Neo4j dataset + query output | Data-to-graph framework; queries are user-driven. |
| Microsoft Defender for Cloud attack paths | CNAPP graph | 4 | No | No | No | Graph paths + recommendations | Prioritization and reachability modeling, not in-cluster proof actions. |
| Wiz attack path analysis | CNAPP graph | 4 | No | No | No | Graph context | Correlation and prioritization, not assumed-breach Pod validation. |
| Prisma Cloud attack path policies | CNAPP graph | 4 | No | No | No | Graph view + correlated risk | Strong visibility; not a proof-executing in-cluster agent. |
| Kubescape | KSPM + runtime | 3 | Yes | No | No | Findings + risk + runtime alerts | Detection posture; no multi-step exploit validation. |
| Falco | Runtime detection | 3 | Yes | No | No | Alerts/events | Defensive detection, not chaining/validation. |
| Tetragon | Runtime observability/enforcement | 3 | Yes | No | No | Events + enforcement | Defensive enforcement, not exploit chain validation. |
| KubeArmor | Runtime enforcement | 3 | Yes | No | No | Policy decisions + logs | Hardening/enforcement focus. |
| Trivy | IaC/posture scanning | 2 | Yes (optional) | No | No | Scanner outputs | Static findings; no chaining/proof. |
| Checkov | IaC scanning | 2 | No (typically) | No | No | Policy violations | Static rules focus. |
| kubeaudit | Cluster audit | 2 | Yes | No | No | Audit findings | Best-practice audit, not exploit validation. |
| Cymulate exposure validation | BAS | 4 | Deployment-dependent | No | Yes | Simulation results | Empirical testing; not Kubernetes-native chain discovery from a Pod. |
| Stratus Red Team (EKS) | Emulation | 3 | No | No | Yes | Technique execution results | Scenario-driven technique execution. |
| Atomic Red Team | Emulation | 2 | No | No | Yes (unit tests) | Atomic test outputs | Technique-level tests, not autonomous chaining. |

## References

1. Horizon3.ai: Kubernetes pentest docs — https://docs.horizon3.ai/portal/test_types/kubernetes/
2. Horizon3.ai: NodeZero Kubernetes pentesting page — https://horizon3.ai/nodezero/kubernetes-pentesting/
3. Datadog Security Labs: KubeHound article — https://securitylabs.datadoghq.com/articles/kubehound-identify-kubernetes-attack-paths/
4. WithSecure Labs: IceKube tool overview — https://labs.withsecure.com/tools/icekube--finding-complex-attack-paths-in-kubernetes-clusters
5. Praetorian: Konstellation (GitHub) — https://github.com/praetorian-inc/konstellation
6. Microsoft Learn: Defender for Cloud attack path concept — https://learn.microsoft.com/en-us/azure/defender-for-cloud/concept-attack-path
7. Wiz: Security Graph — https://www.wiz.io/lp/wiz-security-graph
8. Prisma Cloud docs: Attack path policies — https://docs.prismacloud.io/en/enterprise-edition/content-collections/governance/attack-path-policies
9. Sysdig: Falco — https://www.sysdig.com/opensource/falco
10. Cilium: Tetragon (GitHub) — https://github.com/cilium/tetragon
11. KubeArmor (GitHub) — https://github.com/kubearmor/KubeArmor
12. Kubescape docs: Runtime threat detection — https://kubescape.io/docs/operator/runtime-threat-detection/
13. Prisma Cloud Compute docs: Kubernetes attack incidents — https://docs.prismacloud.io/en/compute-edition/34/admin-guide/runtime-defense/incident-types/kubernetes-attack
14. Kubescape (GitHub) — https://github.com/kubescape/kubescape
15. Trivy docs: Misconfiguration scanning — https://trivy.dev/docs/v0.55/scanner/misconfiguration/
16. Checkov docs: Kubernetes integration — https://www.checkov.io/4.Integrations/Kubernetes.html
17. Polaris (GitHub) — https://github.com/FairwindsOps/polaris
18. kubeaudit (Docker Hub) — https://hub.docker.com/r/shopify/kubeaudit
19. Alcide: rbac-tool (GitHub) — https://github.com/alcideio/rbac-tool
20. Appvia: Krane (GitHub) — https://github.com/appvia/krane
21. CyberArk: kubernetes-rbac-audit (GitHub) — https://github.com/cyberark/kubernetes-rbac-audit
22. Cymulate: Exposure validation datasheet — https://cymulate.com/data-sheet/exposure-validation/
23. Stratus Red Team: Getting started — https://stratus-red-team.cloud/user-guide/getting-started/
24. Atomic Red Team: Kubernetes exec technique (example) — https://github.com/redcanaryco/atomic-red-team/blob/master/atomics/T1609/T1609.md
25. MITRE Caldera (GitHub) — https://github.com/mitre/caldera
26. Microsoft Community Hub: Kubernetes lateral movement and attack paths — https://techcommunity.microsoft.com/blog/microsoftdefendercloudblog/unveiling-kubernetes-lateral-movement-and-attack-paths-with-microsoft-defender-f/4374958
