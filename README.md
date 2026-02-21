# jcs-canon

Standalone RFC 8785 JSON Canonicalization Scheme (JCS) implementation in pure Go.

## Packages

- `jcstoken`: strict JSON tokenizer/parser (RFC 8259 + RFC 7493 constraints required by JCS)
- `jcs`: canonical serializer (RFC 8785)
- `jcsfloat`: ECMAScript-compatible `Number::toString` formatter for `float64`
- `cmd/jcs-canon`: CLI (`canonicalize` and `verify`)

## CLI

```bash
jcs-canon canonicalize [--quiet] [file|-]
jcs-canon verify [--quiet] [file|-]
```

Exit codes:

- `0` success
- `2` invalid input or not canonical
- `10` internal error

## Regenerate Float Goldens

```bash
node jcsfloat/testdata/generate_golden.js > jcsfloat/testdata/golden_vectors.csv
```

## Verify

```bash
go test ./... -count=1
CGO_ENABLED=0 go build -ldflags="-s -w" -o jcs-canon ./cmd/jcs-canon
```
