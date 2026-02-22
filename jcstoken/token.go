// Package jcstoken provides a strict JSON tokenizer/parser for RFC 8785 JCS.
//
// It enforces RFC 8259 JSON grammar plus RFC 7493 I-JSON constraints required
// by JCS, including duplicate-key rejection after unescaping and strict string
// scalar validation (no lone surrogates, no noncharacters).
//
// All errors are returned as *jcserr.Error with a populated FailureClass.
package jcstoken

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lattice-substrate/json-canon/jcserr"
)

// Limits for denial-of-service protection (BOUND-* requirements).
const (
	DefaultMaxDepth         = 1000
	DefaultMaxInputSize     = 64 * 1024 * 1024
	DefaultMaxValues        = 1_000_000
	DefaultMaxObjectMembers = 250_000
	DefaultMaxArrayElements = 250_000
	DefaultMaxStringBytes   = 8 * 1024 * 1024
	DefaultMaxNumberChars   = 4096
)

// Value represents a parsed JSON value.
type Value struct {
	Kind    Kind
	Str     string   // For KindString: decoded Unicode string; for KindBool: "true"/"false"
	Num     float64  // For KindNumber: IEEE 754 double
	Members []Member // For KindObject: ordered members
	Elems   []Value  // For KindArray: ordered elements
}

// Kind identifies the type of a JSON value.
type Kind int

const (
	// KindNull represents JSON null.
	KindNull Kind = iota
	// KindBool represents JSON true/false.
	KindBool
	// KindNumber represents JSON numbers.
	KindNumber
	// KindString represents JSON strings.
	KindString
	// KindArray represents JSON arrays.
	KindArray
	// KindObject represents JSON objects.
	KindObject
)

// Member is a key-value pair in a JSON object.
type Member struct {
	Key   string
	Value Value
}

// Options controls parser behavior.
type Options struct {
	MaxDepth         int
	MaxInputSize     int
	MaxValues        int
	MaxObjectMembers int
	MaxArrayElements int
	MaxStringBytes   int
	MaxNumberChars   int
}

func (o *Options) maxDepth() int {
	if o != nil && o.MaxDepth > 0 {
		return o.MaxDepth
	}
	return DefaultMaxDepth
}

func (o *Options) maxInputSize() int {
	if o != nil && o.MaxInputSize > 0 {
		return o.MaxInputSize
	}
	return DefaultMaxInputSize
}

func (o *Options) maxValues() int {
	if o != nil && o.MaxValues > 0 {
		return o.MaxValues
	}
	return DefaultMaxValues
}

func (o *Options) maxObjectMembers() int {
	if o != nil && o.MaxObjectMembers > 0 {
		return o.MaxObjectMembers
	}
	return DefaultMaxObjectMembers
}

func (o *Options) maxArrayElements() int {
	if o != nil && o.MaxArrayElements > 0 {
		return o.MaxArrayElements
	}
	return DefaultMaxArrayElements
}

func (o *Options) maxStringBytes() int {
	if o != nil && o.MaxStringBytes > 0 {
		return o.MaxStringBytes
	}
	return DefaultMaxStringBytes
}

func (o *Options) maxNumberChars() int {
	if o != nil && o.MaxNumberChars > 0 {
		return o.MaxNumberChars
	}
	return DefaultMaxNumberChars
}

// parser holds the state for parsing.
type parser struct {
	data             []byte
	pos              int
	depth            int
	valueCount       int
	maxDepth         int
	maxValues        int
	maxObjectMembers int
	maxArrayElements int
	maxStringBytes   int
	maxNumberChars   int
}

// Parse parses a complete JSON text under RFC 8785's strict input domain.
// PARSE-UTF8-001: Input must be valid UTF-8.
// PARSE-GRAM-008: Trailing content rejected.
func Parse(data []byte) (*Value, error) {
	return ParseWithOptions(data, nil)
}

// ParseWithOptions is like Parse but accepts configuration options.
func ParseWithOptions(data []byte, opts *Options) (*Value, error) {
	// BOUND-INPUT-001
	maxInput := opts.maxInputSize()
	if len(data) > maxInput {
		return nil, jcserr.New(jcserr.BoundExceeded, 0,
			fmt.Sprintf("input size %d exceeds maximum %d", len(data), maxInput))
	}

	// PARSE-UTF8-001, PARSE-UTF8-002
	if !utf8.Valid(data) {
		return nil, jcserr.New(jcserr.InvalidUTF8, firstInvalidUTF8Offset(data),
			"input is not valid UTF-8")
	}

	p := &parser{
		data:             data,
		pos:              0,
		depth:            0,
		valueCount:       0,
		maxDepth:         opts.maxDepth(),
		maxValues:        opts.maxValues(),
		maxObjectMembers: opts.maxObjectMembers(),
		maxArrayElements: opts.maxArrayElements(),
		maxStringBytes:   opts.maxStringBytes(),
		maxNumberChars:   opts.maxNumberChars(),
	}

	p.skipWhitespace()
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	// PARSE-GRAM-008
	if p.pos != len(p.data) {
		return nil, p.newError("trailing content after JSON value")
	}
	return v, nil
}

