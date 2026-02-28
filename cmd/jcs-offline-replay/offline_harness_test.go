package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

func TestParseBoolToken(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    bool
		wantErr bool
	}{
		{name: "true", raw: "true", want: true},
		{name: "one", raw: "1", want: true},
		{name: "false", raw: "false", want: false},
		{name: "zero", raw: "0", want: false},
		{name: "invalid", raw: "maybe", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBoolToken(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBoolToken(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("parseBoolToken(%q)=%t want %t", tc.raw, got, tc.want)
			}
		})
	}
}

func TestBuildAuditSummary(t *testing.T) {
	digest := strings.Repeat("a", 64)
	evidence := &replay.EvidenceBundle{
		SchemaVersion:      replay.EvidenceSchemaVersion,
		ProfileName:        "maximal-offline",
		Architecture:       "x86_64",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: digest,
		AggregateVerify:    digest,
		AggregateClass:     digest,
		AggregateExitCode:  digest,
		NodeReplays: []replay.NodeRunEvidence{
			{NodeID: "node-a", ReplayIndex: 1, CaseCount: 10, CanonicalSHA256: digest, VerifySHA256: digest, FailureClassSHA256: digest, ExitCodeSHA256: digest},
			{NodeID: "node-a", ReplayIndex: 2, CaseCount: 10, CanonicalSHA256: digest, VerifySHA256: digest, FailureClassSHA256: digest, ExitCodeSHA256: digest},
			{NodeID: "node-b", ReplayIndex: 1, CaseCount: 10, CanonicalSHA256: digest, VerifySHA256: digest, FailureClassSHA256: digest, ExitCodeSHA256: digest},
		},
	}
	summary := buildAuditSummary(evidence, "m.json", "p.json", "e.json", time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC))
	if summary.Result != "PASS" {
		t.Fatalf("result=%s want PASS", summary.Result)
	}
	if !summary.Parity["canonical_single_digest"] {
		t.Fatal("expected canonical parity true")
	}
	if summary.NodeReplayCounts["node-a"] != 2 {
		t.Fatalf("node-a replay count=%d want 2", summary.NodeReplayCounts["node-a"])
	}
}

func TestBuildAuditSummaryDetectsParityFailure(t *testing.T) {
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	evidence := &replay.EvidenceBundle{
		SchemaVersion:      replay.EvidenceSchemaVersion,
		ProfileName:        "maximal-offline",
		Architecture:       "x86_64",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: digestA,
		AggregateVerify:    digestA,
		AggregateClass:     digestA,
		AggregateExitCode:  digestA,
		NodeReplays: []replay.NodeRunEvidence{
			{NodeID: "node-a", ReplayIndex: 1, CaseCount: 10, CanonicalSHA256: digestA, VerifySHA256: digestA, FailureClassSHA256: digestA, ExitCodeSHA256: digestA},
			{NodeID: "node-b", ReplayIndex: 1, CaseCount: 10, CanonicalSHA256: digestB, VerifySHA256: digestA, FailureClassSHA256: digestA, ExitCodeSHA256: digestA},
		},
	}
	summary := buildAuditSummary(evidence, "m.json", "p.json", "e.json", time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC))
	if summary.Result != "FAIL" {
		t.Fatalf("result=%s want FAIL", summary.Result)
	}
	if summary.Parity["canonical_single_digest"] {
		t.Fatal("expected canonical parity false")
	}
}

func TestBuildCrossArchReport(t *testing.T) {
	digest := strings.Repeat("a", 64)
	x86 := &replay.EvidenceBundle{AggregateCanonical: digest, AggregateVerify: digest, AggregateClass: digest, AggregateExitCode: digest}
	arm := &replay.EvidenceBundle{AggregateCanonical: digest, AggregateVerify: digest, AggregateClass: digest, AggregateExitCode: digest}
	report := buildCrossArchReport(x86, arm, "x86.json", "arm.json", time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC))
	if report.Result != "PASS" {
		t.Fatalf("result=%s want PASS", report.Result)
	}
	if len(report.Checks) != 4 {
		t.Fatalf("checks=%d want 4", len(report.Checks))
	}
}

