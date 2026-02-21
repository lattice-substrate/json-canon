# ADR-0004: UTF‑16 Code Unit Sorting for Object Keys

- **ADR ID:** ADR-0004
- **Date:** 2026-02-21
- **Status:** Accepted

## Context
RFC 8785 requires sorting property name strings by UTF‑16 code units (unsigned comparisons, locale independent).  
Source: RFC 8785 §3.2.3: https://www.rfc-editor.org/rfc/rfc8785

## Decision
Convert keys to UTF‑16 code units and sort using unsigned comparisons per RFC 8785.

## Rationale
Ensures compatibility with RFC 8785 and with ECMAScript-based ecosystems.

## Consequences
Requires careful handling of surrogate pairs and correct UTF‑16 conversion (already necessary for JSON escape handling).