func (p *parser) newError(msg string) *jcserr.Error {
	return jcserr.New(jcserr.InvalidGrammar, p.pos, msg)
}

func (p *parser) newErrorf(class jcserr.FailureClass, format string, args ...any) *jcserr.Error {
	return jcserr.New(class, p.pos, fmt.Sprintf(format, args...))
}

func firstInvalidUTF8Offset(data []byte) int {
	for i := 0; i < len(data); {
		_, size := utf8.DecodeRune(data[i:])
		if size == 1 && data[i] >= 0x80 {
			return i
		}
		i += size
	}
	return 0
}

func (p *parser) peek() (byte, bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	return p.data[p.pos], true
}

func (p *parser) next() (byte, bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	b := p.data[p.pos]
	p.pos++
	return b, true
}

func (p *parser) expect(b byte) *jcserr.Error {
	c, ok := p.next()
	if !ok {
		return p.newErrorf(jcserr.InvalidGrammar, "unexpected end of input, expected %q", string(b))
	}
	if c != b {
		return p.newErrorf(jcserr.InvalidGrammar, "expected %q, got %q", string(b), string(c))
	}
	return nil
}

// PARSE-GRAM-006: Insignificant whitespace accepted.
func (p *parser) skipWhitespace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

// BOUND-DEPTH-001.
func (p *parser) pushDepth() *jcserr.Error {
	p.depth++
	if p.depth > p.maxDepth {
		return p.newErrorf(jcserr.BoundExceeded,
			"nesting depth %d exceeds maximum %d", p.depth, p.maxDepth)
	}
	return nil
}

func (p *parser) popDepth() {
	p.depth--
}

func (p *parser) parseValue() (*Value, error) {
	// BOUND-VALUES-001
	p.valueCount++
	if p.valueCount > p.maxValues {
		return nil, p.newErrorf(jcserr.BoundExceeded,
			"value count %d exceeds maximum %d", p.valueCount, p.maxValues)
	}

	c, ok := p.peek()
	if !ok {
		return nil, p.newError("unexpected end of input")
	}

	switch c {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		return p.parseString()
	case 't', 'f':
		return p.parseBool()
	case 'n':
		return p.parseNull()
	default:
		return p.parseNumber()
	}
}

//nolint:gocyclo,cyclop,gocognit // REQ:IJSON-DUP-001 duplicate-key enforcement keeps this parser path branch-heavy by design.
func (p *parser) parseObject() (*Value, error) {
	if err := p.pushDepth(); err != nil {
		return nil, err
	}
	defer p.popDepth()

	if err := p.expect('{'); err != nil {
		return nil, err
	}
	p.skipWhitespace()

	v := &Value{Kind: KindObject}
	seen := make(map[string]int)

	c, ok := p.peek()
	if !ok {
		return nil, p.newError("unexpected end of input in object")
	}
	if c == '}' {
		p.pos++
		return v, nil
	}

	for {
		p.skipWhitespace()
		keyStart := p.pos

		keyVal, err := p.parseString()
		if err != nil {
			return nil, err
		}
		key := keyVal.Str

		// IJSON-DUP-001, IJSON-DUP-002: duplicate check after escape decoding
		if firstOff, exists := seen[key]; exists {
			return nil, jcserr.New(jcserr.DuplicateKey, keyStart,
				fmt.Sprintf("duplicate object key %q (first at byte %d)", key, firstOff))
		}
		seen[key] = keyStart

		p.skipWhitespace()
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		p.skipWhitespace()

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		// BOUND-MEMBERS-001
		if len(v.Members) >= p.maxObjectMembers {
			return nil, p.newErrorf(jcserr.BoundExceeded,
				"object member count exceeds maximum %d", p.maxObjectMembers)
		}
		v.Members = append(v.Members, Member{Key: key, Value: *val})

		p.skipWhitespace()
		c, ok := p.peek()
		if !ok {
			return nil, p.newError("unexpected end of input in object")
		}
		if c == '}' {
			p.pos++
			return v, nil
		}
		if c == ',' {
			p.pos++
			continue
		}
		return nil, p.newErrorf(jcserr.InvalidGrammar,
			"expected ',' or '}' in object, got %q", string(c))
	}
}

