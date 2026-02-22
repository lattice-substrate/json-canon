package main

import (
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
