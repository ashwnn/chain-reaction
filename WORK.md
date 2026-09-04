# Chain Reaction Remaining Work

Last reviewed: 2026-09-03

This document is the execution source of truth for the work that remains before Chain Reaction can support its academic and engineering claims. It preserves the original idea: an assumed-breach Kubernetes Pod uses an LLM-guided, bounded tool loop to discover and validate attack-chain transitions, while machine evidence and an independent scorer determine what is actually proven.

Linear records intent and project history. A Linear issue marked Done is not proof that the work exists in this checkout. A task is complete only when its implementation is present on `main`, the required artifacts exist, and the verification listed here passes.

## Non-negotiable research and safety invariants

- Kubernetes Goat is a public calibration benchmark. It is not the primary evidence of generalization.
- The primary evaluation uses hidden, parameterized scenario instances with committed public hashes and controller-only ground truth.
- The agent under test cannot access hidden seeds, expected chains, oracle state, controller credentials, or benchmark answers.
- Model text never grants validation credit. Only normalized machine evidence evaluated by deterministic semantic predicates can validate a claim.
- A validated chain is a connected causal path. A collection of unrelated successful tools is not a chain.
- TCP, DNS, or HTTP reachability proves only the corresponding network property. It does not prove authentication, authorization, data access, or compromise.
- Mutation is disabled by default, lab-only, narrowly capability-scoped, reversible, TTL-bounded, and cleanup-attested.
- Raw tokens, credentials, authorization headers, and Secret values are never written to planner context or default artifacts.
- An incomplete, contaminated, unsafe, non-independent, duplicated, or integrity-failed run cannot enter the primary analysis.
- Historical v1 artifacts remain available as calibration and regression evidence, but must not be silently reinterpreted under v2 contracts.

## Verified current baseline

At the time of this review:

- `go build ./cmd/chain-reaction` passes.
- `go vet ./...` passes.
- `go test ./...` passes after the WORK-001 planner-mode reconciliation.
- The repository is on `main`, and local `main` matches `origin/main` at `cfbfbb6`.
- The paper calls five catalog families "scenarios," while the matcher uses overlapping tool-name-based family contracts.
- The repeated-run script performs one all-family invocation per repeat and creates per-family symlink views. Those views are not independent scenario runs.
- The evidence JSONL is not hash-chained, despite the paper claiming that it is.
- The configured namespace allow-list can be bypassed when a tool omits `namespace` and applies its own `default` namespace after the runner's policy check.
- The deployed evaluation ServiceAccount has cluster-wide read access to Secrets, RBAC objects, workloads, and related resources.
- Linear describes hidden benchmark, semantic oracle, live baseline, and typed-claim work as complete, but the corresponding documented files and implementations are absent from this `origin/main` checkout.
- Linear names `docs/current-state.md` as implementation truth, but the file is absent.

## Dependency order

Work should proceed in this order. Primary evaluation must not start early.

1. Reconcile implementation truth and restore a green baseline.
2. Recover or implement the benchmark v2 contracts and enforce the agent/controller/oracle trust boundary.
3. Centralize policy enforcement, evidence integrity, telemetry, and cleanup validity.
4. Finish resource-specific semantic claims and proof actions.
5. Integrate claims into causal graph scoring and deterministic replay.
6. Complete adversarial, integration, and kind-based verification.
7. Freeze the evaluation matrix and statistical analysis plan.
8. Run the independent hidden evaluation.
9. Generate final comparisons, rewrite the paper, and package the release.

## Definition of done for every task

- The implementation is present in the checked-out repository and referenced by a commit.
- Unit and negative tests cover the new security or research property.
- Machine-readable schemas are versioned and validated.
- User-facing and research documentation describe actual behavior, not planned behavior.
- `go test ./...`, `go vet ./...`, formatting, script linting, and all task-specific checks pass.
- Generated artifacts identify git SHA, image digest, schema version, tool hash, policy hash, and configuration hash where applicable.
- `WORK.md` is updated with status, evidence, and any intentionally deferred scope.

---

## WORK-000: Reconcile Linear, repository history, and implementation truth

Status: Implemented - all listed Linear issues reopened; missing v2 work is tracked below
Priority: P0  
Linear context: CR-253, CR-254, CR-259 through CR-275, project description

### Why this exists

Linear says the contamination-resistant protocol, hidden range generator, semantic oracle, live baselines, blind planning protections, typed claims, and semantic matcher are complete. Those artifacts are not present on `origin/main`. Autonomous work cannot safely build on features whose location and version are unknown.

### Required work

1. Inspect the Linear issue attachments, linked branches, pull requests, commits, and comments for CR-253, CR-254, CR-259 through CR-275.
2. Determine for every supposedly completed item whether it is:
   - merged under an unexpected path;
   - present on an unmerged branch;
   - implemented in another repository or worktree;
   - only designed in Linear;
   - lost or superseded.