//nolint:gocyclo,cyclop // REQ:PARSE-GRAM-005 array grammar parser path is explicit for deterministic error offsets.
func (p *parser) parseArray() (*Value, error) {
	if err := p.pushDepth(); err != nil {
		return nil, err
	}
	defer p.popDepth()

	if err := p.expect('['); err != nil {
		return nil, err
	}
	p.skipWhitespace()

	v := &Value{Kind: KindArray}

	c, ok := p.peek()
	if !ok {
		return nil, p.newError("unexpected end of input in array")
	}
	if c == ']' {
		p.pos++
		return v, nil
	}

	for {
		p.skipWhitespace()
		elem, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		// BOUND-ELEMS-001
		if len(v.Elems) >= p.maxArrayElements {
			return nil, p.newErrorf(jcserr.BoundExceeded,
				"array element count exceeds maximum %d", p.maxArrayElements)
		}
		v.Elems = append(v.Elems, *elem)

		p.skipWhitespace()
		c, ok := p.peek()
		if !ok {
			return nil, p.newError("unexpected end of input in array")
		}
		if c == ']' {
			p.pos++
			return v, nil
		}
		if c == ',' {
			p.pos++
			continue
		}
		return nil, p.newErrorf(jcserr.InvalidGrammar,
			"expected ',' or ']' in array, got %q", string(c))
	}
}

// parseString parses a JSON string and decodes all escapes.
// IJSON-SUR-001..003: Surrogate handling.
// IJSON-NONC-001: Noncharacter rejection.
// PARSE-GRAM-004: Unescaped control character rejection.
//
//nolint:gocyclo,cyclop,gocognit // REQ:PARSE-GRAM-004 string decode/validation follows RFC and I-JSON rules with explicit branch points.
func (p *parser) parseString() (*Value, error) {
	if err := p.expect('"'); err != nil {
		return nil, err
	}

	var buf []byte
	for {
		if p.pos >= len(p.data) {
			return nil, p.newError("unterminated string")
		}
		b := p.data[p.pos]
		if b == '"' {
			p.pos++
			return &Value{Kind: KindString, Str: string(buf)}, nil
		}
		if b == '\\' {
			escapeStart := p.pos
			p.pos++
			r, err := p.parseEscape(escapeStart)
			if err != nil {
				return nil, err
			}
			if err := validateStringRune(r, escapeStart); err != nil {
				return nil, err
			}
			var tmp [4]byte
			n := utf8.EncodeRune(tmp[:], r)
			if len(buf)+n > p.maxStringBytes {
				return nil, p.newErrorf(jcserr.BoundExceeded,
					"string decoded length exceeds maximum %d bytes", p.maxStringBytes)
			}
			buf = append(buf, tmp[:n]...)
			continue
		}
		// PARSE-GRAM-004: reject unescaped control characters
		if b < 0x20 {
			return nil, p.newErrorf(jcserr.InvalidGrammar,
				"unescaped control character 0x%02X in string", b)
		}
		// Copy UTF-8 character
		sourceOffset := p.pos
		r, size := utf8.DecodeRune(p.data[p.pos:])
		if r == utf8.RuneError && size <= 1 {
			return nil, p.newErrorf(jcserr.InvalidUTF8,
				"invalid UTF-8 byte 0x%02X in string", b)
		}
		if err := validateStringRune(r, sourceOffset); err != nil {
			return nil, err
		}
		if len(buf)+size > p.maxStringBytes {
			return nil, p.newErrorf(jcserr.BoundExceeded,
				"string decoded length exceeds maximum %d bytes", p.maxStringBytes)
		}
		buf = append(buf, p.data[p.pos:p.pos+size]...)
		p.pos += size
	}
}

// parseEscape handles the character after '\'.
func (p *parser) parseEscape(sourceOffset int) (rune, *jcserr.Error) {
	if p.pos >= len(p.data) {
		return 0, jcserr.New(jcserr.InvalidGrammar, sourceOffset, "unterminated escape sequence")
	}
	b := p.data[p.pos]
	p.pos++

	if b == 'u' {
		return p.parseUnicodeEscape(sourceOffset)
	}
	// PARSE-GRAM-010: Valid escape characters
	r, ok := escapedRune(b)
	if !ok {
		return 0, jcserr.New(jcserr.InvalidGrammar, sourceOffset, fmt.Sprintf("invalid escape character %q", string(b)))
	}
	return r, nil
}

