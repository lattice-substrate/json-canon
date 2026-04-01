# ADR-0005: Evidence v2 and Infrastructure Manifest

- ADR ID: ADR-0005
- Date: 2026-04-01
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: OFFLINE-EVIDENCE-001, OFFLINE-EVIDENCE-002, OFFLINE-INFRA-001, OFFLINE-SERVER-001

## Context

The json-canon project uses an offline cold-replay harness to produce SHA-256 digest
chains proving byte-identical determinism across Linux distributions and architectures.
The current evidence.v1 schema binds evidence to source code (git commit + tag), the
replay binary, the vector bundle, the matrix, and the profile — but not to the
provisioned infrastructure itself.

A peer-reviewed research manuscript requires cross-architecture conformance evidence
gathered on native server hardware (AWS Graviton arm64 + x86_64) using
Infrastructure as Code (IaC). A reviewer must be able to:

1. Inspect the IaC at a pinned commit and understand what hardware was provisioned.
2. Verify that the evidence file was produced on that hardware.
3. Reproduce the provisioning and evidence generation independently.

The current evidence.v1 schema has no field for substrate identity. A reviewer sees
that 12 replay lanes on arm64 all agreed, but cannot determine what arm64 means in
hardware terms: instance type, AMI, CPU model. The infra manifest fills this gap.

Additionally, container lanes in the current matrix reference mutable image tags
(e.g., `debian:12-slim`). For server-backed runs the provisioned host must
pre-resolve these to digest references (`debian@sha256:...`) before replay, so the
evidence chain includes image identity, not just a mutable tag.

## Decision

1. Introduce `infra-manifest.v1` as a new, separate artifact type. It records:
   - IaC repo URL and exact commit (the provisioning source of truth)
   - Provider engine, version, and lock file digest (e.g., OpenTofu 1.8.0, `.terraform.lock.hcl` SHA-256)
   - Per-host records: role (`x86_64` or `arm64`), cloud provider, region, instance
     type, AMI/image ID
   - Optional per-host discovered fields: CPU model, kernel version (populated by
     Go-based discovery after provisioning)

2. Introduce `evidence.v2` as a new schema revision (not an optional v1 extension).
   It adds three required top-level fields:
   - `infra_manifest_sha256` — SHA-256 of the infra-manifest.v1 file
   - `infra_repo_url` — the IaC repo URL (duplicated from manifest for direct audit)
   - `infra_repo_commit` — the IaC commit SHA (duplicated for direct audit)
   Node replay items gain three optional fields:
   - `discovered_cpu`, `discovered_kernel` — substrate identity observed at replay time
   - `image_digest` — the resolved container image digest used for this replay

3. Validation accepts both v1 and v2. The schema version determines which checks apply:
   - v1: existing checks unchanged
   - v2: additional checks for the three new required fields; infra_manifest_sha256
     binding enforced if `EvidenceValidationOptions.ExpectedInfraManifestSHA256` is set

4. New server-backed profiles (`server-offline-linux-x86_64`,
   `server-offline-linux-arm64`) include `infra-substrate-binding` in
   `required_suites`. When a profile includes this suite, `ValidateEvidenceBundle`
   rejects v1 evidence. This keeps policy in the profile, not in a new struct field.

5. Trust boundary is unchanged: OpenTofu provisions, `jcs-offline-replay` executes
   the matrix and emits evidence, `go test ./offline/conformance` adjudicates.
   Terraform/OpenTofu never evaluates conformance.

6. One infra manifest per provisioning run covers both architectures. Each
   per-architecture evidence file binds the same `infra_manifest_sha256`. This
   preserves the existing per-arch evidence layout and aligns the manifest with the
   natural unit of infrastructure work (one `tofu apply`).

## Rationale

**Why a separate artifact (not inline in evidence)?**
Substrate identity belongs to the provisioning layer, not the replay layer. The
infra manifest is produced by the IaC tooling; the evidence is produced by the
harness. Keeping them separate preserves the trust boundary separation and avoids
coupling the replay schema to cloud provider concepts.

**Why evidence.v2 rather than optional v1 fields?**
The existing `evidence.v1.json` schema uses `additionalProperties: false` at both
the top-level and the `node_replays` item level. Adding fields to v1 would silently
fail JSON Schema validation for any external tool validating against the v1 schema.
A proper schema revision is cleaner and self-documenting.

**Why suite-driven policy rather than a new Profile field?**
The `required_suites` list already governs policy attestation. Adding a dedicated
`require_infra_binding` boolean would be a second classifier for the same concern.
The suite name `infra-substrate-binding` is precise and consistent with existing
suite names. Validation that cross-references suites and schema version is contained
in `ValidateEvidenceBundle`, the existing trust authority.

**Why duplicate infra_repo_url and infra_repo_commit in evidence?**
A reviewer reading an evidence file should not need to also have the infra manifest
to answer "what IaC produced this?" The duplication is intentional: the evidence
file is the primary audit artifact, and it should stand alone for first-pass review.
The infra manifest SHA provides the binding; the URL and commit provide legibility.

## Alternatives Considered

**Optional fields in evidence.v1** — rejected. `additionalProperties: false` in the
existing schema makes this a schema-breaking change regardless of how it is framed.
An optional extension would require either relaxing the schema constraint (reducing
validation strictness) or silently ignoring the fields in external validators.

**`require_infra_binding` boolean in Profile struct** — rejected. The suite-driven
approach is more consistent with how policy is already declared and validated.
Adding a new boolean to Profile creates a second classification axis for the same
concern.

**Embedding substrate identity directly in evidence top-level** — rejected. Substrate
identity is provisioned externally to the replay harness. Embedding it in evidence
would require the harness to know about cloud provider concepts (AMI, instance type),
coupling two layers that should remain independent.

**Terraform state as evidence artifact** — rejected. Terraform state is mutable,
backend-specific, potentially sensitive, and semantically overloaded. An
`infra-manifest.json` is a purpose-built, immutable, minimal artifact.
