# json-canon

Deterministic JSON for signing, hashing, and audit trails.

## The Problem

JSON is not deterministic. Object key order, number formatting, and whitespace
vary across serializers, languages, and versions. When two systems render the
same logical data differently, cryptographic signatures break, content hashes
diverge, and audit comparisons produce false mismatches.

RFC 8785 (JSON Canonicalization Scheme) defines a canonical form that eliminates
this nondeterminism. `json-canon` is an infrastructure-grade implementation of
RFC 8785, built for machine consumers that need deterministic canonical bytes
they can depend on across releases.

## The Solution

### Go Library

```go
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
			fmt.Printf("failure class: %s\n", je.Class)
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

### CLI

```bash
# Canonicalize
echo '{"b":2,"a":1}' | jcs-canon canonicalize -
# Output: {"a":1,"b":2}

# Verify canonical form
jcs-canon verify document.json
```

## When to Use

- Signing or hashing JSON documents — you need byte-deterministic output.
- Content-addressable storage — identical content must produce identical hashes.
- Audit trails — you need reproducible, traceable canonicalization.
- Pipeline validation — you need strict input gates with stable exit codes.

## When NOT to Use

- Pretty-printing or human-readable formatting — use `jq`.
- macOS or Windows — supported runtime is Linux only.
- Lenient parsing — `json-canon` rejects invalid JSON by design.
- General JSON processing — this is not a query engine or transformation toolkit.

## Install

**Library:**

```bash
go get github.com/lattice-substrate/json-canon
```

**CLI (build from source):**

```bash
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X main.version=v0.0.0-dev" \
  -o jcs-canon ./cmd/jcs-canon
```

Release binaries are available on the
[releases page](https://github.com/lattice-substrate/json-canon/releases).
Verification instructions: [`VERIFICATION.md`](VERIFICATION.md).

## CLI Reference

```text
jcs-canon canonicalize [--quiet] [file|-]
jcs-canon verify [--quiet] [file|-]
jcs-canon --help
jcs-canon --version
```

### Exit Codes

| Code | Meaning | Example Causes |
|------|---------|----------------|
| 0 | Success | Canonical output produced, or document verified |
| 2 | Input rejection | Parse error, policy violation, non-canonical, invalid usage |
| 10 | Internal error | I/O write failure, unexpected state |

Every rejection maps to a stable failure class. See
[`FAILURE_TAXONOMY.md`](FAILURE_TAXONOMY.md) for the full taxonomy.

## Stability Policy

This project enforces strict SemVer for the published CLI ABI.

- Command set and flag semantics are versioned.
- Exit code taxonomy and failure classes are stable.
- Canonical stdout bytes for accepted inputs are deterministic.
- stderr stream contract (`verify` writes `ok\n` to stderr on success) is frozen.

Full ABI contract: [`ABI.md`](ABI.md).

## Test

```bash
go test ./... -count=1 -v
go test ./conformance -count=1 -v -timeout=10m
```

## Documentation

**Getting started:**
- [`docs/QUICKSTART.md`](docs/QUICKSTART.md) — 5-minute guide for library and CLI
- [`docs/INTEGRATION_GUIDE.md`](docs/INTEGRATION_GUIDE.md) — CI/CD patterns, error handling, migration
- [`docs/COMPARISON.md`](docs/COMPARISON.md) — evaluate json-canon vs alternatives

**Architecture and contracts:**
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — package boundaries, runtime model
- [`ABI.md`](ABI.md) — stable CLI contract
- [`SPECIFICATION.md`](SPECIFICATION.md) — normative behavior contract
- [`FAILURE_TAXONOMY.md`](FAILURE_TAXONOMY.md) — error classes and exit codes
- [`BOUNDS.md`](BOUNDS.md) — resource limits and memory behavior

**Handbook:**
- [`docs/book/README.md`](docs/book/README.md) — 13-chapter engineering handbook

**Operations and release:**
- [`CONFORMANCE.md`](CONFORMANCE.md) — conformance gates and evidence
- [`RELEASE_PROCESS.md`](RELEASE_PROCESS.md) — release workflow
- [`VERIFICATION.md`](VERIFICATION.md) — artifact verification
- [`docs/OFFLINE_REPLAY_HARNESS.md`](docs/OFFLINE_REPLAY_HARNESS.md) — offline replay runbook

**Project:**
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contributor guide
- [`GOVERNANCE.md`](GOVERNANCE.md) — maintainer policy
- [`SECURITY.md`](SECURITY.md) — vulnerability reporting
- [`docs/adr/`](docs/adr/) — architectural decision records

Full documentation index: [`docs/README.md`](docs/README.md).

## Normative References

| Spec | Scope |
|------|-------|
| [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) | JSON Canonicalization Scheme |
| [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) | JSON grammar and data model |
| [RFC 7493](https://www.rfc-editor.org/rfc/rfc7493) | I-JSON: duplicate keys, surrogates, noncharacters |
| [RFC 3629](https://www.rfc-editor.org/rfc/rfc3629) | UTF-8 encoding validity |
| [ECMA-262 §6.1.6.1.20](https://tc39.es/ecma262/#sec-numeric-types-number-tostring) | Number::toString |
| IEEE 754-2008 | Binary64 double-precision semantics |

## License

See `LICENSE`.
