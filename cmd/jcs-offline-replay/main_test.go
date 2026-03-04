package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
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

func TestRunSubcommandHelp(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"cross-arch", "--help"}, &out, &err)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, err.String())
	}
	if !strings.Contains(out.String(), "usage: jcs-offline-replay") {
		t.Fatalf("unexpected usage output: %q", out.String())
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

func TestParseKVBoolFlagsAndUnexpectedArg(t *testing.T) {
	flags, err := parseKV([]string{"--strict", "--run-official-vectors"})
	if err != nil {
		t.Fatalf("parseKV bool flags: %v", err)
	}
	if flags["--strict"] != boolTrue || flags["--run-official-vectors"] != boolTrue {
		t.Fatalf("unexpected bool flags: %#v", flags)
	}

	_, err = parseKV([]string{"not-a-flag"})
	if err == nil {
		t.Fatal("expected unexpected argument error")
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

func TestParseTimeout(t *testing.T) {
	got, err := parseTimeout(map[string]string{})
	if err != nil {
		t.Fatalf("default parseTimeout: %v", err)
	}
	if got != 12*time.Hour {
		t.Fatalf("unexpected default timeout: %v", got)
	}

	got, err = parseTimeout(map[string]string{"--timeout": "90s"})
	if err != nil {
		t.Fatalf("parse timeout: %v", err)
	}
	if got != 90*time.Second {
		t.Fatalf("unexpected timeout: %v", got)
	}

	if _, err := parseTimeout(map[string]string{"--timeout": "0"}); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}

func TestResolveVerifyPaths(t *testing.T) {
	evidencePath := "/tmp/release/x86_64/offline-evidence.json"
	bundle, control := resolveVerifyPaths(map[string]string{}, evidencePath)
	if bundle != "/tmp/release/x86_64/offline-bundle.tgz" {
		t.Fatalf("unexpected default bundle path: %q", bundle)
	}
	if control != "/tmp/release/x86_64/bin/jcs-canon" {
		t.Fatalf("unexpected default control path: %q", control)
	}

	bundle, control = resolveVerifyPaths(map[string]string{
		"--bundle":         "/custom/bundle.tgz",
		"--control-binary": "/custom/jcs-canon",
	}, evidencePath)
	if bundle != "/custom/bundle.tgz" || control != "/custom/jcs-canon" {
		t.Fatalf("unexpected explicit verify paths: bundle=%q control=%q", bundle, control)
	}
}

func TestResolveExpectedSourceIdentity(t *testing.T) {
	t.Setenv("JCS_OFFLINE_EXPECTED_GIT_COMMIT", "abc123")
	t.Setenv("JCS_OFFLINE_EXPECTED_GIT_TAG", "v0.0.0-test")
	commit, tag := resolveExpectedSourceIdentity(map[string]string{})
	if commit != "abc123" || tag != "v0.0.0-test" {
		t.Fatalf("unexpected env identity: commit=%q tag=%q", commit, tag)
	}

	commit, tag = resolveExpectedSourceIdentity(map[string]string{
		"--source-git-commit": "fff",
		"--source-git-tag":    "v1.2.3",
	})
	if commit != "fff" || tag != "v1.2.3" {
		t.Fatalf("unexpected explicit identity: commit=%q tag=%q", commit, tag)
	}
}

func TestAdapterFactoryValidation(t *testing.T) {
	factory := adapterFactory()
	_, err := factory(replay.NodeSpec{
		ID:   "bad-container",
		Mode: replay.NodeModeContainer,
		Runner: replay.RunnerConfig{
			Kind: "libvirt_command",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "mode=container") {
		t.Fatalf("expected container kind validation error, got %v", err)
	}

	_, err = factory(replay.NodeSpec{
		ID:   "bad-vm",
		Mode: replay.NodeModeVM,
		Runner: replay.RunnerConfig{
			Kind: "container_command",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "mode=vm") {
		t.Fatalf("expected vm kind validation error, got %v", err)
	}
}
