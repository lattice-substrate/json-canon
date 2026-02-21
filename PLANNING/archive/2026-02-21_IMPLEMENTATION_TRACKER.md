# Implementation Tracker

Status: COMPLETE
Last Updated: 2026-02-21

## Phase A — Release Blockers (Plan §6, §17.1)

| Task ID | Plan Section | Description | Owner | Status | Evidence |
|---------|-------------|-------------|-------|--------|----------|
| INFRA-001 | §6.1 (F-001) | Freeze subcommand help output stream policy: all `--help` → stdout | Agent | done | `cmd/jcs-canon/main.go`, `main_test.go:TestRunSubcommandHelpExitZeroStdout`, `harness_test.go:checkHelpExitsZero` |
| INFRA-002 | §6.2 (F-002) | Correct stale registry references in CONTRIBUTING.md | Agent | done | `CONTRIBUTING.md` references split registries |
| INFRA-003 | §6.3 (F-003) | Add explicit private vulnerability reporting channel | Agent | done | `SECURITY.md` updated with GitHub Security Advisories and response SLAs |
| INFRA-004 | §6.4 (F-004) | Pin all third-party GitHub Actions by commit SHA | Agent | done | `ci.yml` (5 actions), `release.yml` (6 actions) — all SHA-pinned, verified via `gh api` |
| INFRA-005 | §6.5 (F-005) | Add safety invariant comments for digit buffer bounds | Agent | done | `jcsfloat/jcsfloat.go:extractDigits` — 30-byte buffer safety documented |
| INFRA-006 | §6.6 (F-006) | Extend CI Go matrix to 1.22/1.23/1.24 | Agent | done | `ci.yml` matrix, conformance/race/reproducible on 1.24.x |
| INFRA-007 | §6.7 (F-007) | Expand JSONL vector corpus (4 → 73 vectors) | Agent | done | `conformance/vectors/{core,reject,verify,offsets}.jsonl` |
| INFRA-008 | §6.8 (F-008) | Document file-read classification rationale | Agent | done | `FAILURE_TAXONOMY.md` — "File Open Classification Rationale" section |
| INFRA-009 | §6.9 (F-009) | Convert map-iterated test tables to deterministic slices | Agent | done | `jcsfloat_test.go` (4 tables), `harness_test.go` (2 locations) |
| INFRA-010 | §6.10 (F-010) | Publish parser memory behavior and amplification notes | Agent | done | `BOUNDS.md` created |

**Phase A Exit**: All 10 blocker findings closed.

## Phase B — Conformance Certainty (Plan §4, §5, §7–10, §17.2)

| Task ID | Plan Section | Description | Owner | Status | Evidence |
|---------|-------------|-------------|-------|--------|----------|
| INFRA-011 | §4 | Create standards/CITATION_INDEX.md | Agent | done | `standards/CITATION_INDEX.md` — 54 normative reqs mapped to spec clauses |
| INFRA-012 | §5.4 | Create traceability check tests (Go, not shell) | Agent | done | `harness_test.go:TestMatrixRegistryParity`, `TestMatrixImplSymbolsExist`, `TestMatrixTestSymbolsExist`, `TestRegistryIDFormat` |
| INFRA-013 | §3.6 | Create abi_manifest.json | Agent | done | `abi_manifest.json` — commands, flags, exit codes, failure classes, stream policy, compatibility |
| INFRA-014 | §10.4 | Add vector schema validator | Agent | done | `harness_test.go:TestVectorSchemaValid` — validates 73 vectors across 4 files |

**Phase B Exit**: Citation index complete. Traceability gates implemented as Go tests. Vector schema validated. ABI manifest machine-readable and CI-gated.

## Phase C — Release Trust (Plan §12, §17.3)

| Task ID | Plan Section | Description | Owner | Status | Evidence |
|---------|-------------|-------------|-------|--------|----------|
| INFRA-015 | §12.1 | Pin workflow actions by SHA | Agent | done | See INFRA-004 |
| INFRA-016 | §12.2–4 | Add checksums, attestation, and provenance | Agent | done | `release.yml`: `attest` job with `actions/attest-build-provenance@v2`, SHA256SUMS generation |
| INFRA-017 | §12.5 | Publish verification guide | Agent | done | `VERIFICATION.md` — checksum, provenance, reproducible build instructions |

**Phase C Exit**: Release pipeline emits SHA256SUMS + SLSA attestation. Verification guide published.

## Phase D — Longevity Hardening (Plan §14, §15, §17.4)

| Task ID | Plan Section | Description | Owner | Status | Evidence |
|---------|-------------|-------------|-------|--------|----------|
| INFRA-018 | §14 | Add governance docs | Agent | done | `GOVERNANCE.md` — maintainer policy, support window, deprecation policy |
| INFRA-019 | §15 | Documentation quality | Agent | done | `BOUNDS.md`, `VERIFICATION.md`, `FAILURE_TAXONOMY.md` updated, `CONTRIBUTING.md` updated, `CHANGELOG.md` updated |
| INFRA-020 | §13 | CI traceability gates | Agent | done | All traceability tests run via `go test ./conformance` — no shell scripts |

**Phase D Exit**: Governance, documentation, and CI gates complete.

## Validation Evidence

```
go vet ./...                                    → PASS
go test ./... -count=1 -timeout=20m             → PASS (6 packages)
go test ./... -race -count=1 -timeout=25m       → PASS (6 packages)
go test ./conformance -count=1 -timeout=10m -v  → PASS
  86 requirement checks
  73 conformance vectors (4 files)
  181 impl symbol references verified
  181 test symbol references verified
  86 registry IDs validated
  54 normative IDs in citation index
  ABI manifest validated (13 failure classes, 7 required keys)
```