func TestWriteChecksumFile_RepoRelativePaths(t *testing.T) {
	repoRoot := t.TempDir()
	subDir := filepath.Join(repoRoot, "offline", "runs", "v1", "audit")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(repoRoot, "offline", "runs", "v1", "offline-bundle.tgz")
	if err := os.WriteFile(artifactPath, []byte("test-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksumPath := filepath.Join(subDir, "bundle.sha256")
	if err := writeChecksumFile(checksumPath, artifactPath, repoRoot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if strings.Contains(line, repoRoot) {
		t.Fatalf("checksum file contains absolute repo root %q: %s", repoRoot, line)
	}
	if !strings.Contains(line, "offline/runs/v1/offline-bundle.tgz") {
		t.Fatalf("checksum file missing relative artifact path: %s", line)
	}
}

func TestWriteRunIndex_RepoRelativePaths(t *testing.T) {
	repoRoot := t.TempDir()
	outDir := filepath.Join(repoRoot, "offline", "runs", "v1", "x86_64")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(outDir, "RUN_INDEX.txt")
	artifacts := runSuiteArtifacts{
		OutputDir:      outDir,
		BundlePath:     filepath.Join(outDir, "offline-bundle.tgz"),
		EvidencePath:   filepath.Join(outDir, "offline-evidence.json"),
		ControllerPath: filepath.Join(outDir, "bin", "jcs-offline-replay"),
		CanonicalPath:  filepath.Join(outDir, "bin", "jcs-canon"),
		MatrixPath:     "offline/matrix.yaml",
		ProfilePath:    "offline/profiles/maximal.yaml",
	}
	if err := writeRunIndex(indexPath, artifacts, repoRoot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, repoRoot) {
		t.Fatalf("RUN_INDEX.txt contains absolute repo root %q:\n%s", repoRoot, content)
	}
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := parts[1]
		if filepath.IsAbs(val) {
			t.Fatalf("RUN_INDEX.txt key %q has absolute path: %s", parts[0], val)
		}
	}
}

func TestBuildAuditSummary_RepoRelativePaths(t *testing.T) {
	digest := strings.Repeat("a", 64)
	evidence := &replay.EvidenceBundle{
		SchemaVersion:      replay.EvidenceSchemaVersion,
		ProfileName:        "maximal-offline",
		Architecture:       "x86_64",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: digest,
		AggregateVerify:    digest,
		AggregateClass:     digest,
		AggregateExitCode:  digest,
		NodeReplays: []replay.NodeRunEvidence{
			{NodeID: "node-a", ReplayIndex: 1, CaseCount: 10, CanonicalSHA256: digest, VerifySHA256: digest, FailureClassSHA256: digest, ExitCodeSHA256: digest},
		},
	}
	relPath := "offline/runs/v1/x86_64/offline-evidence.json"
	summary := buildAuditSummary(evidence, "offline/matrix.yaml", "offline/profiles/maximal.yaml", relPath, time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC))
	if filepath.IsAbs(summary.EvidencePath) {
		t.Fatalf("audit summary evidence_path is absolute: %s", summary.EvidencePath)
	}
	if summary.EvidencePath != relPath {
		t.Fatalf("evidence_path=%q want %q", summary.EvidencePath, relPath)
	}
}

func TestBuildCrossArchReport_RepoRelativePaths(t *testing.T) {
	digest := strings.Repeat("a", 64)
	x86 := &replay.EvidenceBundle{AggregateCanonical: digest, AggregateVerify: digest, AggregateClass: digest, AggregateExitCode: digest}
	arm := &replay.EvidenceBundle{AggregateCanonical: digest, AggregateVerify: digest, AggregateClass: digest, AggregateExitCode: digest}
	x86Rel := "offline/runs/v1/x86_64/offline-evidence.json"
	armRel := "offline/runs/v1/arm64/offline-evidence.json"
	report := buildCrossArchReport(x86, arm, x86Rel, armRel, time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC))
	if filepath.IsAbs(report.X86Evidence) {
		t.Fatalf("x86_evidence is absolute: %s", report.X86Evidence)
	}
	if filepath.IsAbs(report.Arm64Evidence) {
		t.Fatalf("arm64_evidence is absolute: %s", report.Arm64Evidence)
	}
	if report.X86Evidence != x86Rel {
		t.Fatalf("x86_evidence=%q want %q", report.X86Evidence, x86Rel)
	}
	if report.Arm64Evidence != armRel {
		t.Fatalf("arm64_evidence=%q want %q", report.Arm64Evidence, armRel)
	}
}
