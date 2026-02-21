# Number Corpora

**Status:** Draft

## Files
- `rfc8785_appendix_b.csv` — IEEE‑754 hex patterns and expected JSON strings from RFC 8785 Appendix B (“Number Serialization Samples”).  
  Source: https://www.rfc-editor.org/rfc/rfc8785

## Usage
- Appendix B contains both finite and non-finite samples (NaN/Infinity). RFC 8785 states NaN/Infinity MUST cause an error in a compliant JCS implementation; therefore those entries are expected to be rejected when encountered as numeric values.  
  Source: RFC 8785 §3.2.2.3: https://www.rfc-editor.org/rfc/rfc8785

## Extensions
- A larger V8 differential corpus should be generated and stored separately (or generated in CI) due to size.
