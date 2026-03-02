# ADR-0005: Cross-Platform CI and Release Expansion

- ADR ID: ADR-0005
- Date: 2026-03-01
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: DET-STATIC-001, SUPPLY-PROV-001, OFFLINE-GATE-001

## Context

ADR-0003 established a Linux-only runtime and CI posture for determinism and
operational simplicity. However, `json-canon` is a pure-Go, zero-CGO project
with no platform-specific code. The existing CI cross-compiled Windows binaries
and validated offline evidence for Windows architectures, but never executed
tests or binaries on a native Windows host. This left a verification gap:
Windows test correctness was assumed from cross-compilation rather than proven
by execution.

## Decision

- Expand the CI test matrix to include `windows-latest` alongside
  `ubuntu-latest`, running `go vet` and `go test` natively on Windows.
- Add a native Windows reproducible build job in CI that builds twice on
  `windows-latest` and compares SHA-256 hashes.
- Add a `Windows Pre-Release Validation` job in the release workflow that runs
  vet, unit tests, and race tests on a Windows runner.
- Add `build_windows` jobs in the release workflow that produce
  `jcs-canon-windows-amd64.zip` and `jcs-canon-windows-arm64.zip` artifacts.
- Include Windows artifacts in release checksums and build provenance
  attestation.

## Rationale

- Pure-Go code should be verified on all target platforms, not just
  cross-compiled.
- Native execution catches platform-specific runtime behavior differences
  (path handling, filesystem semantics, binary suffix conventions).
- Windows release artifacts with provenance attestation enable verified
  consumption on Windows without requiring users to build from source.
- The Go toolchain's first-class Windows support makes this expansion
  low-friction.

## Consequences

- CI wall-clock time increases due to additional Windows runner jobs.
- Release artifacts now include Windows `.zip` bundles alongside Linux
  `.tar.gz` bundles.
- ADR-0003's "cross-platform release targets are intentionally excluded"
  consequence is amended: Windows is now an explicit CI and release target.
- Linux remains the primary runtime support target; Windows is a CI-validated
  secondary target.

## Alternatives Considered

- Keep cross-compile-only validation: rejected because it cannot detect
  runtime-level platform divergences.
- Add macOS as well: deferred; no current demand and would triple CI cost.
