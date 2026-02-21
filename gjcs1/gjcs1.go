// Package gjcs1 implements the GJCS1 governed JSON file envelope as specified
// by the lattice-canon specification.
//
// GJCS1 = JCS(value) || 0x0A
//
// This package provides:
//   - Envelope: wraps JCS bytes into GJCS1 format
//   - Verify: validates that a byte sequence is valid GJCS1
//   - Write: atomically writes a GJCS1 file (temp + rename)
//
// File-level constraints are enforced before JSON parsing per §5.2:
//   - UTF-8 validity (no BOM, no CR, no invalid sequences)
//   - Exactly one trailing LF
//   - Non-empty JCS body
package gjcs1

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"lattice-canon/jcs"
	"lattice-canon/jcstoken"
)

// Error codes matching lattice-substrate failure taxonomy.
const (
	ExitSuccess      = 0
	ExitInvalidInput = 2
	ExitInternal     = 10
)

// EnvelopeError indicates a file-level constraint violation detected
// before JSON parsing.
type EnvelopeError struct {
	Msg string
}

func (e *EnvelopeError) Error() string {
	return fmt.Sprintf("gjcs1: envelope: %s", e.Msg)
}

// CanonError indicates the file parsed but is not canonical.
type CanonError struct {
	Msg string
}

func (e *CanonError) Error() string {
	return fmt.Sprintf("gjcs1: non-canonical: %s", e.Msg)
}

// Envelope wraps JCS canonical bytes with a single trailing LF to form GJCS1.
func Envelope(jcsBody []byte) []byte {
	result := make([]byte, len(jcsBody)+1)
	copy(result, jcsBody)
	result[len(jcsBody)] = 0x0A
	return result
}

// Verify validates that data is a conforming GJCS1 file.
//
// It enforces constraints in the order specified by §5.2:
//  1. File-level constraints (§3.2) before JSON parsing
//  2. Strict JSON parsing (§3.3)
//  3. Re-serialization and byte comparison
//
// Returns nil if the file is valid GJCS1. Returns an EnvelopeError for
// file-level violations, a jcstoken.ParseError for strict domain violations,
// or a CanonError if the body is valid JSON but not canonical.
func Verify(data []byte) error {
	// Step 1: File-level constraints (§3.2) — enforced BEFORE parsing.
	body, err := checkEnvelope(data)
	if err != nil {
		return err
	}

	// Step 2: Strict JSON parsing (§3.3)
	v, err := jcstoken.Parse(body)
	if err != nil {
		return fmt.Errorf("gjcs1: parse body: %w", err)
	}

	// Step 3: Re-serialize and byte-compare
	canonical, err := jcs.Serialize(v)
	if err != nil {
		return fmt.Errorf("gjcs1: internal: re-serialization failed: %w", err)
	}

	if !bytesEqual(body, canonical) {
		return &CanonError{Msg: "JCS body bytes differ from canonical re-serialization"}
	}

	return nil
}

// checkEnvelope enforces all file-level constraints from §3.2.
// Returns the JCS body (data without the trailing LF) or an EnvelopeError.
func checkEnvelope(data []byte) ([]byte, error) {
	if err := requireNonEmptyFile(data); err != nil {
		return nil, err
	}
	if err := requireSingleTrailingLF(data); err != nil {
		return nil, err
	}

	body := data[:len(data)-1]

	if err := requireNonEmptyBody(body); err != nil {
		return nil, err
	}
	if err := requireNoBOM(body); err != nil {
		return nil, err
	}
	if err := requireNoCR(data); err != nil {
		return nil, err
	}
	if err := requireNoLFInBody(body); err != nil {
		return nil, err
	}
	if err := requireValidUTF8(body); err != nil {
		return nil, err
	}
	if err := checkNoSurrogateUTF8(body); err != nil {
		return nil, err
	}

	return body, nil
}

// findInvalidUTF8 returns the byte offset of the first invalid UTF-8 sequence.
func findInvalidUTF8(data []byte) int {
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			return i
		}
		i += size
	}
	return len(data)
}

// checkNoSurrogateUTF8 checks for 3-byte UTF-8 sequences that would decode to
// surrogate code points (U+D800-U+DFFF). These are invalid per RFC 3629 and
// Go's utf8.Valid already rejects them, but this provides an explicit check
// with a clear error message.
func checkNoSurrogateUTF8(data []byte) *EnvelopeError {
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r >= 0xD800 && r <= 0xDFFF {
			return &EnvelopeError{
				Msg: fmt.Sprintf("surrogate code point U+%04X in UTF-8 at offset %d", r, i),
			}
		}
		i += size
	}
	return nil
}

