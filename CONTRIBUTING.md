# Contributing

## Prerequisites

- Go 1.22+
- Linux environment (project-supported platform)

## Development Workflow

1. Make focused changes with tests.
2. Run required checks locally.
3. Open a pull request with requirement IDs for behavior changes.

## Required Checks

```bash
go vet ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --config=golangci.yml
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

Single-command Go harness (includes offline evidence gate):

```bash
go run ./cmd/jcs-gate
```

Lint is a required gate and must pass before merge.

## Pre-Push Hook (Optional)

To catch vet and lint failures before pushing release tags:

```bash
git config core.hooksPath .githooks
```

This runs `go vet` and `golangci-lint` automatically when pushing `v*` tags.
Normal branch pushes are not affected.

## Tooling Policy

- Required validation and release-critical automation must be Go-native (`go test`, Go code, or Go tools).
- Do not introduce shell-script-based required gates for conformance, traceability, ABI validation, or release trust.
- Runtime packages must not introduce outbound network calls or subprocess execution.
- Exception path: shell usage requires explicit maintainer approval in the PR and a written rationale covering:
  - why a Go-native implementation is not practical,
  - why compatibility with the supported Linux environment is preserved,
  - and why the shell path does not weaken determinism or auditability.

The conformance suite includes traceability gates that verify:
- Registry/matrix parity
- Implementation and test symbol existence
- Requirement ID format compliance
- Vector schema validity
- ABI manifest integrity
- Citation index coverage

## ABI Compatibility

This project follows strict SemVer for the stable CLI ABI. The machine-readable
ABI contract is in `abi_manifest.json`.

Do not change behavior for existing commands/flags/exit codes in a minor or patch release.
If a breaking change is required, target the next major release and document migration steps.

### Review Requirements

- All changes require review by an active maintainer.
- ABI-impacting changes (commands, flags, exit codes, output format) require:
  - review from two maintainers when two or more active maintainers exist;
  - documented self-review (risk checklist + rationale) when exactly one active
    maintainer exists.
- Major version release requires explicit signoff from all active maintainers.
- Any new shell-script-based required gate requires explicit maintainer approval
  with written rationale in the PR.

## Traceability

Behavioral changes should update:
- `ARCHITECTURE.md` / `SPECIFICATION.md` / `CONFORMANCE.md` when system contract or release criteria change
- `ABI.md` (with `abi_manifest.json`) when CLI/stable ABI behavior changes
- `REQ_REGISTRY_NORMATIVE.md` and/or `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md` (for normative requirement changes)
- `docs/adr/` (for compatibility-impacting architectural decisions)
- tests and conformance checks for each affected requirement

Decisions with compatibility impact are recorded in `CHANGELOG.md` and `docs/adr/`.

## Governance

### Maintainer Responsibilities

1. Triage incoming issues within 10 business days.
2. Review pull requests within 15 business days.
3. Follow the security triage process defined in `SECURITY.md`.
4. Maintain traceability: update registries, matrix, and tests for all
   behavioral changes.
5. Enforce Go-first automation for infrastructure-critical checks; permit shell
   usage only via explicit, documented exception.
6. Enforce no-outbound-call runtime policy: no network egress or subprocess
   execution in core runtime packages.

### Maintainer Succession

- If two or more maintainers are active, inactive maintainers (6+ months) may
  be replaced by documented consensus of remaining maintainers.
- If exactly one maintainer is active and becomes inactive for 6+ months, the
  project enters maintenance-only status until a successor is appointed.

### Support Window

| Version | Support Level |
|---------|-------------|
| Pre-v1 release candidates (`v0.x.y-rcN`) | Best effort: compatibility stabilization, critical bug fixes, release process hardening |
| Current major (v1.x.y) | Full: bug fixes, security patches, compatibility maintenance |
| Previous major (v0.x.y) | Security-only: critical and high severity fixes for 12 months after current major release |
| Older versions | Unsupported |

### Deprecation Policy

1. Deprecations are announced in `CHANGELOG.md` at least one minor version
   before removal.
2. Deprecated features emit a warning to stderr when used.
3. Removal occurs only in a new major version.
4. The ABI manifest is updated to reflect deprecation status.
5. Failure class names and exit code mappings are stable and never deprecated (they are ABI).
6. Diagnostic message wording may change in any release (non-ABI).

## Release Process

### Preconditions

Before creating a release tag, all must be true:

1. Required local/CI gates are green.
2. Registry, matrix, citations, and ABI artifacts are consistent.
3. `CHANGELOG.md` includes release notes.
4. Any compatibility-impacting decisions are recorded in ADRs.
5. Pre-push hook is active (`git config core.hooksPath .githooks`) or
   `go run ./cmd/jcs-gate` has been run manually.

### Versioning

`json-canon` uses strict SemVer for stable CLI ABI.

1. Patch: bug fixes only, no ABI changes.
2. Minor: backward-compatible additions.
3. Major: required for breaking ABI changes.
4. Published version tags are immutable: never force-move, recreate, or retag an existing `vX.Y.Z`.

### Tagging and Build

1. Maintainer creates annotated tag `vX.Y.Z`.
2. CI release workflow builds Linux static artifact with deterministic flags.
3. Workflow publishes artifact bundle and `SHA256SUMS`.
4. Workflow emits build provenance attestation.

### Release Artifacts

Each release should include:

1. compressed Linux release bundle: `jcs-canon-linux-x86_64.tar.gz` (contains `jcs-canon`, `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`),
2. raw `jcs-canon` binary,
3. `SHA256SUMS`,
4. provenance attestation.

### Offline Evidence and Replay Gates

Build the release-gate control binary with the exact release workflow Go patch
version and release tag version string:

```bash
GOTOOLCHAIN=go1.24.13 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=<tag>" \
  -o /abs/path/to/release-control/jcs-canon ./cmd/jcs-canon
