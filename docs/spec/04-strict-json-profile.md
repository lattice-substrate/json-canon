# 04. Strict JSON Profile

This chapter defines profile constraints enforced before canonicalization.

## 4.1 Grammar

Input MUST satisfy RFC 8259 JSON grammar.

Examples of invalid syntax include:

- trailing commas,
- leading zeros in number integer part,
- unescaped control characters in strings.

## 4.2 Object Member Uniqueness

Duplicate member names within the same object MUST be rejected.

- Comparison MUST occur after escape decoding.
- Comparison unit is decoded Unicode scalar sequence.

Same key text at different nesting levels MAY appear.

## 4.3 Unicode Scalar Validity

Strings and member names MUST reject:

- lone surrogate code points,
- Unicode noncharacters (`U+FDD0..U+FDEF`, and `U+xFFFE/U+xFFFF` for planes 0..16).

Valid surrogate pairs in `\\uXXXX\\uXXXX` escapes MUST decode to supplementary scalars.

## 4.4 Numeric Profile Constraints

- `-0` tokens MUST be rejected.
- Non-finite numeric parse results MUST be rejected.
- Non-zero numeric tokens that parse to `0` due to underflow MUST be rejected.

These are profile constraints and are stricter than base RFC 8259 acceptance.

## 4.5 Resource Bounds

Parser bounds MUST be enforced:

- Maximum nesting depth default: `1000`.
- Maximum input size default: `64 MiB`.

Implementations MAY expose configuration for these limits.
