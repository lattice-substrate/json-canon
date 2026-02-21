# JCS Canonical JSON for Go — Infrastructure-Grade Documentation Bundle

**Status:** Draft documentation bundle (version 0.1.0-draft)  
**Date:** 2026-02-21  
**Purpose:** This bundle is a complete documentation corpus for a new, strict, dependency-free Go implementation of the JSON Canonicalization Scheme (JCS, RFC 8785) suitable for cryptographic “byte-identical” workflows and audit requirements.

## What this bundle is
This is not code. It is the **documentation product** that drives an implementation:
- normative scope and conformance language
- requirements registry (IDs, sources, acceptance criteria)
- design and algorithm specs
- error registry and API contract
- testing strategy and vector formats
- comparative analysis of common Go approaches, including `cyberphone/json-canonicalization`

## Where to start
- Read **docs/00-index.md** (navigation and reading paths).
- Then read **docs/01-motivation.md** (why strict JCS is needed for audited cryptographic use).
- For “what is required”, read **docs/02-normative-references.md** + **docs/10-requirements/requirements.md**.
- For “how it works”, read **docs/06-architecture.md** and **docs/07-algorithms/**.

## Bundle structure
- **docs/**: specifications, analysis, and operational guidance
- **corpus/**: test vectors (valid + invalid), RFC-derived samples, and manifests
- **tools/**: optional scripts for generating / validating vectors (e.g., V8 differential number corpus)

## Documentation standards used
This documentation uses IETF BCP 14 requirement keywords (“MUST”, “SHOULD”, etc.) per RFC 2119 and RFC 8174, and follows RFC Series editorial guidance where practical (RFC 7322 and the RFC Editor style guide). See **docs/15-style-guide.md**.

## License / provenance notes
- Standards (RFCs) are referenced by URL and cited as authoritative sources.
- Small code excerpts from third-party repositories are included only to document observed behaviors; they remain subject to the original repository’s license.
