# Guide

## Quick Start

### Go Library

Install:

```bash
go get github.com/lattice-substrate/json-canon
```

Parse and canonicalize:

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

Or use the convenience function to parse and serialize in one call:

```go
canonical, err := jcs.Canonicalize(input)
```

### CLI

Build from source:

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o jcs-canon ./cmd/jcs-canon
```

Or download a release binary from the
[releases page](https://github.com/lattice-substrate/json-canon/releases).

Canonicalize:

```bash
echo '{"b":2,"a":1}' | ./jcs-canon canonicalize -
# Output: {"a":1,"b":2}

./jcs-canon canonicalize input.json > canonical.json
```

Verify canonical form:

```bash
./jcs-canon verify input.json
# Writes "ok" to stderr and exits 0 on success
# Prints diagnostic and exits 2 on failure
```

Check exit codes in scripts:

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

Use `--quiet` to suppress the `ok` status message from `verify`:

```bash
./jcs-canon verify --quiet input.json
```

## Library Usage

### Error Handling

All errors from `jcstoken.Parse` and `jcs.Serialize` are `*jcserr.Error`
values with a stable `FailureClass`. Use `errors.As` to branch on the class:

```go
v, err := jcstoken.Parse(input)
if err != nil {
	var je *jcserr.Error
	if errors.As(err, &je) {
		switch je.Class {
		case jcserr.InvalidGrammar, jcserr.InvalidUTF8:
			// malformed input — reject
		case jcserr.BoundExceeded:
			// input too large — consider adjusting limits
		default:
			// other rejection — log class and offset
			log.Printf("%s at offset %d", je.Class, je.Offset)
		}
	}
	return err
}
```

See [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md) for the full class list.

### Custom Resource Limits

For constrained environments, use `ParseWithOptions` to override default bounds:

```go
v, err := jcstoken.ParseWithOptions(input, &jcstoken.Options{
	MaxDepth:     32,       // nesting depth (default: 1,000)
	MaxValues:    10_000,   // total JSON values (default: 1,000,000)
	MaxInputSize: 1 << 20,  // 1 MiB (default: 64 MiB)
})
```

See [BOUNDS.md](../BOUNDS.md) for all bounds and their defaults.

### Canonical Verification

Check whether bytes are already canonical without transforming:

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

### Round-Trip Hashing

Parse, serialize, and hash the canonical bytes:

```go
v, err := jcstoken.Parse(input)
if err != nil {
	return err
}
canonical, err := jcs.Serialize(v)
if err != nil {
	return err
}
// canonical is deterministic — safe for hashing or signing
hash := sha256.Sum256(canonical)
```

### HTTP Middleware

```go
func canonicalBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		v, err := jcstoken.ParseWithOptions(body, &jcstoken.Options{
			MaxInputSize: 1 << 20,
			MaxDepth:     32,
			MaxValues:    10_000,
		})
		if err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		canonical, err := jcs.Serialize(v)
		if err != nil {
			http.Error(w, "serialization error", http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(canonical))
		r.ContentLength = int64(len(canonical))
		next.ServeHTTP(w, r)
	})
}
```

## CI/CD Patterns

### Canonicalize Gate

Reject non-canonical JSON before it enters a pipeline:

```bash
./jcs-canon verify --quiet input.json || exit 1
```

### Canonicalize-then-Hash

Produce a deterministic hash from arbitrary JSON:

```bash
./jcs-canon canonicalize input.json | sha256sum
```

### Verify-before-Sign

```bash
./jcs-canon verify --quiet payload.json && sign payload.json
```

### GitHub Actions Step

```yaml
- name: Verify canonical JSON
  run: |
    ./jcs-canon verify --quiet config.json
