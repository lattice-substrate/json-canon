---
title: "The Small Decisions That Make Software Infrastructure-Grade"
published: true
tags: go, programming, architecture, softwareengineering
series: "Building Infrastructure-Grade JSON Canonicalization in Go"
---

Infrastructure-grade software isn't defined by one big architectural choice. It's defined by getting dozens of small decisions right — decisions that most projects skip because they seem unimportant until they aren't.

Most software fails at the center: the core algorithm is wrong, the architecture doesn't scale, the data model can't represent the domain. Infrastructure software fails at the margins: the sort order is subtly wrong for one class of inputs, the error code changes in a patch release and breaks a CI pipeline, the upgrade removes a flag that a deployment script depends on.

These marginal failures are harder to prevent because they're harder to see. They don't cause test failures during development. They cause production incidents three months after a routine upgrade.

This article examines three categories of these decisions through the lens of a JSON canonicalization library: a correctness detail that affects sort order, a failure classification system that enables machine automation, and an ABI stability discipline that prevents upgrade emergencies. None of these individually is difficult. Together, they define whether downstream systems can depend on you without reservations.

## Correctness Detail: UTF-16 vs UTF-8 Key Sorting

[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) (JSON Canonicalization Scheme) requires that object keys be sorted by lexicographic order of UTF-16 code units. That is not UTF-8 byte order, and it is not Unicode scalar-value order.

For most strings, these orderings are identical. They diverge when you have characters above U+FFFF (the Basic Multilingual Plane). Here's why.

Characters above U+FFFF require four bytes in UTF-8 but are encoded as *surrogate pairs* in UTF-16 — two 16-bit code units. The high surrogate falls in the range U+D800-U+DBFF. Consider two characters:

- **U+10000** (LINEAR B SYLLABLE B008): UTF-16 encoding is `D800 DC00` (surrogate pair). UTF-8 encoding is `F0 90 80 80`.
- **U+E000** (first Private Use Area character): UTF-16 encoding is `E000` (single code unit). UTF-8 encoding is `EE 80 80`.

In UTF-16 code-unit order: U+10000 (`D800`) < U+E000 (`E000`). The high surrogate `D800` is numerically less than `E000`.

In UTF-8 byte order: U+E000 (`EE`) < U+10000 (`F0`). The three-byte UTF-8 prefix `EE` is numerically less than the four-byte prefix `F0`.

The two orderings disagree. RFC 8785 mandates UTF-16 code-unit order (see §3.2.3). Most canonicalization implementations get this wrong because they use byte-order comparison, which works for UTF-8 strings but produces a different order than the specification requires.

Why does RFC 8785 use UTF-16 code-unit order instead of the more natural (for modern systems) Unicode code-point order? Because JCS is defined for interoperability with ECMAScript string serialization rules, and ECMAScript strings are sequences of 16-bit code units. The required sort is a pure lexicographic code-unit comparison, not locale-aware collation (`localeCompare` is locale-sensitive and is not the JCS rule).

This means every implementation in a language with UTF-8 native strings (Go, Rust, Python 3) must explicitly convert to UTF-16 for comparison. Implementations that skip this conversion will produce correct output for the BMP (U+0000 to U+FFFF) and incorrect output for supplementary-plane characters. Since supplementary-plane characters are rare in practice — emoji, historic scripts, mathematical symbols — the bug is unlikely to surface in casual testing. It will surface when a production system encounters a key containing an emoji or a CJK Extension B character.

The implementation is compact:

```go
func serializeObject(buf []byte, v *jcstoken.Value) ([]byte, error) {
    sorted := make([]sortableMember, len(v.Members))
    for i := range v.Members {
        sorted[i] = sortableMember{
            member: v.Members[i],
            key16:  utf16.Encode([]rune(v.Members[i].Key)),
        }
    }
    sort.Slice(sorted, func(i, j int) bool {
        return compareUTF16Units(sorted[i].key16, sorted[j].key16) < 0
    })
    // ... serialize sorted members
}
```

Each key is converted from Go's native UTF-8 string to a `[]uint16` via `utf16.Encode([]rune(key))`. The sort comparator then performs lexicographic comparison on these 16-bit code unit slices:

```go
func compareUTF16Units(ua, ub []uint16) int {
    minLen := len(ua)
    if len(ub) < minLen {
        minLen = len(ub)
    }
    for i := 0; i < minLen; i++ {
        if ua[i] < ub[i] {
            return -1
        }
        if ua[i] > ub[i] {
            return 1
        }
    }
    if len(ua) < len(ub) {
        return -1
    }
    if len(ua) > len(ub) {
        return 1
    }
    return 0
}
```

