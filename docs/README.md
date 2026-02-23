# Documentation Index

This directory contains official engineering documentation for `json-canon`.
These documents are authoritative unless superseded by a newer committed
document in this directory or by normative registries in the repository root.

## Normative and ABI Sources

- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `FAILURE_TAXONOMY.md`
- `abi_manifest.json`

## Engineering Specs

- `docs/book/README.md` - chaptered handbook (project, architecture, operations, release)
- `docs/BOOK.md` - compatibility portal to the chaptered handbook
- `ARCHITECTURE.md` - package boundaries, runtime model, and invariants
- `ABI.md` - stable CLI ABI contract (human-readable)
- `NORMATIVE_REFERENCES.md` - external/internal normative source and interpretation rules
- `SPECIFICATION.md` - normative product behavior contract
- `CONFORMANCE.md` - conformance gates and evidence requirements
- `THREAT_MODEL.md` - security threat model and control mapping
- `RELEASE_PROCESS.md` - maintainer release workflow and trust checks
- `docs/TRACEABILITY_MODEL.md` - requirement-to-code-to-test evidence model
- `docs/VECTOR_FORMAT.md` - JSONL vector schema and evolution policy
- `docs/ALGORITHMIC_INVARIANTS.md` - strict parsing/canonicalization invariants
- `docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md` - executable differential cases against Cyberphone Go JCS
- `docs/OFFLINE_REPLAY_HARNESS.md` - offline replay runbook and cross-arch proof workflow
- `docs/adr/` - architectural decision records and ADR process

## Operational Docs

- `VERIFICATION.md` - release verification (checksums + provenance)
- `offline/README.md` - offline cold-replay matrix and evidence workflow
- `BOUNDS.md` - resource bounds and memory behavior
- `GOVERNANCE.md` - review, compatibility, and policy commitments

## Navigation

Recommended start points:

1. Project orientation: `docs/book/README.md`
2. System contracts: `ARCHITECTURE.md`, `SPECIFICATION.md`, `ABI.md`
3. Release and trust: `CONFORMANCE.md`, `RELEASE_PROCESS.md`, `VERIFICATION.md`
