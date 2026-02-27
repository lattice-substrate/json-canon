# Cyberphone Go Differential Examples

This document records executable differential cases between:

- recorded Cyberphone Go outputs (captured as fixture data in the differential test),
- `json-canon` CLI (`jcs-canon canonicalize`)

The cases are encoded as a Go test in
`conformance/cyberphone_differential_test.go`.
The test does not import external modules.

## Cases

| Case | Input | Cyberphone Go output | `json-canon` result |
| --- | --- | --- | --- |
| Hex float accepted | `{"n":0x1p-2}` | `{"n":0.25}` | reject (`INVALID_GRAMMAR`) |
| Plus-prefixed number accepted | `{"n":+1}` | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| Leading-zero number accepted | `{"n":01}` | `{"n":1}` | reject (`INVALID_GRAMMAR`) |
| Invalid UTF-8 byte accepted | `{"s":"<0xFF>"}` | `{"s":"<0xFF>"}` | reject (`INVALID_UTF8`) |
| Invalid surrogate pair normalized | `{"s":"\uD800\u0041"}` | `{"s":"\uFFFD"}` | reject (`LONE_SURROGATE`) |

## Reproduce

```bash
go test ./conformance -run TestCyberphoneGoDifferentialInvalidAcceptance -count=1 -v
```
