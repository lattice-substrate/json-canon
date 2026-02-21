# Traceability Model (Requirements → Code → Tests → Evidence)

**Status:** Draft  
**Goal:** make an auditor’s job straightforward.

## 1. Traceability artifacts
This project maintains:
1. **Requirements registry**: `requirements.md` and `requirements.csv`
2. **Code anchors**: each requirement references specific files/functions once code exists
3. **Tests**:
   - unit tests mapped to requirement IDs
   - corpus vectors (valid/invalid) mapped to requirement IDs
   - V8 differential tests (numbers)
   - determinism matrix runs (environment evidence)
4. **Evidence bundle**: generated per release and stored as CI artifacts:
   - corpus manifest results (sha256)
   - environment metadata (go env, uname, distro ID)
   - tool version identifiers

## 2. How mapping is expressed
- Unit tests MUST include the requirement ID(s) in their name or in a structured comment block.
- Corpus files MUST have a metadata entry that includes the relevant requirement ID(s).
- Code anchors are referenced by file path + stable symbol name. Line numbers are informative only.

## 3. Evidence-friendly invariants
- Idempotence tests: canon(canon(x)) == canon(x)
- Negative tests: invalid inputs MUST reject with the correct error code
- Sorting tests: match RFC 8785 provided sorting example
- Number tests: match RFC 8785 Appendix B and V8 differential corpus (as recommended in RFC 8785)

## References
- RFC 8785 (for the general purpose and number-testing guidance): https://www.rfc-editor.org/rfc/rfc8785
- ISO/IEC/IEEE 29148 overview (for requirements engineering concepts): https://standards.ieee.org/standard/29148-2018.html
