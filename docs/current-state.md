# Current implementation state

Status: repository evidence inventory, updated through WORK-001

Reviewed commit: `cfbfbb68d8472fb51fdd748d2666264d73ac6048` (`main`, 2026-09-03)

## Scope and evidence rules

This document records only what is verifiable from the checked-out `main`
branch. A Linear status, paper statement, or historical artifact is not
implementation evidence. Rows marked `unverified` require the referenced
Linear issue, attachment, pull request, or commit before they can be changed.

No hidden seeds, target names, ports, predicates, expected chains, controller
credentials, or evaluation artifacts are recorded here.

## Repository capability map

| Claim | Status | Implementation evidence | Test or artifact evidence | Introduced in |
| --- | --- | --- | --- | --- |
| Go command builds and passes vet | implemented | `cmd/chain-reaction`, all packages | `go build ./cmd/chain-reaction` and `go vet ./...` passed on 2026-09-03 | `729163f`, `eb34441` |
| Planner mode configuration | implemented, blind by default | `internal/config/config.go`, `internal/agent/validation.go`, `internal/agent/state.go` | `internal/config/validation_test.go`, `internal/agent/planner_observations_test.go` | `729163f`, `dec2bf0` |
| Blind-mode prompt filtering and bounded observation rendering | implemented, partial | `internal/agent/planner_observations.go`, `internal/agent/planner_react.go` | `internal/agent/planner_observations_test.go` | `729163f` |
| Kubernetes Goat catalog-family matcher | implemented as v1 | `internal/baseline/catalog.go`, `internal/baseline/matcher.go` | `internal/baseline/matcher_test.go`; five overlapping catalog families | `729163f` |
| Resource-specific semantic `ValidationClaim` | absent | No `ValidationClaim` type or versioned claim schema exists on `main`. | No claim-schema tests or artifacts exist. | unverified |
| Connected causal-chain scorer | absent | `internal/baseline/matcher.go` matches tool name, outcome, order, and limited namespace context. It does not represent actor, exact resource, effect, evidence hash, or reusable predicate semantics. | Matcher tests are v1 catalog tests. | unverified |
| Evidence collection | implemented as v1 | `internal/evidence/collector.go` appends timestamped JSONL records. | `internal/evidence/collector_test.go` | `729163f` |
| Evidence hash chain or signed run manifest | absent | Evidence records have no prior hash, record hash, signature, or offline verifier. | No verifier or integrity-tampering tests exist. | unverified |
| Centralized immutable pre-I/O policy | absent | `internal/guardrails/enforcer.go` provides namespace and rate checks only. Tools retain their own defaults and policy-like inputs. | No policy canonicalization, DNS, destination, or fuzz suite exists. | unverified |
| Secret-read policy boundary | unsafe legacy behavior | `internal/tools/validation/read_secret.go` defaults namespace after runner checks and accepts model-controlled `allow_namespaces`. | Existing tests do not prove immutable operator policy. | `729163f` |
| Restricted agent and privileged controller identities | absent | `deploy/rbac.yaml` binds the agent ServiceAccount to a cluster-wide read ClusterRole, including Secrets and RBAC resources. | No controller/agent visibility or permission-attestation test exists. | `eb34441` |
| Hidden parameterized benchmark v2 | absent | No scenario-manifest, generator, commitment, controller-only range, or oracle package exists on `main`. | No public-fixture determinism or kind positive/blocked matrix exists. | unverified |
| Deterministic replay | absent | No `replay` command or production replay path exists. | No replay golden or tamper-rejection suite exists. | unverified |
| Independent repeated scenario instances | absent | `scripts/run-reproducibility.sh` runs all five families together, then creates per-family symlink views. | Per-family views are not independent runs. | `eb34441` |
| Controlled evaluation matrix and paired analysis | absent | No versioned matrix manifest, hidden-run index, eligibility gate, or paired v2 analysis exists. | Existing analysis is legacy catalog-family output. | `729163f`, `eb34441` |

## Baseline and current verification

The following checks passed against the reviewed commit:

```text
go build ./cmd/chain-reaction
go vet ./...
```

At the reviewed commit, the following 12 `internal/agent` failures were
reproduced:

