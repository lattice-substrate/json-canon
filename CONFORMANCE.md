# Conformance

## Purpose

This document defines what it means for `json-canon` to be conformant and
release-eligible.

## Conformance Artifacts

Conformance is defined by the union of:

- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md`
- `abi_manifest.json`
- conformance tests in `conformance/harness_test.go`
- vector corpus in `conformance/vectors/*.jsonl`
- official external fixtures in `conformance/official/`
- differential interoperability checks in `conformance/cyberphone_differential_test.go`
- offline replay contracts in `offline/matrix.yaml`, `offline/matrix.arm64.yaml`, `offline/profiles/maximal.yaml`, `offline/profiles/maximal.arm64.yaml`, and `offline/schema/evidence.v1.json`

A release is non-conformant if any artifact is inconsistent with the others.

## Required Conformance Properties

1. Every requirement ID exists in exactly one registry.
2. Every requirement ID has at least one matrix mapping row.
3. Every mapped implementation symbol exists.
4. Every mapped test symbol exists.
5. Every normative requirement has citation index coverage.
6. ABI manifest is schema-valid and behavior-consistent.
7. Vector corpus files are schema-valid and executable.

## Mandatory Validation Gates

The following gates are REQUIRED prior to merge:

```bash
go vet ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --config=golangci.yml
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

The following gate is REQUIRED for release readiness:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
```

When offline evidence is available for a release candidate, these gates are also REQUIRED:

```bash
JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/<tag>/x86_64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1

JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/<tag>/arm64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_MATRIX=$(pwd)/offline/matrix.arm64.yaml \
JCS_OFFLINE_PROFILE=$(pwd)/offline/profiles/maximal.arm64.yaml \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

Release validation MUST also execute the official deterministic ES6 checksum
gate at 100,000,000 lines:

```bash
JCS_OFFICIAL_ES6_ENABLE_100M=1 \
go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h
```

For full local offline proof (outside GitHub workflows), operators SHOULD use:

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m
```

## CI Contract

CI configuration MUST enforce at least:

1. pinned action dependencies,
2. Linux runtime validation,
3. supported Go version matrix,
4. conformance suite execution,
5. race checks,
6. reproducible-build check.

## Change Discipline for Conformance

Any behavior change MUST update all relevant artifacts in the same change set:

- registries,
- matrix mappings,
- tests,
- citations,
- ABI manifest (if CLI/ABI affected),
- changelog.

Pull requests that change behavior without traceability updates are incomplete.

## Conformance Failure Policy

1. Failing conformance gates are release blockers.
2. Temporary bypasses are prohibited for mainline and tagged releases.
3. Exception requires documented maintainer approval and time-bounded follow-up.

## Evidence Retention

The repository SHOULD retain durable conformance evidence via:

- committed requirement registries and matrix,
- versioned test vectors,
- changelog entries for behavior-affecting releases,
- ADRs for compatibility/security architecture decisions.

## Third-Party Interoperability

Project conformance is self-validated by internal executable evidence.
External interoperability checks MAY be added but MUST NOT replace internal
traceability gates.

Current differential reference example:

- `go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1`
- `docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md`
