# 08. Errors And Exit Codes

## 8.1 Exit Codes

The CLI MUST use:

- `0`: success
- `2`: invalid input, strict-profile violation, or non-canonical bytes
- `10`: internal/runtime failure

## 8.2 Error Classes

Implementations SHOULD classify failures into:

- `ParseError`: strict-profile JSON parse/validation failed.
- `CanonError`: parsed value does not match canonical byte representation.
- `RuntimeError`: IO or internal failure.

## 8.3 Deterministic Diagnostics

- Error messages MUST be deterministic for the same input.
- Unknown CLI options MUST fail with exit code `2`.
- Success path MUST emit no stdout/stderr noise except documented output (`ok` on non-quiet verify).

## 8.4 Wrapping Discipline

Error values propagated across package boundaries SHOULD preserve causal context via wrapping.
