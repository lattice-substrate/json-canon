[Book Home](README.md) | [Next: What This Project Is](01-what-this-project-is.md)

# Chapter 1: How To Use This Handbook

This handbook is operational documentation for people who run, integrate,
audit, or maintain `json-canon`.

## Documentation Levels

Use documents at the right level:

1. Handbook chapters: orientation and practical guidance.
2. Root contracts: authoritative behavior, policy, and ABI commitments.
3. Test and conformance artifacts: executable proof.

When guidance conflicts, root contracts and requirement registries win.

## What To Read First

- If you are evaluating whether to adopt this project, read chapters 2-4.
- If you are integrating the CLI into automation, read chapters 6 and 13.
- If you are validating release candidate safety, read chapters 8-11.
- If you are contributing code, read chapter 12.

## How To Validate Claims

For every important claim, verify at least one concrete artifact:

- Behavior claims -> `SPECIFICATION.md` + tests.
- ABI claims -> `ABI.md` + `abi_manifest.json`.
- Release gate claims -> `CONFORMANCE.md`, `RELEASE_PROCESS.md`, and
  `offline/conformance` tests.
- Security claims -> `THREAT_MODEL.md` and `SECURITY.md`.

## Fast Orientation Checklist

1. Confirm Linux-only runtime requirement.
2. Confirm static build requirement (`CGO_ENABLED=0`).
3. Confirm required Go-native conformance gates.
4. Confirm offline replay evidence requirements for release.
5. Confirm CLI command/flag/exit semantics from ABI docs.

[Book Home](README.md) | [Next: What This Project Is](01-what-this-project-is.md)