3. Produce `docs/current-state.md` with a table mapping every research claim and Linear issue to the implementing file, test, artifact version, and commit.
4. Recover valid work through reviewable commits. Do not copy hidden seed material into public history.
5. Reopen or relabel Linear issues whose acceptance criteria are not present on main.
6. Record the v1 and v2 schema boundaries. Never make existing v1 artifacts appear to be v2 output.

### Acceptance criteria

- Every Done item in M7 and the completed portion of M8 has verifiable repository evidence or is explicitly reopened.
- `docs/current-state.md` exists and is accurate against main.
- Public benchmark commitments are explicitly absent and tracked by WORK-002 without exposing hidden scenario material.
- No hidden target names, ports, predicates, or solution ordering enter planner-visible or public fixtures.
- The repository has one unambiguous source of truth for current capability.

### Verification

- Compare `git ls-tree -r HEAD`, Linear issue claims, and `docs/current-state.md` mechanically.
- Search public code and artifacts for hidden canaries and benchmark-only identifiers.
- Review recovered commits before starting WORK-003.

---

## WORK-001: Restore a green and internally consistent main branch

Status: Implemented - race verification pending CI/toolchain  
Priority: P0  
Related Linear: CR-258

### Why this exists

The current code compiles but its planner and validation tests disagree with the new blind/goat-hinted behavior. Failing tests include prompt formatting, candidate-step metadata, final-answer gating, early termination, and debug events. Until these failures are resolved, later evaluation artifacts cannot be trusted.

### Required work

1. Classify each failing test as a real regression, an obsolete v1 expectation, or a mode-selection fixture bug.
2. Define termination semantics separately for:
   - `blind`: the agent may conclude, but controller-side scoring decides success;
   - `goat_hinted`: catalog readiness may guide termination, but results are calibration only;
   - `scripted_oracle`: controller-only and never available to the agent planner.
3. Make prompt construction use one stable contract. Update implementation and tests together without leaking catalog state into blind mode.
4. Ensure candidate step IDs are either intentionally v1/goat-hinted-only or removed in favor of v2 semantic claims. Document the decision.
5. Ensure tests do not rely on shared global state, wall-clock races, or stale artifact versions.
6. Make CI run for contract, deployment, documentation, and paper changes that can invalidate research claims.

### Acceptance criteria

- All 12 known failures are resolved for documented reasons.
- Blind-mode tests prove no `KG-*`, target, catalog-progress, or oracle content reaches the planner.
- A blind final answer cannot itself grant chain success.
- Goat-hinted behavior remains available only as a labeled calibration/ablation mode.
- CI uses the Go version declared by `go.mod` or documents a tested compatibility matrix.

### Verification

```text
gofmt -l .
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/chain-reaction
```

Run prompt-leakage and final-answer tests individually with `-count=20` to detect flaky state.

Verification recorded 2026-09-03:

- `gofmt -l .`, `go test -count=1 ./...`, `go vet ./...`, and `go build -o NUL ./cmd/chain-reaction` pass.
- Prompt-leakage and final-answer tests pass with `-count=20`.
- `go test -race ./...` cannot run locally because CGO is disabled and no C compiler is available on `PATH`; run it in CI or a configured Windows C toolchain before closing WORK-001.

---

## WORK-002: Recover or implement benchmark v2 and its deterministic oracle

Status: Open
Priority: P0  
Linear: CR-253, CR-254, CR-259, CR-260, CR-261, CR-262

### Research context

Public, well-known benchmarks can measure benchmark familiarity instead of general capability. The primary result therefore needs hidden, reproducible, parameterized instances. Relevant background includes Happe and Cito's review of offensive-security benchmarking practices and research on LLM benchmark contamination:

- https://arxiv.org/abs/2504.10112
- https://aclanthology.org/2024.findings-emnlp.30/

The goal is not to claim that a closed model is proven uncontaminated. The goal is to reduce direct answer leakage, separate development from evaluation, and report the remaining limitation honestly.

### Required work

1. Define versioned contracts for scenario manifests, run manifests, evidence references, oracle predicates, results, and public seed commitments.
2. Support at least eight attack-path archetypes covering:
   - workload identity and token use;
   - namespaced and cluster-scoped RBAC;
   - Secret access;
   - cross-namespace access;
   - service discovery and network movement;
   - authenticated service interaction;
   - disposable workload execution;
   - policy-blocked negative controls.
