# Changelog

All notable changes to this project are documented in this file.

This project follows strict [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Publication-readiness governance files (`LICENSE`, `NOTICE`, `SECURITY.md`, `CONTRIBUTING.md`).
- Stable top-level CLI flags: `--help`/`-h` and `--version`.
- `GOVERNANCE.md` with maintainer policy, support window, and deprecation policy.
- `BOUNDS.md` documenting parser resource limits, memory amplification, and DoS mitigation.
- `VERIFICATION.md` with release artifact verification instructions.
- `abi_manifest.json` machine-readable ABI contract.
- `standards/CITATION_INDEX.md` mapping all 54 normative requirements to authoritative spec clauses.
- Conformance vector corpus expanded from 4 to 73 vectors across 4 categorized files.
- Traceability gate tests: registry/matrix parity, impl/test symbol existence, ID format validation, vector schema validation, ABI manifest validation, citation index coverage.
- Build provenance attestation in release workflow (SLSA via `actions/attest-build-provenance`).

### Changed
- File-based oversized input now preserves `BOUND_EXCEEDED` classification, matching stdin behavior.
- CI expanded with platform/version matrix, race tests, reproducibility checks, and binary tracking guard.
- Subcommand `--help` output now writes to stdout (was stderr). This is a frozen stream policy.
- CI Go version matrix expanded to 1.22.x, 1.23.x, 1.24.x.
- All GitHub Actions pinned by commit SHA for supply-chain integrity.
- Release workflow uses Go 1.24.x.
- `SECURITY.md` updated with GitHub Security Advisories as the reporting channel and explicit response SLAs.
- `CONTRIBUTING.md` updated to reference split requirement registries.
- `FAILURE_TAXONOMY.md` updated with file-open classification rationale.
- Traceability conformance gates now use AST-based symbol resolution and matrix line validation (domain/level/gate), replacing substring matching.
- Citation coverage conformance gate now validates structured mappings (ID -> source -> clause), not raw text presence.
- `BOUNDS.md` corrected to document canonical output expansion behavior for number normalization.
- Security fallback disclosure path now includes an explicit contact in `NOTICE`.
- Support policy clarified as Linux-only in contributor/governance documentation.
- CI and release workflow platform matrices reduced to Linux-only.
- Stale planning artifacts moved to `PLANNING/archive/` and active planning state reset.
- Conformance gates now enforce fully static Linux binaries and prohibit outbound network/subprocess imports in core runtime packages.

### Fixed
- Map-iterated test tables in `jcsfloat_test.go` and `conformance/harness_test.go` converted to deterministic slices.
- Digit buffer safety invariant documented in `jcsfloat.go`.
