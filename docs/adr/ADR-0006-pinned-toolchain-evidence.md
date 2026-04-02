# ADR-0006: Pinned Toolchain Evidence for Official AWS Release Conformance

- ADR ID: ADR-0006
- Date: 2026-04-01
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: OFFLINE-INFRA-001, OFFLINE-TOOLCHAIN-001, AWS-RELEASE-001

## Context

The official AWS release evidence path already binds replay artifacts to source
identity, infrastructure identity, and observed substrate details. That is not a
complete evidence chain if the binaries used to build, provision, and execute the
run remain ambient host state.

For a load-bearing security primitive, the following gap is not acceptable:

1. `go`, `tofu`, or `jq` can vary by host without being represented in evidence.
2. Ambient release-critical tools can vary by host without being represented in
   evidence, even when the infrastructure itself is recorded.
3. The infra manifest can describe the hosts while omitting the exact binary
   artifacts that produced the evidence.

That boundary is too weak for a project that claims infrastructure-grade,
byte-level conformance and is intended for highly regulated environments.

## Decision

1. Introduce a normative pinned toolchain lock file:
   - `offline/toolchain.lock.tsv`
   - It records each required artifact by stable ID, role (`host` or `remote`),
     purpose, version, source URL, SHA-256, and executable path when applicable.

2. Official AWS evidence generation and release validation MUST consume that lock:
   - artifacts are downloaded from the pinned URLs
   - each download is verified against the pinned SHA-256 before use
   - ambient host binaries are not the authority for `go`, `opentofu`, or any
     other release-critical tool

3. `infra-manifest.v1` is extended to record the exact verified tool artifacts used
   for the run:
   - each tool entry includes its identity, source URL, SHA-256, and relative path
     to the verified artifact
   - host tools also record the relative path to the executable that was actually run

4. `scripts/release-server.sh` becomes a pinned-tool bootstrap flow:
   - bootstrap the pinned Go toolchain from `offline/toolchain.lock.tsv`
   - use pinned Go to materialize the remaining locked artifacts
   - use the pinned OpenTofu binary for provisioning
   - execute the pinned artifacts directly from verified paths

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

## Consequences

1. Official AWS release evidence now has a stronger and more complete evidence trail.
2. Operators must maintain `offline/toolchain.lock.tsv` when tool versions change.
3. `infra-manifest.v1` is stricter: manifests that omit tool artifacts are no longer
   conformant for the official AWS release path.
4. The remaining ambient bootstrap surface is reduced to the minimal shell/runtime
   needed to fetch and verify the pinned artifacts before the evidence-bearing path begins.
