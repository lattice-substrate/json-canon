# Normative References and Contribution Map

**Status:** Draft  
**Rule:** When this project says “MUST/SHOULD/MAY”, it uses BCP 14 (RFC 2119 + RFC 8174). See **docs/03-conformance-terms.md**.

## 1. Normative (required) references

### RFC 8785 — JSON Canonicalization Scheme (JCS)
- Defines canonical JSON generation steps: whitespace elimination, UTF‑16 key sorting, UTF‑8 output encoding, and ECMAScript-compatible primitive serialization.
- Explicitly constrains input to I‑JSON and treats the output as “hashable” for cryptographic use.
- Number serialization algorithm is not restated; V8 and Ryu are cited as reference implementations; Appendix B provides sample mappings; Appendix guidance recommends differential testing against V8.  
Source: https://www.rfc-editor.org/rfc/rfc8785

### RFC 8259 — The JavaScript Object Notation (JSON) Data Interchange Format
- Defines JSON grammar and the strict JSON number syntax (base‑10, optional minus, no leading zeros, no NaN/Infinity).  
Source: https://datatracker.ietf.org/doc/html/rfc8259

### RFC 7493 — The I‑JSON Message Format
- Defines the constrained JSON profile required by RFC 8785:
  - MUST be UTF‑8.
  - MUST NOT contain Surrogates or Noncharacters (in names/strings), including directly encoded UTF‑8 or escaped forms.
  - MUST NOT contain duplicate object member names after unescaping.
  - Guidance for protocols to reject or not trust non-conforming messages.
  - Numeric range and “exact integer” guidance.  
Source: https://www.rfc-editor.org/rfc/rfc7493.html

### RFC 3629 — UTF‑8, a transformation format of ISO 10646
- Defines legal UTF‑8 sequences (restricted to U+0000..U+10FFFF) and prohibits encoding surrogate code points.
- Requires protection against invalid UTF‑8 sequences; notes security consequences of invalid decoding.  
Source: https://www.rfc-editor.org/rfc/rfc3629

### RFC 2119 and RFC 8174 — Requirements language (BCP 14)
- Normative keyword meanings and capitalization rules.  
Sources:
- https://datatracker.ietf.org/doc/html/rfc2119
- https://www.rfc-editor.org/rfc/rfc8174.html

## 2. Informative references (guidance / style)

### RFC 7322 — RFC Style Guide
Used as a style reference for structure and clarity of standards-like documents in this project’s docs.  
Source: https://datatracker.ietf.org/doc/html/rfc7322

### RFC Editor Style Guide (Web portion)
Some guidance here updates parts of RFC 7322 (RFC Editor notes).  
Source: https://www.rfc-editor.org/styleguide/part2/

### IEEE/ISO/IEC 29148 — Requirements engineering guidance (informative)
This project’s requirements registry conventions are inspired by ISO/IEC/IEEE 29148 requirements engineering concepts (good requirement characteristics, traceability). The standard text is not reproduced here.  
Sources:
- ISO overview: https://www.iso.org/obp/ui/en/
- IEEE overview page: https://standards.ieee.org/standard/29148-2018.html

## 3. Primary-source references for observed implementation behaviors

### cyberphone/json-canonicalization (Go)
Primary source for the behaviors analyzed in **docs/12-comparative-analysis/cyberphone-analysis.md**.  
Source: https://github.com/cyberphone/json-canonicalization

### Go standard library sources
Used to cite actual `encoding/json` behavior for invalid UTF‑8 coercion and other relevant behavior.  
Source: https://go.googlesource.com/go/