// parseUnicodeEscape parses \uXXXX (and \uXXXX\uXXXX for surrogate pairs).
//
//nolint:gocyclo,cyclop // REQ:IJSON-SUR-001 surrogate validation paths are explicit to preserve failure-class semantics.
func (p *parser) parseUnicodeEscape(sourceOffset int) (rune, *jcserr.Error) {
	r1, err := p.readHex4(sourceOffset)
	if err != nil {
		return 0, err
	}

	if !utf16.IsSurrogate(r1) {
		return r1, nil
	}
	// IJSON-SUR-002: lone low surrogate
	if r1 >= 0xDC00 {
		return 0, jcserr.New(jcserr.LoneSurrogate, sourceOffset, fmt.Sprintf("lone low surrogate U+%04X", r1))
	}

	// IJSON-SUR-001: high surrogate must be followed by \uXXXX low surrogate
	if p.pos+1 >= len(p.data) || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
		return 0, jcserr.New(jcserr.LoneSurrogate, sourceOffset, fmt.Sprintf("lone high surrogate U+%04X (no following \\u)", r1))
	}
	secondEscapeOffset := p.pos
	p.pos += 2

	r2, err := p.readHex4(secondEscapeOffset)
	if err != nil {
		return 0, err
	}
	if r2 < 0xDC00 || r2 > 0xDFFF {
		return 0, jcserr.New(
			jcserr.LoneSurrogate,
			secondEscapeOffset,
			fmt.Sprintf("high surrogate U+%04X followed by non-low-surrogate U+%04X", r1, r2),
		)
	}

	// IJSON-SUR-003: valid pair decoded to supplementary-plane scalar
	decoded := utf16.DecodeRune(r1, r2)
	if decoded == unicode.ReplacementChar {
		return 0, jcserr.New(jcserr.LoneSurrogate, sourceOffset,
			fmt.Sprintf("invalid surrogate pair U+%04X U+%04X", r1, r2))
	}
	return decoded, nil
}

func escapedRune(b byte) (rune, bool) {
	switch b {
	case '"':
		return '"', true
	case '\\':
		return '\\', true
	case '/':
		return '/', true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	default:
		return 0, false
	}
}

// readHex4 reads exactly 4 hex digits and returns the rune value.
func (p *parser) readHex4(sourceOffset int) (rune, *jcserr.Error) {
	if p.pos+4 > len(p.data) {
		return 0, jcserr.New(jcserr.InvalidGrammar, sourceOffset, "incomplete \\u escape")
	}
	hex := string(p.data[p.pos : p.pos+4])
	p.pos += 4
	val, err := strconv.ParseUint(hex, 16, 16)
	if err != nil {
		return 0, jcserr.New(jcserr.InvalidGrammar, sourceOffset, fmt.Sprintf("invalid hex in \\u escape: %q", hex))
	}
	return rune(val), nil
}

// validateStringRune enforces scalar policy with source-byte offsets.
func validateStringRune(r rune, sourceOffset int) *jcserr.Error {
	if IsNoncharacter(r) {
		return jcserr.New(jcserr.Noncharacter, sourceOffset,
			fmt.Sprintf("string contains Unicode noncharacter U+%04X", r))
	}
	if r >= 0xD800 && r <= 0xDFFF {
		return jcserr.New(jcserr.LoneSurrogate, sourceOffset,
			fmt.Sprintf("string contains surrogate code point U+%04X", r))
	}
	return nil
}

// IsNoncharacter returns true if r is a Unicode noncharacter.
// IJSON-NONC-001: U+FDD0..U+FDEF and U+xFFFE, U+xFFFF for planes 0-16.
func IsNoncharacter(r rune) bool {
	if r >= 0xFDD0 && r <= 0xFDEF {
		return true
	}
	if r&0xFFFE == 0xFFFE && r <= 0x10FFFF {
		return true
	}
	return false
}