This is approximately 30 lines of code. It is the only correct approach for RFC 8785 compliance. The canonical output for an object with keys U+10000 and U+E000 places U+10000 *first* — the opposite of what a naive byte-order sort would produce.

### String Escaping: What Gets Escaped and Why

Canonicalization also requires deterministic string escaping. The rules are specific:

| Character | Canonical Form | Rule |
|-----------|---------------|------|
| U+0008 (backspace) | `\b` | Named escape |
| U+0009 (tab) | `\t` | Named escape |
| U+000A (newline) | `\n` | Named escape |
| U+000C (form feed) | `\f` | Named escape |
| U+000D (carriage return) | `\r` | Named escape |
| U+0022 (quotation mark) | `\"` | Named escape |
| U+005C (reverse solidus) | `\\` | Named escape |
| U+0000-U+001F (other controls) | `\u00xx` | Lowercase hex, zero-padded |
| U+002F (solidus `/`) | `/` | NOT escaped |
| Everything else above U+001F | Raw UTF-8 | No escaping |

The solidus rule is worth noting. JSON allows `\/` as a valid escape, and many serializers produce it. RFC 8785 specifies that solidus is *not* escaped in canonical output. This is a one-line decision in the implementation, but it means any test that compares canonical output byte-for-byte must agree on this point.

## Failure Contracts: Designing a Taxonomy That Machines Can Depend On

Most error handling in CLI tools follows a pattern: print a message to stderr, exit non-zero. The message is for humans. The exit code is a vague signal — usually 1 for "something went wrong."

This is insufficient for machine automation. When a CI pipeline runs a canonicalization check, it needs to distinguish between "the input was malformed JSON" (a build error) and "stdout couldn't be written" (an infrastructure problem). The human can read the message. The script needs the exit code.

### 13 Classes, 3 Exit Codes

The failure taxonomy defines 13 failure classes mapped to 3 exit codes:

| Exit Code | Meaning | Classes |
|-----------|---------|---------|
| 0 | Success | — |
| 2 | Input rejection | INVALID_UTF8, INVALID_GRAMMAR, DUPLICATE_KEY, LONE_SURROGATE, NONCHARACTER, NUMBER_OVERFLOW, NUMBER_NEGZERO, NUMBER_UNDERFLOW, BOUND_EXCEEDED, NOT_CANONICAL, CLI_USAGE |
| 10 | Internal error | INTERNAL_IO, INTERNAL_ERROR |

The exit code space is deliberately sparse. Codes 0, 2, and 10 leave room for future expansion without breaking existing scripts that check `$?`. Eleven classes share exit code 2 because they all represent the same decision for automation: "the input or invocation was wrong; don't retry without changing something." Exit code 1 is intentionally avoided — many shells and tools use 1 as a generic failure code, and conflating tool-specific failures with generic failure would reduce the signal.

The two exit-10 classes represent a fundamentally different situation: the tool itself encountered a problem. A CI pipeline should fail the build on exit 2 (bad input is a build error) but might alert ops on exit 10 (the build infrastructure has a problem).

Why 13 classes instead of 3? Because the exit code is a coarse signal for automation, while the class is a fine-grained signal for diagnostics. A script checks the exit code. A log aggregator or monitoring system can parse the class name from stderr. Having `DUPLICATE_KEY` as a distinct class from `INVALID_GRAMMAR` lets a monitoring dashboard show "we're seeing a spike in duplicate-key rejections" without confusing it with syntax errors.

### Root-Cause Classification

The classification principle is: classify by *root cause*, not by error *origin*.

The clearest example: a missing file path. The user runs `jcs-canon canonicalize /nonexistent.json` and the file doesn't exist. This is an `os.Open` error. An I/O error. So it should be `INTERNAL_IO`, right?

No. It should be `CLI_USAGE` (exit 2).

The root cause is that the user supplied an invalid argument. The file path is user input, just like the JSON content. When the path cannot be opened, the problem is the same as providing an unknown flag or specifying two input files — the invocation is wrong.

`INTERNAL_IO` is reserved for failures *after* a valid I/O channel is established. If `os.Open` succeeds but a later `Read` fails due to a broken pipe, or if writing to stdout fails because the pipe was closed — those are infrastructure problems. The channel was valid; something broke during operation.

This distinction matters for automation:

```bash
jcs-canon canonicalize input.json
case $? in
  0)  echo "Success" ;;
  2)  echo "Input or usage problem — fix the invocation" ;;
  10) echo "Infrastructure problem — investigate the environment" ;;
esac
```

### The Structured Error Type

Every error in the system is a `*jcserr.Error` with four fields:

