# ADR-0003: Number Serialization Under Project Control + V8 Differential Lockdown

- **ADR ID:** ADR-0003
- **Date:** 2026-02-21
- **Status:** Accepted

## Context
RFC 8785 states the number serialization algorithm is complex and is not included; V8 may serve as a reference and Ryu is compatible. RFC 8785 also recommends validation against a large corpus or V8 live reference.  
Source: RFC 8785 §3.2.2 and Appendix guidance: https://www.rfc-editor.org/rfc/rfc8785

## Decision
Implement an ECMAScript-compatible number serializer in Go under project control and lock behavior using:
- RFC 8785 Appendix B samples
- V8 differential corpus (Node-based generator)

## Rationale
Minimizes reliance on Go formatter behavior and provides defensible audit evidence.

## Consequences
Need to maintain the number serializer carefully and keep corpus evidence current.
