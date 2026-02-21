package main

import (
	"bytes"
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

func TestCanonicalizeGJCS1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"canonicalize", "--gjcs1"},
		strings.NewReader(`{"a":1}`),
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := stdout.Bytes()
	if got[len(got)-1] != 0x0A {
		t.Error("GJCS1 output missing trailing LF")
	}
}

func TestVerifyValid(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader("{\"a\":1}\n"),
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
		strings.NewReader("{\"b\":1,\"a\":2}\n"),
		nil, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d: %s", code, stderr.String())
	}
}

func TestVerifyRejectsMissingLF(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		[]string{"verify", "--quiet", "-"},
		strings.NewReader(`{"a":1}`),
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
		strings.NewReader("-0\n"),
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