```go
type Error struct {
    Class   FailureClass   // Stable category (determines exit code)
    Offset  int            // Source-byte position, or -1
    Message string         // Human-readable diagnostic
    Cause   error          // Underlying error (for wrapping)
}
```

The `Class` field is the stable contract. The `Message` field is *not* stable — its wording may change in minor releases. This is explicit policy: machines should switch on the class, not parse the message string. The `Offset` field gives byte-precise error positions for parse failures, enabling editors and diagnostic tools to highlight the exact location.

### Offset Semantics

The offset field deserves its own discussion because it's an area where most error reporting gets lazy.

For a simple parse error like a leading zero in `{"n":01}`, the offset points to the `0` at the start of the number token. Straightforward. But for errors inside escape sequences, the offset must point to the *originating escape*, not the byte where the violation was detected.

Consider the string `"\uD800\u0041"`. This contains a high surrogate (U+D800) followed by a non-low-surrogate (U+0041). The parser detects the error when reading the second escape sequence — but the error offset points to byte 7 (the backslash of `\u0041`), not to byte 1 (the backslash of `\uD800`). This is because the *violation* is in the second escape: a high surrogate was followed by U+0041 instead of a low surrogate. Pointing to the first escape would be misleading — `\uD800` isn't inherently wrong; it's wrong because of what follows.

Conversely, for a lone low surrogate like `"\uDC00"`, the offset points to byte 1 (the backslash) because the low surrogate appearing without a preceding high surrogate is the violation.

These offset semantics are stable across the failure taxonomy and documented as part of the error contract. Changing where an offset points is a behavioral change, even if the class and message remain the same.

The error format is also deliberate:

```
jcserr: INVALID_GRAMMAR at byte 15: leading zero in number
jcserr: CLI_USAGE: read file "config.json": no such file or directory
jcserr: INTERNAL_IO: writing output: write: broken pipe
```

