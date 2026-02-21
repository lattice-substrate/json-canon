# 06. GJCS1 Envelope And Verification

## 6.1 Format

GJCS1 file format is:

- `JCS(value)` followed by exactly one trailing LF byte (`0x0A`).

ABNF-style summary:

- `gjcs1 = body lf`
- `lf = %x0A`

## 6.2 Verification Order

Verification MUST execute in this order:

1. file-level envelope checks,
2. strict-profile JSON parse,
3. canonical re-serialization and byte comparison.

This order is REQUIRED for stable error taxonomy.

## 6.3 File-Level Envelope Constraints

Before JSON parsing, verifier MUST enforce:

- file is non-empty,
- exactly one trailing LF,
- body is non-empty,
- no UTF-8 BOM in body,
- no CR byte (`0x0D`) anywhere,
- no LF byte inside body,
- valid UTF-8 body.

## 6.4 Canonical Equivalence Check

After successful parse, verifier MUST re-serialize and compare bytes exactly with body.

- mismatch MUST fail as non-canonical.

## 6.5 Atomic Write Behavior

Implementations providing file write API SHOULD use temp-file plus same-directory rename.

- cleanup SHOULD be best-effort on failure,
- directory sync MAY be best-effort,
- Linux local filesystem semantics are the target environment.