3. Generate randomized namespace, ServiceAccount, role, binding, workload, Secret, Service, endpoint, port, and decoy names from a seed.
4. Create positive/blocked counterfactual pairs differing by one control, such as RBAC, NetworkPolicy, token audience, or a missing prerequisite.
5. Provide at least one public development seed and three private evaluation seeds per archetype.
6. Keep hidden material controller-side. Publish only schema, public examples, generator behavior, and cryptographic commitments.
7. Implement a deterministic oracle that evaluates exact actor/action/target/effect predicates against evidence and cluster state.
8. Prove deterministic regeneration, isolated setup, health checks, teardown, and residue detection.

### Acceptance criteria

- At least 24 hidden instances exist across the archetypes, with positive and blocked variants.
- Equal seeds generate byte-identical canonical manifests and digests.
- The agent cannot read the hidden manifest, oracle configuration, expected chain, or controller credentials.
- An LLM evaluator cannot upgrade failed or unknown machine evidence.
- The oracle emits a versioned immutable result with predicate-level reasons.
- Kind integration tests cover at least one positive and one blocked instance per archetype.

### Verification

- Regenerate public fixtures twice and compare canonical hashes.
- Attempt to read controller-only objects from the agent identity and require denial.
- Tamper with one predicate, evidence hash, and scenario object and require oracle failure.
- Run setup and teardown repeatedly and require no cross-run resources or credentials.

---

## WORK-003: Enforce agent, controller, and oracle privilege separation

Status: Open  
Priority: P0  
Linear: CR-282

### Why this exists

The research claim concerns what a constrained assumed-breach identity can discover and prove. The current deployment grants the `chain-reaction` ServiceAccount cluster-wide read access, including Secrets and RBAC. If the agent shares controller privileges or can observe oracle state, success measures the harness rather than the attack path.

Kubernetes authorization guidance must be followed precisely. `SelfSubjectRulesReview` is advisory because its result may be incomplete. Exact claims should use targeted `SelfSubjectAccessReview` or actual bounded actions:

- https://kubernetes.io/docs/reference/kubernetes-api/definitions/self-subject-rules-review-v1-authorization/
- https://kubernetes.io/docs/concepts/security/service-accounts/
- https://kubernetes.io/docs/concepts/security/rbac-good-practices/

### Required work

1. Draw and document trust zones for the agent, range controller, hidden oracle/scorer, LLM endpoint, artifact store, and operator.
2. Use different ServiceAccounts and credentials for controller and agent roles.
3. Generate the agent's effective permissions from the scenario's starting-identity contract. Do not grant generic cluster-wide discovery rights unless that is the scenario.
4. Keep controller Secrets, hidden manifests, seed material, expected chains, and privileged kubeconfigs outside the agent Pod and process.
5. Capture a pre-run identity and effective-permission attestation and bind it to the run manifest.
6. Fail a run if actual agent permissions exceed the declared starting envelope.
7. Add negative visibility tests for controller namespaces, Secrets, ConfigMaps, files, environment variables, volumes, and API surfaces.
8. Treat lab-only mutation permissions as temporary explicit capability additions, not permanent RBAC.

### Acceptance criteria

- The controller can build and score a range while the agent remains restricted to the scenario identity.
- The agent cannot enumerate or retrieve controller/oracle resources.
- A machine check compares expected and effective privileges before every run.
- The paper's architecture diagram and threat model show these boundaries.
- The current broad `deploy/rbac.yaml` is removed from primary evaluation or labeled as v1 calibration-only.

### Verification

- Run `kubectl auth can-i --list` or equivalent under each identity and compare with the manifest contract.
- Mount no shared credentials or hidden volumes into the agent Pod.
- Include a kind negative test where a prompt-injected agent attempts to locate oracle material and fails.

---

## WORK-004: Centralize guardrails, integrity, telemetry, and safety state

Status: Open  
Priority: P0  
Linear: CR-257

### Why this exists

The current runner checks only explicitly supplied namespaces before dispatch. Namespace-aware tools can apply `default` after that check, and `read_secret` accepts a caller-supplied `allow_namespaces` parameter. Guardrails must be configured by the operator and enforced after canonicalizing the complete action, never weakened by model-controlled fields.

### Required work

