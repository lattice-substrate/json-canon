package replay

import (
	"strings"
	"testing"
)

func TestValidateEvidenceBundleParity(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	if err := ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
}

func TestValidateEvidenceBundleDetectsDrift(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[3] = mkRun("v1", "vm", "ubuntu", "ga", 2, strings.Repeat("b", 64))
	if err := ValidateEvidenceBundle(e, m, p, opts); err == nil {
		t.Fatal("expected drift validation error")
	}
}

func TestValidateEvidenceBundleRejectsTamperedMetadata(t *testing.T) {
	m, p, base, opts := validEvidenceFixture()
	tests := []struct {
		name   string
		tamper func(*EvidenceBundle)
		want   string
	}{
		{
			name: "bundle_sha256",
			tamper: func(e *EvidenceBundle) {
				e.BundleSHA256 = strings.Repeat("b", 64)
			},
			want: "bundle_sha256 mismatch",
		},
		{
			name: "control_binary_sha256",
			tamper: func(e *EvidenceBundle) {
				e.ControlBinarySHA = strings.Repeat("b", 64)
			},
			want: "control_binary_sha256 mismatch",
		},
		{
			name: "matrix_sha256",
			tamper: func(e *EvidenceBundle) {
				e.MatrixSHA256 = strings.Repeat("b", 64)
			},
			want: "matrix_sha256 mismatch",
		},
		{
			name: "profile_sha256",
			tamper: func(e *EvidenceBundle) {
				e.ProfileSHA256 = strings.Repeat("b", 64)
			},
			want: "profile_sha256 mismatch",
		},
		{
			name: "architecture",
			tamper: func(e *EvidenceBundle) {
				e.Architecture = "arm64"
			},
			want: "architecture mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := cloneEvidence(base)
			tc.tamper(e)
			err := ValidateEvidenceBundle(e, m, p, opts)
			if err == nil {
				t.Fatalf("expected %s validation error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func validEvidenceFixture() (*Matrix, *Profile, *EvidenceBundle, EvidenceValidationOptions) {
	m := &Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []NodeSpec{
			{ID: "c1", Mode: NodeModeContainer, Distro: "debian", KernelFamily: "host", Replays: 2, Runner: RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
			{ID: "v1", Mode: NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: RunnerConfig{Kind: "libvirt_command", Replay: []string{"true"}}},
		},
	}
	p := &Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	digest := strings.Repeat("a", 64)
	e := &EvidenceBundle{
		SchemaVersion:      EvidenceSchemaVersion,
		BundleSHA256:       digest,
		ControlBinarySHA:   digest,
		MatrixSHA256:       digest,
		ProfileSHA256:      digest,
		ProfileName:        "max",
		Architecture:       "x86_64",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: digest,
		AggregateVerify:    digest,
		AggregateClass:     digest,
		AggregateExitCode:  digest,
		NodeReplays: []NodeRunEvidence{
			mkRun("c1", "container", "debian", "host", 1, digest),
			mkRun("c1", "container", "debian", "host", 2, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 1, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 2, digest),
		},
	}
	opts := EvidenceValidationOptions{
		ExpectedBundleSHA256:        digest,
		ExpectedControlBinarySHA256: digest,
		ExpectedMatrixSHA256:        digest,
		ExpectedProfileSHA256:       digest,
		ExpectedArchitecture:        "x86_64",
	}
	return m, p, e, opts
}

func cloneEvidence(in *EvidenceBundle) *EvidenceBundle {
	out := *in
	out.RequiredSuites = append([]string(nil), in.RequiredSuites...)
	out.NodeReplays = append([]NodeRunEvidence(nil), in.NodeReplays...)
	return &out
}

func mkRun(nodeID, mode, distro, kernel string, replayIndex int, digest string) NodeRunEvidence {
	return NodeRunEvidence{
		NodeID:             nodeID,
		Mode:               mode,
		Distro:             distro,
		KernelFamily:       kernel,
		ReplayIndex:        replayIndex,
		SessionID:          "sess",
		StartedAtUTC:       "2026-01-01T00:00:00Z",
		CompletedAtUTC:     "2026-01-01T00:00:01Z",
		CaseCount:          10,
		Passed:             true,
		CanonicalSHA256:    digest,
		VerifySHA256:       digest,
		FailureClassSHA256: digest,
		ExitCodeSHA256:     digest,
	}
}
