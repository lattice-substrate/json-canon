# Offline Replay Harness

This document is the operator runbook for proving deterministic behavior of
`json-canon` in offline Linux labs across distro and kernel lanes, including
cross-architecture comparison.

## 1. What This Proves

The offline harness produces executable evidence that:

1. Canonical output bytes are stable across all matrix lanes.
2. Verify behavior and exit/failure classification are stable across lanes.
3. Bundle inputs are immutable and checksum-validated before execution.
4. Replay evidence is complete, machine-validated, and auditable.

Core artifacts:

- Matrix contract: `offline/matrix.yaml` (x86_64), `offline/matrix.arm64.yaml` (arm64)
- Profile contract: `offline/profiles/maximal.yaml`, `offline/profiles/maximal.arm64.yaml`
- Evidence schema: `offline/schema/evidence.v1.json`
- Orchestrator CLI: `cmd/jcs-offline-replay`
- Worker CLI: `cmd/jcs-offline-worker`

## 2. One-Command Paths

### Single architecture full cold replay (recommended first)

```bash
./offline/scripts/cold-replay-run.sh
```

This performs:

1. static binary builds,
2. preflight checks,
3. immutable bundle creation,
4. full matrix cold replay run,
5. evidence verification,
6. audit summary generation,
7. release-gate test (for canonical x86_64 matrix/profile).

### Cross-architecture replay comparison

```bash
./offline/scripts/cold-replay-cross-arch.sh
```

This executes the full run once for x86_64 and once for arm64, then compares
aggregate digests across architectures.

## 3. Preflight and Runtime Prerequisites

### Preflight only

```bash
./offline/scripts/cold-replay-preflight.sh --matrix offline/matrix.yaml
```

### Required commands

- `go`, `tar`, `python3`
- container lanes: `docker` or `podman`
- VM lanes: `virsh`, `ssh`, `scp`

### Offline prerequisites

1. Container images in matrix are preloaded locally (no pull during run).
2. Libvirt domains and snapshots in matrix exist.
3. VM SSH targets are reachable from the control host.
4. Control host has permissions to container daemon and libvirt.

## 4. Evidence and Audit Outputs

A successful single run produces an output directory under `offline/runs/...`
with at least:

- `offline-bundle.tgz`
- `offline-evidence.json`
- `audit/audit-summary.json`
- `audit/audit-summary.md`
- `audit/bundle.sha256`
- `audit/evidence.sha256`
- `logs/*.log`
- `RUN_INDEX.txt`

Use these to audit and archive proof.

## 5. How To Read Pass/Fail

### Pass conditions

- `jcs-offline-replay verify-evidence` prints `ok`
- `audit/audit-summary.md` shows `Result: PASS`
- For cross-arch: `cross-arch-compare.md` shows `Result: PASS`

### Common failure classes

1. **preflight failures**: missing image/domain/snapshot/SSH reachability.
2. **bundle verification failures**: checksum mismatch in bundle contents.
3. **vector replay failures**: output or exit mismatch against vector contract.
4. **parity failures**: digest drift across nodes/replays/architectures.

## 6. Canonical Operator Commands

### Run full x86_64 harness to explicit directory

```bash
./offline/scripts/cold-replay-run.sh \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml \
  --output-dir offline/runs/proof-x86_64-$(date -u +%Y%m%dT%H%M%SZ)
```

### Run full arm64 harness to explicit directory

```bash
./offline/scripts/cold-replay-run.sh \
  --matrix offline/matrix.arm64.yaml \
  --profile offline/profiles/maximal.arm64.yaml \
  --output-dir offline/runs/proof-arm64-$(date -u +%Y%m%dT%H%M%SZ) \
  --skip-release-gate
```

### Standalone audit summary from existing evidence

```bash
./offline/scripts/cold-replay-audit-report.sh \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml \
  --evidence offline/runs/<run>/offline-evidence.json \
  --output-dir offline/runs/<run>/audit
```

## 7. Cross-Arch Proof Procedure

Use this exact sequence for formal parity proof:

1. Run full x86_64 harness and save output directory.
2. Run full arm64 harness and save output directory.
3. Run cross-arch compare script (or compare evidence files directly).
4. Archive:
   - both run directories,
   - cross-arch compare report,
   - matrix/profile files used,
   - release commit SHA.

Cross-arch proof is invalid if matrix/profile versions differ without explicit
change control.

## 8. Security and Isolation Notes

1. Container lanes run with `--network none`.
2. VM lanes reset via `virsh snapshot-revert` before replay.
3. Worker verifies bundle checksums before executing vectors.
4. Evidence includes per-node replay metadata and digest aggregates.

## 9. Release Gate Integration

For canonical release matrix/profile (`offline/matrix.yaml` +
`offline/profiles/maximal.yaml`):

```bash
JCS_OFFLINE_EVIDENCE=offline/runs/<run>/offline-evidence.json \
go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1
```

This is the final executable gate that the archived evidence is complete and
policy-conformant.
