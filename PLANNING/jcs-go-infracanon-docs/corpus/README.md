# Corpus: Test Vectors and Manifests

**Status:** Draft  
This directory contains canonicalization vectors used for conformance, regression, and determinism evidence.

## Layout
- `rfc8785/` — vectors derived from RFC 8785 examples (sorting example and selected number samples)
- `valid/` — project-maintained valid JSON inputs and expected canonical outputs
- `invalid/` — project-maintained invalid inputs and expected error codes
- `manifest/` — machine-readable manifests tying vectors to requirement IDs and expected results
- `numbers/` — number-only corpora and generators (V8 differential)

## How vectors are used
- Unit tests assert specific properties on small cases.
- Corpus tests iterate over all vectors:
  - for valid vectors: canonical output must match the expected output bytes (sha256)
  - for invalid vectors: canonicalization must fail with the expected error code

## Source references
- RFC 8785 provides a key-sorting test object and expected order (RFC 8785 §3.2.3).  
  Source: https://www.rfc-editor.org/rfc/rfc8785
