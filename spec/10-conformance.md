# 10. Conformance

An implementation is conformant to this specification if all REQUIRED clauses are satisfied and the conformance suite passes.

## 10.1 Required Test Categories

Conformance testing MUST include:

1. JCS number formatting vectors (`jcsfloat` golden corpus).
2. Strict-profile parse vectors (positive and negative).
3. Canonical serialization vectors.
4. Black-box CLI vectors for `canonicalize` and `verify`.
5. Adversarial CLI cases (depth bounds, malformed bytes, invalid numeric ranges).
6. Deterministic replay checks (same input -> same bytes across repeated executions).
7. Requirement-ID traceability checks against `spec/requirements.md`.

## 10.2 Requirement Traceability

All conformance requirements MUST be cataloged in `spec/requirements.md`.

- each requirement ID MUST map to automated checks,
- conformance suite MUST fail on missing or extra mappings,
- requirement catalog and suite MUST run offline.

## 10.3 Black-Box CLI Requirement

Conformance MUST validate the built binary as an executable black box.

- tests MUST execute the binary as a subprocess,
- exit codes and IO streams MUST be asserted.

## 10.4 Minimum Acceptance Gates

A release candidate MUST satisfy all of the following:

1. `go test ./... -count=1` passes.
2. `go test ./conformance -count=1` passes.
3. linter suite passes using repository lint config.
4. static-friendly build of CLI succeeds with `CGO_ENABLED=0`.
5. golden vector line-count and checksum assertions pass.

## 10.5 Non-Goals Of Conformance

Conformance does not imply:

- proof of domain correctness for user-provided payload semantics,
- acceptance of all RFC 8259 lexical forms (profile is stricter by design).
