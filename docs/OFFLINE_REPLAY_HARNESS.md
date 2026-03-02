# Offline Replay Harness

This document is the authoritative operator runbook for proving deterministic
behavior of `json-canon` across distro, kernel, architecture, and OS lanes.

It covers single-architecture runs, cross-architecture comparison, cross-OS
comparison, evidence generation for releases, and release gate integration.

## 1. What This Proves

The offline harness produces executable evidence that:

1. Canonical output bytes are stable across all matrix lanes.
2. Verify behavior and exit/failure classification are stable across lanes.
3. Bundle inputs are immutable and checksum-validated before execution.
4. Replay evidence is complete, machine-validated, and auditable.
5. Determinism holds across architectures (x86_64 vs arm64).
6. Determinism holds across operating systems (Linux vs Windows).

### Core Artifacts

| Artifact | Path |
|----------|------|
| x86_64 matrix | `offline/matrix.yaml` |
| arm64 matrix | `offline/matrix.arm64.yaml` |
| Windows amd64 matrix | `offline/matrix.windows-amd64.yaml` |
| Windows arm64 matrix | `offline/matrix.windows-arm64.yaml` |
| x86_64 profile | `offline/profiles/maximal.yaml` |
| arm64 profile | `offline/profiles/maximal.arm64.yaml` |
| Windows amd64 profile | `offline/profiles/maximal.windows-amd64.yaml` |
| Windows arm64 profile | `offline/profiles/maximal.windows-arm64.yaml` |
| Evidence schema | `offline/schema/evidence.v1.json` |
| Orchestrator CLI | `cmd/jcs-offline-replay` |
| Worker CLI | `cmd/jcs-offline-worker` |

### Execution Modes

The harness supports three runner modes for matrix nodes:

- **Container** (`container_command`): runs inside a Docker/Podman container
  with `--network none`. Used for Linux distro lanes.
- **VM** (`libvirt_command`): runs inside a libvirt-managed VM with snapshot
  revert before each replay. Used for Linux kernel diversity lanes.
- **Direct** (`direct_command`): runs the worker binary directly on the host
  OS without isolation. Used for Windows lanes and host-native testing. The
  `offline/scripts/replay-direct.sh` script extracts the worker from the
  bundle and executes it directly.

## 2. One-Command Paths

### Single architecture full cold replay (recommended first)

```bash
jcs-offline-replay run-suite \
  --matrix offline/matrix.yaml \
  --profile offline/profiles/maximal.yaml
```

This performs:

1. static binary builds (jcs-canon, jcs-offline-worker, jcs-offline-replay),
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

### Cross-OS replay comparison (requires existing Linux evidence)

```bash
jcs-offline-replay cross-os \
  --linux-evidence offline/runs/<run>/x86_64/offline-evidence.json
```

This cross-compiles jcs-canon and the worker for Windows (amd64 and arm64)
from the Linux host, runs `run-suite` for each Windows target, then compares
aggregate digests against the provided Linux evidence.

## 3. Preflight and Runtime Prerequisites

### Preflight only

```bash
jcs-offline-replay preflight --matrix offline/matrix.yaml
```

### Required commands

- `go`, `tar`
- Container lanes: `docker` or `podman`
- VM lanes: `virsh`, `ssh`, `scp`
- Direct lanes: no additional tools (worker runs on the host)

### Offline prerequisites

1. Container images in matrix are preloaded locally (no pull during run).
2. Libvirt domains and snapshots in matrix exist.
3. VM SSH targets are reachable from the control host.
4. Control host has permissions to container daemon and libvirt.

### Windows cross-OS prerequisites

The `cross-os` subcommand runs entirely from a Linux host. It cross-compiles
the binaries (`GOOS=windows`) and uses the direct runner to execute them.
The direct runner calls the cross-compiled Windows worker binary on the host;
this works because the harness validates byte-for-byte output determinism via
vector digests — the direct-mode execution on Linux proves that the
cross-compiled binary produces identical canonical output to the Linux-native
binary. Native Windows execution is separately validated by the
`windows_pre_release` CI job.

## 4. Evidence and Audit Outputs

A successful single run produces an output directory under `offline/runs/...`
with at least:

```
offline/runs/<run>/
  bin/
    jcs-canon(.exe)          # target binary under test
    jcs-offline-worker(.exe) # pre-built worker
    jcs-offline-replay       # host-native controller
  offline-bundle.tgz         # immutable replay inputs
  offline-evidence.json      # schema.v1 evidence with digests
  audit/
    audit-summary.json       # machine-readable audit
    audit-summary.md         # human-readable audit
    controller-report.txt    # controller output capture
    bundle.sha256            # bundle checksum
    evidence.sha256          # evidence checksum
  logs/
    build-jcs-canon.log
    build-jcs-offline-worker.log
    build-jcs-offline-replay.log
    preflight.log
    prepare.log
    run.log
    verify-evidence.log
    report.log
    audit.log
    release-gate.log
  RUN_INDEX.txt              # artifact manifest
```

