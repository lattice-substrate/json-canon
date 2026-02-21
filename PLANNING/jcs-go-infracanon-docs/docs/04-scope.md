# Scope and Non-Scope

**Status:** Draft

## In scope
- Strict RFC 8259 JSON parsing (no extensions).
- I‑JSON enforcement per RFC 7493 where applicable to JCS.
- RFC 8785 canonicalization:
  - no emitted whitespace,
  - UTF‑16 code-unit sorting,
  - UTF‑8 output bytes,
  - ECMAScript-compatible primitive serialization (including -0 → 0),
  - recursive canonicalization of objects and arrays.

## Out of scope
- Permissive JSON variants (JSON5, comments, trailing commas).
- Preserving the original lexical representation (this is canonicalization, not formatting).
- Unicode normalization (explicitly not performed; see RFC 8785 note).
- Arbitrary precision numeric semantics; I‑JSON recommends encoding such values as strings.

## Compatibility goals
- Outputs are invariant across Linux distros and architectures for the same input bytes.
- Public API and error code registry are stable across minor versions.

## Stability / ABI policy
- The Go package API is a “stable interface” (semantics stable; actual binary ABI is not guaranteed across Go toolchains).
- Error codes and their meaning are stable once introduced.
