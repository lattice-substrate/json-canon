# Canonical JSON Emission Rules

**Status:** Draft  
**Normative:** RFC 8785, RFC 8259

## 1. Whitespace
RFC 8785: “Whitespace between JSON tokens MUST NOT be emitted.”  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 2. UTF‑8 output
RFC 8785 requires the canonical output to be encoded in UTF‑8.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 3. String escaping
Emit strings as JSON strings per RFC 8259, escaping:
- quotation mark (`"`)
- reverse solidus (`\`)
- control chars U+0000..U+001F
Other code points are emitted directly as UTF‑8 bytes.  
Source: https://datatracker.ietf.org/doc/html/rfc8259

Note: Do not perform HTML escaping or any other non-JSON escaping. This is a common mismatch with web-focused serializers.

## 4. Object and array emission
- Arrays preserve element order; elements are emitted without extra whitespace.
- Objects emit members in sorted order (RFC 8785 sorting rules) with `:` and `,` separators only.

## 5. Verification
- Idempotence: canonicalize(canonicalize(x)) == canonicalize(x).
- Corpus includes examples with control characters and non-ASCII characters.

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
