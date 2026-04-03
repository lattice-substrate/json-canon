# Policy Requirement Registry

Formal catalog of project policy requirements for `json-canon` (profile, ABI, process, determinism).

## Legend

| Column | Meaning |
|--------|---------|
| ID | Stable requirement identifier: `DOMAIN-NNN` |
| Spec | Policy source or governing basis |
| Section | Section or clause within the source |
| Level | MUST, SHALL, or SHOULD |
| Requirement | Policy text (paraphrased) |

---
## ECMA-VEC: Oracle Validation

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| ECMA-VEC-001 | V8 Oracle | - | MUST | All 54,445 base golden oracle vectors MUST produce byte-identical output. SHA-256: `593bdec...`. |
| ECMA-VEC-002 | V8 Oracle | - | MUST | All 231,917 stress golden oracle vectors MUST produce byte-identical output. SHA-256: `287d21a...`. |
| ECMA-VEC-003 | ECMA-262 | §6.1.6.1.20 | MUST | Boundary constants (0, -0, MIN_VALUE, MAX_VALUE, 1e-6 boundary, 1e21 boundary) MUST match expected strings. |

## OFFICIAL-VEC: Official External Reference Suites

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| OFFICIAL-VEC-001 | cyberphone/json-canonicalization | `testdata/input` + `testdata/output` + `testdata/outhex` | MUST | Vendored official Cyberphone canonicalization fixtures MUST pass byte-identical canonicalization checks. |
| OFFICIAL-VEC-002 | RFC 8785 | §3.2.3 + Appendix B | MUST | Vendored RFC 8785 example fixtures (sorting example and finite Appendix B number mappings) MUST match canonical output/format results. |
| OFFICIAL-VEC-003 | cyberphone/json-canonicalization | `testdata/numgen.go` checksum table | MUST | CI conformance gates MUST validate the official deterministic ES6 number corpus checksum at 10,000 lines (`b9f7a8e...`). |
| OFFICIAL-VEC-004 | CONTRIBUTING.md + .github/workflows/release.yml | release validation | MUST | Release validation MUST run the official deterministic ES6 number corpus checksum gate at 100,000,000 lines (`0f7dda6...`). |

## CONF-VEC: Conformance Vector Corpus

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CONF-VEC-001 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | Every executable conformance vector MUST declare an explicit `intent` classification. |
| CONF-VEC-002 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | Conformance vector `intent` MUST be one of `positive`, `negative`, or `adversarial`. |
| CONF-VEC-003 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | `positive` conformance vectors MUST assert successful execution (`want_exit = 0`) and MUST assert exact success-channel output. |
| CONF-VEC-004 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | `negative` conformance vectors MUST assert fail-closed execution (`want_exit != 0`) and MUST assert stderr diagnostic evidence. |
| CONF-VEC-005 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | `adversarial` conformance vectors MUST assert fail-closed execution (`want_exit != 0`) and MUST assert root-cause or byte-offset stderr diagnostics. |
| CONF-VEC-006 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | Every CLI command represented in the conformance vector corpus MUST have at least one `positive` vector. |
| CONF-VEC-007 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | Every CLI command represented in the conformance vector corpus MUST have at least one `negative` vector. |
| CONF-VEC-008 | CONFORMANCE.md + docs/VECTOR_FORMAT.md | vector corpus contract | MUST | Every CLI command represented in the conformance vector corpus MUST have at least one `adversarial` vector. |

## PROF-NUM: Number Profile Restrictions

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| PROF-NEGZ-001 | Profile | - | MUST | Lexical negative zero token (`-0`, `-0.0`, `-0e0`, etc.) MUST be rejected at parse time. |
| PROF-OFLOW-001 | IEEE 754 | §7.4 | MUST | Number tokens that overflow IEEE 754 binary64 (±Infinity result) MUST be rejected. |
| PROF-UFLOW-001 | IEEE 754 | §7.5 | MUST | Non-zero number tokens that underflow to IEEE 754 zero MUST be rejected. |

