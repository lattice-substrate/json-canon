# Usage Guide

## Build

From repository root:

```bash
go build ./...
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon
```

## CLI

### Canonicalize

```bash
./jcs-canon canonicalize [--quiet] [file|-]
```

Behavior:

- Reads from `stdin` if file is omitted or `-`.
- Parses with strict profile validation.
- Emits canonical RFC 8785 JSON bytes to `stdout`.

### Verify canonical form

```bash
./jcs-canon verify [--quiet] [file|-]
```

Behavior:

- Parses with strict profile validation.
- Canonicalizes parsed value.
- Compares canonical bytes to original input.
- On success, emits `ok` to `stderr` unless `--quiet`.

### Exit codes

- `0`: success
- `2`: invalid input / profile violation / non-canonical bytes
- `10`: internal runtime error

## Library usage (Go)

```go
package main

import (
    "fmt"

    "jcs-canon/jcs"
    "jcs-canon/jcstoken"
)

func main() {
    in := []byte(`{"z":3,"a":1}`)

    v, err := jcstoken.Parse(in)
    if err != nil {
        panic(err)
    }

    out, err := jcs.Serialize(v)
    if err != nil {
        panic(err)
    }

    fmt.Printf("%s\n", out) // {"a":1,"z":3}
}
```
