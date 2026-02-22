package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(stderr.String(), string(jcserr.CLIUsage)) {
		t.Fatalf("expected CLI_USAGE class token, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage output, got %q", stderr.String())
	}
}

func TestRunTopLevelHelpExitZero(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "usage: jcs-canon") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	stdout.Reset()
	code = run([]string{"-h"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "usage: jcs-canon") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
}

func TestRunTopLevelVersionExitZero(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "jcs-canon v") {
		t.Fatalf("expected version output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunSubcommandHelpExitZeroStdout(t *testing.T) {
	for _, cmd := range []string{"canonicalize", "verify"} {
		t.Run(cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{cmd, "--help"}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("expected exit 0, got %d", code)
			}
			if !strings.Contains(stdout.String(), "usage: jcs-canon "+cmd) {
				t.Fatalf("expected help on stdout, got stdout=%q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got %q", stderr.String())
			}
		})
	}
}

func TestRunUnknownCommandExitCode(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"bogus"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != jcserr.CLIUsage.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.CLIUsage.ExitCode(), code)
	}
	if !strings.Contains(stderr.String(), string(jcserr.CLIUsage)) {
		t.Fatalf("expected CLI_USAGE class token, got %q", stderr.String())
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
	assertClass(t, err, jcserr.CLIUsage)
}

func TestParseFlagsDoubleDashRejected(t *testing.T) {
	_, _, err := parseFlags([]string{"--"})
	if err == nil {
		t.Fatal("expected parseFlags error for --")
	}
	assertClass(t, err, jcserr.CLIUsage)
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

func TestRunVerifySuccessWriteFailure(t *testing.T) {
	code := run(
		[]string{"verify", "-"},
		strings.NewReader(`{"a":1}`),
		&bytes.Buffer{},
		failingWriter{},
	)
	if code != jcserr.InternalIO.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalIO.ExitCode(), code)
	}
}

func TestRunTopLevelHelpWriteFailure(t *testing.T) {
	code := run([]string{"--help"}, strings.NewReader(""), failingWriter{}, &bytes.Buffer{})
	if code != jcserr.InternalIO.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalIO.ExitCode(), code)
	}
}

func TestRunTopLevelVersionWriteFailure(t *testing.T) {
	code := run([]string{"--version"}, strings.NewReader(""), failingWriter{}, &bytes.Buffer{})
	if code != jcserr.InternalIO.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalIO.ExitCode(), code)
	}
}

func TestRunSubcommandHelpWriteFailure(t *testing.T) {
	code := run([]string{"canonicalize", "--help"}, strings.NewReader(""), failingWriter{}, &bytes.Buffer{})
	if code != jcserr.InternalIO.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.InternalIO.ExitCode(), code)
	}
}

func TestRunVerifyNotCanonicalIncludesClass(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader(`{"b":1,"a":2}`),
		&bytes.Buffer{},
		&stderr,
	)
	if code != jcserr.NotCanonical.ExitCode() {
		t.Fatalf("expected exit %d, got %d", jcserr.NotCanonical.ExitCode(), code)
	}
	if !strings.Contains(stderr.String(), string(jcserr.NotCanonical)) {
		t.Fatalf("expected NOT_CANONICAL class token, got %q", stderr.String())
	}
}

func TestReadInputOversizeClassBoundExceededForStdinAndFile(t *testing.T) {
	const maxInput = 8
	oversized := strings.Repeat("x", maxInput+1)

	_, err := readInput(nil, strings.NewReader(oversized), maxInput)
	if err == nil {
		t.Fatal("expected oversize stdin failure")
	}
	assertClass(t, err, jcserr.BoundExceeded)

	dir := t.TempDir()
	p := filepath.Join(dir, "oversized.json")
	if writeErr := os.WriteFile(p, []byte(oversized), 0o600); writeErr != nil {
		t.Fatalf("write oversized fixture: %v", writeErr)
	}

	_, err = readInput([]string{p}, strings.NewReader(""), maxInput)
	if err == nil {
		t.Fatal("expected oversize file failure")
	}
	assertClass(t, err, jcserr.BoundExceeded)
}

func TestReadInputDirectoryPathReturnsCLIUsage(t *testing.T) {
	_, err := readInput([]string{t.TempDir()}, strings.NewReader(""), 64)
	if err == nil {
		t.Fatal("expected directory read failure")
	}
	assertClass(t, err, jcserr.CLIUsage)
}

func TestReadInputMissingFileReturnsCLIUsage(t *testing.T) {
	_, err := readInput([]string{filepath.Join(t.TempDir(), "missing.json")}, strings.NewReader(""), 64)
	if err == nil {
		t.Fatal("expected missing file failure")
	}
	assertClass(t, err, jcserr.CLIUsage)
}

func assertClass(t *testing.T, err error, class jcserr.FailureClass) {
	t.Helper()
	var je *jcserr.Error
	if !errors.As(err, &je) {
		t.Fatalf("expected jcserr.Error, got %T (%v)", err, err)
	}
	if je.Class != class {
		t.Fatalf("expected class %s, got %s (%v)", class, je.Class, err)
	}
}
