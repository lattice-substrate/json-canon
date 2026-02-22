package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestInspectMatrix(t *testing.T) {
	dir := t.TempDir()
	matrix := filepath.Join(dir, "matrix.yaml")
	data := []byte(`{
  "version": "v1",
  "architecture": "x86_64",
  "nodes": [
    {
      "id": "n1",
      "mode": "container",
      "distro": "debian",
      "kernel_family": "host",
      "replays": 1,
      "runner": {
        "kind": "container_command",
        "replay": ["echo", "ok"]
      }
    },
    {
      "id": "n2",
      "mode": "vm",
      "distro": "ubuntu",
      "kernel_family": "ga",
      "replays": 1,
      "runner": {
        "kind": "libvirt_command",
        "replay": ["echo", "ok"]
      }
    }
  ]
}
`)
	if err := os.WriteFile(matrix, data, 0o600); err != nil {
		t.Fatalf("write matrix fixture: %v", err)
	}

	var out, err bytes.Buffer
	code := run([]string{"inspect-matrix", "--matrix", matrix}, &out, &err)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, err.String())
	}
	if !strings.Contains(out.String(), "\"architecture\": \"x86_64\"") {
		t.Fatalf("unexpected inspect output: %q", out.String())
	}
}