1. Introduce one immutable policy object and one pre-I/O authorization path used by every tool.
2. Resolve defaults and canonical targets before policy evaluation.
3. Enforce namespace, API group, resource, verb, subresource, object name/prefix, destination, resolved IP/CIDR, port, protocol, redirects, mutation count, response size, time, steps, and rate.
4. Default-deny loopback, link-local, cloud metadata, node/host, control-plane, and external destinations unless explicitly authorized.
5. Resolve DNS and re-check every returned address immediately before connecting to reduce rebinding and mixed-address bypasses.
6. Remove model-controlled policy parameters or require that they can only narrow the immutable policy.
7. Give every proposed action, policy decision, Kubernetes request, and network probe a stable action ID and monotonic sequence number.
8. Count actual Kubernetes API requests and network probes separately. Record waits, retries, blocks by type, output truncation, and the termination-causing event.
9. Add per-artifact SHA-256 hashes and either a hash-chained event stream or signed run manifest. Implement an offline verifier.
10. Apply schema-based redaction before both artifact writes and planner rendering, including nested values and error strings.
11. Define run safety states: eligible, incomplete, unsafe, contaminated, integrity-failed, and cleanup-failed.
12. Remove command-line API-key transport such as `--llm-api-key`. Inject provider credentials through an environment variable or read-only mounted file, and verify they never appear in process arguments, run metadata, logs, planner context, or artifacts.

### Acceptance criteria

- Omitting a namespace cannot bypass an allow-list.
- No tool input can broaden policy.
- Every I/O operation is attributable to a prior policy decision.
- Corruption, truncation, reordering, missing files, or duplicated events causes verification failure.
- ACV, probe volume, rate-limit waits, policy blocks, unsafe attempts, and cleanup state are present in the metric contract.
- A guardrail failure still produces a verifiable terminal artifact when safe to do so.

### Verification

- Add table-driven and fuzz tests for omitted defaults, malformed hosts, IPv4/IPv6 variants, encoded IPs, redirects, multiple DNS answers, and policy precedence.
- Tamper with every artifact class and require verifier failure.
- Search artifacts for synthetic token/Secret canaries and require no matches.

---

## WORK-005: Finish authoritative resource-specific semantic validation

Status: In progress  
Priority: P0  
Linear: CR-255, CR-274, CR-275

### Why this exists

The current v1 matcher grants credit mainly from tool name, successful outcome, order, and limited namespace context. The blind hypothesis artifact groups observations by tool category. Neither is sufficient to prove a specific attack transition.

### Required work

1. Recover or implement a versioned `ValidationClaim` containing:
   - claim and execution ID;
   - actor identity and source foothold;
   - action/verb;
   - exact Kubernetes GVK/name/namespace or network endpoint;
   - prerequisites and predecessor claim IDs;
   - expected and observed effect;
   - normalized machine outcome and failure reason;
   - evidence IDs and hashes;
   - cleanup dependency where applicable;
   - provenance and confidence.
2. Build claims only from normalized tool output and trusted runtime identity. Planner-supplied actor, effect, or success text is non-authoritative.
3. Enforce the outcome lattice: failed/unknown cannot become validated; an optional model evaluator may only annotate or downgrade.
4. Replace generic expected-tool matching with resource, identity, action, effect, and prerequisite predicates.
5. Prevent evidence reuse across unrelated predicates unless the scenario explicitly declares shared evidence.
6. Require connected source-to-target continuity for a complete chain.
7. Preserve separate semantics for observed, theoretical, unclassified, failed, and validated facts.
8. Preserve the deterministic scan path and v1 historical reader.
9. Treat fields decoded from an unverified JWT as asserted token metadata, not verified identity or cluster state. Bind authorization claims to trusted runtime identity plus targeted live Kubernetes authorization checks or executed proof actions.

### Acceptance criteria

- A correct tool against the wrong resource, namespace, identity, protocol, or effect receives no credit.
- Reordered steps fail when a prerequisite is required.
- Replayed evidence cannot satisfy a new execution.
- Missing taxonomy data is unclassified, never validated.
- Positive, negative, wrong-target, wrong-actor, duplicate, malformed-hash, decoy, and fuzz tests pass.

### Verification

- Construct paired fixtures that differ only in target, identity, order, or effect and verify only the exact claim passes.
- Confirm one execution cannot inflate multiple family/scenario scores unless allowed by contract.
- Review schema migrations against existing v1 artifacts.

---

## WORK-006: Add authenticated non-mutating proof actions

Status: Open  
Priority: P0  
Linear: CR-277

### Why this exists

DNS resolution, TCP connection, and generic HTTP response are useful observations but do not prove authenticated application behavior or usable Kubernetes credentials.

### Required work

1. Add a bounded authenticated-service tool with an exact destination, protocol, method, request template, response-size cap, redirect policy, timeouts, and semantic response predicate.
2. Add a current-token Kubernetes API proof using a safe non-mutating endpoint or targeted access review.
3. Never expose tokens or authorization headers in logs, errors, snapshots, or planner context.
4. Emit WORK-005 claims binding actor, request, target, response/effect, and evidence hashes.
5. Apply WORK-004 network and output policy before all resolution and I/O.

### Acceptance criteria

- Transport success alone cannot validate an authenticated-service claim.
- Unauthenticated, wrong-target, wrong-response, redirect, timeout, oversized-response, and policy-denied cases remain failed or observed.
- Token API proof confirms the live actor without persisting token bytes.
- Existing TCP/DNS/HTTP observation semantics remain available but are not overstated.

