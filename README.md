# jcs-canon

Deterministic RFC 8785 JSON Canonicalization Scheme (JCS) implementation in pure Go.

## Scope

- `jcstoken`: strict JSON parser (`RFC 8259` grammar + `RFC 7493` constraints + UTF-8 strictness).
- `jcs`: canonical serializer (`RFC 8785`).
- `jcsfloat`: ECMAScript-compatible `Number::toString` formatter for `float64`.
- `cmd/jcs-canon`: stable CLI ABI (`canonicalize`, `verify`) with fixed exit codes.

## CLI ABI v1

```bash
jcs-canon canonicalize [--quiet] [file|-]
jcs-canon verify [--quiet] [file|-]
```

Exit codes:

- `0` success
- `2` invalid input / profile violation / non-canonical input
- `10` internal runtime failure

Unknown options are rejected.

## Security Bounds (defaults)

- max input size: `64 MiB`
- max nesting depth: `1000`
- max JSON values: `1,000,000`
- max object members per object: `250,000`
- max array elements per array: `250,000`
- max decoded string length: `8 MiB`
- max number token length: `4096`

## Deterministic Build (Linux static-friendly)

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon
```

## Verification

```bash
go test ./... -count=1
```