```

## When to Use

- Signing or hashing JSON documents — you need byte-deterministic output.
- Content-addressable storage — identical content must produce identical hashes.
- Audit trails — you need reproducible, traceable canonicalization.
- Pipeline validation — strict input gates with stable exit codes for automation.

## When Not to Use

- Pretty-printing or reformatting — use `jq`.
- macOS or Windows — supported runtime is Linux only.
- Lenient parsing — `json-canon` rejects invalid JSON by design.
- General JSON processing — this is not a query engine or transformation toolkit.

## Feature Comparison

| Capability | json-canon | Cyberphone Go | encoding/json |
|------------|-----------|---------------|---------------|
| RFC 8785 canonical output | Yes | Yes | No |
| ECMA-262 Number::toString | Yes (Burger-Dybvig) | Yes (strconv-based) | No |
| UTF-16 code-unit key sort | Yes | Yes | No (byte order) |
| Strict RFC 8259 grammar | Yes | Partial | Partial |
| I-JSON constraints (RFC 7493) | Yes | No | No |
| Classified error taxonomy | Yes (12 classes) | No | No |
| Stable CLI ABI (SemVer) | Yes | N/A (library only) | N/A |
| Configurable resource bounds | Yes | No | No |
| Offline replay evidence | Yes | No | No |
| External dependencies | None (stdlib only) | None | N/A (stdlib) |
| Platform support | Linux | Cross-platform | Cross-platform |

### Differential Strictness

These inputs are accepted by Cyberphone Go but rejected by json-canon:

| Input | Cyberphone Go | json-canon |
|-------|---------------|------------|
| `{"n":0x1p-2}` (hex float) | `{"n":0.25}` | reject (`INVALID_GRAMMAR`) |
| `{"n":+1}` (plus prefix) | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| `{"n":01}` (leading zero) | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| `{"s":"<0xFF>"}` (bad UTF-8) | passthrough | reject (`INVALID_UTF8`) |
| `{"s":"\uD800\u0041"}` (bad surrogate) | `{"s":"\uFFFD"}` | reject (`LONE_SURROGATE`) |

Full details: [CYBERPHONE_DIFFERENTIAL_EXAMPLES.md](CYBERPHONE_DIFFERENTIAL_EXAMPLES.md).

Run the differential gate:

```bash
go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1 -v
```

## Migration

### From Cyberphone Go

Behavioral differences:

1. **Stricter parsing** — json-canon rejects hex floats, plus-prefixed numbers,
   leading-zero numbers, invalid UTF-8, and lone surrogates that Cyberphone Go
   silently accepts.
2. **Error types** — json-canon returns `*jcserr.Error` with classified failure
   classes, not generic errors.
3. **API shape** — Cyberphone Go uses `Transform([]byte) ([]byte, error)`.
   json-canon uses a two-step parse/serialize model.

```go
// Before (Cyberphone Go):
// out, err := jsoncanonicalizer.Transform(input)

// After (json-canon):
v, err := jcstoken.Parse(input)
if err != nil {
	return err
}
out, err := jcs.Serialize(v)
```

### From encoding/json + Manual Sorting

Key differences:

1. **Key sort order** — `encoding/json` sorts by UTF-8 byte order. RFC 8785
   requires UTF-16 code-unit order. These differ for supplementary-plane characters.
2. **Number formatting** — `encoding/json` uses `strconv.FormatFloat`. RFC 8785
   requires ECMA-262 Number::toString output.
3. **Whitespace** — `json.MarshalIndent` output is never canonical.

Replace the marshal-then-sort pattern with `jcstoken.Parse` + `jcs.Serialize`.

### From Other Languages

If migrating from a JCS implementation in another language, verify canonical
output matches byte-for-byte on a representative corpus. Watch for:

- Number formatting for edge cases (subnormals, large integers, values near
  powers of 10)
- Key ordering for keys with supplementary-plane characters
- Handling of `\uXXXX` escape sequences and surrogate pairs

The conformance suite (`go test ./conformance -count=1 -v`) validates against
official Cyberphone vectors and RFC 8785-derived fixtures.
