# Architecture

## Purpose

This document defines the architecture contract for `json-canon`.
It is the system-level source of truth for component boundaries, data flow,
determinism properties, and long-term compatibility constraints.

## Scope

This architecture covers:

- parsing and validation domain,
- canonical serialization domain,
- CLI/runtime behavior and failure mapping,
- conformance and evidence system integration,
- release-time build and trust constraints.

## Architectural Goals

1. Correct RFC 8785 canonicalization and verification.
2. Strict-domain JSON acceptance with explicit policy constraints.
3. Deterministic output and stable machine-facing behavior.
4. Audit-grade traceability from requirements to code and tests.
5. Minimal operational attack surface for infra deployment.

## Layered System Model

`json-canon` is intentionally split into small packages with one-way dependencies.

| Layer | Package | Responsibility | Must Not Depend On |
|------|---------|----------------|--------------------|
| L5 | `cmd/jcs-canon` | CLI argument handling, input selection, process exits | parsing internals other than exported APIs |
| L4 | `jcs` | Canonical serialization (`Value` -> canonical bytes) | CLI-specific code, OS-level side effects |
| L3 | `jcstoken` | Strict parser/tokenizer and profile checks (`bytes` -> `Value`) | CLI concerns, networking, subprocesses |
| L2 | `jcsfloat` | ECMA-262-compatible binary64 to string formatting | CLI/runtime dependencies |
| L1 | `jcserr` | Stable error classes and exit code mapping | higher-level logic |

Dependency direction is inward only (L5 -> L1).

## Primary Execution Flows

### Canonicalize Flow

1. Read bounded input bytes from stdin or file.
2. Validate UTF-8 and JSON grammar.
3. Enforce I-JSON and project profile restrictions.
4. Build internal typed value tree.
5. Serialize using canonical RFC 8785 rules.
6. Write canonical bytes to stdout.

### Verify Flow

1. Execute canonicalize flow in-memory.
2. Compare canonical output bytes with original input bytes.
3. Return success only on byte-identical equality.

## Trust Boundaries

Input from stdin/file is untrusted.

Mandatory boundary controls:

1. UTF-8 validation before semantic processing.
2. Grammar and profile checks before canonicalization.
3. Strict resource bounds on size, depth, and cardinality.
4. Stable classed errors for rejected input and internal faults.

## Determinism Model

Determinism is an architectural property, not a test-only property.

1. Output is a pure function of input bytes and options.
2. No wall-clock, RNG, locale, network, or subprocess dependence in runtime path.
3. Object member order is derived from UTF-16 code-unit sorting only.
4. Numeric emission follows ECMA-compatible algorithmic rules, not runtime-dependent formatting shortcuts.

## Number Formatting Subsystem

`jcsfloat` is a dedicated subsystem to avoid stdlib behavior drift and generic
JSON formatter limitations.

Subsystem invariants:

1. NaN and Infinity are rejected by profile.
2. `-0` is normalized to `0` at formatting level; lexical negative zero is rejected by parser policy.
3. Shortest round-tripping decimal representation is required.
4. Branch behavior around 1e-6 and 1e21 boundaries follows ECMA rules.

## Failure Architecture

All externally visible failures are represented by stable classes
(`FAILURE_TAXONOMY.md`) and mapped to stable exit codes.

Architecture rules:

1. Classify by root cause, not by input source.
2. Preserve semantic class through wrapping/layer boundaries.
3. Keep machine-level semantics stable even if message text evolves.

## Compatibility Boundaries

The stable ABI boundary includes:

- CLI commands and flags,
- stream contracts (`stdout` vs `stderr`),
- exit code mapping and failure classes,
- canonical output bytes for accepted input.

Breaking this boundary requires a major version and migration documentation.

## Security and Runtime Surface

Runtime surface is intentionally narrow:

1. Linux-only supported runtime.
2. Static release binary (`CGO_ENABLED=0`).
3. No outbound network calls in core runtime.
4. No subprocess execution in core runtime.

## Architecture Change Policy

Changes that affect boundaries, invariants, or compatibility contracts MUST:

1. update this file,
2. update affected requirement registries and matrix,
3. add/update ADR under `docs/adr/`,
4. include regression tests and conformance evidence.

## Architecture Review Checklist

A change is architecture-safe only if all are true:

1. Layering remains one-way.
2. Determinism constraints are preserved.
3. Failure semantics remain class-stable.
4. ABI surface impact is explicit and versioned.
5. Traceability links remain complete.
