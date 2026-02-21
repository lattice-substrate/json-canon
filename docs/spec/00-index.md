# lattice-canon Specification Index

Status: Draft
Version: 1.0.0-draft

This specification defines the `lattice-canon` governed JSON canonicalization and verification contract for infrastructure tooling.

## Contents

- [01. Introduction](01-introduction.md)
- [02. Conventions And Terminology](02-conventions.md)
- [03. Data Model](03-data-model.md)
- [04. Strict JSON Profile](04-strict-json-profile.md)
- [05. JCS Serialization](05-jcs-serialization.md)
- [06. GJCS1 Envelope And Verification](06-gjcs1-envelope.md)
- [07. CLI Contract](07-cli-contract.md)
- [08. Errors And Exit Codes](08-errors-and-exit-codes.md)
- [09. Security And Determinism](09-security-and-determinism.md)
- [10. Conformance](10-conformance.md)

## Non-Normative Companion Docs

- `docs/usage.md`
- `docs/examples.md`
- `docs/testing.md`
- `docs/runbook-correctness.md`
- `docs/infrastructure-alignment.md`

## Design Goals

1. Deterministic bytes for accepted values.
2. Fail-closed profile validation.
3. Static, self-contained CLI suitable for infrastructure gates.
4. Machine-actionable validation behavior.
