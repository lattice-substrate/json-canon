---
title: "Proving Determinism: Evidence-Based Release Engineering"
published: true
tags: go, devops, programming, testing
series: "Building Infrastructure-Grade JSON Canonicalization in Go"
---

Any project can claim deterministic output. "Our tests pass" is not a determinism proof. It's a confidence signal from one machine, one OS, one kernel, at one point in time.

For a JSON canonicalization library, determinism is the product. If the same input produces different bytes on Ubuntu vs Alpine, on x86_64 vs arm64, or on the third run vs the first run, the tool is broken — regardless of what the test suite says.

This article describes how to build an offline replay harness that produces *executable evidence* of determinism across distributions, kernels, and CPU architectures, and how to gate releases on that evidence. The approach is transferable to any project where output stability matters.

## The Problem: "It Works on My Machine"

Unit tests prove that functions return correct values. Integration tests prove that components work together. Neither proves that the tool produces *identical output* across environments.

Consider what can differ between two Linux machines running the same Go binary:

- **C library**: glibc (Ubuntu, Debian, Fedora) vs musl (Alpine). Go is statically compiled, so this shouldn't matter — but "shouldn't" is not proof.
- **Kernel version**: syscall behavior, filesystem semantics, memory layout. Go's runtime abstracts these, but abstractions can leak.
- **CPU architecture**: x86_64 vs arm64. Floating-point rounding, SIMD optimizations, and endianness. Go generates architecture-specific code.
- **Runtime initialization**: Go's runtime performs memory allocation, goroutine scheduling, and GC initialization. Any of these could influence program behavior if the code contains latent non-determinism.

For a canonicalization tool, any of these differences producing a different output byte is a correctness failure. You can't prove the absence of these failures by testing on one machine. You need to test on *many* machines and compare the results.

## What Evidence Looks Like

The harness runs the tool against a fixed set of inputs on multiple nodes — different Linux distributions, container and VM execution modes, x86_64 and arm64. Each node runs the tool multiple times. Every run records SHA-256 digests of its output. At the end, a validation function checks that *every digest matches*.

The output is a JSON evidence bundle:

```json
{
  "schema_version": "evidence.v1",
  "bundle_sha256": "5654748feaa65318...",
  "control_binary_sha256": "e0296e034d1440a4...",
  "source_git_commit": "da4a4ee6fcefc4f4...",
  "source_git_tag": "v0.2.1",
  "architecture": "x86_64",
  "hard_release_gate": true,
  "node_replays": [
    {
      "node_id": "alpine320-container",
      "distro": "alpine-3.20",
      "replay_index": 1,
      "case_count": 74,
      "passed": true,
      "canonical_sha256": "2818166c21e1b445...",
      "verify_sha256": "66d329b3bd829da5...",
      "failure_class_sha256": "af58643f979138da...",
      "exit_code_sha256": "73d91ef3f2fd6d70..."
    }
  ],
  "aggregate_canonical_sha256": "2818166c21e1b445...",
  "aggregate_verify_sha256": "66d329b3bd829da5..."
}
```

When all 60 replays (12 nodes times 5 replays each) produce identical digests, the evidence shows that canonical output is byte-stable **for that specific binary across the tested matrix**.

## Proof Boundary: What This Evidence Proves

This matters for rigor. The evidence bundle does **not** claim:

- "all possible builds are deterministic,"
- "all Linux environments are covered,"
- or "future toolchains will preserve behavior automatically."

It does claim:

- this exact control binary (by SHA-256),
- built from this exact source commit,
- produced identical output across this explicit replay matrix and replay count.

That distinction is the boundary between engineering evidence and overclaiming. Determinism is established for a concrete artifact under a declared environment envelope.

## Test Bundles: Immutable Input Packages

The first requirement is *immutable inputs*. If the test data can change between runs, comparing digests is meaningless. The harness packages all inputs into a tar archive with properties that eliminate environmental variation.

### Fixed Timestamps and Ownership

```go
func writeBundleTarGz(path string, entries []bundleEntry) error {
    // ...
    fixed := time.Unix(0, 0).UTC()
    for _, e := range entries {
        hdr := &tar.Header{
            Name:    e.path,
            Mode:    e.mode,
            Size:    int64(len(e.data)),
            ModTime: fixed,   // Unix epoch: 1970-01-01T00:00:00Z
            Uid:     0,       // root
            Gid:     0,       // root
            Uname:   "root",
            Gname:   "root",
        }
        tw.WriteHeader(hdr)
        tw.Write(e.data)
    }
}
```