// Canonicalize parses JSON text and returns the JCS canonical bytes.
// Does not append a trailing LF (use Envelope for GJCS1).
func Canonicalize(input []byte) ([]byte, error) {
	return CanonicalizeWithOptions(input, nil)
}

// CanonicalizeWithOptions is like Canonicalize but accepts parser options.
func CanonicalizeWithOptions(input []byte, opts *jcstoken.Options) ([]byte, error) {
	v, err := jcstoken.ParseWithOptions(input, opts)
	if err != nil {
		return nil, fmt.Errorf("gjcs1: parse input: %w", err)
	}
	canonical, err := jcs.Serialize(v)
	if err != nil {
		return nil, fmt.Errorf("gjcs1: serialize input: %w", err)
	}
	return canonical, nil
}

// WriteAtomic writes GJCS1 bytes to the given path atomically using
// temp file + rename. On failure, the temp file is cleaned up and no
// file is left at the target path.
//
// This operation is atomic on Linux local filesystems when the temp file
// and target are on the same mount. Only Linux is supported.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".gjcs1-*.tmp")
	if err != nil {
		return fmt.Errorf("gjcs1: create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Ensure cleanup on any failure
	success := false
	defer func() {
		if !success {
			if closeErr := tmp.Close(); closeErr != nil {
				// Best-effort cleanup.
			}
			if removeErr := os.Remove(tmpPath); removeErr != nil {
				// Best-effort cleanup.
			}
		}
	}()

	_, err = tmp.Write(data)
	if err != nil {
		return fmt.Errorf("gjcs1: write temp file: %w", err)
	}

	// Sync data to disk
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("gjcs1: sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("gjcs1: close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("gjcs1: rename temp to final: %w", err)
	}

	success = true

	// Best-effort directory sync for crash-consistent durability (POSIX)
	syncDir(dir)

	return nil
}

// syncDir attempts to fsync the directory for crash-consistent durability.
// Errors are ignored (this is a SHOULD, not a MUST).
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	if syncErr := d.Sync(); syncErr != nil {
		if closeErr := d.Close(); closeErr != nil {
			return
		}
		return
	}
	if closeErr := d.Close(); closeErr != nil {
		return
	}
}

// WriteGoverned canonicalizes JSON input and writes it as a GJCS1 file atomically.
func WriteGoverned(path string, input []byte) error {
	canonical, err := Canonicalize(input)
	if err != nil {
		return fmt.Errorf("gjcs1: canonicalize governed input: %w", err)
	}
	gjcs1 := Envelope(canonical)
	return WriteAtomic(path, gjcs1)
}

// VerifyReader reads all bytes from r and verifies them as GJCS1.
func VerifyReader(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("gjcs1: read error: %w", err)
	}
	return Verify(data)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func requireNonEmptyFile(data []byte) error {
	if len(data) == 0 {
		return &EnvelopeError{Msg: "file is empty"}
	}
	return nil
}

func requireSingleTrailingLF(data []byte) error {
	if data[len(data)-1] != 0x0A {
		return &EnvelopeError{Msg: "missing trailing LF"}
	}
	if len(data) >= 2 && data[len(data)-2] == 0x0A {
		return &EnvelopeError{Msg: "multiple trailing LFs"}
	}
	return nil
}

func requireNonEmptyBody(body []byte) error {
	if len(body) == 0 {
		return &EnvelopeError{Msg: "empty JCS body (file contains only LF)"}
	}
	return nil
}

func requireNoBOM(body []byte) error {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		return &EnvelopeError{Msg: "UTF-8 BOM detected"}
	}
	return nil
}

func requireNoCR(data []byte) error {
	for i, b := range data {
		if b == 0x0D {
			return &EnvelopeError{Msg: fmt.Sprintf("CR byte (0x0D) at offset %d", i)}
		}
	}
	return nil
}

func requireNoLFInBody(body []byte) error {
	for i, b := range body {
		if b == 0x0A {
			return &EnvelopeError{Msg: fmt.Sprintf("LF byte in JCS body at offset %d", i)}
		}
	}
	return nil
}

func requireValidUTF8(body []byte) error {
	if utf8.Valid(body) {
		return nil
	}
	offset := findInvalidUTF8(body)
	return &EnvelopeError{Msg: fmt.Sprintf("invalid UTF-8 at offset %d", offset)}
}