For cross-arch runs, the parent directory also contains:

```
cross-arch-compare.json
cross-arch-compare.md
```

For cross-os runs, the parent directory also contains:

```
cross-os-windows-amd64-compare.json
cross-os-windows-amd64-compare.md
cross-os-windows-arm64-compare.json
cross-os-windows-arm64-compare.md
```

`offline/runs/...` outputs are operator-local artifacts and are not tracked in
git — except when promoted to `offline/runs/releases/<tag>/` for release
evidence.

## 5. How To Read Pass/Fail

### Pass conditions

- `jcs-offline-replay verify-evidence` prints `ok`
- `audit/audit-summary.md` shows `Result: PASS`
- For cross-arch: `cross-arch-compare.md` shows `Result: PASS`
- For cross-os: both `cross-os-windows-*-compare.md` files show `Result: PASS`

### Four aggregate digests compared

Cross-arch and cross-os compare these four aggregate SHA-256 digests:

1. **canonical** — canonical output bytes for all vectors across all nodes.
2. **verify** — verification output parity.
3. **failure_class** — error classification parity.
4. **exit_code** — exit code parity.

All four MUST match for a PASS result.

### Common failure classes

1. **Preflight failures**: missing image/domain/snapshot/SSH reachability.
2. **Bundle verification failures**: checksum mismatch in bundle contents.
3. **Vector replay failures**: output or exit mismatch against vector contract.
4. **Parity failures**: digest drift across nodes/replays/architectures/OSes.

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

### Cross-OS determinism proof (from Linux host)

```bash
jcs-offline-replay cross-os \
  --linux-evidence offline/runs/<cross-arch-run>/x86_64/offline-evidence.json
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

## 8. Cross-OS Proof Procedure

Use this exact sequence for cross-OS determinism proof:

1. Complete a cross-arch run (or at minimum an x86_64 run-suite) to produce
   Linux x86_64 evidence.
2. Run `cross-os` from the same Linux host, passing `--linux-evidence` pointing
   to the x86_64 evidence file from step 1.
3. The harness cross-compiles `jcs-canon` and `jcs-offline-worker` for
   `windows/amd64` and `windows/arm64` using `GOOS=windows`.
4. For each Windows target, `run-suite` executes the full pipeline:
   build, preflight, bundle, run, verify, audit, release-gate.
5. The harness compares each Windows evidence against the Linux evidence,
   producing `cross-os-windows-{amd64,arm64}-compare.{json,md}`.
6. Both comparisons must show `RESULT=PASS`.

The `cross-os` command produces output under:

```
offline/runs/cross-os-<UTC_STAMP>/
  windows_amd64/          # full run-suite output for windows/amd64
  windows_arm64/          # full run-suite output for windows/arm64
  cross-os-windows-amd64-compare.json
  cross-os-windows-amd64-compare.md
  cross-os-windows-arm64-compare.json
  cross-os-windows-arm64-compare.md
```

## 9. Security and Isolation Notes

1. Container lanes run with `--network none`.
2. VM lanes reset via `virsh snapshot-revert` before replay.
3. Direct lanes run without isolation — used for Windows cross-compilation
   verification. The worker verifies bundle checksums before executing vectors.
4. Evidence includes per-node replay metadata and digest aggregates.

## 10. Full Release Evidence Generation Procedure

This is the step-by-step operator procedure for generating the complete set of
offline evidence required for a release. Every step MUST be completed in order.

### Prerequisites

- Linux host with x86_64 and arm64 build/execution support.
- Container engine (Podman or Docker) with all matrix images preloaded.
- Libvirt with all VM domains and snapshots configured.
- Go toolchain at the release Go version (currently 1.24.13).
- All code and documentation changes finalized and committed (this is commit A).
- `CHANGELOG.md` updated for the release.

### Step 1: Run cross-arch proof (Linux x86_64 + arm64)

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m \
  --output-dir offline/runs/release-prep-$(date -u +%Y%m%dT%H%M%SZ)
```

Wait for completion. Confirm output shows `RESULT=PASS` for both architectures
and the cross-arch comparison.

Record the output directory path. It contains `x86_64/` and `arm64/`
subdirectories with evidence.

### Step 2: Run cross-OS proof (Windows amd64 + arm64)

Use the x86_64 evidence from step 1 as the `--linux-evidence` input:

```bash
jcs-offline-replay cross-os \
  --linux-evidence offline/runs/release-prep-<STAMP>/x86_64/offline-evidence.json \
  --output-dir offline/runs/release-prep-<STAMP>/cross-os
```

Wait for completion. Confirm output shows `RESULT=PASS` for both Windows
architectures.