Every entry in the tar archive has its modification time set to the Unix epoch, its ownership set to root:root, and its permissions set by the code rather than the filesystem. This means the archive is byte-identical regardless of when, where, or by whom it was created.

### Sorted Entries

Tar archives are ordered. If entries are added in filesystem order, the archive depends on the filesystem's iteration behavior — which can vary between ext4 and xfs, between Linux kernel versions, and between NFS and local disk. The harness sorts entries lexicographically before writing:

```go
sort.Slice(entries, func(i, j int) bool {
    return entries[i].path < entries[j].path
})
```

### SHA-256 Binding

The bundle manifest records SHA-256 checksums for every component:

```go
type BundleManifest struct {
    Version         string            `json:"version"`
    BinaryPath      string            `json:"binary_path"`
    BinarySHA256    string            `json:"binary_sha256"`
    WorkerPath      string            `json:"worker_path"`
    WorkerSHA256    string            `json:"worker_sha256"`
    MatrixPath      string            `json:"matrix_path"`
    MatrixSHA256    string            `json:"matrix_sha256"`
    ProfilePath     string            `json:"profile_path"`
    ProfileSHA256   string            `json:"profile_sha256"`
    VectorFiles     []string          `json:"vector_files"`
    VectorSHA256    map[string]string `json:"vector_sha256"`
    VectorSetSHA256 string            `json:"vector_set_sha256"`
}
```

The binary, worker, matrix, profile, and each vector file have independent checksums. The `VectorSetSHA256` is a digest of all vector file checksums combined (sorted by path), so changing any single vector file changes the set digest.

The bundle archive itself also gets a SHA-256 checksum, which the evidence bundle records. This creates a chain: the evidence references the bundle by digest, the bundle references each component by digest, and the release gate validates all of these against the actual files on disk.

This chain has the same integrity property as certificate chains: modifying any component invalidates every layer above it. If someone edits a single vector file, its SHA-256 changes, which changes the vector set SHA-256, which changes the bundle manifest, which changes the bundle archive SHA-256, which no longer matches the evidence bundle's recorded value. The release gate catches this at the top of the chain without needing to know *which* component was modified.

## The Replay Matrix: Defining "Across Environments"

A matrix defines the nodes that must be tested:

```yaml
architecture: x86_64
nodes:
  - id: debian12-container
    mode: container
    distro: debian-12
    kernel_family: host
    replays: 5
  - id: alpine320-container
    mode: container
    distro: alpine-3.20
    kernel_family: host
    replays: 5
  - id: debian12-vm
    mode: vm
    distro: debian-12
    kernel_family: debian
    replays: 5
  # ... more nodes
```

Each node has a mode (container or VM), a distribution, a kernel family, and a replay count. Container nodes share the host kernel but differ in userspace (glibc vs musl, different library versions). VM nodes run their own kernels.

The distinction matters: container-mode tests prove that userspace differences don't affect output. VM-mode tests prove that kernel differences don't affect output. Together, they cover the two main sources of environmental variation on Linux.

A typical x86_64 matrix includes 12 nodes: 6 container lanes (Debian 12, Ubuntu 22.04, Fedora 40, Rocky 9, Alpine 3.20, openSUSE) and 6 VM lanes (Debian, Fedora, Rocky, Ubuntu with GA kernel, Ubuntu with HWE kernel, and a legacy LTS kernel). Each runs 5 replays, for a total of 60 independent executions.

### Environment Pinning and Reproducibility Boundaries

Matrix declarations define *which lanes* must run. Reproducibility additionally depends on how those lanes are provisioned over time. If a lane references a moving base image tag, the lane identity remains the same while underlying bits may drift.

For peer-review-grade replay reproducibility, pin lane substrates as immutable artifacts (for example: container image digests and checksummed VM base images/snapshots) and record those identifiers alongside evidence. Without that, evidence still proves parity for the observed run, but re-running months later may exercise different substrate bits.

A profile defines the policy for what constitutes a valid evidence bundle:

