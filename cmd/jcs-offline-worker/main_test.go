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
	if cfg.schemaVersion != replay.EvidenceSchemaVersion {
		t.Fatalf("unexpected default schema version: %q", cfg.schemaVersion)
	}

	cfg, err = parseWorkerArgs([]string{
		"--bundle", "/tmp/offline-bundle.tgz",
		"--evidence", "/tmp/offline-evidence.json",
		"--node-id", "n1",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "3",
		"--schema-version", replay.EvidenceSchemaVersionV2,
	})
	if err != nil {
		t.Fatalf("parseWorkerArgs v2: %v", err)
	}
	if cfg.schemaVersion != replay.EvidenceSchemaVersionV2 {
		t.Fatalf("unexpected explicit schema version: %q", cfg.schemaVersion)
	}

	cfg, err = parseWorkerArgs([]string{
		"--bundle", "/tmp/offline-bundle.tgz",
		"--evidence", "/tmp/offline-evidence.json",
		"--node-id", "n1",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "3",
		"--schema-version", replay.EvidenceSchemaVersionV3,
	})
	if err != nil {
		t.Fatalf("parseWorkerArgs v3: %v", err)
	}
	if cfg.schemaVersion != replay.EvidenceSchemaVersionV3 {
		t.Fatalf("unexpected explicit schema version: %q", cfg.schemaVersion)
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

	_, err = parseWorkerArgs([]string{
		"--bundle", "b.tgz",
		"--evidence", "e.json",
		"--node-id", "n1",
		"--mode", "container",
		"--distro", "debian",
		"--kernel-family", "host",
		"--replay-index", "1",
		"--schema-version", "evidence.v99",
	})
	if err == nil {
		t.Fatal("expected schema version validation error")
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

func TestParseCPUInfoFields(t *testing.T) {
	raw := strings.Join([]string{
		"processor\t: 0",
		"CPU architecture : 8",
		"CPU implementer : 0x41",
		"CPU part\t: 0xd0c",
		"model name : should not override first",
		"CPU architecture : 9",
		"",
	}, "\n")
	fields := parseCPUInfoFields(raw)
	if fields["CPU architecture"] != "8" {
		t.Fatalf("unexpected CPU architecture: %q", fields["CPU architecture"])
	}
	if fields["CPU implementer"] != "0x41" {
		t.Fatalf("unexpected CPU implementer: %q", fields["CPU implementer"])
	}
	if fields["CPU part"] != "0xd0c" {
		t.Fatalf("unexpected CPU part: %q", fields["CPU part"])
	}
}

func TestFormatARMCPUSummary(t *testing.T) {
	got := formatARMCPUSummary(map[string]string{
		"CPU architecture": "8",
		"CPU implementer":  "0x41",
		"CPU part":         "0xd0c",
	})
	want := "ARM arch 8 impl 0x41 part 0xd0c"
	if got != want {
		t.Fatalf("unexpected ARM summary: got=%q want=%q", got, want)
	}

	if got := formatARMCPUSummary(map[string]string{}); got != "" {
		t.Fatalf("expected empty summary, got %q", got)
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
