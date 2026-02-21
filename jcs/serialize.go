// Package jcs implements RFC 8785 JSON Canonicalization Scheme serialization.
//
// Given a parsed Value tree (from jcstoken), this package produces the exact
// canonical byte sequence specified by RFC 8785. It depends on jcsfloat for
// ECMA-262-compliant number serialization and uses UTF-16 code-unit ordering
// for object property name sorting as required by RFC 8785 §3.2.3.
package jcs

import (
	"fmt"
	"math"
	"sort"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lattice-substrate/json-canon/jcserr"
	"github.com/lattice-substrate/json-canon/jcsfloat"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

// Serialize produces the RFC 8785 JCS canonical byte sequence for a parsed
// JSON value.
//
// CANON-ENC-001: Output is UTF-8.
// CANON-WS-001: No insignificant whitespace.
func Serialize(v *jcstoken.Value) ([]byte, error) {
	if v == nil {
		return nil, jcserr.New(jcserr.InternalError, -1, "jcs: nil value")
	}
	state := &serializeValidationState{}
	if err := validateValueTree(v, 0, state); err != nil {
		return nil, err
	}

	var buf []byte
	var err error
	buf, err = serializeValue(buf, v)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func serializeValue(buf []byte, v *jcstoken.Value) ([]byte, error) {
	switch v.Kind {
	case jcstoken.KindNull:
		// CANON-LIT-001: lowercase literals
		return append(buf, "null"...), nil
	case jcstoken.KindBool:
		// CANON-LIT-001: lowercase literals
		return append(buf, v.Str...), nil
	case jcstoken.KindNumber:
		return serializeNumber(buf, v.Num)
	case jcstoken.KindString:
		return serializeString(buf, v.Str), nil
	case jcstoken.KindArray:
		return serializeArray(buf, v)
	case jcstoken.KindObject:
		return serializeObject(buf, v)
	default:
		return nil, jcserr.New(jcserr.InternalError, -1, fmt.Sprintf("jcs: unknown value kind %d", v.Kind))
	}
}

func serializeNumber(buf []byte, f float64) ([]byte, error) {
	s, err := jcsfloat.FormatDouble(f)
	if err != nil {
		return nil, jcserr.Wrap(err.Class, -1, "jcs: number serialization error", err)
	}
	return append(buf, s...), nil
}

// serializeString applies JCS string escaping rules (RFC 8785 §3.2.2.2):
//
// CANON-STR-001: U+0008 → \b
// CANON-STR-002: U+0009 → \t
// CANON-STR-003: U+000A → \n
// CANON-STR-004: U+000C → \f
// CANON-STR-005: U+000D → \r
// CANON-STR-006: Other controls U+0000..U+001F → \u00xx lowercase
// CANON-STR-007: U+0022 → \"
// CANON-STR-008: U+005C → \\
// CANON-STR-009: U+002F (solidus) NOT escaped
// CANON-STR-010: Characters above U+001F (except " and \) → raw UTF-8
// CANON-STR-011: No Unicode normalization
func serializeString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	for i := 0; i < len(s); {
		next, consumed := appendEscapedByte(buf, s[i])
		if consumed {
			buf = next
			i++
			continue
		}

		size := byteSpanForCopy(s, i)
		buf = append(buf, s[i:i+size]...)
		i += size
	}
	buf = append(buf, '"')
	return buf
}

func appendEscapedByte(buf []byte, b byte) ([]byte, bool) {
	switch b {
	case '"': // CANON-STR-007
		return append(buf, '\\', '"'), true
	case '\\': // CANON-STR-008
		return append(buf, '\\', '\\'), true
	case '\b': // CANON-STR-001
		return append(buf, '\\', 'b'), true
	case '\t': // CANON-STR-002
		return append(buf, '\\', 't'), true
	case '\n': // CANON-STR-003
		return append(buf, '\\', 'n'), true
	case '\f': // CANON-STR-004
		return append(buf, '\\', 'f'), true
	case '\r': // CANON-STR-005
		return append(buf, '\\', 'r'), true
	default:
		if b < 0x20 { // CANON-STR-006
			return append(buf, '\\', 'u', '0', '0', hexDigit(b>>4), hexDigit(b&0x0F)), true
		}
		return buf, false
	}
}

func byteSpanForCopy(s string, i int) int {
	b := s[i]
	if b < 0x80 {
		return 1
	}
	size := utf8SeqLen(b)
	if i+size > len(s) {
		return len(s) - i
	}
	return size
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'a' + (b - 10)
}

func utf8SeqLen(b byte) int {
	switch {
	case b < 0x80:
		return 1
	case b < 0xE0:
		return 2
	case b < 0xF0:
		return 3
	default:
		return 4
	}
}

func serializeArray(buf []byte, v *jcstoken.Value) ([]byte, error) {
	// CANON-SORT-003: array order preserved
	buf = append(buf, '[')
	for i := range v.Elems {
		if i > 0 {
			buf = append(buf, ',')
		}
		var err error
		buf, err = serializeValue(buf, &v.Elems[i])
		if err != nil {
			return nil, err
		}
	}
	buf = append(buf, ']')
	return buf, nil
}

// serializeObject sorts members by key using UTF-16 code-unit ordering.
// CANON-SORT-001: UTF-16 code-unit comparison (NOT UTF-8 byte order).
// CANON-SORT-002: Recursive sorting (nested objects sorted in serializeValue).
func serializeObject(buf []byte, v *jcstoken.Value) ([]byte, error) {
	sorted := make([]sortableMember, len(v.Members))
	for i := range v.Members {
		sorted[i] = sortableMember{
			member: v.Members[i],
			key16:  utf16.Encode([]rune(v.Members[i].Key)),
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return compareUTF16Units(sorted[i].key16, sorted[j].key16) < 0
	})

	buf = append(buf, '{')
	for i := range sorted {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = serializeString(buf, sorted[i].member.Key)
		buf = append(buf, ':')
		var err error
		buf, err = serializeValue(buf, &sorted[i].member.Value)
		if err != nil {
			return nil, err
		}
	}
	buf = append(buf, '}')
	return buf, nil
}

type sortableMember struct {
	member jcstoken.Member
	key16  []uint16
}

func compareUTF16Units(ua, ub []uint16) int {
	minLen := len(ua)
	if len(ub) < minLen {
		minLen = len(ub)
	}
	for i := 0; i < minLen; i++ {
		if ua[i] < ub[i] {
			return -1
		}
		if ua[i] > ub[i] {
			return 1
		}
	}
	if len(ua) < len(ub) {
		return -1
	}
	if len(ua) > len(ub) {
		return 1
	}
	return 0
}

type serializeValidationState struct {
	values int
}

func validateValueTree(v *jcstoken.Value, depth int, state *serializeValidationState) error {
	state.values++
	if state.values > jcstoken.DefaultMaxValues {
		return jcserr.New(jcserr.BoundExceeded, -1,
			fmt.Sprintf("jcs: value count exceeds maximum %d", jcstoken.DefaultMaxValues))
	}
	if depth > jcstoken.DefaultMaxDepth {
		return jcserr.New(jcserr.BoundExceeded, -1,
			fmt.Sprintf("jcs: value nesting depth exceeds maximum %d", jcstoken.DefaultMaxDepth))
	}

	switch v.Kind {
	case jcstoken.KindNull:
		return nil
	case jcstoken.KindBool:
		if v.Str != "true" && v.Str != "false" {
			return jcserr.New(jcserr.InvalidGrammar, -1,
				fmt.Sprintf("jcs: invalid boolean payload %q", v.Str))
		}
		return nil
	case jcstoken.KindNumber:
		if math.IsNaN(v.Num) || math.IsInf(v.Num, 0) {
			return jcserr.New(jcserr.InvalidGrammar, -1, "jcs: number is not finite")
		}
		return nil
	case jcstoken.KindString:
		if err := validateString(v.Str); err != nil {
			return err
		}
		return nil
	case jcstoken.KindArray:
		if len(v.Elems) > jcstoken.DefaultMaxArrayElements {
			return jcserr.New(jcserr.BoundExceeded, -1,
				fmt.Sprintf("jcs: array element count exceeds maximum %d", jcstoken.DefaultMaxArrayElements))
		}
		for i := range v.Elems {
			if err := validateValueTree(&v.Elems[i], depth+1, state); err != nil {
				return err
			}
		}
		return nil
	case jcstoken.KindObject:
		if len(v.Members) > jcstoken.DefaultMaxObjectMembers {
			return jcserr.New(jcserr.BoundExceeded, -1,
				fmt.Sprintf("jcs: object member count exceeds maximum %d", jcstoken.DefaultMaxObjectMembers))
		}
		seen := make(map[string]struct{}, len(v.Members))
		for i := range v.Members {
			if err := validateString(v.Members[i].Key); err != nil {
				return jcserr.Wrap(err.Class, err.Offset, "jcs: invalid object key", err)
			}
			if _, ok := seen[v.Members[i].Key]; ok {
				return jcserr.New(jcserr.DuplicateKey, -1,
					fmt.Sprintf("jcs: duplicate object key %q", v.Members[i].Key))
			}
			seen[v.Members[i].Key] = struct{}{}
			if err := validateValueTree(&v.Members[i].Value, depth+1, state); err != nil {
				return err
			}
		}
		return nil
	default:
		return jcserr.New(jcserr.InternalError, -1, fmt.Sprintf("jcs: unknown value kind %d", v.Kind))
	}
}

func validateString(s string) *jcserr.Error {
	if !utf8.ValidString(s) {
		return jcserr.New(jcserr.InvalidUTF8, -1, "jcs: string is not valid UTF-8")
	}
	if len(s) > jcstoken.DefaultMaxStringBytes {
		return jcserr.New(jcserr.BoundExceeded, -1,
			fmt.Sprintf("jcs: string length exceeds maximum %d bytes", jcstoken.DefaultMaxStringBytes))
	}
	for _, r := range s {
		if jcstoken.IsNoncharacter(r) {
			return jcserr.New(jcserr.Noncharacter, -1,
				fmt.Sprintf("jcs: string contains noncharacter U+%04X", r))
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return jcserr.New(jcserr.LoneSurrogate, -1,
				fmt.Sprintf("jcs: string contains surrogate code point U+%04X", r))
		}
	}
	return nil
}

