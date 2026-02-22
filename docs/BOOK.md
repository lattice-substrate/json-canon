# json-canon Book

This book explains what `json-canon` is, why it exists, and how to operate it
as long-lived infrastructure.

It is intentionally practical: each chapter maps directly to commands,
contracts, and files in this repository.

## 1. What This Project Is

`json-canon` is an infrastructure-grade implementation of RFC 8785 JSON
Canonicalization Scheme (JCS).

Core properties:

1. Deterministic canonical bytes for identical input and options.
2. Strict parsing and profile enforcement for predictable behavior.
3. Stable machine-facing CLI ABI with strict SemVer rules.
4. Executable conformance and traceability evidence.

## 2. Mental Model

Think in layers:

1. `jcstoken`: strict parse and validation.
2. `jcs`: canonical serialization.
3. `jcsfloat`: ECMA-compatible binary64 string formatting.
4. `jcserr`: stable failure classes and exit mapping.
5. `cmd/jcs-canon`: CLI contract and stream behavior.

The offline replay stack (`offline/`) validates deterministic behavior across
execution lanes and architectures with signed evidence artifacts.

## 3. Quickstart

Build:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
```

Canonicalize:

```bash
echo '{"b":2,"a":1}' | ./jcs-canon canonicalize
```

Verify canonical form:

```bash
echo '{"a":1,"b":2}' | ./jcs-canon verify
```

## 4. CLI Behavior You Can Rely On

Stable commands:

1. `canonicalize`
2. `verify`

Stable global flags:

1. `--help`, `-h`
2. `--version`

Stable exit codes:

1. `0` success
2. `2` input rejection / usage violation
3. `10` internal error

Stream contract:

1. `canonicalize` output: `stdout`
2. `verify` success token (`ok`): `stderr` unless `--quiet`

See `abi_manifest.json` and `ABI.md`.

## 5. Development Workflow

Run required gates before merge:

```bash
go vet ./...
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

For release readiness also build a static binary:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
```

## 6. Offline Evidence and Release Gates

Release-quality offline validation is dual-architecture:

1. `x86_64` matrix/profile
2. `arm64` matrix/profile

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
  --arm64-profile offline/profiles/maximal.arm64.yaml
```

Release-gate evidence verification commands:

```bash
JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/<run>/offline-evidence.json \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1

JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/<run-arm64>/offline-evidence.json \
JCS_OFFLINE_MATRIX=$(pwd)/offline/matrix.arm64.yaml \
JCS_OFFLINE_PROFILE=$(pwd)/offline/profiles/maximal.arm64.yaml \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

## 7. Traceability Model

Behavior is not accepted as “done” unless requirement traceability stays
complete.

Primary artifacts:

1. `REQ_REGISTRY_NORMATIVE.md`
2. `REQ_REGISTRY_POLICY.md`
3. `REQ_ENFORCEMENT_MATRIX.md`
4. `standards/CITATION_INDEX.md`

Each behavior change must keep registry, implementation, tests, and conformance
gates aligned in the same change set.

## 8. Security Posture

Runtime constraints:

1. Linux-only supported runtime.
2. Static release binary (`CGO_ENABLED=0`).
3. Core runtime performs no outbound network calls.
4. Core runtime performs no subprocess execution.

Release trust:

1. SHA256 checksums.
2. Build provenance attestation.
3. Reproducible-build checks in CI.

See `THREAT_MODEL.md`, `SECURITY.md`, and `VERIFICATION.md`.

## 9. Troubleshooting

Common offline harness blockers:

1. Docker socket permission denied.
2. Missing container images.
3. Missing libvirt domain or missing `snapshot-cold`.
4. SSH not reachable for VM target.

Quick diagnostic commands:

```bash
./offline/scripts/cold-replay-preflight.sh --matrix offline/matrix.yaml
./offline/scripts/cold-replay-preflight.sh --matrix offline/matrix.arm64.yaml
virsh -c qemu:///system list --all
docker info
```

## 10. How To Read The Rest

Suggested order:

1. `README.md` for top-level orientation.
2. `ARCHITECTURE.md` for system boundaries.
3. `SPECIFICATION.md` for normative behavior.
4. `ABI.md` + `abi_manifest.json` for machine contract.
5. `CONFORMANCE.md` for gate policy.
6. `docs/OFFLINE_REPLAY_HARNESS.md` for replay operations.
7. `RELEASE_PROCESS.md` + `VERIFICATION.md` for release execution.
