# Literature Review

## Reproducible search strategy

### Databases and high-authority sources

- **Academic:** IEEE Xplore, ACM Digital Library, USENIX, arXiv, Semantic Scholar (cross-indexing and forward citation chasing).
- **High-authority industry and maintainer sources:** vendor research blogs and maintainer advisories documenting real exploitation and mitigations (for example: Wiz Research, Kubernetes project advisories, Datadog Security Labs).

### Search strings (copy-paste ready)

Use a year filter (2020–2026) and combine at least one term from each group:

- Kubernetes + attack chaining / graphs
  - `"Kubernetes" AND ("attack graph" OR "attack path" OR "kill chain" OR "reachability")`
  - `"Kubernetes" AND ("RBAC" OR "RoleBinding" OR "ServiceAccount") AND ("privilege escalation" OR "lateral movement")`
  - `"Helm" AND ("attack path" OR "security model" OR "graph")`

- Runtime validation and exploitability context
  - `"Kubernetes" AND ("runtime validation" OR "dynamic analysis" OR "DAST") AND (deployment OR manifest OR Helm)`
  - `"Ingress" AND ("admission controller" OR "validating webhook") AND ("RCE" OR "exploit chain")`

- LLM and agentic reasoning for security validation
  - `("LLM" OR "large language model") AND ("Kubernetes" OR "cloud-native") AND (reasoning OR remediation OR "runtime logs")`

### Inclusion and exclusion criteria

**Include (must satisfy all):**

- Published 2020–2026.
- Directly informs at least one of: multi-step attack paths, attack-graph/attack-chain modeling, runtime exploitability validation, Kubernetes primitives as attack surfaces, or LLM-based reasoning that reduces false positives using context.
- Provides enough methodological detail to evaluate threat model and evidence quality (industry reports are acceptable if detailed).

**Exclude:**

- Generic malware/RL papers with no Kubernetes or cloud-native chaining relevance.
- Compliance-only guidance with no validation, reachability, or chaining angle.

### Screening procedure

- Stage 1: title/abstract screen for Kubernetes + chaining/validation/graphs/agentic reasoning.
- Stage 2: full-text screen for (a) assumed-breach realism, (b) evaluation or well-documented case studies, (c) mapping to Kubernetes primitives.
- Stage 3: backward and forward citation chasing from the strongest seed works.

---

## Literature review matrix (10 works)

| Work | Topic | Method | Key findings | Relevance to proposal | Gap |
|---|---|---|---|---|---|
| Minna et al. (2021) | Kubernetes networking, CNI behavior, unexpected lateral movement | Hybrid: conceptual modeling + reproducible testbed | Kubernetes networking abstractions can invalidate traditional segmentation assumptions and enable unexpected movement. | Motivates runtime reachability validation rather than relying purely on policy intent. | Limited automation; no autonomous validation agent or evidence pipeline. |
| Blaise & Rebecchi (2022) | Attack path extraction from Helm charts | Hybrid: graph construction + scoring + evaluation | Graph-based extraction of deployment security models and risky paths from packaging artifacts. | Matches a phase-labeled attack graph output and configuration-derived hypotheses. | Primarily config-derived; does not resolve runtime truth (reachability, live permissions). |
| Yang et al. (2023) | Multi-stage takeover via excessive permissions in third-party apps | Hybrid: exploitation analysis + empirical characterization | Over-permissive third-party components can enable multi-step escalation; RBAC is a key substrate. | Supports assumed-breach chains involving ServiceAccounts, RBAC, and Secrets. | Focused on a specific ecosystem slice; not a reusable autonomous validation framework. |
| Rahman et al. (2023) | Misconfigurations in Kubernetes manifests at scale | Hybrid: empirical study + tool construction | Catalogs common misconfiguration classes and provides detection tooling at scale. | Good baseline for static scanners and prevalence grounding. | Static-only; cannot confirm exploitability under runtime constraints. |
| Datadog Security Labs: KubeHound (2023) | Kubernetes attack graph tooling | Hybrid: engineering artifact + documented modeling | Builds an attack-graph representation and computes plausible paths between assets. | Strong baseline hypothesis generator to compare against a validator. | Does not execute proof actions or produce step-level evidence. |
| Malul et al. (2024) | LLM-based detection and remediation of Kubernetes misconfigurations (GenKubeSec) | Hybrid: system + precision/recall evaluation | Uses LLMs to explain and remediate misconfigurations beyond rule matching. | Supports LLM reasoning over manifests and justified output structure. | Not focused on chaining or runtime validation from an in-cluster foothold. |
| Shamim et al. (2025) | Dynamic application security testing (DAST) for Kubernetes deployments | Quantitative evaluation | Frames runtime/dynamic testing for Kubernetes and evaluates detection vs static approaches. | Informs evaluation design and baseline comparisons under runtime context. | Tool-driven rather than autonomous; limited multi-step assumed-breach chaining. |
| Wiz Research + Kubernetes advisory (2025) | IngressNightmare (Ingress NGINX admission-controller RCE chain) | Hybrid: vulnerability analysis + exposure assessment | Demonstrates a high-impact chain where internal reachability and admission surfaces can lead to cluster-wide compromise. | Strong motivating exemplar for safe reachability + behavior validation. | Not a standardized experimental framework or benchmark. |
| Rostamipoor et al. (2025) | Protecting Secrets from leakage under excessive permissions (KubeKeeper) | Hybrid: system + evaluation | Protects Kubernetes Secrets under excessive permissions via mechanism-level controls. | Grounds the “impact” end of many chains (Secrets access). | Defensive focus; does not validate diverse offensive chains end-to-end. |
| Sgan Cohen et al. (2025) | LLM-assisted Kubernetes hardening using manifests and runtime logs (KubeGuard) | Hybrid: workflow + quantitative quality metrics | Uses LLM workflows to propose least-privilege changes informed by runtime behavior. | Closest prior art for runtime-context-informed reasoning. | Hardening recommender rather than an attack-chain validator with proven edges and evidence. |

