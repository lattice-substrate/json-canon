# Analysis: Go `encoding/json` vs Audit-Grade JCS Requirements

**Status:** Draft  
**Scope:** explain why Go stdlib JSON is not appropriate as a foundation for strict JCS.

## 1. JCS requires exact canonical emission rules
RFC 8785 requires:
- no emitted whitespace,
- UTF‑16 code unit property sorting,
- UTF‑8 output encoding,
- ECMAScript-compatible primitive serialization,
- no alteration of parsed string data, and “preserve Unicode string data as is”.  
Source: https://www.rfc-editor.org/rfc/rfc8785

Go’s `encoding/json` is designed for general JSON encoding/decoding and does not claim to implement RFC 8785.

## 2. Invalid UTF‑8 coercion (silent normalization)
The Go `encoding/json` encoder states that since Go 1.2, Marshal “coerces the string to valid UTF‑8 by replacing invalid bytes with the Unicode replacement rune U+FFFD.”  
Source (Go source code comments): https://go.googlesource.com/go/+/refs/tags/go1.21.5/src/encoding/json/encode.go

This is incompatible with strict, audited canonicalization, because it changes the data model and therefore changes cryptographic hashes/signatures.

## 3. HTML escaping default (not JCS)
Go’s encoder historically escapes certain characters for safe embedding in HTML contexts. While it can be disabled via `Encoder.SetEscapeHTML(false)`, relying on this behavior increases risk of accidental mismatch in cryptographic contexts.

(See `encoding/json` package documentation.)  
Source: https://pkg.go.dev/encoding/json

## 4. Key ordering and number rules
JCS requires UTF‑16 code-unit sorting and ECMAScript-compatible number serialization; stdlib JSON marshaling is not specified to do either.  
Source: RFC 8785 sorting rules and number rules: https://www.rfc-editor.org/rfc/rfc8785

## 5. Conclusion
For audited cryptographic usage, `encoding/json` is inappropriate as a canonicalizer because it:
- is not an RFC 8785 implementation,
- performs coercions on invalid UTF‑8 strings,
- and lacks I‑JSON enforcement and UTF‑16 sorting semantics.

The new canonicalizer must therefore include its own strict tokenizer, validator, sorter, and emitter.

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- Go `encoding/json` source: https://go.googlesource.com/go/+/refs/tags/go1.21.5/src/encoding/json/encode.go
- Go `encoding/json` docs: https://pkg.go.dev/encoding/json
