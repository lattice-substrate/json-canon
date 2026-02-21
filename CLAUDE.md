# CLAUDE.md

## Document Status

This document defines the infrastructure-grade engineering constitution for
`json-canon`.

- Scope: architecture, implementation, verification, release, and long-term
  maintenance standards.
- Audience: maintainers, contributors, and AI agents.
- Authority: normative for engineering quality and process discipline.

If this document conflicts with normative specifications, the specifications and
requirement registries win on product behavior. If this document conflicts with
explicit agent execution rules in `AGENTS.md`, `AGENTS.md` wins for agent
operating behavior.

## RFC 2119 Keywords

The words `MUST`, `MUST NOT`, `REQUIRED`, `SHALL`, `SHALL NOT`, `SHOULD`,
`SHOULD NOT`, `RECOMMENDED`, `NOT RECOMMENDED`, `MAY`, and `OPTIONAL` in this
file are to be interpreted as described in RFC 2119 and RFC 8174.

## Mission

`json-canon` is long-lived infrastructure. It exists to provide deterministic,
audit-grade RFC 8785 canonicalization and verification for machine consumers
where compatibility and correctness matter more than feature velocity.

## Infrastructure-Grade Definition

The project is infrastructure-grade only if all conditions below remain true:

1. Behavior is explicitly specified and testable.
2. Public ABI is versioned and compatibility-governed.
3. Conformance claims are clause-cited and executable.
4. Failure handling is classed, stable, and machine-actionable.
5. Canonical output determinism is enforced and regression-tested.
6. Build and release artifacts are verifiable.
7. Governance is durable beyond a single maintainer.

Any change that weakens one or more conditions above MUST be rejected or
explicitly approved through the exception process in this document.

## Product Scope

`json-canon` scope includes:

- strict JSON parsing and policy validation,
- RFC 8785 canonical serialization,
- ECMA-262 Number::toString-compatible numeric formatting,
- stable CLI ABI for machine automation,
- conformance and traceability evidence.

Non-goals include convenience parsing modes that compromise strictness,
non-deterministic behavior, and ungoverned compatibility drift.

## Non-Negotiable Project Constraints

The following are hard constraints.

1. Runtime support target is Linux only.
2. Release binary MUST be static (`CGO_ENABLED=0`).
3. Core runtime MUST NOT perform outbound network calls.
4. Core runtime MUST NOT execute subprocesses.
5. Canonicalization core MUST NOT delegate to `encoding/json` as canonical engine.
6. Required conformance/traceability gates MUST be Go-native.
7. Stable CLI ABI MUST follow strict SemVer.
8. Tracked compiled binaries in repository root are prohibited.

## Normative Sources and Evidence Hierarchy

The following order SHALL be used when resolving correctness:

1. Normative standards text.
2. `REQ_REGISTRY_NORMATIVE.md`.
3. `REQ_REGISTRY_POLICY.md`.
4. `REQ_ENFORCEMENT_MATRIX.md`.
5. `standards/CITATION_INDEX.md`.
6. `abi_manifest.json`.
7. `FAILURE_TAXONOMY.md`.
8. Accepted ADRs in `docs/adr/`.
9. Supporting docs (`README.md`, `CONTRIBUTING.md`, `GOVERNANCE.md`, `docs/*`).

Conformance statements without linked requirement IDs and executable checks MUST
be treated as invalid.

## Stable ABI Policy (Strict SemVer)

### ABI Surface

The CLI ABI includes, at minimum:

1. command and subcommand names,
2. flags and option semantics,
3. process exit code mapping,
4. failure class semantics,
5. stdout/stderr stream contract,
6. machine-consumed output grammar,
7. canonical stdout bytes for accepted input.

### SemVer Rules

1. Patch releases MUST NOT break ABI behavior.
2. Minor releases MAY add backward-compatible capabilities.
3. Major releases are REQUIRED for breaking ABI changes.
4. ABI-impacting changes MUST update `abi_manifest.json`, tests, and
   `CHANGELOG.md` in the same change.
5. Undocumented ABI changes are prohibited.

## Failure Taxonomy Contract

1. Every user-visible failure path MUST map to a stable class.
2. Class-to-exit-code mapping MUST remain stable unless a major version change
   is declared.
3. Classification MUST be by root cause, not input source.
4. Equivalent failures on stdin and file input MUST classify identically.
5. Offset semantics MUST remain explicit and stable.

Any change to failure semantics MUST update `FAILURE_TAXONOMY.md`, tests, and
relevant requirement mappings.

## Determinism and Reproducibility

1. Canonicalization MUST be byte-deterministic for identical input and options.
2. Correctness MUST NOT depend on map iteration order, wall-clock time, locale,
   randomness, environment-specific side effects, or external services.
3. Determinism claims MUST be validated by conformance and replay-oriented
   checks.
4. Reproducible static builds SHOULD be continuously validated in CI.

