# Documentation Style Guide

**Status:** Draft  
**Goal:** documentation quality and traceability are treated as part of correctness.

## 1. Requirements language
All normative requirements in this doc set use BCP 14 keywords as defined by RFC 2119 and RFC 8174.  
Sources:
- RFC 2119: https://datatracker.ietf.org/doc/html/rfc2119
- RFC 8174: https://www.rfc-editor.org/rfc/rfc8174.html

## 2. Structure conventions (RFC-style)
This project borrows structural and editorial guidance from the RFC Style Guide:
- clear sectioning,
- explicit terminology,
- normative vs informative references,
- precise language.  
Source: RFC 7322: https://datatracker.ietf.org/doc/html/rfc7322

The RFC Editor maintains web updates to the style guide; where it conflicts, we follow the newer RFC Editor guidance.  
Source: https://www.rfc-editor.org/styleguide/part2/

## 3. Requirements registry conventions
The requirement registry format is designed to support:
- unambiguous statements,
- testable acceptance criteria,
- traceability and change control.

These conventions are inspired by requirements engineering best practices as described in ISO/IEC/IEEE 29148 (overview pages referenced; the standard text is not reproduced).  
Sources:
- IEEE overview: https://standards.ieee.org/standard/29148-2018.html
- ISO overview portal: https://www.iso.org/obp/ui/en/

## 4. Referencing rules
- Cite authoritative sources whenever making factual claims about standards or tooling behavior.
- Prefer primary sources: RFCs, official language/library docs, and upstream source repositories.
- Keep quoted excerpts short and only as necessary to support analysis.

## 5. Document status and versioning
Each document includes a **Status** line (Draft/Accepted/etc.). The bundle has a bundle version (see root README).

## 6. Change control
- Any change that affects requirements, API contract, or error code meaning requires an ADR.
- Any change to the corpus requires a manifest update and rationale.
