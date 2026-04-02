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
| OFFLINE-EVIDENCE-001 | offline/README.md | Contracts | MUST | Offline evidence schema v1 (`offline/schema/evidence.v1.json`) and `verify-evidence` validation path MUST exist and remain executable, including source binding fields (`source_git_commit`, `source_git_tag`). |
| OFFLINE-EVIDENCE-002 | offline/README.md | Contracts | MUST | Offline evidence schema v2 (`offline/schema/evidence.v2.json`) MUST exist with infrastructure manifest binding fields (`infra_manifest_sha256`, `infra_repo_url`, `infra_repo_commit`). Validation MUST accept both v1 and v2 evidence. Node replay items in v2 MUST permit optional discovered substrate fields (`discovered_cpu`, `discovered_kernel`, `image_digest`). |
| OFFLINE-INFRA-001 | offline/README.md | Contracts | MUST | Infrastructure manifest schema (`offline/schema/infra-manifest.v1.json`) MUST exist and validate IaC repo identity (`infra_repo_url`, `infra_repo_commit`), provider lock digest (`provider_lock_sha256`), per-host substrate records (`role`, `cloud_provider`, `region`, `instance_type`, `image_id`), and the pinned tool artifacts used to build/provision/run the server-backed evidence path. |
| OFFLINE-TOOLCHAIN-001 | offline/README.md + docs/adr/ADR-0006-pinned-toolchain-evidence.md | Contracts | MUST | Server-backed offline evidence generation and release validation MUST consume `offline/toolchain.lock.tsv`, download each referenced tool artifact from its pinned URL, verify its SHA-256, and record the exact verified artifacts in `infra-manifest.v1.json`. |
| OFFLINE-AUTO-001 | CONTRIBUTING.md + docs/OFFLINE_REPLAY_HARNESS.md + docs/adr/ADR-0007-go-native-server-evidence-orchestration.md | Contracts | MUST | After pinned Go bootstrap, release-critical server-backed automation MUST execute through Go-native `jcs-offline-replay` subcommands (`init-infra-lock`, `server-evidence`). Shell entrypoints are limited to pinned-Go bootstrap and invoking those Go subcommands. |
| OFFLINE-SERVER-001 | offline/README.md | Contracts | MUST | Server-backed offline profiles (`offline/profiles/server-linux-x86_64.yaml`, `offline/profiles/server-linux-arm64.yaml`) MUST include `infra-substrate-binding` in `required_suites`, MUST enforce `hard_release_gate: true` and `min_cold_replays >= 5`, and validation MUST reject v1 evidence when the profile requires `infra-substrate-binding`. |
| OFFLINE-GATE-001 | CONTRIBUTING.md | Release Process | MUST | Release process MUST include explicit offline replay evidence gate execution via `go test ./offline/conformance` for both `x86_64` and `arm64` matrix/profile contracts and MUST bind evidence to the expected release commit/tag. |
| OFFLINE-ARCH-001 | offline/matrix.yaml + offline/matrix.arm64.yaml | Profile | MUST | Release architecture scope MUST be explicit and constrained to the supported set: `x86_64` and `arm64`. |
| OFFLINE-LOCAL-001 | offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Operator Workflow | MUST | Local operators MUST have a Go-native `jcs-offline-replay cross-arch` workflow that can execute offline vector gates, including the optional official ES6 100,000,000-line gate. |

## AWS: Official Release Evidence

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| AWS-RELEASE-001 | offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Contracts | MUST | Official AWS release matrices (`offline/matrix.server-x86_64.yaml`, `offline/matrix.server-arm64.yaml`) and profiles (`offline/profiles/server-linux-x86_64.yaml`, `offline/profiles/server-linux-arm64.yaml`) MUST be vm-only native-host contracts, MUST schedule 12 lanes / 60 total replays per architecture, MUST enforce `hard_release_gate: true`, and MUST include `infra-substrate-binding` in `required_suites`. |
| AWS-TOOLCHAIN-001 | offline/toolchain.lock.tsv + offline/README.md | Supply Chain | MUST | The official AWS release toolchain lock MUST contain only the pinned host-side artifacts actually executed by the official AWS release path (`go`, `opentofu`, `jq`) and MUST NOT require remote container-runtime artifacts. |
| AWS-GATE-001 | CONTRIBUTING.md + .github/workflows/release.yml | Release Process | MUST | Official release gating MUST require `evidence.v2` for both x86_64 and arm64 AWS evidence artifacts, MUST bind `JCS_OFFLINE_INFRA_MANIFEST`, and MUST NOT accept `evidence.v1` as a release substitute. |

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