## BOUND: Resource Bounds

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| BOUND-DEPTH-001 | Profile | - | MUST | Nesting depth MUST be bounded (default: 1000). |
| BOUND-INPUT-001 | Profile | - | MUST | Input size MUST be bounded (default: 64 MiB). |
| BOUND-VALUES-001 | Profile | - | MUST | Total value count MUST be bounded (default: 1,000,000). |
| BOUND-MEMBERS-001 | Profile | - | MUST | Object member count MUST be bounded (default: 250,000). |
| BOUND-ELEMS-001 | Profile | - | MUST | Array element count MUST be bounded (default: 250,000). |
| BOUND-STRBYTES-001 | Profile | - | MUST | Decoded string byte length MUST be bounded (default: 8 MiB). |
| BOUND-NUMCHARS-001 | Profile | - | MUST | Number token character length MUST be bounded (default: 4096). |

## CLI: Command-Line Interface ABI

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CLI-CMD-001 | ABI | - | MUST | `canonicalize` command MUST parse stdin/file, emit canonical bytes to stdout, exit 0 on success. |
| CLI-CMD-002 | ABI | - | MUST | `verify` command MUST parse, canonicalize, byte-compare, exit 0 if identical. |
| CLI-EXIT-001 | ABI | - | MUST | No command specified MUST exit 2 with usage message on stderr. |
| CLI-EXIT-002 | ABI | - | MUST | Unknown command MUST exit 2 with error on stderr. |
| CLI-EXIT-003 | ABI | - | MUST | Input/parse/profile violations MUST exit 2. |
| CLI-EXIT-004 | ABI | - | MUST | Internal I/O errors (e.g. write failure) MUST exit 10. |
| CLI-FLAG-001 | ABI | - | MUST | Unknown flags MUST be rejected with exit 2. |
| CLI-FLAG-002 | ABI | - | MUST | `--quiet` flag MUST suppress success messages on verify. |
| CLI-FLAG-003 | ABI | - | MUST | `--help`/`-h` MUST display usage and exit 0 at top-level and command-level. |
| CLI-FLAG-004 | ABI | - | MUST | `--version` MUST print a machine-parseable version string (`jcs-canon vX.Y.Z` form) and exit 0. |
| CLI-IO-001 | ABI | - | MUST | `-` argument or no file MUST read from stdin. |
| CLI-IO-002 | ABI | - | MUST | Multiple input files MUST be rejected with exit 2. |
| CLI-IO-003 | ABI | - | MUST | File and stdin MUST produce identical output for identical content. |
| CLI-IO-004 | ABI | - | MUST | `canonicalize` output goes to stdout only; stderr MUST be empty on success. |
| CLI-IO-005 | ABI | - | MUST | `verify` success MUST emit "ok\n" on stderr (unless --quiet). |
| CLI-CLASS-001 | ABI | - | MUST | CLI failure diagnostics MUST include a stable failure class token (`INVALID_*`, `CLI_USAGE`, `NOT_CANONICAL`, etc.) in stderr output. |

## ABI-PARITY: Manifest/Runtime Parity

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| ABI-PARITY-001 | ABI | - | MUST | `abi_manifest.json` command/flag surface MUST match the implemented CLI source and runtime behavior. |

## SUPPLY: Supply Chain Verification

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| SUPPLY-PIN-001 | CLAUDE | Security and Supply-Chain Requirements | MUST | All GitHub Actions workflow dependencies MUST be pinned to immutable full commit SHA references. |
| SUPPLY-PROV-001 | CLAUDE | Security and Supply-Chain Requirements | MUST | Release workflow MUST publish checksums, a compressed Linux release bundle (`jcs-canon-linux-x86_64.tar.gz`), and build provenance attestation steps. |

