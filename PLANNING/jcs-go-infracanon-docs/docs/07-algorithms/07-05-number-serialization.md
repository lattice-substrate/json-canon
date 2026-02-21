# Number Serialization: ECMAScript Compatibility and Audit Control

**Status:** Draft  
**Normative:** RFC 8785, RFC 8259, RFC 7493

## 1. Requirements
- Inputs MUST be valid RFC 8259 numbers (lexically).  
  Source: https://datatracker.ietf.org/doc/html/rfc8259
- Inputs SHOULD be within IEEE‑754 binary64 magnitude/precision per I‑JSON guidance (RFC 7493 §2.2).  
  Source: https://www.rfc-editor.org/rfc/rfc7493.html
- JCS output number serialization MUST be compatible with ECMAScript JSON serialization rules (RFC 8785).  
  Source: https://www.rfc-editor.org/rfc/rfc8785

RFC 8785 notes the number serializer is complex and does not include the full algorithm; it cites V8 and Ryu as references and provides IEEE‑754 sample mappings in Appendix B.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## 2. Engineering approach for infrastructure-grade determinism
To avoid “stdlib drift” and to maximize auditability:

1. Parse numbers lexically as RFC 8259 tokens (no `ParseFloat` on arbitrary strings).
2. Convert to a binary64 value under explicit rules:
   - reject overflow to Infinity
   - reject NaN (should be impossible if lexical rules are enforced and conversion is guarded)
3. Serialize using an algorithm under project control that matches the ECMAScript mapping:
   - -0 serializes as `0`
   - exponent formatting and thresholds match ECMAScript/JCS behavior
4. Lock behavior with:
   - RFC 8785 Appendix B samples (golden tests),
   - V8 differential corpus as recommended by RFC 8785 (“running V8 as a live reference together with random IEEE 754 values”).  
     Source: https://www.rfc-editor.org/rfc/rfc8785

## 3. Non-goals
- Preserving the original lexical form of numbers (canonicalization rewrites).
- Supporting arbitrary precision decimals as JSON numbers; use strings when exactness is required (RFC 7493 recommends strings for such applications).  
  Source: https://www.rfc-editor.org/rfc/rfc7493.html

## 4. Verification
See:
- **docs/11-testing/v8-differential.md**
- **corpus/numbers/**

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259
- RFC 7493: https://www.rfc-editor.org/rfc/rfc7493.html