```yaml
name: maximal-offline-linux-x86_64
required_suites:
  - canonical-byte-stability
  - verify-parity
  - failure-class-parity
  - bounds-limit-parity
  - binary-identity
  - env-independence
  - evidence-completeness
min_cold_replays: 5
hard_release_gate: true
evidence_required: true
```

The profile enforces that every required node runs at least 5 times, that the evidence includes all required test suites, and that the evidence is a hard release gate (not advisory).

## Evidence Capture: What to Record

Each worker node runs the tool against every test vector and accumulates four independent digest streams:

1. **Canonical digest**: SHA-256 of all canonical output bytes, concatenated with vector IDs
2. **Verify digest**: SHA-256 of verify mode results (exit code, stdout, stderr per vector)
3. **Failure class digest**: SHA-256 of failure class tokens ("OK" or the class name) per vector
4. **Exit code digest**: SHA-256 of numeric exit codes per vector

These four streams capture different properties. The canonical digest proves byte-identical output. The verify digest proves that the verify command agrees. The failure class digest proves that error classification is stable. The exit code digest proves that the process-level interface is stable.

The digest accumulation works by concatenating structured records with delimiters, then computing a single SHA-256 over the entire stream. Each record includes the vector ID and the relevant output, separated by a unit separator (0x1F) and terminated by a newline. This produces a deterministic input to SHA-256 regardless of record ordering (vectors are processed in sorted order) or platform-specific line ending behavior.

Separating the four digest streams matters for diagnostics. If the canonical digest matches but the failure class digest doesn't, the tool is producing the same output but classifying errors differently — which could indicate a failure taxonomy change that wasn't intentional. If the exit code digest matches but the verify digest doesn't, the tool exits correctly but produces different stderr text — which is acceptable if the stderr change is non-stable, but should be investigated.

### Source Binding

The evidence bundle records the exact source state:

```json
{
  "source_git_commit": "da4a4ee6fcefc4f43777c76e5235d824d249807c",
  "source_git_tag": "v0.2.1",
  "control_binary_sha256": "e0296e034d1440a4aad2a3620e5663d749c2007f..."
}
```

The git commit SHA pins the source code. The binary SHA-256 pins the compiled artifact. The git tag identifies the release. Together, these create an audit trail from evidence back to source code, with no ambiguity about which code produced the evidence.

## Validation Logic: Detecting Drift

The validation function implements a simple invariant: all replays must produce identical digests.

```go
func ValidateEvidenceBundle(e *EvidenceBundle, m *Matrix, p *Profile,
    opts EvidenceValidationOptions) error {

    // ... schema version, profile match, SHA-256 format checks ...

    var baseline *NodeRunEvidence
    for _, id := range requiredNodes {
        runs := byNode[id]
        wantReplays := requiredReplayCount(matrixByID[id], p)
        if len(runs) < wantReplays {
            return fmt.Errorf("node %s has %d replays, want at least %d",
                id, len(runs), wantReplays)
        }

        for _, run := range runs {
            if baseline == nil {
                r := run
                baseline = &r
                continue
            }
            if run.CanonicalSHA256 != baseline.CanonicalSHA256 {
                return fmt.Errorf("canonical digest drift at node %s replay %d",
                    run.NodeID, run.ReplayIndex)
            }
            if run.VerifySHA256 != baseline.VerifySHA256 {
                return fmt.Errorf("verify digest drift at node %s replay %d",
                    run.NodeID, run.ReplayIndex)
            }
            if run.FailureClassSHA256 != baseline.FailureClassSHA256 {
                return fmt.Errorf("failure-class digest drift at node %s replay %d",
                    run.NodeID, run.ReplayIndex)
            }
            if run.ExitCodeSHA256 != baseline.ExitCodeSHA256 {
                return fmt.Errorf("exit-code digest drift at node %s replay %d",
                    run.NodeID, run.ReplayIndex)
            }
        }
    }

    // Aggregate digests must match baseline
    if e.AggregateCanonical != baseline.CanonicalSHA256 {
        return fmt.Errorf("aggregate canonical digest mismatch")
    }
    // ... verify, failure-class, exit-code aggregates ...
}
```

The first replay becomes the baseline. Every subsequent replay — across all nodes, all distributions, all execution modes — is compared against this baseline. Any divergence is an immediate failure with a message identifying the exact node and replay index where drift was detected.

