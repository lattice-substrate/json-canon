# ADR-0008: Governed Media Type Registry

- ADR ID: ADR-0008
- Date: 2026-04-05
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: JCS-REQ-0216, JCS-REQ-0218, JCS-REQ-0223, JCS-REQ-0224

## Context

json-canon produces and consumes governed JSON artifacts (evidence statements,
infrastructure manifests, transport attestations) that participate in a
multi-repository evidence chain governed by jcs-spec. The spec defines media
types and schema identifiers for these artifacts in Chapter 13 registries
(`registries/media-types.json`, `registries/schema-registry.json`).

json-canon previously validated `schema_version` against hardcoded constants
but had no mapping to the governing media types or schema IDs, and no
fail-closed enforcement referencing the governing requirements. This left
JCS-REQ-0223 (unknown schema identifiers MUST be rejected) and JCS-REQ-0224
(unknown media types MUST be rejected) without mechanical enforcement in the
implementation that produces and consumes the governed artifacts.

This is a regulatory compliance issue. Evidence artifacts that regulatory
regimes depend on must have every link in the identity chain mechanically
enforced. Partial acceptance is a conformance violation.

## Decision

Introduce a compile-time governed media type registry
(`offline/replay/media_types.go`) that maps each known `schema_version` to its
governing media type and schema ID. Route all `schema_version` validation
through this registry at every Load and Write boundary. Fail-closed: unknown
`schema_version` values are rejected with error messages citing the governing
requirement IDs.

The registry covers the three artifact types json-canon produces or consumes
that have entries in jcs-spec's media type registry:

- `evidence.v1` → `application/vnd.jcs.evidence.statement.v1+json`
- `infra-manifest.v1` → `application/vnd.jcs.infra.manifest.v1+json`
- `transport-attestation.v1` → `application/vnd.jcs.transport.attestation.v1+json`

Non-governed internal types (`toolchain-lock.v1`, `server-run.v1`,
`aws-release-hosts.v1`) are intentionally excluded — they have no entry in
jcs-spec's registries and validate their own `schema_version` directly.

## Rationale

The OCI distribution/image/runtime specifications established the pattern:
spec owns the namespace, implementations are black boxes that recognize known
media types and reject unknown ones, parity is enforced externally by
integration tests. This is the correct model for json-canon:

- Compile-time constants with no external loading or network calls satisfy
  json-canon's static binary and no-outbound-network constraints.
- Cross-repo parity enforcement is delegated to jcs-integration-tests,
  keeping json-canon's own tests self-contained.
- Media type is the contract boundary between repositories — no shared code,
  just agreed identifiers.
- Evidence values are compared byte-for-byte as decoded — no trimming, no
  lossy transformation. If a value needs normalization, that happens before
  it becomes evidence, not during comparison.

## Consequences

- All Load and Write functions for governed JSON artifacts reject unknown
  `schema_version` values via the registry.
- Error messages cite governing requirement IDs (JCS-REQ-0223).
- Adding a new governed artifact type requires adding an entry to the
  registry and corresponding constants.
- Toolchain lock (TSV format, no spec registry entry) is intentionally
  excluded.
- LoadEvidence() now rejects unknown `schema_version` — previously it
  returned the raw struct without checking. This closes a fail-open gap.
- WriteEvidence() and WriteTransportAttestation() now reject unknown
  `schema_version` before writing — previously writes succeeded regardless.

## Alternatives Considered

- **Load registry from jcs-spec JSON at runtime**: Rejected. Violates the
  no-outbound-network constraint and creates unnecessary runtime coupling.
  The implementation is a black box that recognizes known types at compile
  time.
- **Shared Go library between repos**: Rejected. The OCI pattern is that
  media type is the contract, not shared code. Each repo independently
  implements recognition of the governed namespace.
- **Embed jcs-spec schema files via go:embed**: Rejected. This was the
  previous approach (removed in a prior change). Embedding external files
  is a dependency. The implementation validates with its own native code.