// parseNumber parses a JSON number.
// PARSE-GRAM-001: leading zeros rejected.
// PARSE-GRAM-009: number grammar enforced.
// PROF-NEGZ-001: lexical -0 rejected.
// PROF-OFLOW-001: overflow rejected.
// PROF-UFLOW-001: underflow-to-zero rejected.
func (p *parser) parseNumber() (*Value, error) {
	start := p.pos

	// Optional minus sign
	if p.pos < len(p.data) && p.data[p.pos] == '-' {
		p.pos++
	}

	// Integer part
	if err := p.scanIntegerPart(); err != nil {
		return nil, err
	}

	// Optional fraction
	if err := p.scanFractionPart(); err != nil {
		return nil, err
	}

	// Optional exponent
	if err := p.scanExponentPart(); err != nil {
		return nil, err
	}

	// BOUND-NUMCHARS-001
	if p.pos-start > p.maxNumberChars {
		return nil, jcserr.New(jcserr.BoundExceeded, start,
			fmt.Sprintf("number token length %d exceeds maximum %d", p.pos-start, p.maxNumberChars))
	}

	raw := string(p.data[start:p.pos])
	return p.buildNumberValue(start, raw)
}

// PARSE-GRAM-001: leading zeros.
func (p *parser) scanIntegerPart() *jcserr.Error {
	if p.pos >= len(p.data) {
		return p.newError("unexpected end of input in number")
	}

	if p.data[p.pos] == '0' {
		p.pos++
		if p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			return p.newError("leading zero in number")
		}
		return nil
	}

	if p.data[p.pos] < '1' || p.data[p.pos] > '9' {
		return p.newErrorf(jcserr.InvalidGrammar, "invalid number character %q", string(p.data[p.pos]))
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

func (p *parser) scanFractionPart() *jcserr.Error {
	if p.pos >= len(p.data) || p.data[p.pos] != '.' {
		return nil
	}
	p.pos++

	if p.pos >= len(p.data) || !isDigit(p.data[p.pos]) {
		return p.newError("expected digit after decimal point")
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

//nolint:gocyclo,cyclop // REQ:PARSE-GRAM-009 exponent scanner mirrors JSON grammar stages for precise diagnostics.
func (p *parser) scanExponentPart() *jcserr.Error {
	if p.pos >= len(p.data) || (p.data[p.pos] != 'e' && p.data[p.pos] != 'E') {
		return nil
	}
	p.pos++

	if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
		p.pos++
	}
	if p.pos >= len(p.data) || !isDigit(p.data[p.pos]) {
		return p.newError("expected digit in exponent")
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

func (p *parser) buildNumberValue(start int, raw string) (*Value, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil && !errorsIsRange(err) {
		return nil, jcserr.New(jcserr.InvalidGrammar, start,
			fmt.Sprintf("invalid number: %v", err))
	}
	// PROF-OFLOW-001
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, jcserr.New(jcserr.NumberOverflow, start,
			"number overflows IEEE 754 double")
	}
	// PROF-NEGZ-001: lexical negative zero
	if strings.HasPrefix(raw, "-") && tokenRepresentsZero(raw) {
		return nil, jcserr.New(jcserr.NumberNegZero, start,
			"negative zero token is not allowed")
	}
	// PROF-UFLOW-001: non-zero underflows to zero
	if f == 0 && !tokenRepresentsZero(raw) {
		return nil, jcserr.New(jcserr.NumberUnderflow, start,
			"non-zero number underflows to IEEE 754 zero")
	}
	return &Value{Kind: KindNumber, Num: f}, nil
}

func tokenRepresentsZero(raw string) bool {
	start := 0
	if len(raw) > 0 && raw[0] == '-' {
		start = 1
	}
	end := len(raw)
	for i := start; i < len(raw); i++ {
		if raw[i] == 'e' || raw[i] == 'E' {
			end = i
			break
		}
	}
	for i := start; i < end; i++ {
		if raw[i] >= '1' && raw[i] <= '9' {
			return false
		}
	}
	return true
}

func errorsIsRange(err error) bool {
	if err == nil {
		return false
	}
	var numErr *strconv.NumError
	ok := errors.As(err, &numErr)
	if !ok {
		return false
	}
	return errors.Is(numErr.Err, strconv.ErrRange)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// PARSE-GRAM-007: invalid literals rejected.
func (p *parser) parseBool() (*Value, error) {
	if p.pos+4 <= len(p.data) && string(p.data[p.pos:p.pos+4]) == "true" {
		p.pos += 4
		return &Value{Kind: KindBool, Str: "true"}, nil
	}
	if p.pos+5 <= len(p.data) && string(p.data[p.pos:p.pos+5]) == "false" {
		p.pos += 5
		return &Value{Kind: KindBool, Str: "false"}, nil
	}
	return nil, p.newError("invalid literal")
}

func (p *parser) parseNull() (*Value, error) {
	if p.pos+4 <= len(p.data) && string(p.data[p.pos:p.pos+4]) == "null" {
		p.pos += 4
		return &Value{Kind: KindNull}, nil
	}
	return nil, p.newError("invalid literal")
}