The aggregate digests provide a summary check: four SHA-256 values that represent the behavior of the entire run. If the aggregates match the baseline's per-node digests, all nodes agreed.

### What the Validation Checks

In addition to digest parity, the validation function enforces:

- **Schema version**: Must be `evidence.v1` (enables future schema evolution)
- **Profile match**: Evidence profile name must match the policy profile
- **SHA-256 format**: All digest fields must be exactly 64 hex characters
- **Git commit format**: Must be exactly 40 hex characters
- **Architecture match**: Evidence architecture must match the matrix
- **Artifact binding**: Bundle, binary, matrix, and profile SHA-256s must match the actual files
- **Replay coverage**: Every required node must have at least the minimum replay count
- **Replay contiguity**: Replay indices must be 1, 2, 3, ..., N (no gaps)
- **Pass status**: Every replay must be marked `passed: true`
- **Suite coverage**: Required test suites must match the profile exactly

## Release Gating: The Go Test That Says No

The release gate is a standard Go test function. It loads the evidence bundle, the matrix, and the profile, then calls the validation function with expected SHA-256 values computed fresh from the actual artifacts:

```go
func TestOfflineReplayEvidenceReleaseGate(t *testing.T) {
    evidencePath := os.Getenv("JCS_OFFLINE_EVIDENCE")
    if evidencePath == "" {
        t.Skip("set JCS_OFFLINE_EVIDENCE to validate offline evidence bundle")
    }

    bundlePath := os.Getenv("JCS_OFFLINE_BUNDLE")
    controlBinaryPath := os.Getenv("JCS_OFFLINE_CONTROL_BINARY")
    // ... load matrix, profile, evidence ...

    if err := replay.ValidateEvidenceBundle(evidence, matrix, profile,
        replay.EvidenceValidationOptions{
            ExpectedBundleSHA256:        mustFileSHA256(t, bundlePath),
            ExpectedControlBinarySHA256: mustFileSHA256(t, controlBinaryPath),
            ExpectedMatrixSHA256:        mustFileSHA256(t, matrixPath),
            ExpectedProfileSHA256:       mustFileSHA256(t, profilePath),
            ExpectedArchitecture:        matrix.Architecture,
            ExpectedSourceGitCommit:     os.Getenv("JCS_OFFLINE_EXPECTED_GIT_COMMIT"),
            ExpectedSourceGitTag:        os.Getenv("JCS_OFFLINE_EXPECTED_GIT_TAG"),
        }); err != nil {
        t.Fatalf("offline evidence gate failed: %v", err)
    }
}
```

The test is gated by an environment variable. In normal development, it's skipped. During release, CI sets the variable and the test becomes a hard gate. The SHA-256 values are computed fresh from the files on disk — they're not hardcoded. This means the test validates that the evidence bundle references the *actual* artifacts being released, not some previously valid set.

### Environment Variable Binding

The release gate uses seven environment variables:

| Variable | Purpose |
|----------|---------|
| `JCS_OFFLINE_EVIDENCE` | Path to the evidence JSON file |
| `JCS_OFFLINE_BUNDLE` | Path to the tar bundle |
| `JCS_OFFLINE_CONTROL_BINARY` | Path to the release binary |
| `JCS_OFFLINE_MATRIX` | Path to the matrix YAML |
| `JCS_OFFLINE_PROFILE` | Path to the profile YAML |
| `JCS_OFFLINE_EXPECTED_GIT_COMMIT` | Expected source commit SHA |
| `JCS_OFFLINE_EXPECTED_GIT_TAG` | Expected release tag |

