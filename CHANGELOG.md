# Changelog

All notable changes to this project are documented in this file.

This project follows strict [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Official external conformance fixture packs under `conformance/official/`:
  - Cyberphone `testdata/{input,output,outhex}` vectors with pinned provenance metadata.
  - RFC 8785-derived fixtures for §3.2.3 key sorting and Appendix B finite number mappings.
- New conformance tests for official suites:
  - `TestOfficialCyberphoneCanonicalPairs`
  - `TestOfficialRFC8785Vectors`
  - `TestOfficialES6CorpusChecksums10K`
  - release-only `TestOfficialES6CorpusChecksums100M`
- New executable differential suite documenting Cyberphone Go invalid-input acceptance vs `json-canon` strict rejection:
  - `TestCyberphoneGoDifferentialInvalidAcceptance`
  - reference table in `docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md`
- New policy requirements `OFFICIAL-VEC-001..004` with matrix mappings and conformance requirement coverage.
- Publication-readiness governance files (`LICENSE`, `NOTICE`, `SECURITY.md`, `CONTRIBUTING.md`).
- Stable top-level CLI flags: `--help`/`-h` and `--version`.
- `GOVERNANCE.md` with maintainer policy, support window, and deprecation policy.
- `BOUNDS.md` documenting parser resource limits, memory amplification, and DoS mitigation.
- `VERIFICATION.md` with release artifact verification instructions.
- `abi_manifest.json` machine-readable ABI contract.
- `standards/CITATION_INDEX.md` mapping all 54 normative requirements to authoritative spec clauses.
- Conformance vector corpus expanded from 4 to 74 vectors across 4 categorized files.
- Traceability gate tests: registry/matrix parity, impl/test symbol existence, ID format validation, vector schema validation, ABI manifest validation, citation index coverage.
- Build provenance attestation in release workflow (SLSA via `actions/attest-build-provenance`).
- Official engineering documentation index and specs under `docs/`:
  - `docs/TRACEABILITY_MODEL.md`
  - `docs/VECTOR_FORMAT.md`
  - `docs/ALGORITHMIC_INVARIANTS.md`
- ADR framework and accepted foundational decisions under `docs/adr/`.
- Offline cold-replay framework under `offline/` with matrix/profile contracts, evidence schema, and offline conformance gate package.
- New operator CLI `jcs-offline-replay` with `prepare`, `run`, `verify-evidence`, and `report` subcommands.
- `jcs-offline-replay inspect-matrix` subcommand for machine-readable matrix introspection.
- `jcs-offline-replay` Go-native operator subcommands for local proof orchestration:
  - `preflight`
  - `audit-summary`
  - `run-suite`
  - `cross-arch` (with optional `--run-official-vectors` and `--run-official-es6-100m`)
- New replay worker CLI `jcs-offline-worker` for per-lane vector execution and evidence emission.
- Runtime adapter execution paths for container and libvirt lanes, plus operational runner scripts (`offline/scripts/replay-container.sh`, `offline/scripts/replay-libvirt.sh`).
- End-to-end operator scripts for offline proof runs:
  - `offline/scripts/cold-replay-preflight.sh`
  - `offline/scripts/cold-replay-run.sh`
  - `offline/scripts/cold-replay-audit-report.sh`
  - `offline/scripts/cold-replay-cross-arch.sh`
- Full offline proof runbook in `docs/OFFLINE_REPLAY_HARNESS.md`.

### Changed
- CI conformance workflow step now explicitly documents that it includes the official ES6 10k checksum gate.
- Release workflow now includes an explicit `official ES6 100M checksum gate` step prior to publish jobs.
- Release/conformance/verification docs now include the required command for the 100M official ES6 checksum gate.
- CI unit test timeout aligned to 20m (matching CONFORMANCE.md and release workflow).
- Release workflow expanded with pre-release validation job.
- `GOVERNANCE.md` updated for single-maintainer review and succession policy.
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
- CLI stderr diagnostics now include stable failure class tokens for usage, parse/profile failures, and non-canonical verification failures.
- Undocumented `--` end-of-options behavior was removed from subcommand flag parsing.
- Conformance suite now enforces ABI manifest/source parity, workflow SHA pinning, release checksum/provenance steps, governance durability clauses, and behavior-test matrix linkage.
- Release and verification documentation now include mandatory offline evidence gate validation (`go test ./offline/conformance` with `JCS_OFFLINE_EVIDENCE`).
- Policy registry and traceability matrix expanded with OFFLINE requirement IDs for matrix coverage, cold replay policy, evidence schema contract, release-gate enforcement, and architecture scope.
- Offline evidence verification now binds metadata digests (`bundle_sha256`, `control_binary_sha256`, `matrix_sha256`, `profile_sha256`) and `architecture` to expected artifacts, and fails on tamper.
- Release workflow now executes `TestOfflineReplayEvidenceReleaseGate` with an explicit archived evidence path before publish jobs.
- README dependency claim now states the precise scope: core runtime has zero external dependencies.
- Offline release gating now requires both `x86_64` and `arm64` evidence validation paths in release workflow and documentation.
- Offline release architecture policy now explicitly supports `x86_64` and `arm64` (including evidence schema enum and architecture contract checks).
- Added `docs/BOOK.md` as a book-style operator/developer guide for architecture, usage, offline replay, and troubleshooting.
- Lint governance hardened to fail-closed: PR/main CI lint gate restored with pinned golangci-lint config/version, local mandatory gates now require the same pinned lint invocation, strict `nolint` rationale enforcement added, and policy traceability expanded with `LINT-*` requirements.

### Fixed
- Performance: `lshByInt` O(n) loop replaced with single `big.Int.Lsh` call for subnormal float formatting.
- Removed unreachable `+` prefix check in `tokenRepresentsZero`.
- Deduplicated `isNoncharacter`; canonical definition exported as `jcstoken.IsNoncharacter`.
- Map-iterated test tables in `jcsfloat_test.go` and `conformance/harness_test.go` converted to deterministic slices.
- Digit buffer safety invariant documented in `jcsfloat.go`.
- CLI now returns exit `10` for write failures in top-level/subcommand help, version output, and verify success status writes.
