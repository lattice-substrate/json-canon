# Public API Contract (Go)

**Status:** Draft  
**Policy:** This API is intentionally small to support decades-long maintenance.

## 1. Package name
Example: `canonjcs` (placeholder). The implementation may choose a different final module path.

## 2. Functions

### 2.1 Transform
```
Transform(src []byte) (dst []byte, err error)
```
- Input: UTF‑8 JSON text bytes.
- Output: canonical UTF‑8 JSON text bytes (RFC 8785).
- On failure: returns a typed error with a stable error code (see docs/09-error-codes.md).

### 2.2 TransformTo (writer)
```
TransformTo(w io.Writer, r io.Reader) error
```
- Streams input, but buffers objects as required for sorting.
- Intended for large inputs and pipeline usage.

## 3. Options (future-proofing)
Options are provided through an `Options` struct and functional options, but defaults correspond to strict JCS behavior.

**No option may relax the accepted domain** without changing the compliance level. If a “lenient mode” is ever introduced, it MUST be a separate entry point or separate package to avoid accidental misuse.

## 4. Compatibility rules
- Outputs are stable for a given version of the canonicalizer and corpus definition.
- Error codes are stable and never change meaning.
- New error codes may be added in minor versions.

## 5. Reference requirements
- Strictness and invariants are defined in docs/10-requirements/requirements.md.
