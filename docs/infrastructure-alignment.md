# Infrastructure Alignment (lattice-substrate)

This document maps `jcs-canon` to lattice-substrate infrastructure expectations.

## Why this component exists

`jcs-canon` is a low-level canonicalization and verification primitive for JSON artifacts.

It supports the substrate invariant:

- Same semantic value + same profile + same tool identity -> identical canonical bytes.

## Alignment to substrate principles

### Determinism

- Canonical output is deterministic by construction.
- Object keys are sorted by UTF-16 code units.
- Number formatting follows ECMAScript-compatible JCS formatting behavior.

### Fail-closed behavior

`verify` rejects, rather than normalizes, profile-invalid or envelope-invalid input:

- invalid UTF-8
- duplicate keys
- lone surrogate or noncharacter
- `-0` token
- underflow-to-zero non-zero token
- non-canonical key order
- non-canonical whitespace

### Infrastructure boundary

`jcs-canon` is semantically blind:

- It validates syntax, profile rules, and canonical bytes.
- It does not interpret domain meaning.

### Black-box CLI compatibility

- Exposed as standalone CLI binary (`canonicalize`, `verify`).
- Exit codes are stable and machine-actionable.
- Safe for gate orchestration in multi-repo tooling.

### Tool identity and static linkage

- Build target is static (`CGO_ENABLED=0`, stripped).
- No runtime dependency on external interpreters/runtimes.

## Go-only operational policy

Repository release gates are Go-only:

- no Node.js requirement
- no external runtime required for tests/build
- golden vectors are pinned in-repo and verified by Go tests

## Production gate recommendations for hub integration

1. Gate: `go test ./... -count=1` must pass.
2. Gate: static build of `./cmd/jcs-canon` must pass with `CGO_ENABLED=0`.
3. Gate: `verify` smoke checks for canonical and invalid-profile cases.
4. Gate: binary digest capture and pin update as part of release promotion.
5. Gate: release blocked if vector checksum test fails.

## Contract summary

`jcs-canon` should be treated as an infrastructure primitive that guarantees mechanical properties only:

- canonical bytes
- profile enforcement
- reproducible verification

Domain semantics remain outside this boundary.
