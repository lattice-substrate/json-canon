# Traceability Model

This project uses an evidence-first traceability model for infrastructure-grade
stability and auditability.

## Scope

Traceability covers:

- normative requirements (`REQ_REGISTRY_NORMATIVE.md`)
- policy requirements (`REQ_REGISTRY_POLICY.md`)
- enforcement mappings (`REQ_ENFORCEMENT_MATRIX.md`)
- executable checks (`go test ./conformance`)

## Required Mapping

Every requirement ID in the registries must map to:

- an implementation anchor (`impl_file`, `impl_symbol`)
- an executable test anchor (`test_file`, `test_function`)
- at least one enforcement row in `REQ_ENFORCEMENT_MATRIX.md`

The matrix schema is enforced by tests and uses:

`requirement_id,domain,level,impl_file,impl_symbol,impl_line,test_file,test_function,gate`

where:

- `domain` is `normative` or `policy`
- `level` is `L1` or `L3`
- `gate` is `TEST` or `CONFORMANCE`

## Automated Gates

The conformance harness contains mandatory traceability gates:

- `TestMatrixRegistryParity`
- `TestMatrixImplSymbolsExist`
- `TestMatrixTestSymbolsExist`
- `TestRegistryIDFormat`
- `TestCitationIndexCoversNormativeRequirements`
- `TestABIManifestValid`
- `TestVectorSchemaValid`

These gates are required in CI and must pass before release.

## Source of Truth Rules

- Requirement IDs are defined only in registry files.
- Mapping rows are defined only in `REQ_ENFORCEMENT_MATRIX.md`.
- Normative clause attribution is defined in `standards/CITATION_INDEX.md`.
- CLI ABI contract is defined in `abi_manifest.json`.

## Change Discipline

Any behavioral change must update, in the same PR:

- affected requirement registry entries
- matrix rows
- tests
- citations (normative changes)
- ABI manifest (CLI changes)

PRs that leave these out are incomplete by policy.
