# ADR-0006: Pinned Toolchain Evidence for Server-Backed Conformance

- ADR ID: ADR-0006
- Date: 2026-04-01
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: OFFLINE-INFRA-001, OFFLINE-TOOLCHAIN-001, OFFLINE-SERVER-001

## Context

The server-backed offline evidence path already binds replay artifacts to source
identity, infrastructure identity, and observed substrate details. That is not a
complete evidence chain if the binaries used to build, provision, and execute the
run remain ambient host state.

For a load-bearing security primitive, the following gap is not acceptable:

1. `go`, `tofu`, or `jq` can vary by host without being represented in evidence.
2. Remote container runtimes can be installed by package manager, introducing
   mutable mirror state and unpinned dependency resolution.
3. The infra manifest can describe the hosts while omitting the exact binary
   artifacts that produced the evidence.

That boundary is too weak for a project that claims infrastructure-grade,
byte-level conformance and is intended for highly regulated environments.

## Decision

1. Introduce a normative pinned toolchain lock file:
   - `offline/toolchain.lock.tsv`
   - It records each required artifact by stable ID, role (`host` or `remote`),
     purpose, version, source URL, SHA-256, and executable path when applicable.

2. Server-backed evidence generation and release validation MUST consume that lock:
   - artifacts are downloaded from the pinned URLs
   - each download is verified against the pinned SHA-256 before use
   - ambient host binaries are not the authority for `go`, `opentofu`, `jq`, or
     the remote container runtime

3. `infra-manifest.v1` is extended to record the exact verified tool artifacts used
   for the run:
   - each tool entry includes its identity, source URL, SHA-256, and relative path
     to the verified artifact
   - host tools also record the relative path to the executable that was actually run

4. `scripts/release-server.sh` becomes a pinned-tool bootstrap flow:
   - bootstrap the pinned Go toolchain from `offline/toolchain.lock.tsv`
   - use pinned Go to materialize the remaining locked artifacts
   - use the pinned OpenTofu binary for provisioning
   - replace `apt-get install docker.io` with pinned Docker static bundles for
     x86_64 and arm64 replay hosts

5. The release workflow also consumes the pinned host toolchain for the offline
   evidence gate instead of relying on ambient `setup-go` + `jq`.

## Rationale

**Why a lock file rather than hard-coded script constants?**
The lock file is the normative pin set. Scripts and CLI commands consume it, but do
not redefine it. This keeps policy, auditability, and implementation aligned.

**Why record tool artifacts in the infra manifest rather than a separate manifest?**
The infra manifest is already the artifact that binds server-backed evidence to the
provisioning layer. Recording the exact tool artifacts there keeps the trust chain
legible: evidence binds the infra manifest hash, and the infra manifest binds the
toolchain.

**Why static Docker bundles instead of package-manager installs?**
Package-manager installation introduces mutable mirror state, dependency resolution,
and timing drift that are not pinned by the project. Static archives from an exact
URL plus SHA-256 provide a stronger, more reviewable boundary.

## Consequences

1. Server-backed release evidence now has a stronger and more complete evidence trail.
2. Operators must maintain `offline/toolchain.lock.tsv` when tool versions change.
3. `infra-manifest.v1` is stricter: manifests that omit tool artifacts are no longer
   conformant for the server-backed path.
4. The remaining ambient bootstrap surface is reduced to the minimal shell/runtime
   needed to fetch and verify the pinned artifacts before the evidence-bearing path begins.
