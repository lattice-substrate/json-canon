package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

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

func TestParseWorkerArgs(t *testing.T) {
	cfg, err := parseWorkerArgs([]string{
		"--bundle", "/tmp/offline-bundle.tgz",
		"--evidence", "/tmp/offline-evidence.json",
		"--node-id", "n1",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "3",
	})
	if err != nil {
		t.Fatalf("parseWorkerArgs: %v", err)
	}
	if cfg.replayIndex != 3 || cfg.nodeID != "n1" {
		t.Fatalf("unexpected worker args: %#v", cfg)
	}

	_, err = parseWorkerArgs([]string{"--bundle", "b.tgz", "--evidence", "e.json"})
	if err == nil {
		t.Fatal("expected missing required flags error")
	}

	_, err = parseWorkerArgs([]string{
		"--bundle", "b.tgz",
		"--evidence", "e.json",
		"--node-id", "n1",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "0",
	})
	if err == nil {
		t.Fatal("expected replay index validation error")
	}
}

func TestVectorArgs(t *testing.T) {
	args, err := vectorArgs(vectorCase{Args: []string{"verify", "-"}}, "f.jsonl", 3)
	if err != nil {
		t.Fatalf("vectorArgs explicit args: %v", err)
	}
	if len(args) != 2 || args[0] != "verify" {
		t.Fatalf("unexpected args: %#v", args)
	}

	args, err = vectorArgs(vectorCase{ID: "case1", Mode: "canonicalize"}, "f.jsonl", 4)
	if err != nil {
		t.Fatalf("vectorArgs mode fallback: %v", err)
	}
	if len(args) != 2 || args[0] != "canonicalize" || args[1] != "-" {
		t.Fatalf("unexpected fallback args: %#v", args)
	}

	_, err = vectorArgs(vectorCase{ID: "missing-mode"}, "f.jsonl", 5)
	if err == nil {
		t.Fatal("expected missing mode/args validation error")
	}
}

func TestAssertVectorResult(t *testing.T) {
	wantStdout := "out"
	wantContains := "OK"

	v := vectorCase{
		ID:                 "id1",
		WantExit:           0,
		WantStdout:         &wantStdout,
		WantStderrContains: &wantContains,
	}
	if err := assertVectorResult("f.jsonl", 10, v, cliResult{exitCode: 0, stdout: "out", stderr: "ok\nOK"}); err != nil {
		t.Fatalf("assertVectorResult: %v", err)
	}
	if err := assertVectorResult("f.jsonl", 10, v, cliResult{exitCode: 2, stdout: "out", stderr: "ok\nOK"}); err == nil {
		t.Fatal("expected exit mismatch error")
	}
}

func TestSafeTarMode(t *testing.T) {
	if got := safeTarMode(-1); got != 0o600 {
		t.Fatalf("unexpected negative mode fallback: %v", got)
	}
	if got := safeTarMode(0o755); got != 0o755 {
		t.Fatalf("unexpected mode: %v", got)
	}
}

func TestVerifyVectorSetChecksum(t *testing.T) {
	manifest := &replay.BundleManifest{
		VectorFiles: []string{"bundle/vectors/a.jsonl", "bundle/vectors/b.jsonl"},
		VectorSHA256: map[string]string{
			"bundle/vectors/a.jsonl": strings.Repeat("a", 64),
			"bundle/vectors/b.jsonl": strings.Repeat("b", 64),
		},
	}
	manifest.VectorSetSHA256 = computeVectorSetChecksum(manifest.VectorFiles, manifest.VectorSHA256)
	if err := verifyVectorSetChecksum(manifest); err != nil {
		t.Fatalf("verifyVectorSetChecksum: %v", err)
	}

	manifest.VectorSetSHA256 = strings.Repeat("0", 64)
	if err := verifyVectorSetChecksum(manifest); err == nil {
		t.Fatal("expected vector_set checksum mismatch")
	}
}

func computeVectorSetChecksum(files []string, checksums map[string]string) string {
	items := make([]string, 0, len(files))
	for _, rel := range files {
		items = append(items, rel+":"+checksums[rel])
	}
	sort.Strings(items)
	sum := sha256.Sum256([]byte(strings.Join(items, "\n")))
	return hex.EncodeToString(sum[:])
}
