# Spec and Profile Boundaries

## Purpose

`lattice-canon` combines:

1. RFC 8785 canonical JSON serialization behavior.
2. A stricter input-domain profile for governed infrastructure bytes.

These are different responsibilities and are intentionally separated.

## Standards used

- RFC 8785: canonical serializer rules.
- RFC 8259: JSON grammar baseline.
- RFC 7493 (I-JSON): profile constraints (duplicate names, scalar validity).
- RFC 3629: UTF-8 validity in envelope handling.
- ECMA-262 Number formatting for JCS number serialization.

## What is RFC 8785 behavior in this project

Implemented in `jcs/serialize.go` and `jcsfloat/jcsfloat.go`:

- Deterministic canonical serialization (no whitespace).
- String escaping rules per RFC 8785.
- UTF-16 code-unit key ordering.
- ECMAScript-compatible number text formatting.

Important: canonicalization is value-based, not byte-preserving. Distinct lexical inputs can map to the same canonical bytes.

## What is profile behavior in this project

Implemented mainly in `jcstoken/token.go` and `gjcs1/gjcs1.go`:

- Reject duplicate object keys.
- Reject lone surrogates and Unicode noncharacters.
- Reject `-0` tokens (project profile decision).
- Reject underflow-to-zero non-zero tokens (for single representable encoding discipline).
- Enforce GJCS1 envelope constraints (single trailing LF, no BOM/CR, valid UTF-8, etc.).

## `-0` policy (explicit)

- JCS number formatting would normalize `-0` to `0` because ECMAScript formatting does so.
- This project profile rejects `-0` at parse time instead of allowing normalization.
- Result: governed inputs containing `-0` fail validation rather than silently changing representation.

This policy is implemented as profile strictness, not as a claim that RFC 8785 itself forbids `-0`.

## Threat-model implication

If your requirement is exact source-byte identity, do not use canonicalization as the identity primitive. Use one of these approaches:

1. Hash/sign raw bytes directly.
2. Enforce canonical-form-only ingestion (this project does that for GJCS1 via `verify`).

Canonicalization provides deterministic bytes per JSON value, not preservation of original token spelling.
