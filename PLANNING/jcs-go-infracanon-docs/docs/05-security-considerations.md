# Security Considerations and Threat Model

**Status:** Draft

## 1. Threat model (canonicalization boundary)
This canonicalizer is intended for systems where the canonical output is an input to security-critical operations such as:
- digital signatures,
- HMACs,
- content-addressing / integrity hashes.

RFC 8785 describes JCS output as a “hashable” representation and warns that cryptographic reliability requires invariant serialization. (RFC 8785 Introduction)  
Source: https://www.rfc-editor.org/rfc/rfc8785

### Primary threats
1. **Silent normalization**
   - Accepting invalid inputs and converting them into “valid-looking” canonical output.
   - Result: signatures/hashes cover different semantics than the sender intended, or allow multiple distinct inputs to map to the same signed output.

2. **Unicode / encoding ambiguity**
   - Accepting invalid UTF‑8 or forbidden code points leads to divergent interpretation across decoders.
   - RFC 3629 explicitly warns that decoding invalid sequences can have security consequences.  
   Source: https://www.rfc-editor.org/rfc/rfc3629

3. **Duplicate key ambiguity**
   - RFC 8259 notes non-unique object member names yield unpredictable behavior across implementations.
   - I‑JSON mandates duplicate names MUST NOT occur after unescaping; receivers can reject.  
   Sources:
   - https://datatracker.ietf.org/doc/html/rfc8259
   - https://www.rfc-editor.org/rfc/rfc7493.html

4. **Numeric interoperability / precision traps**
   - I‑JSON warns against numbers outside IEEE‑754 binary64 range/precision and recommends encoding larger/exact numbers as strings.
   - Canonicalization must reject non-finite results (NaN/Inf).  
   Sources:
   - RFC 7493 §2.2: https://www.rfc-editor.org/rfc/rfc7493.html
   - RFC 8785 number rules: https://www.rfc-editor.org/rfc/rfc8785

## 2. Design security invariants
This project enforces these invariants:

- **Strict input validation**: only RFC 8259 JSON accepted; no extensions or alternative float syntaxes.
- **UTF‑8 correctness**: reject invalid UTF‑8 sequences; reject UTF‑8 surrogate code points (RFC 3629).
- **I‑JSON compliance**:
  - reject Surrogates and Noncharacters (direct UTF‑8 or escaped),
  - reject duplicate keys after unescaping.
- **No Unicode normalization**: preserve Unicode string data “as is” per RFC 8785 note.
- **Deterministic output**: output bytes depend only on input bytes; no locale/TZ/environment factors.

## 3. Operational guidance
- Treat canonicalization errors as protocol violations in security-critical contexts.
- Store both the original received bytes and canonical output bytes when auditing signatures.
- Pin tool versions used for canonicalization in supply-chain workflows.

## 4. References
- RFC 8785 Security Considerations (and Introduction): https://www.rfc-editor.org/rfc/rfc8785
- RFC 3629 Security Considerations: https://www.rfc-editor.org/rfc/rfc3629
- RFC 7493 Software behavior guidance: https://www.rfc-editor.org/rfc/rfc7493.html
