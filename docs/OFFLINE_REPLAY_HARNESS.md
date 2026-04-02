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
- Offline evidence schema: `offline/schema/evidence.v1.json`
- Official AWS release evidence schema: `offline/schema/evidence.v2.json`
- Infra manifest schema: `offline/schema/infra-manifest.v1.json`
- Orchestrator CLI: `cmd/jcs-offline-replay`
- Worker CLI: `cmd/jcs-offline-worker`

## 2. One-Command Paths

### Single architecture full cold replay (recommended first)

```bash
jcs-offline-replay run-suite \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml
```

This performs:

1. static binary builds,
2. preflight checks,
3. immutable bundle creation,
4. full matrix cold replay run,
5. evidence verification,
6. audit summary generation,
7. release-gate test (for the selected matrix/profile architecture).

### Cross-architecture replay comparison

```bash
jcs-offline-replay cross-arch
```

This executes the full run once for x86_64 and once for arm64, then compares
aggregate digests across architectures.

## 3. Preflight and Runtime Prerequisites

### Preflight only

```bash
jcs-offline-replay preflight --matrix offline/matrix.yaml
```

### Required commands for local offline harness

- `go`, `tar`
- container lanes: `docker` or `podman`
- VM lanes: `virsh`, `ssh`, `scp`

### Offline prerequisites for local harness

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

`offline/runs/...` outputs are operator-local artifacts and are not tracked in
git.

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
jcs-offline-replay run-suite \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml \
  --output-dir offline/runs/proof-x86_64-$(date -u +%Y%m%dT%H%M%SZ)
```

### Run full arm64 harness to explicit directory

```bash
jcs-offline-replay run-suite \
  --matrix offline/matrix.arm64.yaml \
  --profile offline/profiles/maximal.arm64.yaml \
  --output-dir offline/runs/proof-arm64-$(date -u +%Y%m%dT%H%M%SZ) \
  --skip-release-gate
```

### Standalone audit summary from existing evidence

```bash
jcs-offline-replay audit-summary \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml \
  --evidence offline/runs/<run>/offline-evidence.json \
  --output-dir offline/runs/<run>/audit
```

### Cross-arch full local vector proof (includes official ES6 100M gate)

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m
```

### Release evidence generation

For release evidence, three additional constraints apply. See
[`CONTRIBUTING.md`](../CONTRIBUTING.md) for the full release evidence
command. In summary:

1. **Go toolchain** must come from the pinned artifact set in
   `offline/toolchain.lock.tsv`, not an ambient host installation.
2. **`--version <tag>`** must be passed so the control binary ldflags
   match the CI build (`-X main.version=${GITHUB_REF_NAME}`).
3. **`JCS_OFFLINE_SOURCE_GIT_TAG=<tag>`** must be set because the tag
   does not exist yet at evidence generation time.

### Official AWS release evidence

Use `./scripts/release-server.sh` when generating official AWS `evidence.v2`.
That wrapper requires `SSH_INGRESS_CIDR`, stages the pinned toolchain under
`offline/runs/releases/<tag>/toolchain/`, then invokes
`jcs-offline-replay server-evidence`, which writes `infra-manifest.v1.json` and
executes the release gates with `--infra-manifest` / `JCS_OFFLINE_INFRA_MANIFEST`.
The committed official AWS matrices are vm-only and run natively on EC2 across 12
lanes / 60 total replays per architecture.

Official AWS release evidence does not use Docker lanes. Each lane is a native EC2 host
selected from the committed AWS release host catalog and executed directly over Go-managed SSH.

Before the first billed AWS run, generate `infra/.terraform.lock.hcl` with the pinned
OpenTofu binary via the Go-native `jcs-offline-replay init-infra-lock` path:

```bash
./scripts/init-infra-lock.sh
git add infra/.terraform.lock.hcl
```

For ad hoc local materialization of the pinned binaries, use:

```bash
./scripts/bootstrap-pinned-toolchain.sh \
  --output-dir ./.tmp/pinned-toolchain \
  --env-file ./.tmp/pinned-toolchain/env.sh
```

After bootstrap, the release-critical entrypoints are:

```bash
source ./.tmp/pinned-toolchain/env.sh
"$JCS_TOOL_GO" run -mod=readonly ./cmd/jcs-offline-replay init-infra-lock
"$JCS_TOOL_GO" run -mod=readonly ./cmd/jcs-offline-replay server-evidence \
  --tag <tag> \
  --aws-region us-east-1 \
  --ssh-key-path ~/.ssh/id_rsa \
  --ssh-ingress-cidr 203.0.113.10/32 \
  --toolchain-lock ./offline/toolchain.lock.tsv \
  --toolchain-root ./offline/runs/releases/<tag>/toolchain
```

## 7. Cross-Arch Proof Procedure

Use this exact sequence for formal parity proof:

1. Run full x86_64 harness and save output directory.
2. Run full arm64 harness and save output directory.
3. Run cross-arch compare command (or compare evidence files directly).
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

For release gate validation:

Release control binaries must be built with the pinned Go artifact selected from
`offline/toolchain.lock.tsv`.

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

For official AWS `evidence.v2`, use the committed server matrix/profile pair and
set `JCS_OFFLINE_INFRA_MANIFEST=$(pwd)/offline/runs/releases/<tag>/infra-manifest.v1.json`.

**Evidence source binding model:** Evidence records `source_git_commit` at
generation time (commit A). Evidence files are then committed on top (commit B),
and the release tag points to commit B. In the standard two-commit flow,
`<release-commit-sha>` above is commit A (typically the parent of the tagged
commit). The release workflow resolves source identity from archived evidence
(`source_git_commit` / `source_git_tag`) rather than assuming `HEAD~1`.

This is the final executable gate that the archived evidence is complete and
policy-conformant.
