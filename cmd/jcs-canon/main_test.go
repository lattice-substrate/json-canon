package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/jcserr"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestWriteClassifiedErrorWrapped(t *testing.T) {
	inner := jcserr.New(jcserr.InvalidUTF8, 3, "bad byte")
	err := fmt.Errorf("outer: %w", inner)
	var stderr bytes.Buffer
	code := writeClassifiedError(&stderr, err)
	if code != jcserr.InvalidUTF8.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InvalidUTF8.ExitCode(), code)
	}
}

func TestWriteClassifiedErrorFallback(t *testing.T) {
	err := fmt.Errorf("unclassified failure")
	var stderr bytes.Buffer
	code := writeClassifiedError(&stderr, err)
	if code != jcserr.InternalError.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalError.ExitCode(), code)
	}
}

func TestRunNoCommandExitCode(t *testing.T) {
	var stderr bytes.Buffer
	code := run(nil, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != jcserr.CLIUsage.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.CLIUsage.ExitCode(), code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage output, got %q", stderr.String())
	}
}

func TestRunUnknownCommandExitCode(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"bogus"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != jcserr.CLIUsage.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.CLIUsage.ExitCode(), code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got %q", stderr.String())
	}
}

func TestParseFlagsUnknownOption(t *testing.T) {
	_, _, err := parseFlags([]string{"--nope"})
	if err == nil {
		t.Fatal("expected parseFlags error for unknown option")
	}
}

func TestRunCanonicalizeWriteFailure(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"canonicalize", "-"},
		strings.NewReader(`{"a":1}`),
		failingWriter{},
		&stderr,
	)
	if code != jcserr.InternalIO.ExitCode() {
		t.Fatalf("expected exit %d, got %d stderr=%q", jcserr.InternalIO.ExitCode(), code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "writing output") {
		t.Fatalf("expected write failure text, got %q", stderr.String())
	}
}
