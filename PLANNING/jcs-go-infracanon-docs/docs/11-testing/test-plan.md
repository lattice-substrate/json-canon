# Test Plan and Evidence Artifacts

**Status:** Draft  
**Objective:** produce evidence that the implementation is (a) standards-conforming, and (b) byte-identical across environments.

## 1. Test layers
### 1.1 Requirement-level unit tests
- Each requirement ID in the registry has at least one unit test.
- Tests are small, targeted, and assert specific error codes or exact canonical bytes.

### 1.2 Corpus (golden vectors)
- Valid vectors: input.json → expected canonical output bytes.
- Invalid vectors: input.json → expected error code.
- RFC-derived vectors: include RFC 8785 sorting example and Appendix B number samples.

### 1.3 Differential tests (numbers)
RFC 8785 explicitly recommends exhaustive validation for number serializers:
- test against a large file of sample values, or
- run V8 as a live reference and generate random IEEE‑754 values.  
Source: RFC 8785 Appendix guidance. https://www.rfc-editor.org/rfc/rfc8785

### 1.4 Fuzz testing
- Tokenizer fuzzing (invalid syntax, deep nesting, odd whitespace)
- UTF‑8 fuzzing (invalid sequences, truncation)
- Structured fuzzing from corpus seeds

### 1.5 Determinism matrix
Run corpus + idempotence tests across:
- multiple Linux distros (containers)
- multiple CPU architectures (native runners)
- multiple kernels (VMs)

## 2. Evidence artifacts (per release)
- `evidence/corpus-results.json` (file → output sha256, or file → error code)
- `evidence/env.json` (go env, uname, distro metadata)
- `evidence/version.txt` (tool version, git commit, build flags)
- `evidence/determinism-matrix.csv` (environment → pass/fail + hashes)

## 3. Pass criteria
A release is compliant if:
- all unit tests pass,
- all corpus vectors pass,
- V8 differential number corpus passes (for the configured corpus size),
- determinism matrix passes for the declared environment set.

## References
- RFC 8785: https://www.rfc-editor.org/rfc/rfc8785
