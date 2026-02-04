# Chain Reaction

An autonomous, in-cluster Kubernetes agent that validates multi-step attack chains.

We generate an evidence-backed, phase-labeled attack graph in which each node/edge is annotated with a phase (for example, recon, execution, privilege escalation) and supported by collected artifacts.

## Problem

Static Kubernetes security scanners and attack-path modeling tools identify *possible* risks—but they rarely prove what is *actually exploitable* from an in-cluster foothold under real runtime constraints. Defenders are left with many plausible attack paths and limited evidence of which multi-step chains can actually be executed.

## Solution

**Chain Reaction** is a Go-based agent that runs as a standard Kubernetes Pod with only its assigned ServiceAccount credentials and normal cluster networking. It autonomously discovers and validates multi-step attack chains spanning RBAC permissions, Secret access, network pivots, and workload takeovers.

A chain step is **validated** only when the agent can execute it from within the Pod, capture supporting evidence (API responses, object snapshots, probe results), and explain why it succeeded or failed (RBAC denial, unreachable target, guardrail block, missing prerequisite).

## Key Features

- **Assumed-breach execution model:** runs as a normal Pod with real cluster credentials and networking; no special node access or external secrets.
- **Adaptive chaining:** uses an LLM-guided, tool-based loop to plan and reprioritize actions based on discovered objects, permissions, and runtime constraints.
- **Safe proof actions:** controlled, read-only probes where possible; bounded-impact validation where necessary; explicit guardrails (allow-lists, rate limits, time budget, stop conditions).
- **Evidence-backed output:** raw API responses, probe outputs, object snapshots, timestamps, and audit trails packaged into a reproducible evidence bundle.
- **Phase-labeled attack graph:** nodes and edges typed by Kubernetes primitive (RBAC, Secret, Service, Pod, etc.) and annotated with a phase (for example, recon, execution, privilege escalation); edges explicitly labeled as validated or theoretical; each validated edge tied to step-level evidence.

## Deliverables

1. **Phase-labeled attack graph** (JSON + optional visual render): multi-step attack paths with validated vs theoretical edges, phase annotations per node/edge, and evidence references.
2. **Evidence bundle**: step logs, API responses, object snapshots, timestamps, and a manifest for integrity verification.
3. **Academic evaluation**: coverage on Kubernetes Goat scenarios, comparison against baselines (static scanners, attack-graph tools), and reproducibility analysis.

## Goals

- Validate at least 80% of Kubernetes Goat scenarios by producing a runtime-validated chain and evidence bundle per scenario.
- Demonstrate reproducibility: repeat runs on the same lab should yield materially similar graphs and evidence.
- Provide a fair baseline for comparing static scanners, attack-graph tools, and LLM-guided autonomous agents.

## Documentation

- [Kubernetes background & exploits](docs/k8s-background-research.md): overview of Kubernetes primitives as attack surfaces and real CVE examples.
- [Literature review](docs/literature-review.md): related work and research gaps.
- [Similar projects & tools](docs/similar-projects.md): comparison to existing tools and frameworks.
- [Milestones & plan](docs/milestones.md): project execution plan with phases and issue definitions.

## Prerequisites

- A Kubernetes cluster (local kind or minikube for development; Kubernetes Goat for evaluation).
- Go 1.21+ (for building the agent).
- kubectl configured to access the target cluster.

## Quick Start

(Coming soon—placeholder for CLI build, deployment, and lab setup instructions.)

## Safety & Guardrails

Chain Reaction is designed for **controlled lab environments only**. All actions are bounded by:

- Explicit allow-lists for target resources and namespaces.
- Rate limiting and retry bounds to avoid cluster disruption.
- A time budget and step limit to prevent runaway loops.
- Read-only API operations where possible; write operations are opt-in and heavily constrained.
- Logging and auditing of all actions for reproducibility and review.

## License

[MIT](LICENSE)