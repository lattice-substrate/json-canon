# Planning Documents

This directory contains **implementation plans** that are official while they exist, but are intentionally temporary.

## Status Model

- `ACTIVE_IMPLEMENTATION_PLAN.md` is the single active plan of record when present.
- Plans in `archive/` are historical context only.
- Any plan outside `ACTIVE_IMPLEMENTATION_PLAN.md` is non-authoritative unless explicitly stated.
- Current state: no active implementation plan file is present; latest plans are archived.

## Lifecycle Policy

1. A plan is official only while present in `PLANNING/ACTIVE_IMPLEMENTATION_PLAN.md`.
2. When superseded, maintainers choose one of two actions:
- Replace in place (preferred for fast-moving work).
- Move old plan to `PLANNING/archive/` and then publish the new active plan.
3. Archived plans are informational and must not be treated as current commitments.
4. If no active file exists, there is no current implementation plan.

## Scope Boundary

Planning files are not normative protocol specs, not ABI guarantees, and not compatibility contracts.
Normative and compatibility sources remain:
- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `FAILURE_TAXONOMY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `README.md` / release notes
- `CONTRIBUTING.md` / `GOVERNANCE.md` (process and approval policy)
