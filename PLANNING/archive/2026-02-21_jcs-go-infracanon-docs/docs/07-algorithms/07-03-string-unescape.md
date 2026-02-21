# String Decoding, Escapes, and I‑JSON Forbidden Code Points

**Status:** Draft  
**Normative:** RFC 8259, RFC 7493, RFC 8785

## 1. JSON string decoding (RFC 8259)
RFC 8259 defines JSON strings, required escapes, and `\uXXXX` escape forms (RFC 8259 §7–§8).  
Source: https://datatracker.ietf.org/doc/html/rfc8259

### 1.1 Escape handling
Implement:
- standard single-character escapes (`\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t`)
- `\u` followed by 4 hex digits

### 1.2 Surrogate pairs in `\u` escapes
RFC 8259’s ABNF allows unpaired surrogates, but warns that behavior is unpredictable across implementations (RFC 8259 §8.2).  
I‑JSON tightens this: unpaired surrogates are forbidden in strings and names.  
Sources:
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html

Implement:
- If a `\uXXXX` value is a high surrogate, it MUST be followed by another `\uYYYY` low surrogate; otherwise reject.
- Decode surrogate pairs into a single Unicode scalar value.

## 2. I‑JSON forbidden code points
RFC 7493 §2.1:
- Names and string values MUST NOT include code points that identify Surrogates or Noncharacters, whether encoded directly in UTF‑8 or escaped.  
Source: https://www.rfc-editor.org/rfc/rfc7493.html

Implement:
- reject any scalar in surrogate range (U+D800..U+DFFF)
- reject any Unicode Noncharacter (per Unicode definition referenced by RFC 7493)

## 3. Canonical string emission constraints (RFC 8785)
RFC 8785 requires that parsed JSON string data MUST NOT be altered during subsequent serializations and explicitly excludes Unicode normalization.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 4. Verification
- Unit tests for each escape sequence.
- Invalid cases: unpaired surrogate (`"\uDEAD"`), invalid hex digits, invalid UTF‑8 bytes, forbidden noncharacter scalars.
- Round-trip invariant: decode → re-emit must preserve the decoded scalar sequence exactly.

## References
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
