package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var out, err bytes.Buffer
	code := run(nil, &out, &err)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "usage: jcs-offline-replay") {
		t.Fatalf("unexpected usage output: %q", out.String())
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"nope"}, &out, &err)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(err.String(), "unknown subcommand") {
		t.Fatalf("unexpected stderr: %q", err.String())
	}
}

func TestParseKV(t *testing.T) {
	flags, err := parseKV([]string{"--matrix", "a.yaml", "--profile=b.yaml"})
	if err != nil {
		t.Fatalf("parseKV: %v", err)
	}
	if flags["--matrix"] != "a.yaml" {
		t.Fatalf("unexpected matrix flag: %#v", flags)
	}
	if flags["--profile"] != "b.yaml" {
		t.Fatalf("unexpected profile flag: %#v", flags)
	}
}
