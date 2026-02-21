# 10. Conformance

An implementation is conformant to this specification if all REQUIRED clauses are satisfied and the conformance suite passes.

## 10.1 Required Test Categories

Conformance testing MUST include:

1. JCS number formatting vectors (`jcsfloat` golden corpus).
2. Strict-profile parse vectors (positive and negative).
3. Canonical serialization vectors.
4. GJCS1 envelope and verification-order vectors.
5. Black-box CLI vectors for `canonicalize` and `verify`.
6. Adversarial CLI cases (depth bounds, malformed bytes, invalid numeric ranges).

## 10.2 Black-Box CLI Requirement

Conformance MUST validate the built binary as an executable black box.

- tests MUST execute the binary as a subprocess,
- exit codes and IO streams MUST be asserted.

## 10.3 Minimum Acceptance Gates

A release candidate MUST satisfy all of the following:

1. `go test ./... -count=1` passes.
2. linter suite passes using repository lint config.
3. static build of CLI succeeds with `CGO_ENABLED=0`.
4. golden vector line-count and checksum assertions pass.

## 10.4 Non-Goals Of Conformance

Conformance does not imply:

- proof of domain correctness for user-provided payload semantics,
- acceptance of all RFC 8259 lexical forms (profile is stricter by design).
