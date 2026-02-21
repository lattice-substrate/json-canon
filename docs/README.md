# jcs-canon Documentation

This folder documents how `jcs-canon` works, how to use it in tooling, and how to produce objective correctness evidence for production gates.

Design target: Go-only, self-contained tooling with static binary builds (`CGO_ENABLED=0`), zero runtime external dependencies.

## Document map

1. `docs/spec-profile.md`
   Defines what is RFC behavior vs project profile behavior (including `-0` rejection).
2. `docs/usage.md`
   CLI usage, exit codes, integration patterns, and library-level API usage.
3. `docs/examples.md`
   End-to-end examples for canonicalization and verification (valid and invalid cases).
4. `docs/testing.md`
   Test structure, pinned vectors, and validation gates.
5. `docs/runbook-correctness.md`
   Operator runbook to generate reproducible evidence of correctness for release approvals.
6. `docs/infrastructure-alignment.md`
   Maps `jcs-canon` behavior to lattice-substrate infrastructure invariants and gate usage.
7. `spec/00-index.md`
   OCI-style normative specification chapters for jcs-canon behavior and conformance.

## Fast path

For release-quality proof, run the Go-only steps in `docs/runbook-correctness.md` from repository root.
