# Integration Guide

Practical patterns for deploying `json-canon` in production pipelines and Go
applications.

## CI/CD Pipeline Patterns

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

Ensure a document is canonical before signing:

```bash
./jcs-canon verify --quiet payload.json && sign payload.json
```

### GitHub Actions Step

```yaml
- name: Verify canonical JSON
  run: |
    ./jcs-canon verify --quiet config.json
```

## Error Handling

### Exit Code Decision Table

| Exit Code | Meaning | Pipeline Action |
|-----------|---------|-----------------|
| 0 | Success | Continue |
| 2 | Input rejected (parse error, policy violation, non-canonical, usage error) | Fail the build / reject the input |
| 10 | Internal error (I/O failure, unexpected state) | Alert ops / retry |

### Failure Classes by Operator Action

**Fix your input** — the JSON document is malformed or policy-disallowed:

| Class | Cause |
|-------|-------|
| `INVALID_UTF8` | Invalid UTF-8 byte sequences |
| `INVALID_GRAMMAR` | JSON syntax error (leading zeros, trailing commas, bad escapes) |
| `DUPLICATE_KEY` | Duplicate object key after escape decoding |
| `LONE_SURROGATE` | Lone surrogate code point in string |
| `NONCHARACTER` | Unicode noncharacter in string |
| `NUMBER_OVERFLOW` | Number exceeds IEEE 754 binary64 range |
| `NUMBER_NEGZERO` | Lexical negative zero (`-0`, `-0.0`) |
| `NUMBER_UNDERFLOW` | Non-zero number underflows to zero |
| `NOT_CANONICAL` | Valid JSON but not byte-identical to canonical form |

**Fix your configuration** — bounds or usage problem:

| Class | Cause |
|-------|-------|
| `BOUND_EXCEEDED` | Input exceeds a resource limit (depth, size, count) |
| `CLI_USAGE` | Invalid command, flag, or file path |

**Alert ops** — system-level failure:

| Class | Cause |
|-------|-------|
| `INTERNAL_IO` | Write failure, pipe breakage, I/O stream error |
| `INTERNAL_ERROR` | Unexpected internal error |

Full reference: [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md).

### Go Library Error Handling

All errors from `jcstoken.Parse` and `jcs.Serialize` are `*jcserr.Error`
values. Use `errors.As` to extract the failure class:

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

## Resource Limits and Tuning

### Default Bounds

| Bound | Default | Option Field |
|-------|---------|--------------|
| Max nesting depth | 1,000 | `MaxDepth` |
| Max input size | 64 MiB | `MaxInputSize` |
| Max JSON values | 1,000,000 | `MaxValues` |
| Max object members | 250,000 per object | `MaxObjectMembers` |
| Max array elements | 250,000 per array | `MaxArrayElements` |
| Max string bytes | 8 MiB (decoded UTF-8) | `MaxStringBytes` |
| Max number chars | 4,096 | `MaxNumberChars` |

### When to Tune

- **API payloads**: reduce `MaxInputSize` to match expected payload size
  (e.g., 1 MiB).
- **Simple structures**: reduce `MaxValues` (e.g., 10,000) and `MaxDepth`
  (e.g., 32).
- **Adversarial environments**: tighten all bounds and budget for ~3x input
  size in process memory.

### Example

```go
opts := &jcstoken.Options{
	MaxDepth:     32,
	MaxInputSize: 1 << 20,  // 1 MiB
	MaxValues:    10_000,
}
v, err := jcstoken.ParseWithOptions(input, opts)
```

Full bounds documentation: [BOUNDS.md](../BOUNDS.md).

## Library Integration Patterns

### Round-Trip Canonicalization

Parse, serialize, and use the canonical bytes:

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

### Canonical Verification

Check whether bytes are already canonical without transforming:

```go
v, err := jcstoken.Parse(input)
if err != nil {
	return false, err
}
canonical, err := jcs.Serialize(v)
if err != nil {
	return false, err
}
return bytes.Equal(input, canonical), nil
```

### HTTP Middleware Sketch

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

## Migration

### From Cyberphone Go

Behavioral differences to be aware of:

1. **Stricter parsing** — `json-canon` rejects hex floats, plus-prefixed
   numbers, leading-zero numbers, invalid UTF-8, and lone surrogates that
   Cyberphone Go silently accepts.
2. **Error types** — `json-canon` returns `*jcserr.Error` with classified
   failure classes, not generic errors.
3. **API shape** — Cyberphone Go uses `Transform([]byte) ([]byte, error)`.
   `json-canon` uses a two-step parse/serialize model that exposes the
   intermediate Value tree.

Migration:

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

See [COMPARISON.md](COMPARISON.md) for the full differential strictness table
and feature matrix.

### From encoding/json + Manual Sorting

Key differences:

1. **Key sort order** — `encoding/json` sorts keys by UTF-8 byte order.
   RFC 8785 requires UTF-16 code-unit order. These differ for keys containing
   supplementary-plane characters.
2. **Number formatting** — `encoding/json` uses Go's `strconv.FormatFloat`.
   RFC 8785 requires ECMA-262 Number::toString output.
3. **Whitespace** — canonical JSON has no whitespace. If you use
   `json.MarshalIndent`, the output is never canonical.

Migration: replace the marshal-then-sort pattern with `jcstoken.Parse` +
`jcs.Serialize`.

### From Other Languages

If migrating from a JCS implementation in another language, verify that
canonical output matches by comparing byte-for-byte on a representative corpus.
Pay particular attention to:

- Number formatting for edge cases (subnormals, large integers, values near
  powers of 10)
- Key ordering for keys with supplementary-plane characters
- Handling of `\uXXXX` escape sequences and surrogate pairs

The conformance test suite (`go test ./conformance -count=1 -v`) validates
against official Cyberphone vectors and RFC 8785-derived fixtures.

## Further Reading

- [QUICKSTART.md](QUICKSTART.md) — 5-minute getting started
- [COMPARISON.md](COMPARISON.md) — evaluate json-canon vs alternatives
- [FAILURE_TAXONOMY.md](../FAILURE_TAXONOMY.md) — complete failure class reference
- [BOUNDS.md](../BOUNDS.md) — resource bounds and memory behavior
- [ABI.md](../ABI.md) — stable CLI contract
- [docs/book/README.md](book/README.md) — full engineering handbook
