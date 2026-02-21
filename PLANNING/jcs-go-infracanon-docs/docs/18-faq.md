# FAQ

**Status:** Draft

## Why not allow “JSON-ish” inputs?
Because audited cryptographic workflows require fail-closed behavior. Accepting invalid JSON and canonicalizing it is silent normalization and breaks auditability.

## Why is key sorting based on UTF‑16?
RFC 8785 mandates sorting by UTF‑16 code units for compatibility with ECMAScript, Java, and .NET string models.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## Why is number formatting so strict?
RFC 8785 requires ECMAScript-compatible number serialization and suggests validating against V8; Appendix B provides sample mappings.  
Source: https://www.rfc-editor.org/rfc/rfc8785

## Can we support big integers?
I‑JSON recommends encoding numbers requiring exactness beyond binary64 as strings.  
Source: https://www.rfc-editor.org/rfc/rfc7493.html
