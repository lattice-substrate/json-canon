[Previous: Contributing, Traceability, and Change Control](11-contributing-traceability-and-change-control.md) | [Book Home](README.md)

# Chapter 13: FAQ

## Is this a general JSON parser library?

No. The project is a canonicalization-focused system with strict validation and
stable CLI behavior guarantees.

## Does this support non-Linux runtime targets?

No. Supported runtime platform is Linux.

## Why does `verify` use stderr for `ok`?

It is part of the stable CLI stream contract for machine consumers and remains
versioned behavior.

## Is offline replay optional for release candidates?

For release-grade assurance, no. Release validation expects both architecture
evidence gates (`x86_64` and `arm64`) to pass with current artifacts.

## If unit tests pass, why run offline replay?

Unit/conformance tests prove local correctness. Offline replay proves
cross-lane and cross-architecture determinism under controlled cold-start
conditions.

## Why is there a module dependency if core runtime claims minimal deps?

The canonicalization core is standard-library based. The repository module also
contains:

- operational tooling that uses `gopkg.in/yaml.v3` for matrix/profile handling,
- differential conformance tests that import Cyberphone Go JCS
  (`github.com/cyberphone/json-canonicalization`).

## Where should new contributors start?

Read, in order:

1. `AGENTS.md`
2. `docs/book/README.md`
3. `README.md`
4. `ARCHITECTURE.md`
5. `CONFORMANCE.md`

[Previous: Contributing, Traceability, and Change Control](11-contributing-traceability-and-change-control.md) | [Book Home](README.md)