```text
TestReactValidationPlannerUsesPromptModule
TestReactValidationPlannerPromptHistoryFormatting
TestReactValidationPlannerGoalModePrefixByteStable
TestBuildPlannerStateSummary_AppearsInUserMessage
TestRunValidationLoopWritesDebugLogArtifact
TestValidationFinalAnswerPath
TestValidationReactLoopLifecycleIntegration
TestValidationTraceCandidateStepIDsPopulated
TestValidationFinalAnswerRejectedWhenKG005S3Unmet
TestValidationFinalAnswerRejectedWhenOnlyKG005S3Validated
TestValidationFinalAnswerRejectedWhenOnlyKG005S3BlockedByRBAC
TestValidationAllFamiliesValidatedEarlyStop
```

These failures divided into prompt-contract drift, final-answer and
early-termination semantics, candidate-step metadata, and debug-event
expectations. WORK-001 resolved them in `dec2bf0` by making blind mode the
internal default, requiring Goat-hinted fixtures to opt in, and pinning the
mode-specific prompt contract.

The current worktree passed:

```text
gofmt -l .
go test -count=1 ./...
go vet ./...
go build -o NUL ./cmd/chain-reaction
go test -count=20 ./internal/agent -run Test(BlindPlannerObservationsAreBoundedAndRedacted|BlindPlannerSummaryOmitsCatalogProgress|BlindGoalAndPromptAuditRejectCatalogCanary|ValidationFinalAnswerPath|ValidationFinalAnswerRejectedWhenKG005S3Unmet)
```

Race testing is blocked locally: `CGO_ENABLED=0`, and enabling it fails because
`gcc` is unavailable on `PATH`. It remains required in CI or a Windows toolchain
with a C compiler.

## Linear reconciliation ledger

Linear was not authenticated during this review. The following rows therefore
state the repository result, not the issue's actual status. Do not close,
reopen, relabel, or infer an issue from this table until its attachments,
comments, branches, pull requests, and commits have been reviewed.

| Linear issue | Repository evidence on `main` | Current disposition |
| --- | --- | --- |
| CR-253 | No benchmark v2 contract, generator, or commitment implementation found. | unverified - inspect Linear history |
| CR-254 | No deterministic semantic oracle or controller-only oracle configuration found. | unverified - inspect Linear history |
| CR-255 | No versioned `ValidationClaim` or exact resource/actor/effect predicate system found. | unverified - inspect Linear history |
| CR-259 | No versioned scenario-manifest contract found. | unverified - inspect Linear history |
| CR-260 | No hidden-instance generator or public seed-commitment artifact found. | unverified - inspect Linear history |
| CR-261 | No controller-only hidden range setup or teardown contract found. | unverified - inspect Linear history |
| CR-262 | No immutable oracle result or predicate-level output found. | unverified - inspect Linear history |
| CR-263 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-264 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-265 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-266 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-267 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-268 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-269 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-270 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-271 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-272 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-273 | Acceptance criteria are not present in this checkout or in `WORK.md`. | unverified - inspect Linear history |
| CR-274 | No typed semantic-claim implementation found. | unverified - inspect Linear history |
| CR-275 | No typed semantic-claim implementation found. | unverified - inspect Linear history |

## Recovery search

The current repository has only `origin/main`, no open GitHub pull requests,
and no registered Git worktrees. Its unreachable Git objects contain only
legacy Goat scenario/runbook material, not v2 contracts, generators, claims,
or an oracle.

The separately owned `E:\Projects\school\chain-reaction-old` checkout was
inspected read-only with a command-scoped Git trust exception that was not
persisted. Its `main` is 241 commits ahead of `origin/main`, with HEAD
`48a47eb` tagged `submission-v1.1`, and it also contains `submission` and
`origin/submission`. That history contains v1 catalog, tooling, and artifact
material. Its former current-state document and baseline explicitly describe a
manual Goat v2 baseline without an authoritative per-scenario manifest.
Searches found no typed semantic claim, scenario-contract repository,
generator, oracle, hidden benchmark, replay pipeline, or CR-253 through
CR-275 history. No old commit is eligible for recovery into the v2 primary
path; the checkout remains only historical v1 calibration evidence.

## v1 and v2 boundary

The existing catalog matcher, evidence JSONL, Goat environment scripts, and
paper figures are v1 calibration or historical material. They must not be
included in a v2 primary analysis. V2 begins only when versioned scenario, run,
evidence, claim, and oracle contracts exist and their trust boundaries and
integrity checks pass.
