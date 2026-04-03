# Guide

json-canon has two interfaces: a Go library for embedding canonicalization into your code, and a CLI for pipelines and verification scripts. This guide covers both, along with CI/CD integration and migration from other implementations.

## Quick Start

### Go Library

```bash
go get github.com/lattice-substrate/json-canon
```

The two-step API gives you a parsed value tree that you can inspect before serializing:

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

If you don't need the intermediate value tree, `jcs.Canonicalize` does both steps in one call:

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

Canonicalize from stdin or a file:

```bash
echo '{"b":2,"a":1}' | ./jcs-canon canonicalize -
# Output: {"a":1,"b":2}

./jcs-canon canonicalize input.json > canonical.json
```

Verify that a document is already in canonical form:

```bash
./jcs-canon verify input.json
# Writes "ok" to stderr and exits 0 on success
# Prints diagnostic and exits 2 on failure
```

The exit codes are stable and designed for scripts: 0 is success, 2 is input rejection (parse error, policy violation, non-canonical, bad usage), 10 is internal error (I/O failure). Switch on them directly:

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

Every error from `jcstoken.Parse` and `jcs.Serialize` is a `*jcserr.Error` with a stable failure class. The class is the machine contract; switch on it, not the message text:

```go
v, err := jcstoken.Parse(input)
if err != nil {
	var je *jcserr.Error
	if errors.As(err, &je) {
		switch je.Class {
		case jcserr.InvalidGrammar, jcserr.InvalidUTF8:
			// malformed input, reject
		case jcserr.BoundExceeded:
			// input too large, consider adjusting limits
		default:
			// other rejection, log class and offset
			log.Printf("%s at offset %d", je.Class, je.Offset)
		}
	}
	return err
}
```

The 13 failure classes and their exit code mappings are defined in [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md).

### Custom Resource Limits

The parser enforces seven independent bounds by default (see [BOUNDS.md](../BOUNDS.md)). For constrained environments (API servers, embedded systems, hostile-input pipelines), override them with `ParseWithOptions`:

```go
v, err := jcstoken.ParseWithOptions(input, &jcstoken.Options{
	MaxDepth:     32,       // nesting depth (default: 1,000)
	MaxValues:    10_000,   // total JSON values (default: 1,000,000)
	MaxInputSize: 1 << 20,  // 1 MiB (default: 64 MiB)
})
```

The same options work through `CanonicalizeWithOptions` (parse + serialize) and `SerializeWithOptions` (value tree only):

```go
canonical, err := jcs.CanonicalizeWithOptions(input, &jcstoken.Options{
	MaxDepth:     32,
	MaxValues:    10_000,
	MaxInputSize: 1 << 20,
})
```

### Canonical Verification

Check whether bytes are already in canonical form without transforming them. This is what the CLI's `verify` command does internally:

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

The canonical output is deterministic. The same input always produces the same bytes. That makes it safe to hash or sign directly:

```go
v, err := jcstoken.Parse(input)
if err != nil {
	return err
}
canonical, err := jcs.Serialize(v)
if err != nil {
	return err
}
hash := sha256.Sum256(canonical)
```

### HTTP Middleware

Canonicalize request bodies before they reach your handler. This ensures downstream code always sees canonical JSON, regardless of how the client formatted it:

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

### GitHub Actions

```yaml
- name: Verify canonical JSON
  run: |
    ./jcs-canon verify --quiet config.json
```

## Comparison with Other Implementations

| Capability | json-canon | Cyberphone Go | encoding/json |
|------------|-----------|---------------|---------------|
| RFC 8785 canonical output | Yes | Yes | No |
| ECMA-262 Number::toString | Yes (Burger-Dybvig) | Yes (strconv-based) | No |
| UTF-16 code-unit key sort | Yes | Yes | No (byte order) |
| Strict RFC 8259 grammar | Yes | Partial | Partial |
| I-JSON constraints (RFC 7493) | Yes | No | No |
| Classified error taxonomy | Yes (13 classes) | No | No |
| Stable CLI ABI (SemVer) | Yes | N/A (library only) | N/A |
| Configurable resource bounds | Yes | No | No |
| Offline replay evidence | Yes | No | No |
| External dependencies | None (stdlib only) | None | N/A (stdlib) |
| Platform support | Linux | Cross-platform | Cross-platform |

The "strconv-based" distinction for Cyberphone Go matters. Both libraries produce RFC 8785-compliant output for well-formed input. The difference is in the input contract: Cyberphone Go's parser is more lenient, which means inputs that json-canon rejects are silently canonicalized by Cyberphone Go. Whether that's a feature or a defect depends on whether you trust your input pipeline.

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

Three things change:

1. **Stricter parsing.** json-canon rejects hex floats, plus-prefixed numbers, leading-zero numbers, invalid UTF-8, and lone surrogates that Cyberphone Go silently accepts. If your inputs contain any of these, you'll get rejections where you previously got output.
2. **Typed errors.** json-canon returns `*jcserr.Error` with a failure class and byte offset, not a generic `error`. You can switch on the class for programmatic handling.
3. **Two-step API.** Cyberphone Go uses `Transform([]byte) ([]byte, error)`. json-canon separates parsing from serialization, giving you access to the value tree between steps. Use `jcs.Canonicalize` if you want the single-call equivalent.

```go
// Before (Cyberphone Go):
// out, err := jsoncanonicalizer.Transform(input)

// After (json-canon):
v, err := jcstoken.Parse(input)
if err != nil {
	return err
}
out, err := jcs.Serialize(v)

// Or, single call:
// out, err := jcs.Canonicalize(input)
```

### From encoding/json + Manual Sorting

Three things will break if you're hand-rolling canonicalization on top of `encoding/json`:

1. **Key sort order.** `encoding/json` sorts by UTF-8 byte order. RFC 8785 requires UTF-16 code-unit order. These produce different results for keys containing supplementary-plane characters (emoji, CJK Extension B, mathematical symbols).
2. **Number formatting.** `encoding/json` uses `strconv.FormatFloat`, which doesn't match ECMA-262 Number::toString output at key exponent boundaries.
3. **Whitespace.** `json.MarshalIndent` output is never canonical. Neither is any output with trailing newlines or spaces.

Replace the marshal-then-sort pattern with `jcstoken.Parse` + `jcs.Serialize`, or `jcs.Canonicalize` for a single call.

### From Other Languages

If migrating from a JCS implementation in another language, verify canonical output matches byte-for-byte on a representative corpus. The divergence points are predictable:

- **Number formatting** for edge cases: subnormals, large integers, values near powers of 10, values at the 1e-6 and 1e21 notation boundaries.
- **Key ordering** for keys with supplementary-plane characters (anything above U+FFFF).
- **Surrogate pair handling** in `\uXXXX` escape sequences. Some implementations silently replace lone surrogates with U+FFFD instead of rejecting them.

The conformance suite (`go test ./conformance -count=1 -v`) validates against official Cyberphone vectors and RFC 8785-derived fixtures.
