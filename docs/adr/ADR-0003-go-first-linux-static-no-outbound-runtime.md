# ADR-0003: Go-First Gates, Linux-Only Static Runtime, No Outbound Calls

- ADR ID: ADR-0003
- Date: 2026-02-21
- Status: Amended by ADR-0005
- Deciders: Maintainers
- Related Requirements: DET-STATIC-001, DET-NOSOURCE-001

## Context

Infrastructure-grade tooling requires deterministic, portable, and auditable
validation paths with minimal external moving parts.

## Decision

- Required conformance/traceability gates are implemented as Go tests.
- Supported operating environment is Linux.
- Release binaries are built as static binaries.
- Core runtime packages forbid outbound network calls and subprocess execution.

## Rationale

- Reduces CI/runtime variability and external dependency drift.
- Keeps enforcement logic inside the same language/toolchain as the product.
- Shrinks security and supply-chain attack surface for runtime behavior.

## Consequences

- Cross-platform release targets are intentionally excluded.
- New shell-script-based required gates require explicit maintainer exception.
- Runtime code that needs network/process behaviors must be out-of-scope or
  redesigned.

## Alternatives Considered

- Mixed shell/script validation stack: rejected for consistency and portability
  risk.
- Multi-platform support by default: rejected in favor of tighter Linux-only
  operational guarantees.

## Amendment Note

ADR-0005 expands the CI and release posture to include native Windows test
execution and Windows release artifacts. The Go-first gates, static build, and
no-outbound-runtime constraints from this ADR remain in effect. The change is
limited to platform scope: Windows is now a CI-validated secondary target
alongside the primary Linux runtime target.
