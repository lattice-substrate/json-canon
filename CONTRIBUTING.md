# Contributing

## Prerequisites

- Go 1.22+

## Development Workflow

1. Make focused changes with tests.
2. Run required checks locally.
3. Open a pull request with requirement IDs for behavior changes.

## Required Checks

```bash
go vet ./...
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

The conformance suite includes traceability gates that verify:
- Registry/matrix parity
- Implementation and test symbol existence
- Requirement ID format compliance
- Vector schema validity
- ABI manifest integrity
- Citation index coverage

## ABI Compatibility Rules

This project follows strict SemVer for the stable CLI ABI. The machine-readable
ABI contract is in `abi_manifest.json`.

Do not change behavior for existing commands/flags/exit codes in a minor or patch release.
If a breaking change is required, target the next major release and document migration steps.

See `GOVERNANCE.md` for review requirements on ABI-impacting changes.

## Traceability Expectations

Behavioral changes should update:
- `REQ_REGISTRY_NORMATIVE.md` and/or `REQ_REGISTRY_POLICY.md` (see `REQ_REGISTRY.md` for index)
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md` (for normative requirement changes)
- `abi_manifest.json` (for CLI behavior changes)
- tests and conformance checks for each affected requirement
