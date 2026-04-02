# ADR-0005: Finalize `evidence.v1` and `infra-manifest.v1`

- ADR ID: ADR-0005
- Date: 2026-04-02
- Status: Accepted
- Deciders: Maintainers
- Related Requirements: OFFLINE-EVIDENCE-001, OFFLINE-EVIDENCE-002, OFFLINE-INFRA-001, OFFLINE-SERVER-001, AWS-GATE-001

## Context

The project’s published methodology is explicit: determinism is the product, strict
rejection is preferred to leniency, and releases are governed by an evidence chain
rather than test passage alone. The offline harness, pinned toolchain, source binding,
matrix/profile binding, replay evidence, infra binding, and release gate therefore form
one deterministic conformance DAG.

AWS native execution is an additional branch in that DAG. It does not replace the
offline harness.

Earlier implementation drift proposed extra public schema revisions to carry
infrastructure binding and native-host measurement. That was the wrong model for this
project. The original intention was to finish `evidence.v1` and `infra-manifest.v1`,
not version around unfinished work.

## Decision

1. `evidence.v1` remains the only evidence schema version.
2. `infra-manifest.v1` remains the only infrastructure manifest schema version.
3. The additional fields needed by the full conformance DAG are finalized inside those
   v1 artifacts:
   - top-level infra binding in `evidence.v1`
   - node-level discovered substrate identity in `evidence.v1`
   - node-level native-host measurement in `evidence.v1`
   - host-level native-host attestation in `infra-manifest.v1`
4. Validation strictness is driven by profile/matrix intent, not by schema version:
   - plain offline profiles require only the base evidence chain
   - infra-bound profiles additionally require the v1 infra-binding and discovered
     substrate fields
   - official AWS native-host profiles additionally require the v1 native-host
     measurement and attested manifest fields
5. Official release gating consumes `evidence.v1` plus `infra-manifest.v1` and fails
   closed unless the complete deterministic DAG is present.

## Rationale

The published methodology does not permit “mostly right” evidence. A split public schema
story introduced the wrong abstraction boundary and implied that AWS was a special
conformance system rather than one substrate in the same proof chain.

Finalizing v1 in place keeps the contract aligned with the project’s actual purpose:
one governed, auditable evidence system whose requirements become stricter as the
selected profile demands more proof.

## Consequences

- External validators need only one evidence schema and one infra-manifest schema.
- The offline harness remains first-class and mandatory.
- AWS native evidence becomes additive, not substitutive.
- Release policy can be expressed directly as fail-closed profile/matrix requirements
  instead of version branching.