Most have sensible defaults (matrix and profile default to the repository's standard files). The evidence path has no default — it must be explicitly provided, which prevents accidental release without evidence.

The design as a `go test` function (rather than a standalone script) is intentional. It integrates with Go's standard testing infrastructure — `go test -v` shows progress, `-run` selects specific gates, `-count=1` disables caching. The test is part of the same codebase as the tool it validates, which means the validation logic is versioned alongside the evidence schema. And because it's a Go test, it can import the same `replay` package that generates the evidence, ensuring the validation code and generation code share type definitions.

The `t.Skip` pattern — skip when the environment variable is absent, fail when it's present but the evidence is invalid — means the gate is silent during normal development and enforced during release. Developers running `go test ./...` don't see the offline gate. The release pipeline, which sets the environment variables, does.

## Schema Versioning

The evidence schema is versioned independently from the tool version. The current schema is `evidence.v1`. The validation function checks the schema version as its first action:

```go
if e.SchemaVersion != EvidenceSchemaVersion {
    return fmt.Errorf("unsupported schema_version %q", e.SchemaVersion)
}
```

This enables schema evolution without invalidating existing evidence. A future `evidence.v2` could add new fields (node CPU architecture, memory constraints, filesystem type) without breaking the validation of v1 evidence bundles. The validation function would branch on the schema version and apply the appropriate checks for each.

The schema is also defined as a JSON Schema file (`offline/schema/evidence.v1.json`), enabling validation by external tools. Any system that consumes evidence bundles can validate them against the schema independently of the Go validation code.

## Cross-Architecture Parity

The same harness runs independently for x86_64 and arm64, each with its own matrix, profile, and evidence bundle. The CI pipeline runs both and validates both independently.

Cross-architecture parity is not *required* to match — the aggregate digests between x86_64 and arm64 are compared separately. This is a deliberate design choice. Go's runtime, floating-point behavior, and standard library may produce different intermediate results on different architectures. What matters is that each architecture is *internally* consistent: all x86_64 nodes agree with each other, and all arm64 nodes agree with each other.

If cross-architecture aggregate digests *do* match (which they do in practice for this tool, since the implementation uses pure integer arithmetic and explicit formatting), that's additional confidence. But the harness doesn't make it a requirement, because mandating it would create false failures if a future Go release changed architecture-specific behavior in a standard library function.

## What Running 5+ Times Per Node Proves

The replay count (5 per node in the maximal profile) is not arbitrary. A single run proves the tool works. Multiple runs prove it works *deterministically*.

Non-determinism in software can come from several sources: uninitialized memory, map iteration order, concurrent goroutine scheduling, time-dependent behavior, or environment-sensitive code paths. A single run may happen to hit the "correct" ordering every time. Multiple cold runs — starting from a fresh process each time, with no warm caches — increase the probability of surfacing non-deterministic behavior.

Five cold replays is a pragmatic balance between coverage and execution time. Each replay runs the full vector suite from a fresh process invocation, exercising the tool's startup path, parser initialization, and output formatting from scratch. If any of these paths contain non-deterministic behavior, five independent executions have a reasonable chance of producing divergent digests.

## Why This Matters: Evidence as a First-Class Artifact

Most release processes treat testing as a gate: tests pass, the release ships. The evidence is a test log — ephemeral, human-consumed, not structured for machine validation.

Evidence-based release engineering treats evidence as a *first-class artifact* — versioned, checksummed, machine-readable, and committed to the repository alongside the code it validates. The evidence for v0.2.1 is available at the same commit as the v0.2.1 source code. Anyone can re-validate the release gate by running a single `go test` command with the evidence path.

This approach has three practical benefits:

1. **Auditability**: The evidence bundle is a complete record of what was tested, on what environments, at what time, from what source. There's no ambiguity about whether the tests actually ran or what they covered.

2. **Reproducibility**: The bundle contains everything needed to reproduce the test — the binary, the vectors, the matrix. Re-running the harness with the same bundle should produce the same evidence (modulo wall-clock timestamps).

3. **Trust**: The SHA-256 chain from evidence to bundle to binary to source code means each layer's integrity is independently verifiable. Tampering with any component breaks the chain.

The cost is real: maintaining the harness, running replays across multiple environments, committing evidence bundles to the repository. The evidence bundle for a single architecture is approximately 1,000 lines of JSON. The bundle archive contains the test binary, worker binary, all vector files, the matrix, and the profile. Running the full matrix takes minutes, not seconds.

For most projects, this is overkill. A well-written test suite with good coverage provides sufficient confidence for application software. But "sufficient confidence" and "proof" are different claims. When your README says "byte-deterministic output," the evidence bundle makes that claim auditable. Anyone can examine the evidence, verify the SHA-256 chain, and confirm that 60 independent executions across 12 environments produced identical output.

For infrastructure that downstream systems depend on for correctness — not just convenience — this is the minimum required to make the claim credible.

The implementation described here is from [json-canon](https://github.com/lattice-substrate/json-canon), an RFC 8785 JSON canonicalization library written in Go.
