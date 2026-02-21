# Vector Corpus Format

**Status:** Draft  
**Goal:** stable, machine-readable vectors that support audits.

## 1. Directory layout
- `corpus/valid/` — inputs expected to canonicalize successfully
- `corpus/invalid/` — inputs expected to fail with a specific error code
- `corpus/rfc8785/` — vectors derived from RFC 8785 examples (sorting + number samples)
- `corpus/numbers/` — number-only vectors and/or generated corpora

## 2. Metadata files
### 2.1 Valid manifest
`corpus/manifest/valid-manifest.json`:
```json
{
  "valid/example.json": {
    "requires": ["JCS-REQ-0001", "JCS-REQ-0301"],
    "expected_output": "valid/example.out.json",
    "expected_sha256": "..."
  }
}
```

### 2.2 Invalid manifest
`corpus/manifest/invalid-manifest.json`:
```json
{
  "invalid/num_plus.json": {
    "requires": ["JCS-REQ-0200"],
    "expected_error": "JCS_ERR_BAD_NUMBER_SYNTAX"
  }
}
```

## 3. Hashing rule
- SHA-256 is computed over the **canonical output bytes** as emitted (UTF‑8), including no trailing newline unless the tool explicitly defines one.

## 4. Corpus evolution policy
- New corpus files may be added.
- Existing corpus files MUST NOT change without a versioned corpus bump and a changelog entry.

## References
- RFC 8785 output is intended to be usable as input to cryptographic methods: https://www.rfc-editor.org/rfc/rfc8785