### Step 3: Promote evidence to release directory

Create the release evidence directory and copy all four architecture outputs:

```bash
TAG=vX.Y.Z
PREP=offline/runs/release-prep-<STAMP>

mkdir -p offline/runs/releases/${TAG}
cp -r ${PREP}/x86_64     offline/runs/releases/${TAG}/x86_64
cp -r ${PREP}/arm64      offline/runs/releases/${TAG}/arm64
cp -r ${PREP}/cross-os/windows_amd64  offline/runs/releases/${TAG}/windows_amd64
cp -r ${PREP}/cross-os/windows_arm64  offline/runs/releases/${TAG}/windows_arm64
```

### Step 4: Verify evidence source identity

Confirm each evidence file records the correct source commit and tag:

```bash
for arch in x86_64 arm64 windows_amd64 windows_arm64; do
  echo "=== ${arch} ==="
  jq '.source_git_commit, .source_git_tag' \
    offline/runs/releases/${TAG}/${arch}/offline-evidence.json
done
```

- `source_git_commit` MUST match the SHA of commit A (the finalized code
  commit, which is `HEAD` at evidence generation time).
- `source_git_tag` will be `untagged` at this point (the tag does not exist
  yet). Alternatively, set `JCS_OFFLINE_SOURCE_GIT_TAG=<tag>` before
  running steps 1-2 to pre-bind the tag.

To pre-bind the tag at generation time:

```bash
export JCS_OFFLINE_SOURCE_GIT_TAG=vX.Y.Z
# then run steps 1 and 2
```

### Step 5: Commit evidence (commit B)

```bash
git add offline/runs/releases/${TAG}/
git commit -m "evidence: offline replay evidence for ${TAG}"
```

This creates commit B. The evidence inside records commit A as
`source_git_commit`.

### Step 6: Create annotated tag on commit B

```bash
git tag -a ${TAG} -m "Release ${TAG}"
```

### Step 7: Local release gate verification (optional but recommended)

Build the control binary from the evidence source commit and run all four gates
locally before pushing:

```bash
GOTOOLCHAIN=go1.24.13 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=${TAG}" \
  -o .tmp/release-control/jcs-canon ./cmd/jcs-canon

SOURCE_COMMIT=$(jq -r '.source_git_commit' \
  offline/runs/releases/${TAG}/x86_64/offline-evidence.json)

for arch in x86_64 arm64 windows_amd64 windows_arm64; do
  case ${arch} in
    x86_64)
      MATRIX=offline/matrix.yaml
      PROFILE=offline/profiles/maximal.yaml ;;
    arm64)
      MATRIX=offline/matrix.arm64.yaml
      PROFILE=offline/profiles/maximal.arm64.yaml ;;
    windows_amd64)
      MATRIX=offline/matrix.windows-amd64.yaml
      PROFILE=offline/profiles/maximal.windows-amd64.yaml ;;
    windows_arm64)
      MATRIX=offline/matrix.windows-arm64.yaml
      PROFILE=offline/profiles/maximal.windows-arm64.yaml ;;
  esac

  echo "=== release gate: ${arch} ==="
  JCS_OFFLINE_EVIDENCE=$(pwd)/offline/runs/releases/${TAG}/${arch}/offline-evidence.json \
  JCS_OFFLINE_CONTROL_BINARY=$(pwd)/.tmp/release-control/jcs-canon \
  JCS_OFFLINE_MATRIX=$(pwd)/${MATRIX} \
  JCS_OFFLINE_PROFILE=$(pwd)/${PROFILE} \
  JCS_OFFLINE_EXPECTED_GIT_COMMIT=${SOURCE_COMMIT} \
  JCS_OFFLINE_EXPECTED_GIT_TAG=${TAG} \
  go test ./offline/conformance -run TestOfflineReplayEvidenceReleaseGate -count=1 -v
done
```

All four MUST pass.

### Step 8: Push and release

```bash
git push origin main
git push origin ${TAG}
```

The CI release workflow triggers on the tag push and independently re-validates
all evidence gates (see section 11).

### Summary: two-commit model

```
Commit A:  all code/doc/changelog changes    ← evidence records this SHA
Commit B:  evidence files only               ← tag points here
```

Because a commit can never contain its own SHA, the evidence source commit is
the parent of the tagged commit in the standard two-commit release flow. The
release workflow resolves source identity directly from archived evidence
(`source_git_commit` / `source_git_tag`) and validates against those values.

## 11. CI Release Workflow Integration

When the release tag is pushed, `.github/workflows/release.yml` runs:

### pre_release job (ubuntu-latest)

1. Standard gates: vet, unit tests, race tests, conformance, ES6 100M,
   golangci-lint, fuzz.
2. Resolves `source_git_commit` and `source_git_tag` from
   `offline/runs/releases/<tag>/x86_64/offline-evidence.json`.
