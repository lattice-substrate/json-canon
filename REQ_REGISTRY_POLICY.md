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
## ECMA-VEC — Oracle Validation

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| ECMA-VEC-001 | V8 Oracle | — | MUST | All 54,445 base golden oracle vectors MUST produce byte-identical output. SHA-256: `593bdec...`. |
| ECMA-VEC-002 | V8 Oracle | — | MUST | All 231,917 stress golden oracle vectors MUST produce byte-identical output. SHA-256: `287d21a...`. |
| ECMA-VEC-003 | ECMA-262 | §6.1.6.1.20 | MUST | Boundary constants (0, -0, MIN_VALUE, MAX_VALUE, 1e-6 boundary, 1e21 boundary) MUST match expected strings. |

## OFFICIAL-VEC — Official External Reference Suites

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| OFFICIAL-VEC-001 | cyberphone/json-canonicalization | `testdata/input` + `testdata/output` + `testdata/outhex` | MUST | Vendored official Cyberphone canonicalization fixtures MUST pass byte-identical canonicalization checks. |
| OFFICIAL-VEC-002 | RFC 8785 | §3.2.3 + Appendix B | MUST | Vendored RFC 8785 example fixtures (sorting example and finite Appendix B number mappings) MUST match canonical output/format results. |
| OFFICIAL-VEC-003 | cyberphone/json-canonicalization | `testdata/numgen.go` checksum table | MUST | CI conformance gates MUST validate the official deterministic ES6 number corpus checksum at 10,000 lines (`b9f7a8e...`). |
| OFFICIAL-VEC-004 | RELEASE_PROCESS.md + .github/workflows/release.yml | release validation | MUST | Release validation MUST run the official deterministic ES6 number corpus checksum gate at 100,000,000 lines (`0f7dda6...`). |

## PROF-NUM — Number Profile Restrictions

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| PROF-NEGZ-001 | Profile | — | MUST | Lexical negative zero token (`-0`, `-0.0`, `-0e0`, etc.) MUST be rejected at parse time. |
| PROF-OFLOW-001 | IEEE 754 | §7.4 | MUST | Number tokens that overflow IEEE 754 binary64 (±Infinity result) MUST be rejected. |
| PROF-UFLOW-001 | IEEE 754 | §7.5 | MUST | Non-zero number tokens that underflow to IEEE 754 zero MUST be rejected. |

## BOUND — Resource Bounds

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| BOUND-DEPTH-001 | Profile | — | MUST | Nesting depth MUST be bounded (default: 1000). |
| BOUND-INPUT-001 | Profile | — | MUST | Input size MUST be bounded (default: 64 MiB). |
| BOUND-VALUES-001 | Profile | — | MUST | Total value count MUST be bounded (default: 1,000,000). |
| BOUND-MEMBERS-001 | Profile | — | MUST | Object member count MUST be bounded (default: 250,000). |
| BOUND-ELEMS-001 | Profile | — | MUST | Array element count MUST be bounded (default: 250,000). |
| BOUND-STRBYTES-001 | Profile | — | MUST | Decoded string byte length MUST be bounded (default: 8 MiB). |
| BOUND-NUMCHARS-001 | Profile | — | MUST | Number token character length MUST be bounded (default: 4096). |

## CLI — Command-Line Interface ABI

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| CLI-CMD-001 | ABI | — | MUST | `canonicalize` command MUST parse stdin/file, emit canonical bytes to stdout, exit 0 on success. |
| CLI-CMD-002 | ABI | — | MUST | `verify` command MUST parse, canonicalize, byte-compare, exit 0 if identical. |
| CLI-EXIT-001 | ABI | — | MUST | No command specified MUST exit 2 with usage message on stderr. |
| CLI-EXIT-002 | ABI | — | MUST | Unknown command MUST exit 2 with error on stderr. |
| CLI-EXIT-003 | ABI | — | MUST | Input/parse/profile violations MUST exit 2. |
| CLI-EXIT-004 | ABI | — | MUST | Internal I/O errors (e.g. write failure) MUST exit 10. |
| CLI-FLAG-001 | ABI | — | MUST | Unknown flags MUST be rejected with exit 2. |
| CLI-FLAG-002 | ABI | — | MUST | `--quiet` flag MUST suppress success messages on verify. |
| CLI-FLAG-003 | ABI | — | MUST | `--help`/`-h` MUST display usage and exit 0 at top-level and command-level. |
| CLI-FLAG-004 | ABI | — | MUST | `--version` MUST print a machine-parseable version string (`jcs-canon vX.Y.Z` form) and exit 0. |
| CLI-IO-001 | ABI | — | MUST | `-` argument or no file MUST read from stdin. |
| CLI-IO-002 | ABI | — | MUST | Multiple input files MUST be rejected with exit 2. |
| CLI-IO-003 | ABI | — | MUST | File and stdin MUST produce identical output for identical content. |
| CLI-IO-004 | ABI | — | MUST | `canonicalize` output goes to stdout only; stderr MUST be empty on success. |
| CLI-IO-005 | ABI | — | MUST | `verify` success MUST emit "ok\n" on stderr (unless --quiet). |
| CLI-CLASS-001 | ABI | — | MUST | CLI failure diagnostics MUST include a stable failure class token (`INVALID_*`, `CLI_USAGE`, `NOT_CANONICAL`, etc.) in stderr output. |

