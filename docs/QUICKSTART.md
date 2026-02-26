# Quickstart

Get started with `json-canon` in 5 minutes. Choose your path:

- [Go Library](#go-library) — embed canonicalization in your application
- [CLI](#cli) — canonicalize and verify from the command line or in pipelines

## Go Library

### Install

```bash
go get github.com/lattice-substrate/json-canon
```

### Parse and Canonicalize

```go
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

func main() {
	input := []byte(`{"b": 2, "a": 1, "c": 3.0}`)

	v, err := jcstoken.Parse(input)
	if err != nil {
		var je *jcserr.Error
		if errors.As(err, &je) {
			fmt.Printf("failure class: %s (offset %d)\n", je.Class, je.Offset)
		}
		log.Fatal(err)
	}

	canonical, err := jcs.Serialize(v)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(canonical))
	// Output: {"a":1,"b":2,"c":3}
}
```

All parse and serialize errors are `*jcserr.Error` values with a stable
`FailureClass`. Use `errors.As` to inspect the class for programmatic error
handling. See [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md) for the full class
list.

### Custom Resource Limits

For constrained environments, use `ParseWithOptions` to override default bounds:

```go
v, err := jcstoken.ParseWithOptions(input, &jcstoken.Options{
	MaxDepth:    32,       // nesting depth (default: 1,000)
	MaxValues:   10_000,   // total JSON values (default: 1,000,000)
	MaxInputSize: 1 << 20, // 1 MiB (default: 64 MiB)
})
```

All bounds and their defaults are documented in [BOUNDS.md](../BOUNDS.md).

### Verify Canonical Form

To check whether a document is already in canonical form without transforming
it, parse and re-serialize, then compare bytes:

```go
func isCanonical(input []byte) (bool, error) {
	v, err := jcstoken.Parse(input)
	if err != nil {
		return false, err
	}
	canonical, err := jcs.Serialize(v)
	if err != nil {
		return false, err
	}
	return bytes.Equal(input, canonical), nil
}
```

## CLI

### Build from Source

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o jcs-canon ./cmd/jcs-canon
```

Or download a release binary from the
[releases page](https://github.com/lattice-substrate/json-canon/releases)
and verify it with [VERIFICATION.md](../VERIFICATION.md).

### Canonicalize a File

```bash
./jcs-canon canonicalize input.json > canonical.json
```

### Canonicalize from stdin

```bash
echo '{"b":2,"a":1}' | ./jcs-canon canonicalize -
# Output: {"a":1,"b":2}
```

### Verify a File

Check whether a file is already in canonical form:

```bash
./jcs-canon verify input.json
```

On success, `verify` writes `ok` to stderr and exits `0`. On failure, it prints
a diagnostic and exits `2`.

### Check Exit Codes in Scripts

```bash
if ./jcs-canon canonicalize input.json > canonical.json; then
  echo "Canonicalized successfully"
else
  exit_code=$?
  if [ "$exit_code" -eq 2 ]; then
    echo "Input rejected (parse error, policy violation, or invalid usage)"
  elif [ "$exit_code" -eq 10 ]; then
    echo "Internal error (I/O failure)"
  fi
fi
```

### Quiet Mode

Suppress the `ok` status message from `verify`:

```bash
./jcs-canon verify --quiet input.json
```

## Next Steps

- [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md) — CI/CD patterns, error handling, migration guides
- [COMPARISON.md](COMPARISON.md) — evaluate json-canon vs alternatives
- [docs/book/README.md](book/README.md) — full engineering handbook