### Verification

- Use local test servers for status/body/auth variations and redirect chains.
- Add fake API tests for accepted, denied, expired, wrong-audience, and canceled requests.
- Scan all artifacts for credential canaries.

---

## WORK-007: Complete lab-only reversible mutation proofs

Status: In progress  
Priority: P0  
Linear: CR-276, CR-279, CR-280, CR-281

### Shared transaction requirements

- Mutation tools are not registered unless an immutable lab capability is enabled.
- Policy specifies namespace, resource, verb, object prefix, subject/image/command constraints, TTL, maximum object count, and cleanup deadline.
- Every transaction records before-state, create/use/delete actions, UID/resourceVersion, benign marker hash, rollback result, and residue check.
- Failure, cancellation, timeout, partial creation, unknown cleanup, or residue invalidates the proof and every dependent chain.
- Pre-existing resources are never modified or deleted.

### WORK-007A: Disposable workload marker proof

Linear: CR-280

Implement create/execute/delete of an ephemeral workload using an allow-listed image and fixed benign marker protocol. Do not expose arbitrary shell execution. Persist only the marker hash and bounded metadata, not arbitrary Pod output.

Acceptance requires disabled-default, namespace/prefix/image/command/TTL/count escape, cancellation, partial-create, delete-failure, residue, marker-mismatch, and successful-cleanup tests.

### WORK-007B: Reversible RBAC or impersonation proof

Linear: CR-281

Use the narrowest namespaced object and benign verification action. Enforce exact subject, rules, resource, verb, namespace, object prefix, TTL, and count. Never grant wildcards or privileges broader than the one proof. Roll back on every exit path and verify absence.

Acceptance requires disabled-default, wildcard, subject, rule, namespace, prefix, partial-create, cancellation, rollback-failure, residue, and success tests.

### Completion gate

CR-276 is complete only when both child proofs use the shared transaction boundary and cleanup failure propagates into claims, graphs, metrics, oracle results, and run eligibility.

---

## WORK-008: Integrate claims into causal graphs and scoring

Status: Open  
Priority: P0  
Linear: CR-278

### Why this exists

The current runtime graph is largely a star of tool executions from `pod:current`. A useful attack graph must show which specific capability enabled the next transition and why each edge is validated, blocked, observed, or theoretical.

### Required work

1. Build nodes from concrete identities, Kubernetes resources, workloads, credentials-as-capabilities, and endpoints.
2. Build edges from authoritative WORK-005 claims, not from generic tool invocation.
3. Persist predecessor references and require connected causal continuity.
4. Represent observed, theoretical, unclassified, failed, and validated separately.
5. Attach predicate results, evidence hashes, policy decisions, and cleanup state to each edge.
6. Invalidate mutating edges and downstream chains when cleanup is failed or unknown.
7. Keep blind planner input independent of controller-side scoring and oracle progress.
8. Version the graph and scoring schemas and provide explicit v1 migration/read-only support.

### Acceptance criteria

- No unrelated successful calls are rendered as one attack chain.
- Every validated edge identifies exact source, target, action, effect, and evidence.
- Graph-level chain success equals deterministic oracle success for the same run.
- Decoy, replay, reorder, corruption, cleanup-failure, and wrong-target tests fail correctly.
- Production-tool offline integration covers discovery to hypothesis to proof to claim to score to graph.

### Verification

- Compare graph paths with oracle predicate traces for every public fixture.
- Validate JSON schema and DOT/visual rendering.
- Recompute graph output twice from identical normalized events and require canonical equality.

---

## WORK-009: Add deterministic offline replay and artifact reconstruction

Status: Backlog  
Priority: P1  
Linear: CR-283

### Why this exists

The paper should call artifacts auditable until a reviewer can reconstruct claims, graph state, policy decisions, metrics, and final scoring without contacting the LLM or cluster.

### Required work

1. Add a command such as `chain-reaction replay <run-dir>`.
2. Verify the run manifest and artifact hashes before parsing semantic content.
3. Feed recorded normalized outcomes through the same production scorer used by live evaluation.
4. Reconstruct claims, causal graph, policy events, termination, cleanup state, metrics, false-positive claims, and final score.
5. Make replay perform zero LLM, network, and Kubernetes calls.
6. Refuse incompatible mixed schemas unless an explicit migration exists.
7. Compare reconstructed and recorded outputs while ignoring only declared presentation timestamps or paths.

### Acceptance criteria

- Golden runs replay to identical canonical semantic output.
- Missing, reordered, truncated, duplicate, corrupt, wrong-resource, and cleanup-failed bundles are rejected.
- Live and replay modes share one scoring implementation.
- Documentation separates independently reproducible conclusions from observations that still depend on external state.

