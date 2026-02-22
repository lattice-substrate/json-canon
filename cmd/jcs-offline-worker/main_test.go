package main

import "testing"

func TestParseKV(t *testing.T) {
	flags, err := parseKV([]string{"--bundle", "b.tgz", "--evidence=e.json"})
	if err != nil {
		t.Fatalf("parseKV: %v", err)
	}
	if flags["--bundle"] != "b.tgz" {
		t.Fatalf("unexpected bundle flag: %#v", flags)
	}
	if flags["--evidence"] != "e.json" {
		t.Fatalf("unexpected evidence flag: %#v", flags)
	}
}

func TestExtractFailureClass(t *testing.T) {
	got := extractFailureClass("error: jcserr: CLI_USAGE: unknown option")
	if got != "CLI_USAGE" {
		t.Fatalf("expected CLI_USAGE, got %q", got)
	}
	got = extractFailureClass("no known class")
	if got != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN, got %q", got)
	}
}
