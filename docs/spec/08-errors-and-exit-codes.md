# 08. Errors And Exit Codes

## 8.1 Exit Codes

The CLI MUST use the following exit codes:

- `0`: success
- `2`: invalid input, profile violation, or non-canonical bytes
- `10`: internal/runtime failure

## 8.2 Error Classes

Implementations SHOULD classify failures into these groups:

- `EnvelopeError`: file-level envelope constraints failed.
- `ParseError`: strict-profile JSON parse/validation failed.
- `CanonError`: parsed value does not match canonical byte representation.

## 8.3 Ordering Guarantees

If both envelope and parse failures are possible, envelope failure MUST be reported first.

Example:

- `BOM + duplicate keys` MUST surface as envelope error class.

## 8.4 Wrapping Discipline

Error values propagated across package boundaries SHOULD preserve causal context via wrapping.

Diagnostics MUST remain machine-actionable and deterministic.
