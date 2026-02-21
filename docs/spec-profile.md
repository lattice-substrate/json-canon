# Spec and Profile Boundaries

## Purpose

`jcs-canon` provides:

1. RFC 8785 canonical JSON serialization.
2. A strict parse profile for infrastructure safety and replay stability.

## Standards used

- RFC 8785: canonical serializer rules.
- RFC 8259: JSON grammar baseline.
- RFC 7493 (I-JSON): duplicate-name and scalar constraints.
- RFC 3629: UTF-8 validity.
- ECMA-262 Number formatting for JCS number serialization.

## RFC 8785 behavior in this project

Implemented in `jcs/serialize.go` and `jcsfloat/jcsfloat.go`:

- Deterministic canonical serialization (no whitespace).
- String escaping rules per RFC 8785.
- UTF-16 code-unit key ordering.
- ECMAScript-compatible number text formatting.

Canonicalization is value-based, not byte-preserving. Distinct lexical inputs can map to one canonical output.

## Strict profile behavior in this project

Implemented mainly in `jcstoken/token.go`:

- Reject duplicate object keys after escape decoding.
- Reject lone surrogates and Unicode noncharacters.
- Reject `-0` numeric tokens.
- Reject non-zero tokens that underflow to IEEE 754 zero.
- Enforce bounded input/depth/value/member/element/string/number sizes.

## Threat-model implication

If your requirement is original byte identity, canonicalization is the wrong primitive. Use raw-byte hashing/signing instead.
