# Fuzzing Strategy

**Status:** Draft

## 1. Goals
- Discover parser differentials (accepting invalid RFC 8259 inputs).
- Discover encoding vulnerabilities (invalid UTF‑8 sequences accepted).
- Discover performance hazards (deep nesting, huge strings, large objects).

## 2. Seeds
- Start from `corpus/valid/` and `corpus/invalid/`.
- Add known tricky JSON snippets:
  - extreme exponent forms (valid and invalid)
  - surrogate escapes (paired and unpaired)
  - invalid UTF‑8 sequences (overlongs, truncations)

## 3. Oracles
- For valid inputs: verify output matches manifest and idempotence holds.
- For invalid inputs: verify error code matches manifest.
- For fuzz discoveries: minimize and add to corpus, assign requirement IDs.

## 4. Guardrails
- Put strict limits on recursion depth and maximum memory per object to avoid resource exhaustion, per RFC 8785 security considerations guidance about sanity checks.  
  Source: RFC 8785 Security Considerations. https://www.rfc-editor.org/rfc/rfc8785

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
- RFC 3629 invalid UTF‑8 consequences: https://www.rfc-editor.org/rfc/rfc3629
