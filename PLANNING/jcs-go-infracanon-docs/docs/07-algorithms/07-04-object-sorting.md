# Object Member Ordering and Duplicate Detection

**Status:** Draft  
**Normative:** RFC 8785, RFC 7493

## 1. Duplicate member names (I‑JSON)
RFC 7493 §2.3: objects MUST NOT have members with duplicate names after processing escapes; duplicates are identical Unicode sequences.  
Source: https://www.rfc-editor.org/rfc/rfc7493.html

Implementation rule:
- Decode each member name to a Unicode scalar sequence.
- Compare decoded sequences for equality; if any duplicates exist, reject.

## 2. Sorting algorithm (RFC 8785)
RFC 8785 §3.2.3 defines object property sorting:
- applied to property name strings in their raw (unescaped) form
- property names are formatted as arrays of UTF‑16 code units
- sort by pure value comparisons (unsigned code units), independent of locale
- shorter string precedes longer if equal prefix  
Source: https://www.rfc-editor.org/rfc/rfc8785

Implementation rule:
- Convert each decoded key to UTF‑16 code units.
- Sort using unsigned comparisons by code unit.

## 3. Edge cases
- Keys containing characters outside ASCII MUST still sort per UTF‑16 code unit order, not UTF‑8 byte order.
- Keys containing newline or other control characters must sort on their actual code point value (example: U+000A).

## 4. Verification vectors
RFC 8785 provides a concrete sorting test object and the expected order (see RFC 8785 §3.2.3).  
Source: https://www.rfc-editor.org/rfc/rfc8785

The **corpus/rfc8785/** directory includes this test vector.

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
