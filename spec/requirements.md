# Requirement Catalog

This catalog defines normative requirement IDs for offline conformance testing.

Every requirement ID in this file MUST have at least one automated check in `conformance/harness_test.go`.

| Requirement ID | Scope | Requirement |
| --- | --- | --- |
| REQ-ABI-001 | CLI ABI | `canonicalize` and `verify` commands are functional. |
| REQ-ABI-002 | CLI ABI | Missing command exits with code `2`. |
| REQ-ABI-003 | CLI ABI | Unknown command exits with code `2`. |
| REQ-ABI-004 | CLI ABI | Internal output-write failures exit with code `10`. |
| REQ-CLI-001 | CLI | Unknown options are rejected with exit code `2`. |
| REQ-CLI-002 | CLI | Multiple positional inputs are rejected with exit code `2`. |
| REQ-CLI-003 | CLI | File and stdin input paths are behaviorally equivalent. |
| REQ-CLI-004 | CLI | `verify` success emits `ok` on stderr when not quiet. |
| REQ-CLI-005 | CLI | `verify --quiet` success does not emit `ok`. |
| REQ-CLI-006 | CLI | Successful `canonicalize` writes canonical bytes to stdout only. |
| REQ-RFC8259-001 | RFC 8259 | Leading-zero numbers are rejected. |
| REQ-RFC8259-002 | RFC 8259 | Object trailing comma is rejected. |
| REQ-RFC8259-003 | RFC 8259 | Array trailing comma is rejected. |
| REQ-RFC8259-004 | RFC 8259 | Unescaped control characters in strings are rejected. |
| REQ-RFC8259-005 | RFC 8259 | Top-level scalar values are accepted. |
| REQ-RFC8259-006 | RFC 8259 | Insignificant surrounding whitespace is accepted. |
| REQ-RFC8259-007 | RFC 8259 | Invalid literals are rejected. |
| REQ-RFC3629-001 | RFC 3629 | Invalid UTF-8 byte sequences are rejected. |
| REQ-RFC3629-002 | RFC 3629 | Overlong UTF-8 encodings are rejected. |
| REQ-RFC7493-001 | RFC 7493 | Duplicate object keys are rejected. |
| REQ-RFC7493-002 | RFC 7493 | Duplicate keys after escape decoding are rejected. |
| REQ-RFC7493-003 | RFC 7493 | Lone high surrogate escapes are rejected. |
| REQ-RFC7493-004 | RFC 7493 | Lone low surrogate escapes are rejected. |
| REQ-RFC7493-005 | RFC 7493 | Unicode noncharacters are rejected. |
| REQ-NUM-001 | Numeric profile | Numbers overflowing binary64 are rejected. |
| REQ-NUM-002 | Numeric profile | `-0` lexical token is rejected. |
| REQ-NUM-003 | Numeric profile | Non-zero underflow-to-zero tokens are rejected. |
| REQ-BOUND-001 | Resource bounds | Maximum nesting depth is enforced. |
| REQ-BOUND-002 | Resource bounds | Maximum object members is enforced. |
| REQ-BOUND-003 | Resource bounds | Maximum array elements is enforced. |
| REQ-BOUND-004 | Resource bounds | Maximum decoded string byte length is enforced. |
| REQ-BOUND-005 | Resource bounds | Maximum number token length is enforced. |
| REQ-BOUND-006 | Resource bounds | Maximum JSON value count is enforced. |
| REQ-RFC8785-001 | RFC 8785 | Canonical output removes insignificant whitespace. |
| REQ-RFC8785-002 | RFC 8785 | Object key ordering uses UTF-16 code-unit order. |
| REQ-RFC8785-003 | RFC 8785 | Control character escaping is exact per RFC 8785. |
| REQ-RFC8785-004 | RFC 8785 | Solidus `/` is not escaped in canonical output. |
| REQ-RFC8785-005 | RFC 8785 | `\u00xx` hex escapes use lowercase hex digits. |
| REQ-RFC8785-006 | RFC 8785 | Object sorting is applied recursively. |
| REQ-RFC8785-007 | RFC 8785 | Top-level scalar canonicalization is supported. |
| REQ-RFC8785-008 | RFC 8785 | `verify` rejects parseable non-canonical key ordering. |
| REQ-RFC8785-009 | RFC 8785 | `verify` rejects parseable non-canonical whitespace. |
| REQ-ECMA-001 | ECMAScript | Base pinned number oracle vectors match exactly. |
| REQ-ECMA-002 | ECMAScript | Deterministic stress oracle vectors match exactly. |
| REQ-ECMA-003 | ECMAScript | Critical threshold/boundary constants format exactly. |
| REQ-DET-001 | Determinism | Repeated canonicalization of same input is byte-identical. |
| REQ-DET-002 | Determinism | Parse->serialize->parse->serialize is idempotent. |
| REQ-BUILD-001 | Build | Static-friendly deterministic build command succeeds offline. |