## ABI-PARITY — Manifest/Runtime Parity

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| ABI-PARITY-001 | ABI | — | MUST | `abi_manifest.json` command/flag surface MUST match the implemented CLI source and runtime behavior. |

## SUPPLY — Supply Chain Verification

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| SUPPLY-PIN-001 | CLAUDE | Security and Supply-Chain Requirements | MUST | All GitHub Actions workflow dependencies MUST be pinned to immutable full commit SHA references. |
| SUPPLY-PROV-001 | CLAUDE | Security and Supply-Chain Requirements | MUST | Release workflow MUST publish checksums and build provenance attestation steps. |

## GOV — Governance Durability

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| GOV-DUR-001 | CLAUDE | Infrastructure-Grade Definition | MUST | Governance durability clauses (review policy, succession policy, support policy) MUST be present in `GOVERNANCE.md` and validated by tests. |

## TRACE — Traceability Integrity

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| TRACE-LINK-001 | docs/TRACEABILITY_MODEL.md | Required Mapping | MUST | Behavior tests in runtime packages MUST be linked from `REQ_ENFORCEMENT_MATRIX.md`. |

## LINT — Lint Governance

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| LINT-CI-001 | .github/workflows/ci.yml | CI Gates | MUST | Pull-request and main-branch CI MUST execute golangci-lint via pinned action SHA, pinned linter version, and explicit `--config=golangci.yml`. |
| LINT-GATE-001 | AGENTS.md + CONTRIBUTING.md + cmd/jcs-gate/main.go | Mandatory Validation Gates | MUST | Required local validation gates MUST include the same pinned golangci-lint command path used for repository lint governance. |
| LINT-CONFIG-001 | golangci.yml | Lint Policy | MUST | Lint configuration MUST enforce strict suppression governance (`nolintlint` require-specific/explanation/used) and include determinism/supply-hardening linters (`forbidigo`, `depguard`, `bidichk`, `asciicheck`, `gocognit`, `copyloopvar`, `durationcheck`, `makezero`). |
| LINT-NOLINT-001 | golangci.yml + source tree | Suppression Discipline | MUST | Every `//nolint` directive MUST be linter-specific, MUST NOT use blanket `all`, and MUST include an explicit requirement-ID rationale. |

## OFFLINE — Cold Replay Assurance

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| OFFLINE-MATRIX-001 | offline/README.md | Contracts | MUST | Offline replay matrix manifest (`offline/matrix.yaml`) MUST exist, parse, and include both `container` and `vm` lanes. |
| OFFLINE-COLD-001 | offline/README.md | Contracts | MUST | Maximal offline profile (`offline/profiles/maximal.yaml`) MUST enforce at least 5 cold replays per required lane and `hard_release_gate: true`. |
| OFFLINE-EVIDENCE-001 | offline/README.md | Contracts | MUST | Offline evidence schema (`offline/schema/evidence.v1.json`) and `verify-evidence` validation path MUST exist and remain executable. |
| OFFLINE-GATE-001 | RELEASE_PROCESS.md | Verification Requirements | MUST | Release process MUST include explicit offline replay evidence gate execution via `go test ./offline/conformance` for both `x86_64` and `arm64` matrix/profile contracts. |
| OFFLINE-ARCH-001 | offline/matrix.yaml + offline/matrix.arm64.yaml | Profile | MUST | Release architecture scope MUST be explicit and constrained to the supported set: `x86_64` and `arm64`. |
| OFFLINE-LOCAL-001 | offline/README.md + docs/OFFLINE_REPLAY_HARNESS.md | Operator Workflow | MUST | Local operators MUST have a Go-native `jcs-offline-replay cross-arch` workflow that can execute offline vector gates, including the optional official ES6 100,000,000-line gate. |

## DET — Determinism

| ID | Spec | Section | Level | Requirement |
|----|------|---------|-------|-------------|
| DET-REPLAY-001 | Profile | — | MUST | 200 consecutive runs MUST produce byte-identical output. |
| DET-IDEMPOTENT-001 | Profile | — | MUST | parse→serialize→parse→serialize MUST be idempotent (output₁ == output₂). |
| DET-STATIC-001 | Profile | — | MUST | Binary MUST build with CGO_ENABLED=0, -trimpath, -buildvcs=false, -buildid=. |
| DET-NOSOURCE-001 | Profile | — | MUST | Core runtime implementation MUST NOT use maps for iteration order, time/random nondeterminism sources, outbound network calls, or subprocess execution. |
