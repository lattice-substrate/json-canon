package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"canonicalize"},
		strings.NewReader(`  { "z" : 3, "a" : 1 }  `),
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := stdout.String()
	expect := `{"a":1,"z":3}`
	if got != expect {
		t.Errorf("got %q, want %q", got, expect)
	}
}

func TestVerifyValid(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader("{\"a\":1}"),
		nil, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
}

func TestVerifyRejectsNonCanonical(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader("{\"b\":1,\"a\":2}"),
		nil, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d: %s", code, stderr.String())
	}
}

func TestVerifyRejectsTrailingWhitespace(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader("{\"a\":1}\n"),
		nil, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestVerifyRejectsNegativeZero(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader("-0"),
		nil, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"bogus"}, nil, nil, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestNoCommand(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{}, nil, nil, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestReadBoundedRejectsLargeInput(t *testing.T) {
	_, err := readBounded(strings.NewReader("abcd"), 3)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadBoundedAcceptsLimit(t *testing.T) {
	got, err := readBounded(strings.NewReader("abc"), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("unexpected payload: %q", got)
	}
}

func TestReadInputFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/in.json"
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	got, err := readInput([]string{path}, strings.NewReader(""), 1024)
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("unexpected input: %q", got)
	}
}