---

## WORK-010: Build the capstone-grade verification program

Status: Blocked by WORK-004, WORK-005, WORK-008  
Priority: P0  
Linear: CR-258

### Required test layers

1. Unit tests for every Kubernetes wrapper and discovery/introspection tool, including forbidden, timeout, pagination, cancellation, malformed-object, and output-bound cases.
2. Offline production-tool integration using a fake or envtest API plus scripted LLM provider. Do not replace production boundaries with only `fakeTool` tests.
3. Guardrail property and fuzz tests for canonicalization, policy precedence, destinations, response limits, and concurrency.
4. Semantic matcher/oracle fuzz and adversarial tests for wrong actor, wrong target, reordered steps, duplicate evidence, decoys, and malformed hashes.
5. Planner tests for unknown tools, schema smuggling, repeated actions, output floods, prompt-injected object content, secret exfiltration attempts, and mutation-policy bypasses.
6. Kind end-to-end tests for one positive and one blocked instance per v2 archetype.
7. Artifact schema/golden tests and rejection of incomplete or mixed-version bundles.
8. Shell orchestration tests using mocked `kubectl`, `kind`, `docker`, `jq`, Graphviz, and agent commands.
9. CI tiers:
   - every PR: formatting, unit, vet, build, schema, docs/manifest lint;
   - scheduled: race and bounded fuzzing;
   - opt-in/nightly: kind matrix with retained artifacts.

### Security-focused adversarial cases

- Indirect prompt injection in names, labels, annotations, ConfigMap values, error strings, and service responses.
- Kubernetes object designed to resemble planner or tool instructions.
- Omitted/default parameters, alternative IP encodings, IPv6, DNS rebinding, redirects, metadata IPs, and mixed allowed/denied DNS answers.
- Oversized results, recursive/nested sensitive keys, tokens embedded in errors, and Unicode/control-character smuggling.
- Controller credential discovery, hidden-manifest discovery, and oracle side-channel attempts.
- Evidence truncation, reordering, replay, duplicate IDs, wrong hashes, mixed schemas, path traversal, and filename collisions.
- Cancellation during mutation, cleanup timeout, residue, and pre-existing-object collision.

### Acceptance criteria

- Security-critical packages have explicit branch/requirement thresholds.
- Every research invariant has at least one positive and one negative automated test.
- CI cannot pass when the paper or current-state document claims a missing artifact contract.
- The exact release commit passes the full required test tier before evaluation.

---

## WORK-011: Freeze and implement the controlled evaluation matrix

Status: Blocked by WORK-002 through WORK-010  
Priority: P1  
Linear: CR-256

### Research question

Measure the contribution of LLM planning separately from the tools, oracle, catalog, prompt hints, and guardrails.

### Required conditions

- Live configuration/static analyzer.
- Deterministic heuristic planner.
- Random valid-action lower bound with recorded random seed.
- Scripted oracle upper bound that remains controller-only.
- Blind LLM planner as the primary agent condition.
- Goat-hinted LLM planner as leakage/calibration ablation only.
- At least three materially different models from at least two providers, including an open/local model when feasible.
- Strict, default, and relaxed guardrail profiles with explicit numeric policy settings.

All conditions must receive the same observable environment, starting identity, scenario seeds, action vocabulary where applicable, and comparable time/step/request budgets. A catalog export is not a live static baseline.

### Required work

1. Create a versioned matrix manifest listing every scenario, seed commitment, condition, model, provider, policy, repeat, and expected artifact ID.
2. Randomize or counterbalance condition order where environmental drift can matter.
3. Make execution resumable without replacing failed hidden cases or changing seeds after unblinding.
4. Fail closed on missing, duplicate, incomplete, contaminated, unsafe, or integrity-failed cells.
5. Record model identifier/version/date, provider, decoding parameters, prompts, tool schemas, retries, token use, cost, and all software/image/cluster digests.
6. Run a separate pilot and freeze all tuning before primary execution.

### Acceptance criteria

- The matrix can be generated deterministically and audited before runs begin.
- Identical seed-condition pairs are truly paired.
- Hidden primary cases are never used for prompt, tool, or policy tuning.
- Results separate capability, generalization, efficiency, safety, and evidence completeness.

---

## WORK-012: Correct the metric and statistical analysis contract

Status: Blocked by WORK-004 and WORK-011  
Priority: P1  
Linear: CR-149

### Current methodological problem

The paper promises a paired agent-versus-static SVR test, while the implementation reports a one-sample Wilcoxon test of five aggregate family-coverage values against 0.80. Those five observations are discrete, share one all-family run structure, and do not answer the stated comparative hypothesis.

