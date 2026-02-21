# Release, Compatibility, and Change Control

**Status:** Draft

## 1. Compatibility promise
- Canonicalization output is stable for the same input bytes within a major version.
- Error codes are stable and never change meaning.

## 2. Change classes
### Patch releases
- bug fixes that do not change canonical output for valid inputs (except to fix prior non-conformance)
- improvements to diagnostics

### Minor releases
- new error codes
- new corpus vectors
- performance improvements

### Major releases
- behavior changes that can affect canonical output for previously accepted inputs
- changes to public API surface

## 3. Conformance regressions
If a regression is found against RFC 8785/7493/8259/3629:
- fix in the smallest release consistent with safety
- add regression vectors to corpus
- record an ADR if the fix affects edge-case behavior

## 4. Documentation standards
See docs/15-style-guide.md. The doc corpus is part of the “product” and changes follow the same review discipline as code.
