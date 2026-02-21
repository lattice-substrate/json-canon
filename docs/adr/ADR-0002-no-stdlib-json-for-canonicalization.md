# ADR-0002: No `encoding/json` for Canonicalization Core

- ADR ID: ADR-0002
- Date: 2026-02-21
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: PARSE-*, CANON-*, ECMA-FMT-*

## Context

The standard library JSON package is not an RFC 8785 canonicalizer and does not
provide the exact strict-domain and canonical-emission guarantees required here.

## Decision

Canonicalization behavior is implemented by project-owned parser/tokenizer,
number formatter, and serializer paths. `encoding/json` is not used as the
canonicalization engine.

## Rationale

- Prevents silent coercions and behavioral drift from generic JSON tooling.
- Preserves explicit control over UTF-16 sorting and ECMA-compatible number
  serialization.
- Supports audit-grade traceability from spec clauses to code and tests.

## Consequences

- More code is maintained in-repo.
- Conformance/oracle test rigor is mandatory to preserve confidence.

## Alternatives Considered

- Wrapping stdlib behavior with patches: rejected due to insufficient control of
  canonicalization-critical semantics.
