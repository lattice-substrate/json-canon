# Release Process

## Purpose

This document defines the maintainer release process for producing trusted,
reproducible `json-canon` releases.

## Release Preconditions

Before creating a release tag, all MUST be true:

1. Required local/CI gates are green.
2. Registry, matrix, citations, and ABI artifacts are consistent.
3. `CHANGELOG.md` includes release notes.
4. Any compatibility-impacting decisions are recorded in ADRs.

## Versioning Rules

`json-canon` uses strict SemVer for stable CLI ABI.

1. Patch: bug fixes only, no ABI changes.
2. Minor: backward-compatible additions.
3. Major: required for breaking ABI changes.

## Tagging and Build

1. Maintainer creates annotated tag `vX.Y.Z`.
2. CI release workflow builds Linux static artifact with deterministic flags.
3. Workflow publishes artifact bundle and `SHA256SUMS`.
4. Workflow emits build provenance attestation.

## Release Artifacts

Each release SHOULD include:

1. `jcs-canon` binary,
2. `SHA256SUMS`,
3. provenance attestation,
4. `LICENSE`, `NOTICE`, `README.md`, and `CHANGELOG.md` snapshot.

## Verification Requirements

Maintainers and consumers verify integrity and provenance using `VERIFICATION.md`.

A release is incomplete if verification steps are missing or invalid.

For offline cold-replay assurance, release validation MUST include both architecture gates:

```bash
JCS_OFFLINE_EVIDENCE=/path/to/x86_64/offline-evidence.json \
JCS_OFFLINE_MATRIX=/abs/path/to/offline/matrix.yaml \
JCS_OFFLINE_PROFILE=/abs/path/to/offline/profiles/maximal.yaml \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1

JCS_OFFLINE_EVIDENCE=/path/to/arm64/offline-evidence.json \
JCS_OFFLINE_MATRIX=/abs/path/to/offline/matrix.arm64.yaml \
JCS_OFFLINE_PROFILE=/abs/path/to/offline/profiles/maximal.arm64.yaml \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

Release validation MUST also include the official ES6 deterministic number
corpus checksum gate at 100,000,000 lines:

```bash
JCS_OFFICIAL_ES6_ENABLE_100M=1 \
go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h
```

## Release Checklist

1. Confirm CI status for target commit/tag.
2. Confirm ABI-impact classification for this version.
3. Confirm changelog accuracy and migration guidance (if applicable).
4. Validate offline replay evidence gates for both `x86_64` and `arm64` release matrices.
5. Validate official ES6 100,000,000-line checksum gate.
6. Publish tag and release.
7. Verify checksums and attestation on published artifacts.
8. Announce release with compatibility notes.

## Rollback/Revocation

If a release is found defective or untrustworthy:

1. publish a security or corrective advisory,
2. mark release as superseded or revoked,
3. cut a corrected release,
4. document root cause and preventive actions.

## Post-Release Maintenance

After release:

1. monitor security reports per `SECURITY.md`,
2. triage compatibility issues against ABI contract,
3. keep support policy in `GOVERNANCE.md` current.