## GOV: Governance Durability

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| GOV-DUR-001 | CLAUDE | Infrastructure-Grade Definition | MUST | Governance durability clauses (review policy, succession policy, support policy) MUST be present in `CONTRIBUTING.md` and validated by tests. |

## TRACE: Traceability Integrity

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| TRACE-LINK-001 | docs/TRACEABILITY_MODEL.md | Required Mapping | MUST | Behavior tests in runtime packages MUST be linked from `REQ_ENFORCEMENT_MATRIX.md`. |
| TRACE-LINK-002 | REQ_ENFORCEMENT_MATRIX.md + REQ_ENFORCEMENT_MATRIX.csv | Machine-Readable Mirrors | MUST | The committed CSV enforcement-matrix artifact MUST remain row-identical to the markdown enforcement matrix. |
| TRACE-LINK-003 | REQ_ENFORCEMENT_MATRIX.md + REQ_ENFORCEMENT_MATRIX.jsonl | Machine-Readable Mirrors | MUST | The committed JSONL enforcement-matrix artifact MUST remain row-identical to the markdown enforcement matrix. |
| TRACE-LINK-004 | CONFORMANCE.md + conformance/harness_test.go | Machine-Readable Mirrors | MUST | Conformance gates MUST fail closed when registry IDs, markdown matrix rows, CSV matrix rows, JSONL matrix rows, or linked code/test symbols drift from each other or from the committed source tree. |

## LINT: Lint Governance

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| LINT-CI-001 | .github/workflows/ci.yml | CI Gates | MUST | Pull-request and main-branch CI MUST execute golangci-lint via pinned action SHA, pinned linter version, and explicit `--config=golangci.yml`. |
| LINT-GATE-001 | CLAUDE.md + CONTRIBUTING.md + cmd/jcs-gate/main.go | Mandatory Validation Gates | MUST | Required local validation gates MUST include the same pinned golangci-lint command path used for repository lint governance. |
| LINT-CONFIG-001 | golangci.yml | Lint Policy | MUST | Lint configuration MUST enforce strict suppression governance (`nolintlint` require-specific/explanation/used) and include determinism/supply-hardening linters (`forbidigo`, `depguard`, `bidichk`, `asciicheck`, `gocognit`, `copyloopvar`, `durationcheck`, `makezero`). |
| LINT-NOLINT-001 | golangci.yml + source tree | Suppression Discipline | MUST | Every `//nolint` directive MUST be linter-specific, MUST NOT use blanket `all`, and MUST include an explicit requirement-ID rationale. |
| LINT-NOLINT-002 | conformance/nolint_inventory.tsv + conformance/harness_test.go | Suppression Evidence | MUST | Conformance gates MUST mechanically gather a complete inventory of every `//nolint` directive, including file, line, linter list, requirement IDs, rationale text, and directive text, and MUST fail on drift from the checked-in inventory artifact. |

