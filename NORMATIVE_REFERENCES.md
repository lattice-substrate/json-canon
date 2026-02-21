# Normative References

## Purpose

This document defines the external normative references and interpretation
policy used by `json-canon`.

## External Normative References

1. RFC 8785 - JSON Canonicalization Scheme (JCS)
2. RFC 8259 - JSON Data Interchange Format
3. RFC 7493 - I-JSON Message Format
4. RFC 3629 - UTF-8 Encoding
5. ECMA-262 - ECMAScript `Number::toString`
6. IEEE 754 - Floating-point arithmetic semantics

## Internal Normative Artifacts

1. `REQ_REGISTRY_NORMATIVE.md`
2. `REQ_REGISTRY_POLICY.md`
3. `REQ_ENFORCEMENT_MATRIX.md`
4. `standards/CITATION_INDEX.md`

## Interpretation Order

When interpretation is ambiguous, use this precedence:

1. External normative spec clauses.
2. Requirement registries and citation mappings.
3. Accepted ADR decisions.
4. Other project documentation.

## Conformance Interpretation Rules

1. Requirement IDs are the unit of conformance.
2. No conformance claim is valid without executable test evidence.
3. Registry/citation/matrix drift is a conformance failure.
4. Policy requirements MUST NOT be represented as external normative mandates.

## Number Semantics Clarification

Numeric canonicalization uses ECMA-262 `Number::toString` behavior over IEEE 754
binary64 values, with project policy constraints for lexical negative zero,
overflow, and underflow rejection.

## UTF and String Clarification

1. Input validity is enforced on UTF-8 byte streams.
2. Key sort order is based on UTF-16 code units of raw property names.
3. No Unicode normalization is applied.

## Maintenance Rule

Any change to normative interpretation MUST update:

1. affected registry entries,
2. `standards/CITATION_INDEX.md`,
3. conformance tests,
4. relevant ADR(s).
