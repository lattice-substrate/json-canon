# Testing and Validation Strategy

## Test topology

The project currently validates behavior across five packages:

- `jcsfloat`: number formatting correctness and golden-vector conformance.
- `jcstoken`: strict parse-domain enforcement.
- `jcs`: RFC 8785 canonical serialization behavior.
- `gjcs1`: envelope verification order, canonical byte checks, and atomic write behavior.
- `cmd/lattice-canon`: CLI and exit-code contract.

## Golden vector policy

`jcsfloat/testdata/golden_vectors.csv` is generated from Node.js `String(number)` output and is the reference oracle for ECMAScript number formatting behavior.

Current expected invariants:

- Exactly `54,445` lines.
- CSV rows of `<16 hex chars>,<expected string>`.
- No header row.

Regeneration command:

```bash
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv
```

## Required release validation commands

```bash
go build ./...
go test ./... -count=1
CGO_ENABLED=0 go build -ldflags="-s -w" -o lattice-canon ./cmd/lattice-canon
```

In restricted environments (sandboxed CI), set writable caches:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -count=1
```

## Correctness properties to keep enforced

1. Canonicalization is deterministic for accepted values.
2. Verify path enforces envelope checks before parse.
3. Re-serialization mismatch is rejected as non-canonical.
4. Profile constraints (`-0`, underflow-to-zero, duplicate keys, invalid Unicode scalars) are hard failures.

## Additional production gates (recommended)

1. Add `go test -race ./...` in CI.
2. Add corpus-based fuzzing for `jcstoken.Parse` and `gjcs1.Verify`.
3. Record and publish binary and vector-file checksums for each release.
4. Add regression tests for every discovered production defect before patch release.