## Security and Supply-Chain Requirements

1. Vulnerability reporting MUST support private disclosure (`SECURITY.md`).
2. CI and release actions MUST be pinned by immutable commit SHA.
3. Release artifacts MUST include checksums and provenance attestation.
4. Verification instructions MUST be published and maintained (`VERIFICATION.md`).
5. Security posture regressions REQUIRE explicit maintainer signoff and record.

## Bounds and DoS Resilience Requirements

1. Parser and canonicalization paths MUST enforce explicit resource bounds.
2. Bound defaults MUST be conservative and documented (`BOUNDS.md`).
3. Bound violations MUST classify predictably under stable failure classes.
4. Changes affecting bounds MUST include adversarial tests and documentation
   updates.

## Test and Verification Requirements

The test strategy MUST prove behavior, not just increase confidence.

Required layers:

1. Unit tests for local invariants and edge cases.
2. CLI/blackbox tests for ABI behavior.
3. Conformance checks for all requirement IDs.
4. Oracle/differential tests where applicable.
5. Fuzz-compatible validation for parser/token paths.
6. Determinism/replay checks for stable output properties.
7. Race and static analysis gates.

Minimum required local gates before merge:

```bash
go vet ./...
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

## Change Control Workflow

Every non-trivial change MUST follow this sequence:

1. Classify change domain:
   - normative behavior,
   - policy/profile behavior,
   - ABI/CLI behavior,
   - refactor/internal,
   - documentation only.
2. Identify impacted requirement IDs before editing.
3. Implement minimal coherent code + regression tests.
4. Update traceability artifacts in the same commit series.
5. Execute mandatory gates.
6. Update changelog and docs for user-visible behavior changes.
7. Record architectural decisions in ADRs when compatibility/security/platform
   policy is affected.

## Traceability Maintenance Requirements

When behavior changes, contributors MUST update all applicable artifacts:

- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md`
- `abi_manifest.json`
- `FAILURE_TAXONOMY.md`
- `docs/adr/*` (when decision-level impact exists)
- `CHANGELOG.md`

Requirement, implementation, and test links MUST remain parity-complete.

## Go-First Automation Policy

1. Required conformance/traceability/compatibility gates MUST run via Go tools
   and `go test`.
2. Shell scripts MAY be used only for convenience and MUST NOT become required
   conformance gates.
3. New required shell-based gates are prohibited unless explicit maintainer
   exception is granted with written rationale.

## Planning and Official Documentation Policy

1. `PLANNING/` is non-authoritative working space.
2. Normative, compatibility-relevant, or operationally binding content MUST be
   promoted to root docs or `docs/`.
3. Stale planning artifacts SHOULD be archived under dated
   `PLANNING/archive/...` or removed when superseded.
4. Duplicate authoritative statements across planning and official docs are
   prohibited.

## Exception Process

An exception MAY be granted only when all criteria are met:

1. A written rationale explains why compliance is not practical now.
2. Risk to compatibility, determinism, security, and conformance is explicitly
   analyzed.
3. Scope and expiration of the exception are defined.
4. Mitigations and follow-up plan are recorded.
5. Maintainer approval is documented.
6. ADR is added when the exception affects architecture/policy long term.

Undefined or silent exceptions are prohibited.

## Prohibited Practices

1. Silent ABI changes.
2. Behavior changes without test and traceability updates.
3. Undocumented conformance claims.
4. Runtime network egress or subprocess execution in core packages.
5. Reintroducing `encoding/json` as canonicalization engine.
6. Weakening error-class contracts without a versioning decision.
7. Introducing nondeterministic behavior into canonical paths.
8. Depending on shell-only required gates for core quality enforcement.

## Definition of Done

A change is complete only when all conditions hold:

1. Correctness validated by appropriate tests.
2. Requirement traceability is complete and passing.
3. ABI impact is either unchanged with evidence or intentionally versioned with
   documentation and migration guidance.
4. Determinism and bounds guarantees are preserved.
5. Security and release trust posture is not weakened.
6. Documentation updates ship with the behavior change.

## Official Document Map

Primary contract and policy files:

- `README.md`
- `ARCHITECTURE.md`
- `ABI.md`
- `NORMATIVE_REFERENCES.md`
- `SPECIFICATION.md`
- `CONFORMANCE.md`
- `THREAT_MODEL.md`
- `RELEASE_PROCESS.md`
- `CONTRIBUTING.md`
- `GOVERNANCE.md`
- `SECURITY.md`
- `CHANGELOG.md`
- `abi_manifest.json`
- `FAILURE_TAXONOMY.md`
- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md`
- `docs/README.md`
- `docs/adr/`
- `VERIFICATION.md`
- `BOUNDS.md`

## Final Principle

Infrastructure trust is earned through repeatable evidence over time.

In this project, correctness, compatibility, determinism, and auditability are
product features, not optional engineering preferences.
