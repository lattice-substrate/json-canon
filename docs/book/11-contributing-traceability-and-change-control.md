[Previous: Troubleshooting](10-troubleshooting.md) | [Book Home](README.md) | [Next: FAQ](12-faq.md)

# Chapter 12: Contributing, Traceability, and Change Control

Changes are accepted only when requirements, implementation, tests, and docs
remain aligned.

## Required Workflow

For non-trivial changes:

1. classify change type,
2. identify impacted requirement IDs,
3. implement minimal coherent update with tests,
4. update traceability artifacts in the same change,
5. run required gates,
6. update changelog/docs/ADR as needed.

## Traceability Artifacts

Depending on impact, update:

- `REQ_REGISTRY_NORMATIVE.md`
- `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md`
- `abi_manifest.json`
- `FAILURE_TAXONOMY.md`
- `CHANGELOG.md`
- `docs/adr/*`

## Change Categories

1. Normative behavior change.
2. Policy/profile behavior change.
3. ABI/CLI behavior change.
4. Internal refactor.
5. Docs-only change.

Each category has different artifact update obligations; see `AGENTS.md`.

## Release Sensitivity

Any change that weakens verification or compatibility requires explicit
maintainer review and usually ADR-level justification.

## Maintainer Discipline

- Keep CI and release actions pinned.
- Do not add required non-Go compatibility gates without approved exception.
- Do not ship undocumented ABI behavior shifts.

[Previous: Troubleshooting](10-troubleshooting.md) | [Book Home](README.md) | [Next: FAQ](12-faq.md)
