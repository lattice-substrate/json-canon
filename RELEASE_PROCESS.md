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

1. compressed Linux release bundle: `jcs-canon-linux-x86_64.tar.gz` (contains `jcs-canon`, `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`),
2. raw `jcs-canon` binary,
3. `SHA256SUMS`,
4. provenance attestation.

## Verification Requirements

Maintainers and consumers verify integrity and provenance using `VERIFICATION.md`.

A release is incomplete if verification steps are missing or invalid.

For offline cold-replay assurance, release validation MUST include both architecture gates:

Build the release-gate control binary with the exact release workflow Go patch
version and release tag version string:

```bash
GOTOOLCHAIN=go1.24.13 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=<tag>" \
  -o /abs/path/to/release-control/jcs-canon ./cmd/jcs-canon
```

```bash
JCS_OFFLINE_EVIDENCE=/abs/path/to/offline/runs/releases/<tag>/x86_64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_MATRIX=/abs/path/to/offline/matrix.yaml \
JCS_OFFLINE_PROFILE=/abs/path/to/offline/profiles/maximal.yaml \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1

JCS_OFFLINE_EVIDENCE=/abs/path/to/offline/runs/releases/<tag>/arm64/offline-evidence.json \
JCS_OFFLINE_CONTROL_BINARY=/abs/path/to/release-control/jcs-canon \
JCS_OFFLINE_MATRIX=/abs/path/to/offline/matrix.arm64.yaml \
JCS_OFFLINE_PROFILE=/abs/path/to/offline/profiles/maximal.arm64.yaml \
JCS_OFFLINE_EXPECTED_GIT_COMMIT=<release-commit-sha> \
JCS_OFFLINE_EXPECTED_GIT_TAG=<tag> \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

Release validation MUST also include the official ES6 deterministic number
corpus checksum gate at 100,000,000 lines:

```bash
JCS_OFFICIAL_ES6_ENABLE_100M=1 \
go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h
```

Maintainers can execute the full offline local proof path (cross-arch + official vectors + 100M gate) with:

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m
```

For release tagging, move the validated cross-arch output under:

- `offline/runs/releases/<tag>/x86_64/...`
- `offline/runs/releases/<tag>/arm64/...`

and ensure `offline-evidence.json` records:

- `source_git_commit` matching the evidence generation commit (parent of the
  tagged commit),
- `source_git_tag` matching the release tag.

For interoperability regression evidence, maintainers SHOULD also run:

```bash
go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1
```

## Evidence Commit Sequence

Offline evidence records `source_git_commit` at generation time using
`git rev-parse HEAD`. The evidence files are then committed on top of that
commit, producing a new SHA. The release tag points to this evidence commit.

The correct commit sequence is:

```
Commit A:  all code/doc changes         ← evidence records this SHA
Commit B:  evidence files only           ← tag points here
```

1. Finalize all code and documentation changes (commit A).
2. Generate offline evidence with `JCS_OFFLINE_SOURCE_GIT_TAG=<tag>` — evidence
   binds `source_git_commit` to commit A.
3. Commit evidence files only (commit B).
4. Create annotated tag on commit B.

Because a commit can never contain its own SHA, the evidence source commit is
structurally always the parent of the tagged commit. The release workflow
resolves this by comparing against `HEAD~1` (commit A) rather than `HEAD`
(commit B).

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
