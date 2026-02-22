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
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

Single-command Go harness (includes offline evidence gate):

```bash
go run ./cmd/jcs-gate
```

## Lint Readiness

When ready to begin lint remediation, run lint explicitly against repository
rules:

```bash
golangci-lint run --config golangci.yml
```

## Tooling Policy (Infrastructure/ABI)

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

## ABI Compatibility Rules

This project follows strict SemVer for the stable CLI ABI. The machine-readable
ABI contract is in `abi_manifest.json`.

Do not change behavior for existing commands/flags/exit codes in a minor or patch release.
If a breaking change is required, target the next major release and document migration steps.

See `GOVERNANCE.md` for review requirements on ABI-impacting changes.

## Traceability Expectations

Behavioral changes should update:
- `ARCHITECTURE.md` / `SPECIFICATION.md` / `CONFORMANCE.md` when system contract or release criteria change
- `ABI.md` (with `abi_manifest.json`) when CLI/stable ABI behavior changes
- `NORMATIVE_REFERENCES.md` when normative interpretation policy changes
- `REQ_REGISTRY_NORMATIVE.md` and/or `REQ_REGISTRY_POLICY.md` (see `REQ_REGISTRY.md` for index)
- `REQ_ENFORCEMENT_MATRIX.md`
- `standards/CITATION_INDEX.md` (for normative requirement changes)
- `abi_manifest.json` (for CLI behavior changes)
- `docs/adr/` (for compatibility-impacting architectural decisions)
- tests and conformance checks for each affected requirement