## OFFLINE: Cold Replay Assurance

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| OFFLINE-MATRIX-001 | offline/README.md | Contracts | MUST | Offline replay matrix manifest (`offline/matrix.yaml`) MUST exist, parse, and include both `container` and `vm` lanes. |
| OFFLINE-COLD-001 | offline/README.md | Contracts | MUST | Maximal offline profile (`offline/profiles/maximal.yaml`) MUST enforce at least 5 cold replays per required lane and `hard_release_gate: true`. |
| OFFLINE-BUNDLE-001 | offline/README.md + offline/replay/bundle.go | Contracts | MUST | Offline replay bundle creation MUST remain byte-deterministic for identical inputs. |
| OFFLINE-EVIDENCE-001 | offline/README.md | Contracts | MUST | Offline evidence schema v1 (`offline/schema/evidence.v1.json`) and `verify-evidence` validation path MUST exist and remain executable, including source binding fields (`source_git_commit`, `source_git_tag`). |
| OFFLINE-EVIDENCE-002 | offline/README.md | Contracts | MUST | Offline evidence schema v1 (`offline/schema/evidence.v1.json`) MUST remain the only evidence schema and MUST carry the full conformance DAG binding surface: optional infrastructure manifest binding fields (`infra_manifest_sha256`, `infra_repo_url`, `infra_repo_commit`), optional discovered substrate fields (`discovered_cpu`, `discovered_kernel`, `image_digest`), and optional native-host measurement fields (`measured_architecture`, `measured_os_id`, `measured_os_version_id`, `measured_kernel`, `measured_cpu`, `aws_instance_id`, `aws_image_id`, `transport_attestation_sha256`). Validation MUST enforce those fields fail-closed when the selected profile/matrix requires infra-substrate-binding or native-host attestation. |
| OFFLINE-INFRA-001 | offline/README.md | Contracts | MUST | Infrastructure manifest schema (`offline/schema/infra-manifest.v1.json`) MUST exist and validate IaC repo identity (`infra_repo_url`, `infra_repo_commit`), provider lock digest (`provider_lock_sha256`), per-host substrate records (`role`, `cloud_provider`, `region`, `instance_type`, `image_id`), verified native-host attestation fields (`iid_document_sha256`, `iid_signature_sha256`, `iid_pkcs7_sha256`, `iid_verified`), and the pinned tool artifacts used to build/provision/run the server-backed evidence path. |
| OFFLINE-INFRA-002 | offline/README.md + offline/schema/infra-manifest.v1.json | Contracts | MUST | `infra-manifest.v1` documentation and schema annotations MUST distinguish AWS-IID-backed identity fields from self-reported host measurements. |
| OFFLINE-TOOLCHAIN-001 | offline/README.md + docs/adr/ADR-0006-pinned-toolchain-evidence.md | Contracts | MUST | Server-backed offline evidence generation and release validation MUST consume `offline/toolchain.lock.tsv`, download each referenced tool artifact from its pinned URL, verify its SHA-256, and record the exact verified artifacts in `infra-manifest.v1.json`. |
| OFFLINE-AUTO-001 | CONTRIBUTING.md + docs/OFFLINE_REPLAY_HARNESS.md + docs/adr/ADR-0007-go-native-server-evidence-orchestration.md | Contracts | MUST | After pinned Go bootstrap, release-critical server-backed automation MUST execute through Go-native `jcs-offline-replay` subcommands (`init-infra-lock`, `server-evidence`). Shell entrypoints are limited to pinned-Go bootstrap and invoking those Go subcommands. |
| OFFLINE-SOURCE-001 | CONTRIBUTING.md + cmd/jcs-offline-replay/server_evidence.go | Contracts | MUST | Server-backed evidence generation MUST reject dirty source trees before billed orchestration begins. |
| OFFLINE-SOURCE-002 | CONTRIBUTING.md + cmd/jcs-offline-replay/server_evidence.go | Contracts | MUST | Server-backed evidence generation MUST build from a detached source worktree at an exact recorded commit. |
| OFFLINE-SOURCE-003 | cmd/jcs-offline-replay/server_evidence.go | Contracts | MUST | Server-backed evidence generation MUST fail closed if the detached source worktree HEAD does not match the recorded source commit. |
| OFFLINE-SERVER-001 | offline/README.md | Contracts | MUST | Server-backed offline profiles (`offline/profiles/server-linux-x86_64.yaml`, `offline/profiles/server-linux-arm64.yaml`) MUST include `infra-substrate-binding` in `required_suites`, MUST enforce `hard_release_gate: true` and `min_cold_replays >= 5`, and validation MUST reject any evidence artifact that omits the required v1 infra-binding, native-host measurement, or transport-attestation fields for those profiles. |
| OFFLINE-GATE-001 | CONTRIBUTING.md | Release Process | MUST | Release process MUST include explicit offline replay evidence gate execution via `go test ./offline/conformance` for both `x86_64` and `arm64` matrix/profile contracts and MUST bind evidence to the expected release commit/tag. |
| OFFLINE-ARCH-001 | offline/matrix.yaml + offline/matrix.arm64.yaml | Profile | MUST | Release architecture scope MUST be explicit and constrained to the supported set: `x86_64` and `arm64`. |
| OFFLINE-LOCAL-001 | offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Operator Workflow | MUST | Local operators MUST have a Go-native `jcs-offline-replay cross-arch` workflow that can execute offline vector gates, including the optional official ES6 100,000,000-line gate. |
| OFFLINE-RECOVERY-001 | CONTRIBUTING.md + offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Operator Workflow | MUST | Server-backed evidence runs MUST emit `server-run.v1.json` as the recovery/audit anchor. |
| OFFLINE-RECOVERY-002 | CONTRIBUTING.md + offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Operator Workflow | MUST | Server-backed evidence runs MUST provide a Go-native `jcs-offline-replay server-cleanup --run-record <path>` recovery path. |

