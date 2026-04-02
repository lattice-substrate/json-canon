# Changelog

All notable changes to this project are documented in this file.

This project follows strict [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `evidence.v3` schema (`offline/schema/evidence.v3.json`) for official AWS release
  evidence. It binds per-replay measured host identity (`measured_architecture`,
  `measured_os_id`, `measured_os_version_id`, `measured_kernel`, `measured_cpu`,
  `aws_instance_id`, `aws_image_id`) to the infrastructure manifest.
- `infra-manifest.v2` schema (`offline/schema/infra-manifest.v2.json`) for attested
  official AWS host identity, including instance IDs, availability zones, measured
  OS/CPU/kernel identity, and IMDS instance-identity digests.
- `evidence.v2` schema (`offline/schema/evidence.v2.json`) extending evidence.v1
  with three required infrastructure binding fields: `infra_manifest_sha256`,
  `infra_repo_url`, `infra_repo_commit`. Node replay items gain three optional
  discovered substrate fields: `discovered_cpu`, `discovered_kernel`, `image_digest`.
- `infra-manifest.v1` schema (`offline/schema/infra-manifest.v1.json`) recording
  IaC repo identity, provider engine and lock digest, and per-host substrate records
  (role, cloud provider, region, instance type, AMI/image ID).
- `InfraManifest` and `InfraManifestHost` Go structs in `offline/replay` package
  with `LoadInfraManifest` and `ValidateInfraManifest` functions.
- `ValidateEvidenceBundle` now accepts both `evidence.v1` and `evidence.v2`.
  V2 validation enforces infra binding fields. Suite-driven policy:
  profiles with `infra-substrate-binding` in `required_suites` reject v1 evidence.
  Downgrade prevention: v2 infra fields present in a v1-schema bundle are rejected.
- `discoverCPU` and `discoverKernel` in `cmd/jcs-offline-worker` read substrate
  identity from `/proc/cpuinfo` and `/proc/version` at replay runtime (pure file
  reads, no subprocess). Results written to `discovered_cpu` / `discovered_kernel`
  in each node replay record.
- `--infra-manifest <path>` flag on `jcs-offline-replay run`, `run-suite`, and
  `cross-arch`. When set the manifest is loaded, validated, and its SHA-256 and
  identity fields are bound into the emitted evidence.v2.
- OpenTofu IaC in `infra/` now provisions the committed official AWS native-host fleet
  declared in `infra/aws_release_hosts.json`, covering 12 vm-only lanes / 60 total replays
  per architecture across x86_64 and arm64.
- Ubuntu AMI resolution in the official AWS host catalog now uses Canonical public SSM
  parameters instead of brittle AMI name globs, and the unsupported Ubuntu 20.04 minimal
  arm64 lane is replaced with a valid native AWS arm64 lane.
- The official AWS `provisioned_hosts` OpenTofu output is now explicitly marked sensitive so
  SSM-backed AMI selectors remain exportable to the Go orchestrator without weakening the
  root-module output contract.
- `offline/toolchain.lock.tsv`: normative pinned-tool lock for the server-backed
  evidence path and release workflow. It records the exact Go, OpenTofu, and jq
  artifacts by URL and SHA-256.
- `scripts/bootstrap-pinned-toolchain.sh`: shell bootstrap that materializes the
  pinned binaries from `offline/toolchain.lock.tsv` without relying on ambient
  host-installed Go/OpenTofu/jq.
- `scripts/init-infra-lock.sh`: bootstrap-only wrapper that materializes pinned
  Go and then delegates lock generation to `jcs-offline-replay init-infra-lock`.
- `jcs-offline-replay sync-toolchain`: Go-native pinned artifact downloader and
  verifier that stages host/remote tool artifacts under the release output tree and
  emits a shell env file for the release workflow.
- `jcs-offline-replay init-infra-lock`: Go-native post-bootstrap infra lock
  generator that executes the pinned OpenTofu binary directly.
- `jcs-offline-replay server-evidence`: Go-native post-bootstrap server evidence
  orchestrator. It provisions the official AWS host fleet via pinned OpenTofu,
  manages native replay execution over private SSM/S3 transport, writes the infra manifest
  through the Go CLI, validates both release gates with `JCS_OFFLINE_INFRA_MANIFEST`,
  and destroys instances on exit.
- `infra/aws_release_hosts.lock.json`: committed AMI lock document for official AWS
  release runs, plus `jcs-offline-replay refresh-aws-ami-lock` to refresh it from the
  mutable selector catalog under review.
- `conformance/nolint_inventory.tsv`: checked-in mechanical inventory of every
  `//nolint` directive, including linter names, requirement IDs, rationale text,
  and directive text. Conformance now fails on drift from the live source tree.
- `scripts/release-server.sh`: bootstrap-only wrapper that materializes pinned Go
  from `offline/toolchain.lock.tsv` and hands off billed orchestration to
  `jcs-offline-replay server-evidence`.
- `TestOfflineReplayEvidenceReleaseGate` accepts optional `JCS_OFFLINE_INFRA_MANIFEST`
  environment variable to bind infra-manifest SHA-256 during server-backed evidence
  gate validation. Semantic cross-check: evidence `infra_repo_url` and
  `infra_repo_commit` must match the loaded manifest. Server profiles with
  `infra-substrate-binding` require the manifest to be set.
- `infra-manifest.v1` now records the exact pinned tool artifacts used for the
  evidenced run via the `tools` array.
- Requirements OFFLINE-EVIDENCE-002, OFFLINE-INFRA-001, OFFLINE-TOOLCHAIN-001, and
  OFFLINE-SERVER-001 added to `REQ_REGISTRY_POLICY.md` with full enforcement matrix coverage.
- Requirement LINT-NOLINT-002 added to `REQ_REGISTRY_POLICY.md` to require
  mechanically gathered suppression evidence in conformance.
- Requirements AWS-RELEASE-001, AWS-TOOLCHAIN-001, and AWS-GATE-001 added to
  `REQ_REGISTRY_POLICY.md` to lock official AWS release breadth, host-only pinned
  toolchain scope, and v2-only release gating.
- Requirement AWS-AMI-001 added to `REQ_REGISTRY_POLICY.md` to lock stable official AWS
  image selector policy and forbid the unsupported Ubuntu 20.04 minimal arm64 lane.
- Requirement AWS-OUTPUT-001 added to `REQ_REGISTRY_POLICY.md` to lock the sensitive
  `provisioned_hosts` output contract used by the Go-native AWS release orchestration.
- ADR-0005: Evidence v2 and Infrastructure Manifest design decision.
- ADR-0006: Pinned toolchain evidence for server-backed conformance.
- ADR-0007: Go-native post-bootstrap server evidence orchestration.

## [v0.3.2] - 2026-03-06

### Added
- Zenodo DOI badge in README for citation discoverability.
- Oracle vector provenance metadata (`jcsfloat/testdata/PROVENANCE.json`)
  documenting Node.js v23.3.0 / V8 12.9.202.28 generation environment.
- arm64 runner in CI test matrix.

### Changed
- CI Go version matrix expanded to include Go 1.25.x and 1.26.x. Go 1.26
  switched `strconv.FormatFloat` from Ryu to Dragonbox; the matrix row
  validates that json-canon's Burger-Dybvig digit generation is independent
  of standard library algorithm changes.
- `go.mod` minimum version aligned to `go 1.24` to match ARTIFACT.md
  requirements.
- README and CONFORMANCE.md updated to reflect arm64 CI coverage.

### Fixed
- ECMA-262 section reference and V8 stability claim in oracle vector
  provenance metadata.

## [v0.3.1] - 2026-03-04

### Fixed
- `writeClassifiedError` now formats the unwrapped `*jcserr.Error` instead of the
  outer `fmt.Errorf` wrapper. Previously, the `verify` command emitted stderr like
  `error: parse canonical input: jcserr: PARSE_ERROR ...` while `canonicalize` emitted
  `error: jcserr: PARSE_ERROR ...` for the same malformed input. Both commands now
  emit the consistent `error: jcserr: <CLASS> ...` format. Callers scripting against
  stderr diagnostics should expect the shorter, uniform prefix on all error paths.

### Changed
- PARSE-GRAM-008 requirement wording clarified across `REQ_REGISTRY_NORMATIVE.md`,
  `jcstoken/token.go`, and `standards/CITATION_INDEX.md` to explicitly distinguish
  trailing non-whitespace (rejected) from trailing whitespace (consumed per
  RFC 8259 section 2, `JSON-text = ws value ws`). No behavioral change -- the parser
  already implemented this correctly.
- ABI output stream contract (`ABI.md`) now explicitly states that `canonicalize`
  emits no trailing newline. The canonical output is byte-exact; appending any
  bytes (including a newline) would violate the RFC 8785 byte-identity guarantee.
  No behavioral change -- the CLI has never appended a trailing newline.
- Added "Building from Source" section to `CONTRIBUTING.md` documenting the
  deterministic build command with `-X main.version=vX.Y.Z` injection. Users
  building from source to verify release reproducibility now have an explicit
  reference for version-stamped builds.

## [v0.3.0] - 2026-03-04

### Added
- `jcs.Canonicalize` and `jcs.CanonicalizeWithOptions` convenience functions (parse + serialize in one call).
- `jcs.SerializeWithOptions` for value-tree canonicalization with caller-supplied bounds.
- Benchmark suites for `jcsfloat.FormatDouble`, `jcstoken.Parse`, `jcs.Serialize`, and end-to-end canonicalization.
- Optional pre-push git hook (`.githooks/pre-push`) that validates vet and lint before tag pushes.
- Engineering articles section in README linking to the published technical article series.

### Changed
- Reduced `big.Int` allocations in Burger-Dybvig digit generation (scratch values reused across iterations).
- Added ASCII fast path in `parseString`: pure ASCII strings with no escapes parse with a single allocation instead of per-byte appends.
- Serializer bound validation now honors caller options for `CanonicalizeWithOptions` and `SerializeWithOptions` instead of always applying parser defaults.
- Binary hygiene policy now fails CI when compiled `jcs-*` artifacts are tracked under `offline/runs/**/bin/`.
- Offline run artifacts are now ignored recursively under `offline/runs/**` (while preserving `offline/runs/.gitkeep`).
- Conformance `CLI-EXIT-004` now skips on Linux environments where `/dev/full` is unavailable or not writable.
- Lint CI now enforces a `>=70%` coverage floor for `cmd/jcs-offline-replay`, `cmd/jcs-offline-worker`, `offline/runtime/executil`, and total project coverage.
- README rewritten for voice and structure. Documentation consolidated (removed redundant GOVERNANCE.md, THREAT_MODEL.md, RELEASE_PROCESS.md, VERIFICATION.md, NORMATIVE_REFERENCES.md, docs/QUICKSTART.md, docs/COMPARISON.md, docs/INTEGRATION_GUIDE.md, docs/book/).
- New unified `docs/GUIDE.md` replaces QUICKSTART.md and INTEGRATION_GUIDE.md.

### Fixed
- `--version` now reports the Go module version when installed via `go install` without explicit `-ldflags`.
- Release workflow offline evidence gate now derives `source_git_commit`/`source_git_tag` from archived evidence and builds the control binary from that commit, preventing false failures when a tag is not exactly one commit after evidence generation.
- CI and Coverage workflows now ignore `articles/**`-only changes so docs/article release commits do not trigger full runtime gates.
- Offline evidence validation now enforces SHA-256 token format for node-level and aggregate digest fields (not just bundle-level metadata), preventing malformed digest acceptance.
- Added regression tests for malformed node/aggregate digest rejection and serializer option-bound parity.
- Expanded offline replay and worker command tests to exercise full harness/worker orchestration paths under deterministic fake toolchains.

## [v0.2.1] - 2026-02-27

### Fixed
- Offline harness now writes repo-relative paths in all evidence metadata artifacts (RUN_INDEX.txt, checksums, audit summaries, cross-arch reports).
- Removed v0.2.0-rc.3 and v0.2.0-rc.4 evidence that was derived from manual edits of rc.3 output rather than independent harness runs.
- Replaced v0.2.0 evidence with supersession notice.

### Added
- Coverage CI workflow and discoverability badges.
- ADR-0004: evidence metadata path remediation.
- Regression tests for absolute path prevention in evidence artifacts.

### Changed
- Release policy now explicitly states published SemVer tags are immutable.
- Repository metadata/topics populated for public discoverability.

## [v0.2.0] - 2026-02-27

### Fixed
- Release workflow now emits a deterministic compressed Linux bundle (`jcs-canon-linux-x86_64.tar.gz`) and includes it in `SHA256SUMS` and published GitHub Release assets.
- Release workflow offline evidence gate now pins Go to `1.24.13`, uses `actions/checkout` with `fetch-depth: 2`, and resolves the parent commit via `git rev-parse --verify HEAD^` to prevent control-binary/toolchain drift and shallow-clone `HEAD~1` mis-resolution.

### Changed
- Repository hygiene for public visibility: removed tracked `offline/runs/...` artifacts (local binaries, bundles, logs, and machine-path-bearing outputs) and now ignore this path by default.
- Removed the Cyberphone Go module dependency from test code (`go.mod` is now dependency-free) by converting differential conformance checks to recorded-output vectors.

## [v0.2.0-rc.2] - 2026-02-26

### Fixed
- Release workflow evidence gate now correctly resolves the evidence source commit (`HEAD~1`) instead of the tagged commit, fixing a structural mismatch where evidence always binds to the parent of the commit containing evidence files.

### Changed
- Release documentation now explicitly describes the evidence commit sequence and source-identity binding model.
- Documentation now explains the engineering rationale ("why") behind architectural decisions, governance weight, conformance gates, ABI stability, and the infrastructure-grade design model.

## [v0.1.0-rc.2] - 2026-02-26

### Added
- Developer-oriented documentation overhaul:
  - `docs/QUICKSTART.md`: 5-minute getting started guide for Go library and CLI.
  - `docs/INTEGRATION_GUIDE.md`: CI/CD patterns, error handling, resource limits, migration guides.
  - `docs/COMPARISON.md`: evaluation guide with differential strictness table, feature matrix, and use-case guidance.
  - `README.md` rewritten as a developer-oriented landing page with progressive disclosure to new guides and handbook.
  - `docs/README.md` updated with new "Getting Started and Integration" section.
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
- Evidence schema `$id` URL updated from `solutionsexcite.github.io` to `lattice-substrate.github.io` to match module org migration.
- Module identity is now aligned to the public repository path (`github.com/lattice-substrate/json-canon`) across `go.mod`, imports, CI package filters, and verification docs.
- Offline evidence contract now binds source identity via `source_git_commit` and `source_git_tag` in schema/model/validation.
- Release workflow offline evidence gates now target per-tag evidence paths (`offline/runs/releases/<tag>/...`) and enforce expected commit/tag binding (`JCS_OFFLINE_EXPECTED_GIT_COMMIT`, `JCS_OFFLINE_EXPECTED_GIT_TAG`).
- Release workflow now supports `workflow_dispatch` reruns, marks `-rc` tags as GitHub prereleases, and fails publish if artifact globs do not match (`fail_on_unmatched_files: true`).
- `jcs-offline-replay run` now stamps source commit/tag into generated evidence and `verify-evidence` accepts source identity expectations.
- Offline release-gate docs and runbooks now require commit/tag-bound evidence validation commands.
- `parseCanonicalFromInput` now wraps parser/serializer errors with additional context while preserving failure-class extraction (`errors.As`/`%w`) for stable exit-class behavior.
- Number parser integer-part scanner was refactored into smaller helper stages to satisfy strict complexity lint policy without suppressions.
- `ABI.md` command-flag contract now explicitly matches runtime/manifest behavior: `canonicalize` accepts `--quiet` for command symmetry.
- `CONFORMANCE.md` mandatory validation gate list now includes the pinned local golangci-lint command, aligning with `AGENTS.md` and `CONTRIBUTING.md`.
- Conformance requirement check `CLI-EXIT-004` is now fail-closed on Linux if `/dev/full` cannot be opened (instead of skipping), removing silent gate bypass in supported runtime environments.
- Release workflow offline evidence gates now reference the current cross-arch evidence bundle set and align arm64 matrix path with published release/verification docs (`offline/matrix.arm64.yaml`).
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
- CI/release lint workflow compatibility: pinned `golangci/golangci-lint-action` to `v6.5.2` SHA so enforced `golangci-lint v1.64.8` runs successfully in GitHub Actions.
- Performance: `lshByInt` O(n) loop replaced with single `big.Int.Lsh` call for subnormal float formatting.
- Removed unreachable `+` prefix check in `tokenRepresentsZero`.
- Deduplicated `isNoncharacter`; canonical definition exported as `jcstoken.IsNoncharacter`.
- Map-iterated test tables in `jcsfloat_test.go` and `conformance/harness_test.go` converted to deterministic slices.
- Digit buffer safety invariant documented in `jcsfloat.go`.
- CLI now returns exit `10` for write failures in top-level/subcommand help, version output, and verify success status writes.
