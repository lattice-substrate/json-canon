[Previous: CLI and ABI Contract](06-cli-and-abi.md) | [Book Home](README.md) | [Next: Security, Trust, and Threat Model](08-security-trust-and-threat-model.md)

# Chapter 8: Offline Replay and Release Gates

This chapter explains release-grade offline proof expectations.

## Why This Gate Exists

Offline replay produces deterministic evidence across matrix lanes and binds
results to matrix/profile/bundle metadata.

For release candidates, this is the practical proof layer beyond unit tests.

## Architecture Coverage

Release proof requires both architecture tracks:

1. `x86_64`: `offline/matrix.yaml` + `offline/profiles/maximal.yaml`
2. `arm64`: `offline/matrix.arm64.yaml` + `offline/profiles/maximal.arm64.yaml`

## Standard Harness Commands

Single architecture run:

```bash
./offline/scripts/cold-replay-run.sh \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml
```

Cross-architecture run:

```bash
./offline/scripts/cold-replay-cross-arch.sh \
  --x86-matrix offline/matrix.yaml \
  --x86-profile offline/profiles/maximal.yaml \
  --arm64-matrix offline/matrix.arm64.yaml \
  --arm64-profile offline/profiles/maximal.arm64.yaml \
  --output-dir "offline/runs/cross-arch-$(date -u +%Y%m%dT%H%M%SZ)"
```

## Required Release Gate Tests

```bash
GOTOOLCHAIN=go1.24.13 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=<tag>" \
  -o /abs/path/to/release-control/jcs-canon ./cmd/jcs-canon

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

## Evidence Artifacts to Archive

For each architecture run archive:

- `offline-evidence.json`
- `audit/audit-summary.md`
- `audit/audit-summary.json`
- `audit/controller-report.txt`
- `RUN_INDEX.txt`

Use immutable paths and retain artifact checksums.

## Authoritative References

- `docs/OFFLINE_REPLAY_HARNESS.md`
- `offline/README.md`
- `CONFORMANCE.md`
- `RELEASE_PROCESS.md`

[Previous: CLI and ABI Contract](06-cli-and-abi.md) | [Book Home](README.md) | [Next: Security, Trust, and Threat Model](08-security-trust-and-threat-model.md)
