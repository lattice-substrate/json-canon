# Testing and Validation Strategy

## Test topology

The project validates behavior across four packages:

- `jcsfloat`: number formatting correctness and golden-vector conformance.
- `jcstoken`: strict parse-domain enforcement and resource bounds.
- `jcs`: RFC 8785 serialization behavior and library misuse resistance.
- `cmd/jcs-canon`: black-box CLI contract and exit-code behavior.

## Golden vector policy

`jcsfloat/testdata/golden_vectors.csv` is vendored in-repo as a pinned oracle for ECMAScript number formatting.

Expected invariants:

- exactly `54,445` lines,
- rows are `<16 hex chars>,<expected string>`,
- no header row,
- SHA-256 checksum:
  - `593bdecbe0dccbc182bc3baf570b716887db25739fc61b7808764ecb966d5636`

These invariants are enforced in Go tests.

## Required release validation commands

```bash
go build ./...
go test ./... -count=1
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon
```

## Correctness properties to keep enforced

1. Canonicalization is deterministic for accepted values.
2. Strict profile constraints fail closed.
3. Verify path rejects any byte-level non-canonical input.
4. Unknown CLI options are rejected.
5. Resource bounds are enforced.

## Additional production gates (recommended)

1. `go test -race ./... -count=1`.
2. Periodic fuzzing for `jcstoken.Parse` and `jcs.Serialize`.
3. Publish binary and vector checksums per release.
