# Architectural Decision Records

This directory contains Architecture Decision Records (ADRs) for decisions that
affect compatibility, security posture, or long-term maintainability.

## Status Values

- `Proposed`
- `Accepted`
- `Superseded`

## When an ADR Is Required

Create/update an ADR when a change affects:

- stable ABI behavior
- requirement interpretation
- failure taxonomy semantics
- determinism/supply-chain/security posture
- supported platform policy

## Process

1. Create ADR from `docs/adr/ADR-TEMPLATE.md`.
2. Link related requirements and affected docs.
3. Mark status and date.
4. Update references in `GOVERNANCE.md` and release notes.
