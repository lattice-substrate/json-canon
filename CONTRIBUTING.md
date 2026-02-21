# Contributing

## Prerequisites

- Go 1.22+
- POSIX shell (for local scripts)

## Development Workflow

1. Make focused changes with tests.
2. Run required checks locally.
3. Open a pull request with requirement IDs for behavior changes.

## Required Checks

```bash
go vet ./...
go test ./... -count=1 -timeout=10m
go test ./conformance -count=1 -timeout=10m
```

## ABI Compatibility Rules

This project follows strict SemVer for the stable CLI ABI.

Do not change behavior for existing commands/flags/exit codes in a minor or patch release.
If a breaking change is required, target the next major release and document migration steps.

## Traceability Expectations

Behavioral changes should update:
- `REQ_REGISTRY.md`
- `REQ_ENFORCEMENT_MATRIX.md`
- tests and conformance checks for each affected requirement