The Wilcoxon signed-rank test assumes a symmetric distribution of paired differences. Small sample size alone is not sufficient justification:

- https://www.itl.nist.gov/div898/software/dataplot/refman1/auxillar/signrank.htm

### Required work

1. Define the experimental unit as scenario instance/seed x condition x independent repeat.
2. Define one preregistered primary outcome: full semantic-chain success on hidden instances.
3. Define secondary outcomes:
   - partial predicate progress;
   - precision, recall, false-positive and false-negative claims;
   - right-censored TTC;
   - steps, Kubernetes requests, network probes, tokens, cost, and wall time;
   - policy blocks and unsafe attempts;
   - cleanup and evidence-integrity success;
   - public-to-hidden generalization gap.
4. Use paired comparisons on identical seeds.
5. Report effect sizes and 95 percent confidence intervals, not p-values alone.
6. Use Wilson or exact intervals for binary success. For paired binary conditions, use an exact paired method or paired bootstrap/permutation. For repeated scenario/model data, consider a hierarchical or mixed-effects logistic model if sample size supports it.
7. Treat unsuccessful or timed-out TTC observations as right-censored. Do not report completion time only for successes.
8. Perform an a priori or simulation-based power/sensitivity analysis to determine repeats.
9. Correct for multiple comparisons across models and policies or clearly label secondary comparisons exploratory.
10. Reject ineligible runs with explicit machine-readable reasons.
11. Retain v1 descriptive plots as historical output, clearly separated from primary inference.

### Acceptance criteria

- Analysis code implements the frozen estimands and paired units.
- Tests cover ties, zeros, missing cells, censoring, duplicate runs, exclusions, and exact small-sample cases.
- Independent software reproduces a sample of core statistics.
- Every table states denominator, unit, confidence interval, and exclusion count.
- No claim of baseline superiority is derived from defining the static baseline's runtime success as zero by construction.

---

## WORK-013: Execute independent reproducibility runs

Status: Blocked by WORK-011 and WORK-012  
Priority: P1  
Linear: CR-150

### Required work

1. Freeze the release candidate, protocol, public commitments, matrix, exclusions, and analysis plan before the first primary run.
2. Run every scenario-seed/condition/repeat cell with a unique run ID and private artifact directory.
3. Reset and health-check the cluster between instances, or prove equivalent snapshot isolation.
4. Verify pre-run identity, scenario digest, prompt/tool/policy hashes, image digest, and absence of residue.
5. Verify evidence integrity, oracle result, cleanup, and artifact completeness after every run.
6. Support exact resumability. Do not replace a failed cell, change its seed, or tune after hidden outcomes are observed.
7. Emit a hash-committed run-set manifest and analysis-ready index.
8. Preserve raw artifacts separately from generated summaries.

### Acceptance criteria

- No symlinked family view is counted as an independent observation.
- Missing, duplicate, corrupt, unsafe, contaminated, or incomplete cells fail the run set.
- Repeat count matches WORK-012's power/sensitivity rationale.
- A reviewer can trace every analyzed row to one eligible run bundle.

---

## WORK-014: Generate the final benchmark comparison

Status: Blocked by WORK-013  
Priority: P1  
Linear: CR-151

### Required work

1. Compare blind LLM performance with every baseline on paired hidden seeds.
2. Report public Goat calibration, renamed/perturbed Goat, and hidden generated performance separately.
3. Report full-chain success, partial progress, false claims, efficiency, safety, cleanup, integrity, and generalization gap.
4. Distinguish model effects from tool, policy, and prompt-mode effects.
5. Include uncertainty, effect sizes, denominators, exclusions, and censored TTC treatment.
6. Generate machine-readable comparison output, publication tables, plots, and a concise evidence-backed interpretation.

### Acceptance criteria

- Every figure and number is generated from the frozen eligible run index.
- Re-running analysis produces identical semantic results.
- No debug batch, tuning batch, or v1 calibration run enters the primary table.
- Claims are limited to the evaluated scenarios and conditions.

---

## WORK-015: Rewrite the paper as a final empirical study

Status: Blocked by WORK-014  
Priority: P0 for submission  
Linear context: CR-152, CR-200, CR-201, CR-207 through CR-210 require reopening or superseding if the paper changes materially

### Structural correction

The current paper combines a proposal, future project plan, architecture specification, and final results. The timeline says implementation begins after some reported experiments occurred. Rewrite it as one temporally consistent final paper:

1. Introduction and contributions.
2. Related work and reproducible search method.
3. Threat model, safety boundary, and agent/controller/oracle architecture.
4. System design and proof semantics.
5. Experimental design, preregistration, baselines, metrics, and statistical methods.
6. Frozen primary results.
7. Discussion, threats to validity, ethics, and limitations.
8. Conclusion and future work.

