# json-canon

[![Go Reference](https://pkg.go.dev/badge/github.com/lattice-substrate/json-canon.svg)](https://pkg.go.dev/github.com/lattice-substrate/json-canon)
[![Go Report Card](https://goreportcard.com/badge/github.com/lattice-substrate/json-canon)](https://goreportcard.com/report/github.com/lattice-substrate/json-canon)
[![CI](https://github.com/lattice-substrate/json-canon/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/lattice-substrate/json-canon/actions/workflows/ci.yml)
[![Coverage](https://github.com/lattice-substrate/json-canon/actions/workflows/coverage.yml/badge.svg?branch=main&event=push)](https://github.com/lattice-substrate/json-canon/actions/workflows/coverage.yml)
[![DOI](https://zenodo.org/badge/doi/10.5281/zenodo.18890835.svg)](https://doi.org/10.5281/zenodo.18890835)

json-canon produces byte-deterministic JSON. Same input, same bytes, across Go versions, architectures, and kernel versions. It implements [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) (JSON Canonicalization Scheme) with a strict parser that rejects ambiguous input, a hand-written Burger-Dybvig number formatter validated against 286,362 oracle test vectors, and a stable CLI contract under SemVer. Zero external dependencies. Linux only.

## Go API

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

For a single call that parses and serializes in one step:

```go
canonical, err := jcs.Canonicalize(input)
```

## CLI

```bash
# Canonicalize
echo '{"b":2,"a":1}' | jcs-canon canonicalize -
# Output: {"a":1,"b":2}

# Verify canonical form
jcs-canon verify document.json
```

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
Verification instructions: [`CONTRIBUTING.md`](CONTRIBUTING.md#release-verification).

## Design

json-canon owns every stage of the pipeline from input bytes to canonical output. Nothing is delegated to `encoding/json` or `strconv.FormatFloat`.

**Parser.** A strict RFC 8259 parser that rejects what `encoding/json` silently accepts: lone surrogates, duplicate keys after escape decoding, noncharacters, lexical negative zero, overflow, underflow. Seven independent resource bounds (input size, nesting depth, total values, object members, array elements, string bytes, number token length), all fail-fast, all configurable. If the parser returns a value, that value has exactly one canonical representation.

**Number formatting.** Burger-Dybvig algorithm with ECMA-262 even-digit tie-breaking, implemented from scratch in pure `math/big` arithmetic. `strconv.FormatFloat` is a high-quality formatter, but it is not an ECMA-262 conformance contract. Its output policy can change across Go versions and doesn't match RFC 8785's required formatting rules at key exponent boundaries. The implementation is validated against 286,362 oracle test vectors (54,445 boundary cases + 231,917 stress vectors) with pinned SHA-256 checksums on the test data itself.

**Key sorting.** UTF-16 code-unit order, not UTF-8 byte order. These orderings disagree for characters above U+FFFF. A supplementary-plane character like U+10000 sorts *before* U+E000 in UTF-16 code-unit order and *after* it in UTF-8 byte order. Most implementations get this wrong because the bug only surfaces with emoji, CJK Extension B, or historic script keys. Rare in testing, not rare in production. RFC 8785 §3.2.3 mandates UTF-16 code-unit order because JCS is defined for interoperability with ECMAScript string semantics.

**Error taxonomy.** 13 failure classes mapped to 3 exit codes (0, 2, 10). Classified by root cause, not error origin. A missing file path is `CLI_USAGE` (the invocation is wrong), not `INTERNAL_IO` (infrastructure broke). Exit code 2 means "fix your input or invocation." Exit code 10 means "investigate the environment." The class name is the stable contract; the surrounding message text is not. Machines should switch on the class, not parse the message.

**Determinism evidence.** Unit tests prove correctness. They do not prove determinism across environments. CI runs the full test suite on both x86_64 and arm64 on every push and PR, catching architecture-specific regressions before merge. An offline replay harness provides deeper coverage at release time, running the tool across Linux distributions (Debian, Ubuntu, Alpine, Fedora, Rocky, openSUSE) in both container and VM execution modes on both architectures, capturing SHA-256 digests of all output. Releases are gated on byte-identical digests across the full matrix. The evidence bundle (checksummed, machine-readable, committed alongside the source) makes the determinism claim auditable, not just asserted.

## Engineering Articles

Detailed technical articles covering the engineering behind json-canon:

1. [Shortest Round-Trip: Implementing IEEE 754 to Decimal Conversion in Go](https://lattice-substrate.github.io/blog/2026/02/27/shortest-roundtrip-ieee754-burger-dybvig/) - Burger-Dybvig algorithm, multiprecision arithmetic, ECMA-262 formatting
2. [A Strict RFC 8259 JSON Parser](https://lattice-substrate.github.io/blog/2026/02/26/strict-rfc8259-json-parser/) - single-pass parser design, surrogate validation, bounds enforcement
3. [The Small Decisions That Infrastructure Depends On](https://lattice-substrate.github.io/blog/2026/02/25/small-decisions-infrastructure-primitive/) - UTF-16 sort order, failure taxonomy, ABI contracts
4. [Proving Determinism: Evidence-Based Release Engineering](https://lattice-substrate.github.io/blog/2026/02/24/proving-determinism-evidence-release/) - offline replay harness, SHA-256 evidence chains, release gating
5. [IEEE 754 Compliance Does Not Mean Platform Independence](https://lattice-substrate.github.io/blog/2026/03/04/fma-go-floating-point-determinism/) - FMA instructions, Go compiler fusion policy, platform-independent digit generation

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

Full taxonomy: [`FAILURE_TAXONOMY.md`](FAILURE_TAXONOMY.md).

## Stability

The CLI ABI (command names, flag semantics, exit codes, failure classes, stdout/stderr contracts, canonical output bytes) is governed by strict SemVer. Patch releases change no behavior. Minor releases are additive only. Any breaking change requires a major version bump. The machine-readable contract is [`abi_manifest.json`](abi_manifest.json); the full specification is [`ABI.md`](ABI.md).

## Documentation

- [`docs/GUIDE.md`](docs/GUIDE.md): usage, CI/CD integration, migration, feature comparison
- [`ARCHITECTURE.md`](ARCHITECTURE.md): package boundaries, runtime model
- [`SPECIFICATION.md`](SPECIFICATION.md): normative behavior contract
- [`FAILURE_TAXONOMY.md`](FAILURE_TAXONOMY.md): error classes and exit codes
- [`BOUNDS.md`](BOUNDS.md): resource limits and memory behavior
- [`CONFORMANCE.md`](CONFORMANCE.md): conformance gates and evidence
- [`CONTRIBUTING.md`](CONTRIBUTING.md): contributor guide, release process, verification
- [`SECURITY.md`](SECURITY.md): vulnerability reporting
- [`docs/adr/`](docs/adr/): architectural decision records

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