---

## Synthesis aligned to the proposal

### Why static scanners miss contextual exploitability

- Static checks can identify risky conditions but do not reliably determine whether a finding is reachable and chainable in a specific live cluster.
- Networking and enforcement behavior may differ from “intended” segmentation; runtime conditions matter.

### Why deterministic scanners fail to adapt

- Real compromise paths depend on what is deployed, reachable, and permissioned at runtime.
- Deterministic tools typically do not re-plan when blocked by missing permissions, changing topology, or environment-specific constraints.

### How autonomous reasoning agents could validate multi-stage attack chains

A defensible design pattern is:

- Use an attack-graph or rule-based model as a hypothesis generator (what might chain).
- Use runtime observations (API responses, reachability probes, logs) to confirm or refute edges.
- Generate an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase and supported by collected artifacts.

---

## Defensible research gaps that justify Chain Reaction

1. **End-to-end autonomous validation from an assumed-breach Pod is rarely evaluated as a complete system.**
2. **Benchmarks for “validated” Kubernetes attack chains are not standardized across tool classes.**
3. **Runtime context is acknowledged as crucial, but rarely integrated into explainable, reviewable outputs with auditable evidence.**

## References

1. Wiz Research (2025): Ingress NGINX vulnerabilities / IngressNightmare overview — https://www.wiz.io/blog/ingress-nginx-kubernetes-vulnerabilities
2. Minna et al. (2021): Understanding the security implications of Kubernetes networking — https://balakrishnanc.github.io/papers/minna-ieeesp2021.pdf
3. Blaise & Rebecchi (2022): Stay at the Helm: secure Kubernetes deployments via graph generation and attack reconstruction — https://www.researchgate.net/publication/362926162_Stay_at_the_Helm_secure_Kubernetes_deployments_via_graph_generation_and_attack_reconstruction
4. Yang et al. (2023): Attacking Kubernetes via excessive permissions of third-party applications — https://www.cs.wm.edu/~smherwig/readings/papers/23-ccs-kubernetes_excessive_permissions.pdf
5. Rahman et al. (2023): Security misconfigurations in open-source Kubernetes manifests — https://dl.acm.org/doi/full/10.1145/3579639
6. KubeHound (GitHub) — https://github.com/DataDog/KubeHound
7. Datadog Security Labs (2023): KubeHound article — https://securitylabs.datadoghq.com/articles/kubehound-identify-kubernetes-attack-paths/
8. Malul et al. (2024): GenKubeSec — https://arxiv.org/pdf/2405.19954
9. Shamim et al. (2025): Dynamic application security testing for Kubernetes deployments (preprint) — https://akondrahman.github.io/files/papers/fse25.pdf
10. Shamim et al. (2025): Dynamic application security testing for Kubernetes deployments (ACM DL entry) — https://dl.acm.org/doi/abs/10.1145/3696630.3728573
11. Rostamipoor et al. (2025): KubeKeeper — https://www3.cs.stonybrook.edu/~mikepo/papers/kubekeeper.eurosp25.pdf
12. Sgan Cohen et al. (2025): KubeGuard — https://arxiv.org/abs/2509.04191