## AWS: Official Release Evidence

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| AWS-RELEASE-001 | offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Contracts | MUST | Official AWS release matrices (`offline/matrix.server-x86_64.yaml`, `offline/matrix.server-arm64.yaml`) and profiles (`offline/profiles/server-linux-x86_64.yaml`, `offline/profiles/server-linux-arm64.yaml`) MUST be vm-only native-host contracts, MUST schedule 12 lanes / 60 total replays per architecture, MUST enforce `hard_release_gate: true`, and MUST include `infra-substrate-binding` in `required_suites`. |
| AWS-AMI-001 | infra/aws_release_hosts.json + infra/aws_release_hosts.lock.json + infra/instances.tf | Infrastructure Selectors | MUST | Official AWS release host selectors MUST remain explicit in `infra/aws_release_hosts.json`, and the release path MUST provision only pinned AMI IDs from `infra/aws_release_hosts.lock.json`. Ubuntu selector entries MUST resolve through Canonical public SSM parameter paths instead of brittle name globs, and the catalog MUST NOT declare unsupported Ubuntu 20.04 minimal ARM64 lanes. |
| AWS-NET-001 | infra/instances.tf | Infrastructure Isolation | MUST | Official AWS evidence instances MUST launch without public IP assignment. |
| AWS-NET-002 | infra/main.tf | Infrastructure Isolation | MUST | Official AWS evidence networking MUST not include an internet-gateway or NAT-gateway egress path. |
| AWS-NET-003 | infra/main.tf | Infrastructure Isolation | MUST | Official AWS evidence networking MUST expose only the required VPC endpoint / S3 prefix-list reachability for SSM-managed replay traffic. |
| AWS-OUTPUT-001 | infra/outputs.tf + cmd/jcs-offline-replay/server_evidence.go | Infrastructure Outputs | MUST | Official AWS infrastructure outputs consumed by the release evidence path MUST explicitly mark `provisioned_hosts` as `sensitive = true` when they transit provider-sensitive values, and the Go orchestrator MUST continue consuming the named JSON output contract directly. |
| AWS-STATE-001 | scripts/release-server.sh + CONTRIBUTING.md | Release State | MUST | The supported official AWS release wrapper MUST enforce remote OpenTofu state for conformant runs. |
| AWS-STATE-002 | scripts/release-server.sh + CONTRIBUTING.md | Release State | MUST | The supported official AWS release wrapper MUST require explicit remote backend coordinates before billed orchestration starts. |
| AWS-STAGING-001 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket MUST enable SSE-S3 encryption before use. |
| AWS-STAGING-002 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket MUST enable versioning before use. |
| AWS-STAGING-003 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket MUST enable full public-access blocking before use. |
| AWS-STAGING-004 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket MUST enforce bucket-owner object ownership before use. |
| AWS-STAGING-005 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket MUST enforce an HTTPS-only bucket policy before use. |
| AWS-STAGING-006 | cmd/jcs-offline-replay/server_aws.go | Artifact Transit | MUST | The official AWS staging bucket teardown MUST delete object versions and delete markers before bucket deletion. |
| AWS-TOOLCHAIN-001 | offline/toolchain.lock.tsv + offline/README.md | Supply Chain | MUST | The official AWS release toolchain lock MUST contain only the pinned host-side artifacts actually executed by the official AWS release path (`go`, `opentofu`, `jq`) and MUST NOT require remote container-runtime artifacts. |
| AWS-ATTEST-001 | cmd/jcs-offline-replay/server_attestation.go | Native-Host Attestation | MUST | Official AWS native-host evidence acceptance MUST verify transport attestation before evidence is accepted. |
| AWS-ATTEST-002 | cmd/jcs-offline-replay/server_attestation.go | Native-Host Attestation | MUST | Official AWS native-host evidence acceptance MUST verify AWS instance-identity signatures against committed pinned certificates. |
| AWS-ATTEST-003 | cmd/jcs-offline-replay/server_attestation.go | Native-Host Attestation | MUST | Official AWS native-host evidence acceptance MUST fail closed when the AWS region has no committed pinned instance-identity certificate. |
| AWS-ATTEST-004 | CONTRIBUTING.md + offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Native-Host Attestation | MUST | Operator-facing docs MUST state the currently supported pinned-region scope for AWS instance-identity verification. |
| AWS-GATE-001 | CONTRIBUTING.md + .github/workflows/release.yml | Release Process | MUST | Official release gating MUST require `evidence.v1` plus `infra-manifest.v1` for both x86_64 and arm64 AWS evidence artifacts, MUST bind `JCS_OFFLINE_INFRA_MANIFEST`, and MUST fail closed unless the full deterministic conformance DAG is present: pinned toolchain, offline harness evidence, infra binding, verified native-host measurement, transport-attestation digest binding, and release-gate verification bound to the expected source commit/tag. |
| AWS-XARCH-001 | cmd/jcs-offline-replay/server_evidence.go | Cross-Architecture Parity | MUST | The official AWS server-backed release path MUST mechanically compare x86_64 and arm64 aggregate evidence digests after both per-architecture release gates pass. |
| AWS-XARCH-002 | cmd/jcs-offline-replay/server_evidence.go + offline/README.md | Cross-Architecture Parity | MUST | The official AWS server-backed release path MUST emit a machine-readable cross-arch comparison report. |
| AWS-XARCH-003 | cmd/jcs-offline-replay/server_evidence.go + offline/README.md | Cross-Architecture Parity | MUST | The official AWS server-backed release path MUST emit a human-readable cross-arch comparison report. |
| AWS-XARCH-004 | cmd/jcs-offline-replay/server_evidence.go | Cross-Architecture Parity | MUST | The official AWS server-backed release path MUST fail closed on cross-arch aggregate digest mismatch. |

## API: Library Public API

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| API-CANON-001 | Profile | - | MUST | `jcs.Canonicalize([]byte)` MUST produce output identical to `jcstoken.Parse` followed by `jcs.Serialize`. |
| API-CANON-002 | Profile | - | MUST | `jcs.CanonicalizeWithOptions` MUST pass options through to `jcstoken.ParseWithOptions`. |

## DET: Determinism

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| DET-REPLAY-001 | Profile | - | MUST | 200 consecutive runs MUST produce byte-identical output. |
| DET-IDEMPOTENT-001 | Profile | - | MUST | parse→serialize→parse→serialize MUST be idempotent (output₁ == output₂). |
| DET-STATIC-001 | Profile | - | MUST | Binary MUST build with CGO_ENABLED=0, -trimpath, -buildvcs=false, -buildid=. |
| DET-NOSOURCE-001 | Profile | - | MUST | Core runtime implementation MUST NOT use maps for iteration order, time/random nondeterminism sources, outbound network calls, or subprocess execution. |
