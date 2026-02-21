# UTF‑8 Validation and Character Constraints

**Status:** Draft  
**Normative:** RFC 3629, RFC 7493, RFC 8259

## 1. Requirements
- JSON text exchanged between systems “MUST be encoded using UTF‑8” (RFC 8259 §8.1).  
  Source: https://datatracker.ietf.org/doc/html/rfc8259
- I‑JSON messages MUST be UTF‑8 (RFC 7493 §2.1).  
  Source: https://www.rfc-editor.org/rfc/rfc7493.html
- UTF‑8 prohibits encoding surrogate code points U+D800..U+DFFF (RFC 3629).  
  Source: https://www.rfc-editor.org/rfc/rfc3629
- UTF‑8 decoders MUST protect against decoding invalid sequences; invalid decoding may have security consequences (RFC 3629 §4, §10).  
  Source: https://www.rfc-editor.org/rfc/rfc3629

## 2. Implementation rules
### 2.1 Validate full UTF‑8 stream
Reject any invalid UTF‑8 byte sequence in:
- the JSON text as a whole (outside strings, only ASCII is legal in JSON syntax),
- decoded string content that is directly encoded as UTF‑8 bytes inside a string (i.e., bytes >= 0x80).

### 2.2 Reject UTF‑8 surrogate code points
Even if the byte sequence is structurally valid UTF‑8, reject any decoded scalar value in U+D800..U+DFFF (RFC 3629 forbids such character numbers).

### 2.3 Do not perform Unicode normalization
RFC 8785 specifies JCS does not consider Unicode normalization; components MUST preserve Unicode string data “as is”.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 3. Verification
- Unit tests covering overlong sequences (e.g., `C0 80`), invalid continuation bytes, truncated sequences, and surrogate UTF‑8 sequences (`ED A0 80` .. `ED BF BF`).
- Corpus tests include raw-byte invalid UTF‑8 in strings and in the top-level JSON text.

## References
- RFC 3629: https://www.rfc-editor.org/rfc/rfc3629
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 8785 note on normalization: https://www.rfc-editor.org/rfc/rfc8785
