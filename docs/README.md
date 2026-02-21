# lattice-canon Documentation

This folder documents how `lattice-canon` works, how to use it in tooling, and how to produce objective correctness evidence for production gates.

## Document map

1. `docs/spec-profile.md`
   Defines what is RFC behavior vs project profile behavior (including `-0` rejection).
2. `docs/usage.md`
   CLI usage, exit codes, integration patterns, and library-level API usage.
3. `docs/examples.md`
   End-to-end examples for canonicalization and verification (valid and invalid cases).
4. `docs/testing.md`
   Test structure, vector generation, and validation gates.
5. `docs/runbook-correctness.md`
   Operator runbook to generate reproducible evidence of correctness for release approvals.

## Fast path

For a release-quality proof run, execute the runbook in `docs/runbook-correctness.md` from repository root.
