# json-canon Handbook

This handbook is the chaptered guide for `json-canon`.

Use it to understand what the project is for, what guarantees it makes, how to
operate it, and how to evaluate release candidates.

For binding contracts, always prioritize the source-of-truth documents listed in `docs/README.md`.

## Table of Contents

1. [How To Use This Handbook](00-how-to-use-this-book.md)
2. [What This Project Is](01-what-this-project-is.md)
3. [What This Project Is Not](02-what-this-project-is-not.md)
4. [Why This Exists](03-why-this-exists.md)
5. [Architecture](04-architecture.md)
6. [How Canonicalization Works](05-how-canonicalization-works.md)
7. [CLI and ABI Contract](06-cli-and-abi.md)
8. [Offline Replay and Release Gates](07-offline-replay-and-release-gates.md)
9. [Security, Trust, and Threat Model](08-security-trust-and-threat-model.md)
10. [Operations and Runbooks](09-operations-runbooks.md)
11. [Troubleshooting](10-troubleshooting.md)
12. [Contributing, Traceability, and Change Control](11-contributing-traceability-and-change-control.md)
13. [FAQ](12-faq.md)

## Reader Paths

- New operator: chapters 1, 2, 6, 8, 10, 11
- Integrator pinning an RC: chapters 2, 6, 8, 9, 10, 13
- Maintainer preparing release: chapters 5, 7, 8, 9, 12
- Auditor: chapters 2, 4, 5, 7, 8, 9, 12

## Companion References

- System index: `docs/README.md`
- Product overview: `README.md`
- Architecture contract: `ARCHITECTURE.md`
- Normative behavior contract: `SPECIFICATION.md`
- ABI contract: `ABI.md`, `abi_manifest.json`
- Conformance policy: `CONFORMANCE.md`
- Release workflow: `RELEASE_PROCESS.md`
- Artifact verification: `VERIFICATION.md`
- Differential strictness examples: `docs/CYBERPHONE_DIFFERENTIAL_EXAMPLES.md`
- Offline harness runbook: `docs/OFFLINE_REPLAY_HARNESS.md`