3. Validates the commit is a full 40-char hex SHA and the tag matches the
   release tag.
4. Builds a control binary from the exact evidence source commit via
   `git worktree add --detach`.
5. Runs `TestOfflineReplayEvidenceReleaseGate` four times — once for each
   architecture (`x86_64`, `arm64`, `windows_amd64`, `windows_arm64`).

### windows_pre_release job (windows-latest)

Runs in parallel with `pre_release`:

1. go vet
2. Unit tests (excluding conformance and offline packages)
3. Race tests (excluding conformance and offline packages)
4. Conformance tests (native Windows execution)

### Build, checksum, attestation, publish

Only after both pre-release jobs pass:

1. Linux static binary + tar.gz bundle built on ubuntu-latest.
2. Windows .exe + .zip bundles built natively on windows-latest
   (amd64 and arm64).
3. SHA256SUMS generated covering all artifacts.
4. SLSA provenance attestation emitted for each platform artifact.
5. GitHub Release published (RCs marked as prerelease).

## 12. Flags Reference

### run-suite

```
--matrix <path>           Matrix file (default: offline/matrix.yaml)
--profile <path>          Profile file (default: offline/profiles/maximal.yaml)
--output-dir <path>       Output directory (default: timestamped under offline/runs/)
--timeout <duration>      Run timeout (default: 12h)
--version <string>        Version string for binary (default: v0.0.0-dev)
--target-goos <os>        Cross-compile GOOS (e.g., "windows")
--target-goarch <arch>    Cross-compile GOARCH (e.g., "amd64")
--skip-preflight          Skip preflight checks
--skip-release-gate       Skip release gate test
```

### cross-arch

```
--x86-matrix <path>       x86_64 matrix (default: offline/matrix.yaml)
--x86-profile <path>      x86_64 profile (default: offline/profiles/maximal.yaml)
--arm64-matrix <path>     arm64 matrix (default: offline/matrix.arm64.yaml)
--arm64-profile <path>    arm64 profile (default: offline/profiles/maximal.arm64.yaml)
--output-dir <path>       Output directory
--timeout <duration>      Run timeout (default: 12h)
--version <string>        Version string
--local-no-rocky          Use local-no-rocky matrices (skip Rocky Linux lanes)
--run-official-vectors    Run official Cyberphone/RFC8785/ES6-10K vector gates
--run-official-es6-100m   Run official ES6 100M checksum gate
--skip-preflight          Skip preflight checks
--skip-release-gate       Skip release gate test
```

### cross-os

```
--linux-evidence <path>            REQUIRED: path to existing Linux x86_64 evidence
--windows-amd64-matrix <path>      Windows amd64 matrix (default: offline/matrix.windows-amd64.yaml)
--windows-amd64-profile <path>     Windows amd64 profile (default: offline/profiles/maximal.windows-amd64.yaml)
--windows-arm64-matrix <path>      Windows arm64 matrix (default: offline/matrix.windows-arm64.yaml)
--windows-arm64-profile <path>     Windows arm64 profile (default: offline/profiles/maximal.windows-arm64.yaml)
--output-dir <path>                Output directory
--timeout <duration>               Run timeout (default: 12h)
--skip-preflight                   Skip preflight checks
--skip-release-gate                Skip release gate test
```

### Other subcommands

```
prepare           Create immutable bundle from built binaries
run               Execute full matrix replay from bundle
preflight         Check matrix prerequisites (--strict / --no-strict)
audit-summary     Generate audit reports from evidence
verify-evidence   Validate evidence schema and checksums
report            Print evidence summary
inspect-matrix    Dump matrix as JSON
```

## 13. Environment Variables

| Variable | Purpose |
|----------|---------|
| `JCS_OFFLINE_SOURCE_GIT_COMMIT` | Override source commit SHA for evidence |
| `JCS_OFFLINE_SOURCE_GIT_TAG` | Override source tag for evidence |
| `JCS_OFFLINE_EVIDENCE` | Evidence path for release gate test |
| `JCS_OFFLINE_BUNDLE` | Bundle path for release gate test |
| `JCS_OFFLINE_CONTROL_BINARY` | Control binary path for release gate test |
| `JCS_OFFLINE_MATRIX` | Matrix path for release gate test |
| `JCS_OFFLINE_PROFILE` | Profile path for release gate test |
| `JCS_OFFLINE_EXPECTED_GIT_COMMIT` | Expected commit for release gate validation |
| `JCS_OFFLINE_EXPECTED_GIT_TAG` | Expected tag for release gate validation |
| `JCS_CONTAINER_ENGINE` | Override container engine (default: auto-detect podman/docker) |
| `JCS_OFFICIAL_ES6_ENABLE_100M` | Set to "1" to enable ES6 100M gate |
