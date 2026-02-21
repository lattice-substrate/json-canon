package gjcs1_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lattice-canon/gjcs1"
	"lattice-canon/jcstoken"
)

func TestEnvelopeAppendsLF(t *testing.T) {
	got := gjcs1.Envelope([]byte(`{"a":1}`))
	if string(got) != "{\"a\":1}\n" {
		t.Fatalf("got %q", string(got))
	}
}

func TestVerifyValidObject(t *testing.T) {
	if err := gjcs1.Verify([]byte("{\"a\":1}\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyValidNonObject(t *testing.T) {
	if err := gjcs1.Verify([]byte("42\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsBOM(t *testing.T) {
	err := gjcs1.Verify([]byte{0xEF, 0xBB, 0xBF, '{', '}', '\n'})
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsCR(t *testing.T) {
	err := gjcs1.Verify([]byte("{}\r\n"))
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsInvalidUTF8(t *testing.T) {
	err := gjcs1.Verify([]byte{'"', 0xFF, 0xFE, '"', '\n'})
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsMissingTrailingLF(t *testing.T) {
	err := gjcs1.Verify([]byte("{}"))
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsMultipleTrailingLF(t *testing.T) {
	err := gjcs1.Verify([]byte("{}\n\n"))
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsEmptyBody(t *testing.T) {
	err := gjcs1.Verify([]byte("\n"))
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsDuplicateKey(t *testing.T) {
	err := gjcs1.Verify([]byte("{\"a\":1,\"a\":2}\n"))
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsLoneSurrogate(t *testing.T) {
	err := gjcs1.Verify([]byte("\"\\uD800\"\n"))
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsNoncharacter(t *testing.T) {
	err := gjcs1.Verify([]byte("\"\\uFDD0\"\n"))
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsUnsortedKeys(t *testing.T) {
	err := gjcs1.Verify([]byte("{\"b\":1,\"a\":2}\n"))
	var ce *gjcs1.CanonError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CanonError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsNegativeZero(t *testing.T) {
	err := gjcs1.Verify([]byte("-0\n"))
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T (%v)", err, err)
	}
}

func TestVerifyRejectsUnderflowZero(t *testing.T) {
	err := gjcs1.Verify([]byte("1e-400\n"))
	var pe *jcstoken.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseError, got %T (%v)", err, err)
	}
}

func TestVerifyEnvelopeOrderBeforeParse(t *testing.T) {
	err := gjcs1.Verify([]byte{0xEF, 0xBB, 0xBF, '{', '"', 'a', '"', ':', '1', ',', '"', 'a', '"', ':', '2', '}', '\n'})
	var ee *gjcs1.EnvelopeError
	if !errors.As(err, &ee) {
		t.Fatalf("expected EnvelopeError precedence, got %T (%v)", err, err)
	}
}

func TestWriteAtomicAndVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.gjcs1")
	data := []byte("{\"a\":1}\n")
	if err := gjcs1.WriteAtomic(path, data); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := gjcs1.Verify(b); err != nil {
		t.Fatalf("Verify written file: %v", err)
	}
}

func TestWriteGoverned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.gjcs1")
	if err := gjcs1.WriteGoverned(path, []byte(`{"z":3,"a":1}`)); err != nil {
		t.Fatalf("WriteGoverned: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "{\"a\":1,\"z\":3}\n" {
		t.Fatalf("unexpected contents: %q", string(b))
	}
}

func TestVerifyReader(t *testing.T) {
	if err := gjcs1.VerifyReader(strings.NewReader("{\"a\":1}\n")); err != nil {
		t.Fatalf("VerifyReader: %v", err)
	}
}
