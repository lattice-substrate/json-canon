# CLAUDE.md

## Purpose

This document defines the engineering bar for **infrastructure-grade, stable-ABI, decades-lived tooling**.

It is not a feature roadmap. It is a contract for how work is designed, implemented, validated, released, and maintained.

If there is any conflict between speed and this bar, this bar wins.

## Scope

This policy applies to:

1. Public command-line interfaces and machine-consumed outputs.
2. Failure classes, exit semantics, and diagnostics consumed by automation.
3. Deterministic data transforms and conformance claims.
4. Release artifacts and supply-chain trust.
5. Long-term compatibility and maintenance governance.

## Infrastructure-Grade Definition

A tool is infrastructure-grade only if all of the following are true:

1. Behavior is specified and testable, not implicit.
2. Public interfaces are versioned and compatibility-governed.
3. Conformance claims are tied to authoritative references.
4. Failures are classified, stable, and machine-actionable.
5. Builds and outputs are deterministic under controlled conditions.
6. Releases are auditable, reproducible, and authenticity-verifiable.
7. Governance is durable beyond any single maintainer.

If any item above is missing, the tool is not infrastructure-grade.

## Stable ABI Contract

Stable ABI means compatibility for machine consumers, not human convenience.

The ABI surface must be explicitly enumerated and treated as versioned API:

1. Command names and subcommands.
2. Flags and flag semantics.
3. Exit code classes and mapping rules.
4. Output channel contract (`stdout` vs `stderr`).
5. Output grammar for machine-parsed text.
6. Error class identifiers and invariants.

### ABI Rules

1. Additive changes are allowed only if backward compatible.
2. Behavioral changes to existing contract require major version unless explicitly pre-declared as unstable.
3. Accidental behavior relied on by users becomes part of ABI once documented or widely consumed.
4. ABI changes must be accompanied by contract tests and release notes.

### ABI Anti-Patterns

1. Changing exit behavior without major version.
2. Rewording machine-parsed output without compatibility guard.
3. Treating stderr text as unbounded freeform when tooling consumes it.
4. Introducing hidden environment-dependent behavior.

## Compatibility Policy

Strict Semantic Versioning is required.

1. Major: breaking ABI or compatibility contract changes.
2. Minor: backward-compatible capability additions.
3. Patch: bug fixes only, no compatibility surface changes.

Compatibility must be measured by automated compatibility tests, not intuition.

## Standards and Conformance Discipline

Conformance claims must be evidence-backed.

1. Every normative requirement must map to an authoritative source clause.
2. Every requirement must map to implementation symbols.
3. Every requirement must map to executable tests.
4. Registry and enforcement matrix must remain parity-complete.
5. No "compliant" claim without source-cited proof.

### Source Hierarchy

Use primary authorities first:

1. Official specifications and standards bodies.
2. Normative language in standards text.
3. Official language/runtime documentation.
4. Secondary sources only for context, never as sole authority.

## Failure Taxonomy and Diagnostics Contract

Error handling is part of ABI.

1. Every failure maps to a stable class.
2. Classes map to stable process exit codes.
3. Classification is by root cause, not input source.
4. Equivalent failures across input modes must classify identically.
5. Offset/location semantics must be explicit and stable.

Diagnostics should be layered:

1. Stable machine-level class/exit semantics.
2. Human-readable message that can evolve within compatibility policy.
3. Optional wrapped causes for debugging fidelity.

## Determinism and Reproducibility

Infrastructure tooling must avoid hidden nondeterminism.

1. Canonical outputs must be byte-stable for identical input and config.
2. Build determinism requirements must be documented and tested.
3. Nondeterminism sources (time, random, map iteration side effects) must be controlled or forbidden on critical paths.
4. Determinism claims must be validated by replay and idempotence tests.

Reproducibility is a release gate, not a best effort.

## Security and Supply-Chain Requirements

Security posture must include both code and release system.

1. Vulnerability disclosure path must be private and actionable.
2. Security policy must define support window and response targets.
3. CI and release dependencies must be pinned or integrity-controlled.
4. Release artifacts must include checksums and authenticity mechanism.
5. Provenance and verification instructions must be published.

## Performance, Bounds, and DoS Resilience

Low-level infra tools must defend against adversarial inputs.

1. Resource bounds must exist for depth, size, and cardinality risks.
2. Bound defaults must be conservative and documented.
3. Bound violations must fail predictably with stable classification.
4. Complexity hotspots must be tested with adversarial workloads.
5. Performance regressions must be tracked with benchmarks.

## Test Strategy Requirements

Test strategy must prove behavior, not just raise confidence.

Required layers:

1. Unit tests for local invariants and edge cases.
2. Blackbox CLI tests for public contract behavior.
3. Conformance tests for requirement IDs.
4. Differential/oracle tests where available.
5. Fuzz tests for parser/decoder and adversarial paths.
6. Determinism/replay and idempotence tests.
7. Race and static analysis gates.

Test evidence quality rules:

1. Every bug fix adds a regression test.
2. Every compatibility-sensitive behavior has explicit contract tests.
3. Every normative requirement has at least one executable check.
4. Flaky tests are treated as release blockers until root-cause is resolved.

## CI and Release Gate Model

Minimum CI gates:

1. Lint/static checks.
2. Full test suite.
3. Race tests.
4. Conformance suite.
5. Requirement registry-matrix parity checks.
6. Determinism/reproducibility check.

Release gates:

1. All CI gates green on supported platform/version matrix.
2. Changelog and compatibility notes complete.
3. Signed or authenticity-verifiable artifacts published.
4. Release verification instructions validated.

No manual bypass for failed conformance, compatibility, or reproducibility gates.

## Documentation and Evidence Standards

Documentation must be operationally useful.

Required documents:

1. README (consumer contract and supported behavior).
2. SECURITY policy (contact and process).
3. CONTRIBUTING guide (engineering gates and expectations).
4. CHANGELOG (versioned behavioral history).
5. Requirement registries and enforcement matrix.
6. Audit artifacts with evidence summaries.

Documentation quality rules:

1. No stale references to removed architecture.
2. No vague compatibility language.
3. No conformance claim without linked evidence.
4. Contract docs updated in same change as behavior.

## Governance and Longevity Requirements

Decades-lived tooling cannot rely on tribal memory.

1. Decisions with compatibility impact must be recorded.
2. Review standards must require technical justification, not preference.
3. Ownership model must survive maintainer turnover.
4. Release process must be repeatable by a new maintainer.
5. Archive and supersession policy must exist for plans and audits.

## Engineering Decision Rules

When uncertain, choose the option that:

1. Preserves compatibility.
2. Increases explicitness of contract.
3. Improves machine verifiability.
4. Reduces hidden complexity and nondeterminism.
5. Strengthens evidence and auditability.

## Prohibited Practices

1. Silent behavior changes in public interfaces.
2. "Should be fine" reasoning without tests and evidence.
3. Untested conformance claims.
4. Release artifacts without verification path.
5. Compatibility assumptions not encoded in CI.
6. Relying on undocumented runtime quirks for correctness.

## Pull Request Acceptance Checklist

A change is not ready unless all are true:

1. Public contract impact assessed and documented.
2. Requirement mappings updated (if behavior changes).
3. Tests added/updated for all changed guarantees.
4. CI gates pass, including compatibility-sensitive checks.
5. Release notes/changelog updated when user-facing behavior changes.
6. No new unexplained nondeterminism or safety risk introduced.

## Final Principle

Infrastructure trust is earned by repeatable evidence over time.

Correctness, compatibility, and verifiability are product features.
They are not optional engineering preferences.