```

Validate both architecture gates:

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

Validate the official ES6 100M checksum gate:

```bash
JCS_OFFICIAL_ES6_ENABLE_100M=1 \
go test ./conformance -run TestOfficialES6CorpusChecksums100M -count=1 -timeout=6h
```

Expected checksum: `0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272`.

Full local offline proof (cross-arch + official vectors + 100M gate):

```bash
jcs-offline-replay cross-arch \
  --run-official-vectors \
  --run-official-es6-100m
```

### Release Evidence Generation

Release evidence must be generated with three constraints that match the
CI release workflow:

1. **Go version.** The release workflow pins Go to the version in
   `.github/workflows/release.yml` (currently `1.24.13`). The harness
   builds the control binary internally, and the CI workflow rebuilds it
   from the source commit using the pinned Go version. If the Go versions
   differ, the `control_binary_sha256` in the evidence will not match the
   CI-built binary and the release gate will fail. Use `GOTOOLCHAIN` or
   `PATH` to select the matching Go version.

2. **Version ldflags.** Pass `--version <tag>` to `cross-arch`. The
   harness builds the control binary with `-X main.version=<value>`. The
   CI workflow builds with `-X main.version=${GITHUB_REF_NAME}`. If
   these differ, the binary hashes diverge.

3. **Source identity.** Set `JCS_OFFLINE_SOURCE_GIT_TAG=<tag>` and
   `JCS_OFFLINE_SOURCE_GIT_COMMIT=$(git rev-parse HEAD)`. The tag does
   not exist yet at evidence generation time, so `git describe` returns
   "untagged" by default. The CI workflow validates that the evidence
   `source_git_tag` matches the release tag.

Full release evidence command:

```bash
GOTOOLCHAIN=go1.24.13 \
JCS_OFFLINE_SOURCE_GIT_COMMIT=$(git rev-parse HEAD) \
JCS_OFFLINE_SOURCE_GIT_TAG=<tag> \
go run ./cmd/jcs-offline-replay cross-arch \
  --version <tag> \
  --run-official-vectors \
  --run-official-es6-100m \
  --output-dir offline/runs/releases/<tag>
