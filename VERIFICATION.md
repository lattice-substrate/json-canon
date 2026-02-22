# Release Verification Guide

This document describes how to verify the authenticity and integrity of
`jcs-canon` release artifacts.

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`) version 2.49+ for attestation verification
- `sha256sum` (Linux)

## 1. Download Artifacts

Download the release artifacts from the GitHub Releases page:

```bash
gh release download vX.Y.Z --repo lattice-substrate/json-canon --dir ./release
```

## 2. Verify Checksums

```bash
cd release
sha256sum --check SHA256SUMS
```

All listed binaries must show `OK`. Any mismatch indicates a corrupted or
tampered artifact.

## 3. Verify Build Provenance (SLSA Attestation)

Each binary has a GitHub-signed build attestation proving it was built by the
repository's CI workflow from the tagged source commit.

```bash
gh attestation verify ./jcs-canon-linux/jcs-canon \
  --repo lattice-substrate/json-canon
```

Successful output confirms:
- The binary was built by GitHub Actions
- The build used the repository's release workflow
- The source commit matches the tagged release

## 4. Verify Reproducible Build

To independently verify the binary is reproducible from source:

```bash
git clone https://github.com/lattice-substrate/json-canon.git
cd json-canon
git checkout vX.Y.Z

CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=vX.Y.Z" \
  -o jcs-canon ./cmd/jcs-canon

sha256sum jcs-canon
```

Compare the resulting checksum against the `SHA256SUMS` file for your platform.
Note: reproducibility requires the same Go version and OS/arch used in CI.

## 5. Verify Offline Cold-Replay Evidence

For release candidates that include offline matrix validation, verify archived
evidence bundles for both release architectures against repository contracts:

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

This check validates:
- matrix/profile contract alignment,
- per-node cold-replay completeness,
- cross-node digest parity for canonical/verify/failure/exit outputs.

## 6. Verify Official ES6 100M Checksum Gate

Release candidates must pass the official deterministic ES6 number corpus
checksum gate at 100,000,000 lines:

```bash
JCS_OFFICIAL_ES6_ENABLE_100M=1 \
go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h
```

Expected checksum: `0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272`.

## Trust Model

| Property | Mechanism |
|----------|-----------|
| Integrity | SHA-256 checksums published with each release |
| Provenance | GitHub artifact attestation (Sigstore-based) |
| Reproducibility | Deterministic build flags, verified in CI |
| Source binding | Attestation links binary to exact source commit |

## What to Do if Verification Fails

1. **Checksum mismatch**: Do not use the binary. Re-download from the official release page.
2. **Attestation failure**: The binary may not have been produced by the official CI. Do not use it.
3. **Reproducibility mismatch**: Check that you are using the exact Go version from the release CI. File an issue if the discrepancy persists.
