# Performance, Limits, and Resource Safety

**Status:** Draft

## 1. Time complexity
- Sorting objects dominates: O(n log n) comparisons per object; keys compared by UTF‑16 code units.
- Arrays are linear but recurse into contained objects.

## 2. Memory considerations
- Each object must be buffered to sort members (RFC 8785 requires deterministic sorting).
- Large objects can consume memory proportional to number of members + key/value sizes.

## 3. Safety limits
To align with RFC 8785 security considerations (“sanity checks on input data”), implement configurable limits:
- max nesting depth
- max string length
- max object members
- max total input size

Source: RFC 8785 Security Considerations. https://www.rfc-editor.org/rfc/rfc8785

## 4. Failure modes
Limits MUST fail closed with deterministic error codes.