Move the old proposal timeline to an appendix or remove it from the final paper.

### Required scientific corrections

- Relabel existing `SC` as v1 catalog-family coverage.
- Present April Goat batches as engineering/calibration history, not controlled primary evidence.
- Replace the one-sample 0.80 Wilcoxon result with the frozen v2 paired analysis, or label it descriptive historical output only.
- Do not call the per-family symlink views independent runs.
- State that `SelfSubjectRulesReview` may be incomplete and targeted checks or actual proof actions are authoritative.
- Describe projected ServiceAccount tokens accurately, including opt-out, audience, expiry, and rotation behavior.
- Replace "decoded secret values" with redacted metadata and proof of authorized read.
- Use "auditable" until WORK-009 proves deterministic replay.
- Claim hash-chained or signed evidence only after WORK-004 is implemented and verified.
- State RQ3 as unanswered unless the guardrail ablation is completed.
- Distinguish network reachability from authenticated or security-impacting effects.
- Replace universal novelty claims with "within the surveyed sources" and document databases, queries, dates, and inclusion/exclusion criteria.
- Frame the contribution as a benchmark and evidence-verification platform demonstrated on Kubernetes, not a production exploitability estimator.

### Bibliography corrections

- Correct `happe2025benchmarking`: the ArXiv record lists Andreas Happe and Jurgen Cito, not Amiram Kaplan.
- Do not call the Happe and Cito paper a statistical meta-analysis.
- Rename misleading BibTeX keys such as `miller2021automated` for a 2018 report and `applebaum2022characterizing` for Chang et al. 2025.
- Cite the pinned Kubernetes Goat repository release/commit used by the evaluation.
- Verify every author, title, year, venue, DOI, and characterization against a primary source.

### Acceptance criteria

- Every quantitative statement is generated from a retained artifact and traceable by run ID.
- Methods and results use the same research questions, estimands, units, and tests.
- Threats to internal, construct, external, and conclusion validity are explicit.
- The paper builds without warnings that indicate missing references, labels, figures, or malformed tables.
- A claim-to-evidence appendix maps each contribution and result to code, schema, test, and artifact.

### Verification

- Add a reproducible LaTeX build and bibliography check to CI.
- Run a scripted check for uncited bibliography entries, missing references, stale figure files, and numbers copied outside generated macros/tables.
- Perform a final independent source and statistical audit.

---

## WORK-016: Decide the fate of few-shot exemplars

Status: Decision required after primary design freeze  
Priority: P3  
Linear: CR-163

Few-shot examples for five public chain types create a contamination and attribution risk. They must not be added to the primary blind condition. Choose one of these outcomes:

1. Cancel CR-163 because evidence-derived blind planning is the intended contribution.
2. Keep examples only for a Goat-hinted calibration condition.
3. Evaluate them as a preregistered prompt ablation using public development scenarios only, then freeze before hidden evaluation.

Acceptance requires explicit labeling, separate artifacts, and proof that hidden identifiers, predicates, and target structure are absent.

---

## WORK-017: Final release, demo, and submission package

Status: Blocked by WORK-015  
Priority: P1  
Linear context: M6 artifacts must be regenerated from the final release

### Required work

1. Tag the exact evaluated commit and pin container, Go, kind, Kubernetes, Kubernetes Goat, and analysis dependencies.
2. Build the submission package from a clean checkout.
3. Include protocol, public commitments, schemas, current-state map, test report, eligible run index, generated paper, plots, sample redacted evidence, and replay/verifier instructions.
4. Provide a demo with:
   - one positive hidden-style public fixture;
   - one blocked counterfactual;
   - a policy denial;
   - an evidence-integrity failure;
   - a causal graph and predicate trace;
   - cleanup and residue attestation.
5. Make the demo deterministic enough for grading. Keep live model variance optional rather than making the core demonstration depend on it.
6. Review the repository for credentials, hidden seeds, raw Secret values, private endpoints, and stale artifacts.

### Acceptance criteria

- A clean machine can build, test, run the public demonstration, verify artifacts, and compile the paper from documented commands.
- The release package contains no hidden evaluation material or secrets.
- All displayed results identify the evaluated commit and run-set manifest.
- Repository, Linear, paper, and demo describe the same implemented system.

## Final completion gate

Chain Reaction is academically ready only when all of the following are true:

- main is green;
- implementation truth is reconciled;
- hidden benchmark and trust separation are verified;
- semantic causal claims replace tool-name credit;
- guardrails and evidence integrity are fail-closed and tested;
- independent hidden runs are complete;
- paired statistical analysis is frozen and reproducible;
- the paper reports only supported claims;
- the release can be independently built and audited.
