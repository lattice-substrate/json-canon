# Offline Cold Replay Suite

Operator-focused entrypoint for offline replay proof runs.

Full runbook: `docs/OFFLINE_REPLAY_HARNESS.md`

## Quick Start

### 1) Preflight

```bash
jcs-offline-replay preflight --matrix offline/matrix.yaml
```

### Matrix introspection (machine-readable)

```bash
jcs-offline-replay inspect-matrix --matrix offline/matrix.yaml
```

### 2) Single-architecture full proof run

```bash
jcs-offline-replay run-suite --matrix offline/matrix.yaml --profile offline/profiles/maximal.yaml
```

### 3) Cross-architecture full proof run

```bash
jcs-offline-replay cross-arch
```

### 4) Cross-architecture + full offline official vectors (including ES6 100M)

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m
```

## Key Contracts

- x86_64 matrix: `offline/matrix.yaml`
- arm64 matrix: `offline/matrix.arm64.yaml`
- x86_64 profile: `offline/profiles/maximal.yaml`
- arm64 profile: `offline/profiles/maximal.arm64.yaml`
- evidence schema: `offline/schema/evidence.v1.json`

## Outputs to Audit

Each full run emits an `offline/runs/...` directory containing:

- immutable bundle (`offline-bundle.tgz`)
- replay evidence (`offline-evidence.json`)
- controller logs (`logs/*.log`)
- audit summaries (`audit/audit-summary.md`, `audit/audit-summary.json`)
- checksums (`audit/bundle.sha256`, `audit/evidence.sha256`)
- run index (`RUN_INDEX.txt`)

## Release Gate

For release gate validation (x86_64 and arm64):

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
