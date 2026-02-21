# ADR-0001: Strict Domain, Fail-Closed Canonicalization

- ADR ID: ADR-0001
- Date: 2026-02-21
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: PARSE-*, IJSON-*, CLI-EXIT-003

## Context

This tool is used in cryptographic and integrity-sensitive workflows where
silent normalization is unacceptable.

## Decision

The canonicalizer enforces strict RFC 8259 + I-JSON acceptance and rejects any
out-of-domain input with deterministic failure classes and exit codes.

## Rationale

- Prevents ambiguous signing/hashing inputs.
- Preserves interoperability and auditability.
- Aligns behavior with explicit conformance requirements.

## Consequences

- Non-JSON and permissive JSON-like inputs are rejected.
- Integrators must treat validation failures as protocol-level rejection.

## Alternatives Considered

- Lenient parsing mode in same API: rejected due to misuse risk and weakened
  security boundary semantics.
