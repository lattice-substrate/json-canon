# Usage Guide

## Build

From repository root:

```bash
go build ./...
CGO_ENABLED=0 go build -ldflags="-s -w" -o lattice-canon ./cmd/lattice-canon
```

## CLI

### Canonicalize JSON input

```bash
./lattice-canon canonicalize [--gjcs1] [--quiet] [file|-]
```

Behavior:

- Reads from `stdin` if file is omitted or `-`.
- Emits canonical JCS JSON to `stdout`.
- With `--gjcs1`, appends exactly one trailing LF.

### Verify governed file/input

```bash
./lattice-canon verify [--quiet] [file|-]
```

Behavior:

- Validates envelope constraints first.
- Parses with strict profile constraints.
- Re-serializes and byte-compares with original body.

## Exit codes

- `0`: success
- `2`: invalid input / non-canonical / profile violation
- `10`: internal runtime error

## Integration patterns

### Pipeline canonicalization

```bash
cat input.json | ./lattice-canon canonicalize > output.jcs
```

### Produce governed file

```bash
cat input.json | ./lattice-canon canonicalize --gjcs1 > output.gjcs1
```

### Pre-commit or CI gate

```bash
./lattice-canon verify --quiet path/to/file.gjcs1
```

Fail pipeline on non-zero exit code.

## Library usage (Go)

```go
package main

import (
    "fmt"
    "lattice-canon/gjcs1"
)

func main() {
    in := []byte(`{"z":3,"a":1}`)

    canonical, err := gjcs1.Canonicalize(in)
    if err != nil {
        panic(err)
    }

    governed := gjcs1.Envelope(canonical)
    if err := gjcs1.Verify(governed); err != nil {
        panic(err)
    }

    fmt.Printf("%s", governed)
}
```
