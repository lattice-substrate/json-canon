# V8 Differential Testing for Number Serialization

**Status:** Draft  
**Normative driver:** RFC 8785 recommends validating a JCS number serializer against V8 or a large IEEE‑754 sample file.

## 1. Why V8
RFC 8785 states that the number serialization algorithm is not included and that Google’s implementation in V8 may serve as a reference; Ryu is another compatible reference.  
Source: RFC 8785 §3.2.2. https://www.rfc-editor.org/rfc/rfc8785

## 2. Test method
1. Generate random IEEE‑754 binary64 bit patterns.
2. Convert to a JS Number and serialize with `JSON.stringify(n)`.
3. Compare to the Go canonicalizer’s number serialization for the same binary64 value.
4. Exclude NaN/Infinity inputs (RFC 8259 disallows them as numbers).

## 3. Evidence
The harness emits a file of tuples:
- `u64_hex` (bit pattern)
- `expected_string` (V8 JSON output)
- optional flags (e.g., boundary cases)

## 4. Tooling note
The generator script in `tools/gen_v8_numbers.js` requires Node.js to run V8. The canonicalizer itself remains dependency-free.

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 8259 number restrictions: https://datatracker.ietf.org/doc/html/rfc8259