The class name always appears after `jcserr:`. Byte offsets appear only for parse-time errors (where they're meaningful). The cause chain preserves the underlying error for debugging without losing the classification.

## ABI Stability: Preventing Upgrade Emergencies

For a CLI tool consumed by scripts and CI pipelines, the *interface* is the product. Changing a flag name, moving output between stdout and stderr, or redefining an exit code breaks every downstream consumer. These breakages are invisible in your test suite and visible only in your users' build failures.

### Defining the Stable Surface

The stable ABI surface includes:

1. **Command names**: `canonicalize`, `verify`
2. **Flag names and semantics**: `--quiet`, `--help`, `--version`
3. **Exit code mapping**: 0/2/10 with defined semantics
4. **Output stream placement**: canonical data to stdout, diagnostics to stderr, verify result to stderr
5. **Failure class names in stderr**: The class name `INVALID_GRAMMAR` is stable; the surrounding message text is not
6. **Canonical output bytes**: identical input must produce identical stdout bytes

Notably absent from the stable surface: the exact wording of error messages and help text. These are explicitly non-stable, allowing improvements to diagnostics without a major version bump.

There is one subtlety here: failure class *names* that appear in stderr (e.g., `INVALID_GRAMMAR` in the error output) are stable, even though the surrounding message text is not. This lets machines extract the class name from stderr as a fallback when they can't use the exit code alone — for instance, when a script needs to distinguish between `INVALID_GRAMMAR` and `DUPLICATE_KEY`, both of which share exit code 2.

The stream placement policy is also part of the stable surface. The `canonicalize` command writes canonical bytes to stdout and diagnostics to stderr. The `verify` command writes nothing to stdout and writes `ok\n` to stderr on success (suppressible with `--quiet`). If a future version moved the verify result from stderr to stdout, every script that captures `verify` output would break. This is why stream placement is documented, tested, and governed by SemVer.

### The Machine-Readable Contract

The ABI is documented in two forms. `ABI.md` is the human-readable specification. `abi_manifest.json` is the machine-readable contract:

```json
{
  "abi_version": "1.0.0",
  "tool": "jcs-canon",
  "commands": {
    "canonicalize": {
      "stable": true,
      "flags": {
        "--quiet": {"short": "-q", "stable": true},
        "--help": {"short": "-h", "stable": true}
      },
      "stdout": "Canonical JSON bytes (on success)",
      "stderr": "Error diagnostics (on failure)",
      "exit_codes": [0, 2, 10]
    },
    "verify": {
      "stable": true,
      "flags": {
        "--quiet": {"short": "-q", "stable": true},
        "--help": {"short": "-h", "stable": true}
      },
      "stdout": "Empty (verify never writes to stdout)",
      "stderr": "'ok\\n' on success (unless --quiet), error diagnostics on failure",
      "exit_codes": [0, 2, 10]
    }
  },
  "exit_codes": {
    "0": {"class": "SUCCESS"},
    "2": {"class": "INPUT_REJECTION"},
    "10": {"class": "INTERNAL_ERROR"}
  },
  "failure_classes": [
    {"name": "INVALID_UTF8", "exit_code": 2},
    {"name": "INVALID_GRAMMAR", "exit_code": 2},
    {"name": "DUPLICATE_KEY", "exit_code": 2},
    ...
  ],
  "compatibility": {
    "policy": "Strict SemVer. Any change to items marked 'stable: true' requires major version bump.",
    "stderr_wording": "Non-stable. Diagnostic message text may change in minor/patch releases.",
    "exit_code_mapping": "Stable. Failure class → exit code mapping is frozen."
  }
}
```

This file serves two purposes. First, it's a test fixture — CI validates that the tool's actual behavior matches the manifest's claims. Second, it's a communication device for downstream consumers. A script author can read the manifest to understand what's stable, what might change, and what exit codes to expect, without parsing prose documentation.

### Why a Manifest Instead of Just Documentation

A reasonable question: why maintain both `ABI.md` (human-readable) and `abi_manifest.json` (machine-readable)? Isn't the documentation sufficient?

Documentation drifts. When a developer adds a flag, they update the code and the tests, but they may forget to update the ABI documentation. The manifest is testable — CI can validate that the tool's actual behavior matches the manifest's claims. A test that parses `abi_manifest.json` and checks that `jcs-canon verify --help` exits 0, that `jcs-canon verify` produces `ok\n` on stderr for valid canonical input, and that error output goes to stderr, will catch ABI drift that documentation review would miss.

The manifest also serves as a communication device. A downstream consumer can `curl` the manifest from a release and programmatically determine what the stable surface looks like. This is more reliable than scraping a README.

### Change Control

The SemVer rules are strict and non-negotiable:

- **Patch releases** (0.2.0 → 0.2.1): No behavior changes to stable surface items
- **Minor releases** (0.2.x → 0.3.0): May add new commands or flags, but existing behavior is preserved
- **Major releases** (0.x.x → 1.0.0): May change anything, with migration guidance

Any ABI-impacting change must update *four artifacts in the same commit series*: the implementation, `abi_manifest.json`, test assertions, and `CHANGELOG.md`. This co-update requirement prevents the manifest from drifting out of sync with the code. A test that validates the manifest against actual behavior catches drift that documentation alone would miss.

## The Compound Effect

Each of these decisions is individually minor:

- UTF-16 key sorting: ~30 lines of code, ~5 lines of test
- Failure classification: 88 lines of code, one type and 13 constants
- ABI manifest: 69 lines of JSON

None of them requires novel engineering. None of them is exciting. A project that omits all three still works — it parses JSON, it serializes JSON, it runs from the command line.

But the project that includes all three communicates something different to its consumers. It says: "We have thought about the cases you will encounter. The sort order is correct, not just close. The exit codes mean what we say they mean, and we won't change them without a major version. The error you get for a missing file is classified by root cause, not by which system call failed."

Infrastructure trust is not earned by one big decision. It's earned by consistent attention to the decisions that are easy to defer and hard to fix later. The sort order that's wrong by one comparison. The exit code that changes in a patch release. The error message that scripts parse because there's no stable classification.

These are the margins where infrastructure fails.

Consider the alternative. A project that uses UTF-8 byte-order sorting passes 99.9% of real-world key comparisons correctly — supplementary-plane characters in JSON keys are rare. But the 0.1% failure is a silent correctness bug, not a crash. The canonical output is wrong for those inputs, and the consumer has no way to detect it without an independent reference implementation. This is the kind of bug that ships, is discovered months later, and then can't be fixed without a breaking change because existing consumers depend on the (wrong) output.

A project with unstructured exit codes works fine for human use. But the first time someone writes a CI pipeline that retries on exit 1, they'll retry on parse errors (which will never succeed) and on I/O errors (which might). Distinguishing these requires parsing stderr text, which breaks when the message wording changes in a patch release.

A project without an ABI manifest accumulates undocumented behavioral changes. Each one seems minor — renaming a flag, changing where help text goes, adding a new exit code. But each one is a potential breaking change for a downstream consumer who discovered the behavior empirically and depended on it.

Getting these details right is not the interesting part of the engineering. But it's the part that determines whether your downstream can depend on you.

The implementation discussed here is from [json-canon](https://github.com/lattice-substrate/json-canon), an RFC 8785 JSON canonicalization library written in Go.
