# CLAUDE.md

**NEVER CHANGE LINTING RULES OR LINT CONFIGURATION FILES**

Specifications and requirement registries govern product behavior.

## Hard Constraints

1. Linux only.
2. Static release binary (`CGO_ENABLED=0`).
3. No outbound network calls or subprocess execution in core runtime.
4. Canonicalization core does not use `encoding/json`.
5. Required conformance gates are Go-native (`go test`), not shell.
6. CLI ABI follows strict SemVer.
7. No tracked compiled binaries in repo root.

## Source-of-Truth Hierarchy

1. Normative standards text.
2. `REQ_REGISTRY_NORMATIVE.md` and `REQ_REGISTRY_POLICY.md`.
3. `REQ_ENFORCEMENT_MATRIX.md` and `standards/CITATION_INDEX.md`.
4. `abi_manifest.json` and `FAILURE_TAXONOMY.md`.
5. Accepted ADRs in `docs/adr/`.
6. This file, then supporting docs (`README.md`, `CONTRIBUTING.md`, `docs/*`).

`PLANNING/` documents are non-authoritative drafts. Normative content
must be promoted to root docs or `docs/`. Archive or delete stale
planning artifacts.

## Validation Gates

Run before merge:

```bash
go vet ./...
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --config=golangci.yml
go test ./... -count=1 -timeout=20m
go test ./... -race -count=1 -timeout=25m
go test ./conformance -count=1 -timeout=10m -v
```

For ABI-sensitive changes, also verify deterministic static build:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o ./jcs-canon ./cmd/jcs-canon
```

## Change Workflow

For every non-trivial change:

1. Classify: normative, policy, ABI/CLI, internal refactor, or docs-only.
2. Identify impacted requirement IDs before editing.
3. Implement minimal change + regression tests.
4. Update traceability artifacts in the same change.
5. Run validation gates.
6. Update changelog and docs for user-visible changes.
7. Add ADR when compatibility, security, or platform policy is affected.

## Traceability

When behavior changes, update all applicable artifacts:

- `REQ_REGISTRY_NORMATIVE.md` / `REQ_REGISTRY_POLICY.md`
- `REQ_ENFORCEMENT_MATRIX.md` / `standards/CITATION_INDEX.md`
- `abi_manifest.json` / `FAILURE_TAXONOMY.md`
- `docs/adr/` (when decision-level impact exists)
- `CHANGELOG.md`

## ABI Rules

The CLI ABI surface: command names, flag semantics, exit codes, failure
classes, stdout/stderr contract, output grammar, canonical bytes.

- Patch: no ABI changes.
- Minor: additive, backward-compatible only.
- Major: required for breaking changes.
- Any ABI change updates `abi_manifest.json`, tests, and `CHANGELOG.md` together.

## Engineering Invariants

1. Identical input + options produces identical bytes.
2. Classification by root cause, not input source.
3. Bounds enforcement is explicit, stable, and test-covered.
4. No dependence on map iteration order, time, locale, or environment.
5. No panic-based control flow in production paths.
6. Conformance claims require executable evidence.

## Prohibited

1. Silent ABI changes.
2. Behavior changes without test and traceability updates.
3. Undocumented conformance claims.
4. Network or subprocess execution in core packages.
5. `encoding/json` as canonicalization engine.
6. Weakening error-class contracts without a versioning decision.
7. Nondeterministic behavior in canonical paths.
8. Required non-Go conformance gates without approved exception.

## Definition of Done

1. Tests prove correctness at the right layer.
2. Traceability complete and passing conformance gates.
3. ABI impact handled (or explicitly none, with evidence).
4. Determinism and bounds guarantees intact.
5. Documentation ships with the behavior change.
6. Security posture not degraded.

## File Map

- Product: `README.md`, `abi_manifest.json`, `FAILURE_TAXONOMY.md`, `BOUNDS.md`
- Specs: `ARCHITECTURE.md`, `ABI.md`, `SPECIFICATION.md`, `CONFORMANCE.md`
- Requirements: `REQ_REGISTRY_NORMATIVE.md`, `REQ_REGISTRY_POLICY.md`, `REQ_ENFORCEMENT_MATRIX.md`, `standards/CITATION_INDEX.md`
- Process: `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`
- Guides: `docs/README.md`, `docs/GUIDE.md`, `docs/adr/`
