[Previous: What This Project Is](01-what-this-project-is.md) | [Book Home](README.md) | [Next: Why This Exists](03-why-this-exists.md)

# Chapter 3: What This Project Is Not

Understanding boundaries is as important as understanding features.

## Not a General-Purpose JSON Toolkit

`json-canon` is not a broad data-processing framework.

It does not aim to provide schema validation, query languages, transformation
DSLs, or application-level business semantics.

## Not a Multi-Platform Runtime Target

Supported runtime platform is Linux only.

Cross-architecture replay exists for release proof (`x86_64`, `arm64`), but
runtime support policy remains Linux.

## Not a "Best Effort" Canonicalizer

The project rejects invalid or policy-disallowed input with stable failure
classification. It does not silently repair malformed content.

## Not a Mutable ABI Surface

CLI behavior is versioned as a public contract.

Breaking command/flag/exit/stream behavior is not allowed in patch or minor
releases.

## Not Trusting Shell Scripts as the Only Gate

Convenience scripts exist for operators, but required compatibility/conformance
gates are enforced via Go tests.

## Not Hiding Supply-Chain Risk

Release trust relies on checksums, provenance attestation, reproducible build
checks, and offline evidence verification. If those fail, the release is not
trusted.

[Previous: What This Project Is](01-what-this-project-is.md) | [Book Home](README.md) | [Next: Why This Exists](03-why-this-exists.md)