```

### Evidence Commit Sequence

Offline evidence records `source_git_commit` at generation time using
`git rev-parse HEAD`. The evidence files are then committed on top of that
commit, producing a new SHA. The release tag points to this evidence commit.

```
Commit A:  all code/doc changes         <- evidence records this SHA
Commit B:  evidence files only           <- tag points here
```

1. Finalize all code and documentation changes (commit A).
2. Generate offline evidence with the release evidence command above.
   Evidence binds `source_git_commit` to commit A and `source_git_tag`
   to the release tag.
3. Commit evidence files only (commit B). Exclude binaries under
   `offline/runs/**/bin/` (CI rejects tracked binaries).
4. Create tag on commit B.

Because a commit can never contain its own SHA, the evidence source commit is
often the parent of the tagged commit. The release workflow resolves source
identity from archived evidence (`source_git_commit` / `source_git_tag`) and
validates against those values.

### Release Checklist

1. Confirm CI status for target commit/tag.
2. Confirm ABI-impact classification for this version.
3. Confirm changelog accuracy and migration guidance (if applicable).
4. Validate offline replay evidence gates for both `x86_64` and `arm64`.
5. Validate official ES6 100M checksum gate.
6. Run `go run ./cmd/jcs-gate` or confirm pre-push hook is active.
7. Publish tag and release.
8. Verify checksums and attestation on published artifacts.
9. Announce release with compatibility notes.

### Rollback/Revocation

If a release is found defective or untrustworthy:

1. publish a security or corrective advisory,
2. mark release as superseded or revoked,
3. cut a corrected release,
4. document root cause and preventive actions.

## Release Verification

How to verify the authenticity and integrity of release artifacts.

### Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`) version 2.49+ for attestation verification
- `sha256sum` (Linux)

### 1. Download Artifacts

```bash
gh release download vX.Y.Z --repo lattice-substrate/json-canon --dir ./release
```

### 2. Verify Checksums

```bash
cd release
sha256sum --check SHA256SUMS
```

All listed artifacts must show `OK`. Any mismatch indicates a corrupted or
tampered artifact.

### 3. Verify Build Provenance (SLSA Attestation)

```bash
gh attestation verify ./jcs-canon-linux-x86_64.tar.gz \
  --repo lattice-substrate/json-canon
```

Successful output confirms:
- The binary was built by GitHub Actions
- The build used the repository's release workflow
- The source commit matches the tagged release

### 4. Verify Reproducible Build

```bash
git clone https://github.com/lattice-substrate/json-canon.git
cd json-canon
git checkout vX.Y.Z

CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=vX.Y.Z" \
  -o jcs-canon ./cmd/jcs-canon

sha256sum jcs-canon
```

Compare the resulting checksum against the `SHA256SUMS` file. Reproducibility
requires the same Go version and OS/arch used in CI.

### 5. Verify Offline Cold-Replay Evidence

For release candidates with offline matrix validation, verify archived evidence
bundles using the offline replay gate commands in the
[Release Process](#offline-evidence-and-replay-gates) section above.

### 6. Verify Differential Strictness (Optional)

```bash
go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1 -v
```

Reference: [docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md](docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md).

### Trust Model

| Property | Mechanism |
|----------|-----------|
| Integrity | SHA-256 checksums published with each release |
| Provenance | GitHub artifact attestation (Sigstore-based) |
| Reproducibility | Deterministic build flags, verified in CI |
| Source binding | Attestation links binary to exact source commit |

### What to Do if Verification Fails

1. **Checksum mismatch**: Do not use the binary. Re-download from the official release page.
2. **Attestation failure**: The binary may not have been produced by the official CI. Do not use it.
3. **Reproducibility mismatch**: Check that you are using the exact Go version from the release CI. File an issue if the discrepancy persists.
