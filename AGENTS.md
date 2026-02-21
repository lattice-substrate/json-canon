# AGENTS.md

## Status

This is the authoritative operating contract for human and AI agents working in
`json-canon`.

If any document conflicts with this file, this file takes precedence for agent
execution behavior. Normative standards and explicit ABI artifacts still govern
product correctness.

## Mission

Maintain `json-canon` as decades-lived infrastructure:

- correct against RFC 8785 and related normative references,
- deterministic by construction,
- stable for machine consumers (strict SemVer ABI),
- auditable with requirement-to-code-to-test evidence,
- secure in runtime and release supply chain.

## Hard Constraints (Non-Negotiable)

1. Supported runtime platform is Linux only.
2. Release binary is static (`CGO_ENABLED=0`).
3. Core runtime must not perform outbound network calls.
4. Core runtime must not execute subprocesses.
5. Canonicalization core does not use `encoding/json` as engine.
6. Required conformance/traceability gates are Go-native (`go test`), not shell-required gates.
7. Public CLI ABI follows strict SemVer.
8. Tracked compiled binaries in repo root are prohibited.

## Source-of-Truth Hierarchy

1. Normative specs and standards clauses referenced by requirement IDs.
2. `REQ_REGISTRY_NORMATIVE.md` and `REQ_REGISTRY_POLICY.md`.
3. `REQ_ENFORCEMENT_MATRIX.md` and `standards/CITATION_INDEX.md`.
4. `abi_manifest.json` and `FAILURE_TAXONOMY.md`.
5. Accepted ADRs in `docs/adr/`.
6. This file (`AGENTS.md`) for execution/process discipline.
7. `PLANNING/` documents are non-authoritative drafts unless promoted.

## Stable ABI Contract

Treat the following as versioned public API:

- command and subcommand names,
- flag names and semantics,
- exit code mapping and failure class behavior,
- stdout/stderr channel contract,
- machine-consumed output grammar,
- deterministic canonical stdout bytes for accepted inputs.

Rules:

1. Patch releases: no ABI behavior changes.
2. Minor releases: additive, backward-compatible only.
3. Major releases: required for breaking ABI changes.
4. Any ABI-affecting change must update `abi_manifest.json`, tests, and `CHANGELOG.md`.

## Engineering Invariants

1. Determinism: identical input + options => identical bytes.
2. Classification by root cause, not input source.
3. Bounds enforcement is explicit, stable, and test-covered.
4. No hidden dependence on map iteration order, time, locale, or environment for correctness.
5. No panic-based control flow in production paths.
6. Conformance claims require executable evidence.

## Required Change Workflow

For every non-trivial change, agents must execute this sequence.

1. Classify change type:
   - normative behavior,
   - policy/profile behavior,
   - ABI/CLI behavior,
   - internal refactor,
   - docs-only.
2. Identify impacted requirement IDs before editing.
3. Implement minimal coherent change + regression tests.
4. Update traceability artifacts in the same change when behavior moves.
5. Run required gates locally.
6. Update changelog/docs/ADR as required.
7. Confirm no prohibited patterns were introduced.

## Traceability Update Rules

If behavior changes, update all applicable artifacts:

- `REQ_REGISTRY_NORMATIVE.md` for normative requirement changes.
- `REQ_REGISTRY_POLICY.md` for project policy/ABI/process requirements.
- `REQ_ENFORCEMENT_MATRIX.md` for requirement-to-symbol-to-test mapping.
- `standards/CITATION_INDEX.md` for normative requirement citation coverage.
- `abi_manifest.json` for CLI/ABI changes.
- `FAILURE_TAXONOMY.md` for error-class contract changes.
- `docs/adr/` for compatibility/security/platform decision changes.
- `CHANGELOG.md` for any user-visible behavior shift.

## Mandatory Validation Gates

Run (or justify why not possible) before merge:

```bash
go vet ./...
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

For release and ABI-sensitive changes, also validate deterministic static build:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
```

## Security and Supply-Chain Discipline

1. Keep CI/release actions pinned by commit SHA.
2. Preserve checksum + provenance attestation flow in release pipeline.
3. Do not bypass vulnerability handling process in `SECURITY.md`.
4. Any change reducing verification strength requires ADR + maintainer approval.

## Go-First Automation Policy

1. Required gates must live in Go tests/tools.
2. Shell scripts may be used for convenience only, not as required compatibility/conformance gates.
3. Exception requires explicit maintainer approval with written rationale proving:
   - Go-native path is impractical,
   - Linux-only support remains intact,
   - determinism/auditability are not weakened.

## Planning vs Official Documentation

`PLANNING/` is working space, not contract.

Promotion rule:

1. If guidance is normative, compatibility-relevant, or operationally binding,
   move it to root docs or `docs/`.
2. Once promoted or obsolete, archive planning content under dated
   `PLANNING/archive/...` or delete if redundant.
3. Avoid duplicate authoritative statements across planning and official docs.

## Prohibited Changes

1. Silent CLI behavior changes.
2. Untracked requirement deltas (code changes without registry/matrix updates).
3. Introducing nondeterministic behavior in canonical paths.
4. Introducing runtime network/process execution in core packages.
5. Replacing canonicalization logic with `encoding/json` behavior.
6. Weakening bounds or failure-class semantics without tests and docs.
7. Adding required non-Go conformance gates without approved exception.

## Definition of Done (Infrastructure Grade)

A change is done only when all are true:

1. Correctness proven by tests at the right layer (unit/blackbox/conformance).
2. Traceability is complete and passes conformance gates.
3. ABI impact is explicitly handled (or explicitly none, with evidence).
4. Determinism and bounds guarantees remain intact.
5. Documentation is updated in the same change.
6. Security and release trust posture is not degraded.

## Quick File Map

- Product contract: `README.md`, `abi_manifest.json`, `FAILURE_TAXONOMY.md`
- Architecture/spec contract: `ARCHITECTURE.md`, `ABI.md`, `NORMATIVE_REFERENCES.md`, `SPECIFICATION.md`, `CONFORMANCE.md`, `THREAT_MODEL.md`, `RELEASE_PROCESS.md`
- Requirement system: `REQ_REGISTRY_NORMATIVE.md`, `REQ_REGISTRY_POLICY.md`, `REQ_ENFORCEMENT_MATRIX.md`, `standards/CITATION_INDEX.md`
- Process/governance: `CONTRIBUTING.md`, `GOVERNANCE.md`, `SECURITY.md`, `CHANGELOG.md`
- Engineering docs: `docs/README.md`, `docs/TRACEABILITY_MODEL.md`, `docs/VECTOR_FORMAT.md`, `docs/ALGORITHMIC_INVARIANTS.md`, `docs/adr/`
- Release verification: `VERIFICATION.md`
