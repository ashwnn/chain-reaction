# Benchmark v2 scenario and oracle contract

Status: controller-side contract foundation. Range generation and lifecycle
integrity checks are implemented; runtime scoring remains incomplete.

## Trust boundary

`internal/benchmark` is controller-side code. It has no dependency on planner
or agent packages. A hidden `ScenarioManifest` and `OracleContract` must never
be mounted into the agent Pod, sent to a model provider, or committed as a
public fixture.

The only pre-evaluation artifact permitted in public history is a
`benchmark.commitments.v2` manifest. Its entries contain opaque instance IDs
and SHA-256 commitments for the seed, finalized scenario, and oracle. They do
not have fields for raw seeds, resources, identities, predicates, variants,
targets, or solution order.

## Versioned contracts

- `benchmark.scenario.v2`: controller-only scenario identity, split,
  counterfactual variant, attacker identity, resources, approved proof actions,
  oracle reference, and lifecycle ownership.
- `benchmark.oracle.v2`: controller-only exact actor, action, target, effect,
  and predecessor predicates.
- `benchmark.run.v2`: immutable run metadata and digests for code, image,
  scenario, oracle, prompt, tool policy, and seed commitment.
- `benchmark.commitments.v2`: public pre-evaluation commitment inventory.

All decoders reject unknown fields, trailing JSON values, invalid versions,
invalid enums, malformed digests, duplicate identifiers, unknown predecessors,
and invalid lifecycle or proof-action bounds.

## Digest binding

The scenario and oracle refer to each other. To avoid a circular digest,
`ScenarioProjection` omits only `oracle_ref.digest`; the oracle stores the
digest of that projection. `FinalizeScenarioOracle` then sets the scenario's
oracle digest to the finalized oracle digest. A change to either document
invalidates the binding.

Canonical encoding rejects maps and floating-point values. Contracts use
declared struct fields, ordered slices, compact JSON, and lowercase SHA-256
digests. Commitment entries are sorted by instance ID before publication.

## Seed derivation

Raw seeds must remain in controller-only storage outside this repository.
`CommitSeed` produces a domain-separated public HMAC-SHA-256 commitment.
`DeriveDNSName` and `DerivePort` use domain-separated HMAC-SHA-256 output and
rejection sampling, so they do not embed raw seed or scope text and avoid modulo
bias.

`WritePrivateSeed` refuses unsafe identifiers, symlink directories, and
overwrites. It creates owner-only storage where the host supports POSIX modes.
`BuildCommitmentManifest` and `WritePublicCommitmentManifest` emit only the
public commitment shape, sort entries, and refuse to replace an existing
pre-evaluation inventory. `benchmark-private/` is ignored as a defense in
depth measure; controller deployments should use storage outside the checkout.

## Current limits

The contract generator creates deterministic controller-only contract pairs for
eight archetype shapes and one declared positive/blocked control. Its in-memory
renderer creates the Namespace, ServiceAccount, Role, conditional RoleBinding,
workload, Secret, Service, Endpoints, decoy, and conditional NetworkPolicy
objects in stable order. It does not provision a cluster or perform Kind setup
and teardown automatically. `ApplyRenderedRange` creates controller-rendered
objects through the Kubernetes API, rejects pre-existing run state, and
attests each returned object UID. `CleanupAppliedRange` uses UID-preconditioned
deletion in reverse order and waits for residue to disappear, rejecting a
replacement object under a range name. The opt-in Kind lifecycle test verifies
Namespace creation, Service-generated Endpoints, and namespace deletion. The
controller-only scorer evaluates
exact normalized actor, action, target, effect, sequence, and evidence-digest
bindings, but does not yet query live cluster state. Those capabilities remain
required before benchmark v2 results may be reported.
