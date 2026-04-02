package replay_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	testCPUIntel  = "Intel"
	testKernel610 = "6.1.0"
)

func TestValidateEvidenceBundleParity(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
}

func TestValidateEvidenceBundleDetectsDrift(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[3] = mkRun("v1", "vm", "ubuntu", "ga", 2, strings.Repeat("b", 64))
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err == nil {
		t.Fatal("expected drift validation error")
	}
}

func TestValidateEvidenceBundleRejectsTamperedMetadata(t *testing.T) {
	m, p, base, opts := validEvidenceFixture()
	tests := []struct {
		name   string
		tamper func(*replay.EvidenceBundle)
		want   string
	}{
		{
			name: "bundle_sha256",
			tamper: func(e *replay.EvidenceBundle) {
				e.BundleSHA256 = strings.Repeat("b", 64)
			},
			want: "bundle_sha256 mismatch",
		},
		{
			name: "control_binary_sha256",
			tamper: func(e *replay.EvidenceBundle) {
				e.ControlBinarySHA = strings.Repeat("b", 64)
			},
			want: "control_binary_sha256 mismatch",
		},
		{
			name: "matrix_sha256",
			tamper: func(e *replay.EvidenceBundle) {
				e.MatrixSHA256 = strings.Repeat("b", 64)
			},
			want: "matrix_sha256 mismatch",
		},
		{
			name: "profile_sha256",
			tamper: func(e *replay.EvidenceBundle) {
				e.ProfileSHA256 = strings.Repeat("b", 64)
			},
			want: "profile_sha256 mismatch",
		},
		{
			name: "architecture",
			tamper: func(e *replay.EvidenceBundle) {
				e.Architecture = "arm64"
			},
			want: "architecture mismatch",
		},
		{
			name: "source_git_commit",
			tamper: func(e *replay.EvidenceBundle) {
				e.SourceGitCommit = strings.Repeat("b", 40)
			},
			want: "source_git_commit mismatch",
		},
		{
			name: "source_git_tag",
			tamper: func(e *replay.EvidenceBundle) {
				e.SourceGitTag = "v0.0.0-wrong"
			},
			want: "source_git_tag mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := cloneEvidence(base)
			tc.tamper(e)
			err := replay.ValidateEvidenceBundle(e, m, p, opts)
			if err == nil {
				t.Fatalf("expected %s validation error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateEvidenceBundleRejectsMalformedNodeDigestTokens(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[0].CanonicalSHA256 = "abc"

	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected malformed canonical digest validation error")
	}
	if !strings.Contains(err.Error(), "canonical_sha256 must be 64 hex characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleRejectsMalformedAggregateDigestTokens(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.AggregateCanonical = "abc"

	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected malformed aggregate digest validation error")
	}
	if !strings.Contains(err.Error(), "aggregate_canonical_sha256 must be 64 hex characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func validEvidenceFixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []replay.NodeSpec{
			{ID: "c1", Mode: replay.NodeModeContainer, Distro: "debian", KernelFamily: "host", Replays: 2, Runner: replay.RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
			{ID: "v1", Mode: replay.NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: replay.RunnerConfig{Kind: "libvirt_command", Replay: []string{"true"}}},
		},
	}
	p := &replay.Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	digest := strings.Repeat("a", 64)
	sourceCommit := strings.Repeat("f", 40)
	sourceTag := "v0.0.0-dev"
	e := &replay.EvidenceBundle{
		SchemaVersion:      replay.EvidenceSchemaVersion,
		BundleSHA256:       digest,
		ControlBinarySHA:   digest,
		MatrixSHA256:       digest,
		ProfileSHA256:      digest,
		SourceGitCommit:    sourceCommit,
		SourceGitTag:       sourceTag,
		ProfileName:        "max",
		Architecture:       "x86_64",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: digest,
		AggregateVerify:    digest,
		AggregateClass:     digest,
		AggregateExitCode:  digest,
		NodeReplays: []replay.NodeRunEvidence{
			mkRun("c1", "container", "debian", "host", 1, digest),
			mkRun("c1", "container", "debian", "host", 2, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 1, digest),
			mkRun("v1", "vm", "ubuntu", "ga", 2, digest),
		},
	}
	opts := replay.EvidenceValidationOptions{
		ExpectedBundleSHA256:        digest,
		ExpectedControlBinarySHA256: digest,
		ExpectedMatrixSHA256:        digest,
		ExpectedProfileSHA256:       digest,
		ExpectedArchitecture:        "x86_64",
		ExpectedSourceGitCommit:     sourceCommit,
		ExpectedSourceGitTag:        sourceTag,
	}
	return m, p, e, opts
}

func cloneEvidence(in *replay.EvidenceBundle) *replay.EvidenceBundle {
	out := *in
	out.RequiredSuites = append([]string(nil), in.RequiredSuites...)
	out.NodeReplays = append([]replay.NodeRunEvidence(nil), in.NodeReplays...)
	return &out
}

func mkRun(nodeID, mode, distro, kernel string, replayIndex int, digest string) replay.NodeRunEvidence {
	return replay.NodeRunEvidence{
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

// validEvidenceV2Fixture returns a valid evidence.v2 fixture with infra-manifest binding.
func validEvidenceV2Fixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	m, p, e, opts := validEvidenceFixture()
	infraDigest := strings.Repeat("c", 64)
	infraCommit := strings.Repeat("d", 40)
	e.SchemaVersion = replay.EvidenceSchemaVersionV2
	e.InfraManifestSHA256 = infraDigest
	e.InfraRepoURL = "https://github.com/example/json-canon-conformance-infra"
	e.InfraRepoCommit = infraCommit
	e.NodeReplays[0].DiscoveredCPU = testCPUIntel
	e.NodeReplays[0].DiscoveredKernel = testKernel610
	e.NodeReplays[0].ImageDigest = "debian@sha256:" + strings.Repeat("1", 64)
	e.NodeReplays[1].DiscoveredCPU = testCPUIntel
	e.NodeReplays[1].DiscoveredKernel = testKernel610
	e.NodeReplays[1].ImageDigest = "debian@sha256:" + strings.Repeat("1", 64)
	e.NodeReplays[2].DiscoveredCPU = "Neoverse"
	e.NodeReplays[2].DiscoveredKernel = testKernel610
	e.NodeReplays[3].DiscoveredCPU = "Neoverse"
	e.NodeReplays[3].DiscoveredKernel = testKernel610
	opts.ExpectedInfraManifestSHA256 = infraDigest
	opts.ExpectedInfraRepoURL = e.InfraRepoURL
	opts.ExpectedInfraRepoCommit = infraCommit
	return m, p, e, opts
}

func validEvidenceV3Fixture() (*replay.Matrix, *replay.Profile, *replay.EvidenceBundle, replay.EvidenceValidationOptions) {
	m, p, e, opts := validEvidenceFixture()
	m.Nodes = []replay.NodeSpec{
		{ID: "v1", Mode: replay.NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: replay.RunnerConfig{Kind: "libvirt_command", Replay: []string{"true"}}},
	}
	infraDigest := strings.Repeat("c", 64)
	infraCommit := strings.Repeat("d", 40)
	manifest := validInfraManifestV2Fixture()
	manifest.InfraRepoCommit = infraCommit
	manifest.Hosts = []replay.InfraManifestHost{
		manifest.Hosts[0],
	}
	manifest.Hosts[0].NodeIDs = []string{"v1"}
	manifest.Hosts[0].Architecture = "x86_64"
	manifest.Hosts[0].ImageID = "ami-0abc1234"
	manifest.Hosts[0].InstanceID = "i-0123456789abcdef0"
	manifest.Hosts[0].OSID = "ubuntu"
	manifest.Hosts[0].OSVersionID = "22.04"
	manifest.Hosts[0].CPU = "Neoverse"
	manifest.Hosts[0].Kernel = testKernel610

	e.SchemaVersion = replay.EvidenceSchemaVersionV3
	e.InfraManifestSHA256 = infraDigest
	e.InfraRepoURL = manifest.InfraRepoURL
	e.InfraRepoCommit = infraCommit
	e.NodeReplays = []replay.NodeRunEvidence{
		{
			NodeID:               "v1",
			Mode:                 "vm",
			Distro:               "ubuntu",
			KernelFamily:         "ga",
			ReplayIndex:          1,
			SessionID:            "sess-v3-1",
			StartedAtUTC:         "2026-01-01T00:00:00Z",
			CompletedAtUTC:       "2026-01-01T00:00:01Z",
			CaseCount:            10,
			Passed:               true,
			CanonicalSHA256:      strings.Repeat("a", 64),
			VerifySHA256:         strings.Repeat("a", 64),
			FailureClassSHA256:   strings.Repeat("a", 64),
			ExitCodeSHA256:       strings.Repeat("a", 64),
			MeasuredArchitecture: "x86_64",
			MeasuredOSID:         "ubuntu",
			MeasuredOSVersionID:  "22.04",
			MeasuredKernel:       testKernel610,
			MeasuredCPU:          "Neoverse",
			AWSInstanceID:        "i-0123456789abcdef0",
			AWSImageID:           "ami-0abc1234",
		},
		{
			NodeID:               "v1",
			Mode:                 "vm",
			Distro:               "ubuntu",
			KernelFamily:         "ga",
			ReplayIndex:          2,
			SessionID:            "sess-v3-2",
			StartedAtUTC:         "2026-01-01T00:00:02Z",
			CompletedAtUTC:       "2026-01-01T00:00:03Z",
			CaseCount:            10,
			Passed:               true,
			CanonicalSHA256:      strings.Repeat("a", 64),
			VerifySHA256:         strings.Repeat("a", 64),
			FailureClassSHA256:   strings.Repeat("a", 64),
			ExitCodeSHA256:       strings.Repeat("a", 64),
			MeasuredArchitecture: "x86_64",
			MeasuredOSID:         "ubuntu",
			MeasuredOSVersionID:  "22.04",
			MeasuredKernel:       testKernel610,
			MeasuredCPU:          "Neoverse",
			AWSInstanceID:        "i-0123456789abcdef0",
			AWSImageID:           "ami-0abc1234",
		},
	}
	e.AggregateCanonical = strings.Repeat("a", 64)
	e.AggregateVerify = strings.Repeat("a", 64)
	e.AggregateClass = strings.Repeat("a", 64)
	e.AggregateExitCode = strings.Repeat("a", 64)

	opts.ExpectedInfraManifestSHA256 = infraDigest
	opts.ExpectedInfraRepoURL = manifest.InfraRepoURL
	opts.ExpectedInfraRepoCommit = infraCommit
	opts.ExpectedInfraManifest = manifest
	return m, p, e, opts
}

func TestValidateEvidenceBundleV1StillValid(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("v1 evidence must still be valid: %v", err)
	}
}

func TestValidateEvidenceBundleV2Parity(t *testing.T) {
	m, p, e, opts := validEvidenceV2Fixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate v2 evidence: %v", err)
	}
}

func TestValidateEvidenceBundleV3Parity(t *testing.T) {
	m, p, e, opts := validEvidenceV3Fixture()
	if err := replay.ValidateEvidenceBundle(e, m, p, opts); err != nil {
		t.Fatalf("validate v3 evidence: %v", err)
	}
}

func TestValidateEvidenceBundleV2RejectsEmptyInfraFields(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(*replay.EvidenceBundle)
		want   string
	}{
		{
			name: "missing infra_manifest_sha256",
			tamper: func(e *replay.EvidenceBundle) {
				e.InfraManifestSHA256 = ""
			},
			want: "infra_manifest_sha256",
		},
		{
			name: "missing infra_repo_url",
			tamper: func(e *replay.EvidenceBundle) {
				e.InfraRepoURL = ""
			},
			want: "infra_repo_url",
		},
		{
			name: "missing infra_repo_commit",
			tamper: func(e *replay.EvidenceBundle) {
				e.InfraRepoCommit = ""
			},
			want: "infra_repo_commit",
		},
	}
	m, p, base, opts := validEvidenceV2Fixture()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := cloneEvidence(base)
			tc.tamper(e)
			err := replay.ValidateEvidenceBundle(e, m, p, opts)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateEvidenceBundleV2InfraManifestMismatch(t *testing.T) {
	m, p, e, opts := validEvidenceV2Fixture()
	opts.ExpectedInfraManifestSHA256 = strings.Repeat("e", 64)
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected infra_manifest_sha256 mismatch error")
	}
	if !strings.Contains(err.Error(), "infra_manifest_sha256 mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleDowngradePrevented(t *testing.T) {
	// A v1-schema bundle that accidentally includes v2 infra fields must be rejected.
	m, p, e, opts := validEvidenceFixture()
	e.InfraManifestSHA256 = strings.Repeat("a", 64)
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected error: v1 schema must not carry v2 infra fields")
	}
	if !strings.Contains(err.Error(), "v2/v3 infra fields") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateEvidenceBundleRejectsNodeLevelV2FieldsInV1(t *testing.T) {
	m, p, e, opts := validEvidenceFixture()
	e.NodeReplays[0].DiscoveredCPU = testCPUIntel
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected error for v1 node-level v2 fields")
	}
	if !strings.Contains(err.Error(), "v2/v3-only node fields") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleInfraSubstrateBindingRequiresV2(t *testing.T) {
	m, _, e, opts := validEvidenceFixture()
	// profile with infra-substrate-binding suite
	p := &replay.Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability", "infra-substrate-binding"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	// evidence is v1 and has required_suites matching the profile
	e.RequiredSuites = []string{"canonical-byte-stability", "infra-substrate-binding"}
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected error: v1 evidence cannot satisfy infra-substrate-binding profile")
	}
	if !strings.Contains(err.Error(), "infra-substrate-binding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleInfraSubstrateBindingRequiresDiscoveredFields(t *testing.T) {
	m, _, e, opts := validEvidenceV2Fixture()
	p := &replay.Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability", "infra-substrate-binding"},
		MinColdReplays:   2,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	e.RequiredSuites = []string{"canonical-byte-stability", "infra-substrate-binding"}
	e.NodeReplays[0].DiscoveredCPU = ""
	e.NodeReplays[0].DiscoveredKernel = ""
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected discovered field validation error")
	}
	if !strings.Contains(err.Error(), "discovered_cpu") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleRejectsImageDigestOnVMLane(t *testing.T) {
	m, p, e, opts := validEvidenceV2Fixture()
	e.NodeReplays[2].ImageDigest = "debian@sha256:" + strings.Repeat("2", 64)
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected vm image_digest validation error")
	}
	if !strings.Contains(err.Error(), "image_digest is only allowed for container lanes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleV2InfraRepoBindingMismatch(t *testing.T) {
	m, p, e, opts := validEvidenceV2Fixture()
	opts.ExpectedInfraRepoURL = "https://github.com/example/other"
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected infra_repo_url mismatch error")
	}
	if !strings.Contains(err.Error(), "infra_repo_url mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEvidenceBundleV3ManifestMismatch(t *testing.T) {
	m, p, e, opts := validEvidenceV3Fixture()
	e.NodeReplays[0].AWSInstanceID = "i-wrong"
	err := replay.ValidateEvidenceBundle(e, m, p, opts)
	if err == nil {
		t.Fatal("expected manifest-bound v3 mismatch")
	}
	if !strings.Contains(err.Error(), "aws_instance_id mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadEvidenceRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	dir := t.TempDir()
	unknownPath := filepath.Join(dir, "unknown.json")
	unknownDoc := `{"schema_version":"evidence.v1","bundle_sha256":"` + strings.Repeat("a", 64) + `","control_binary_sha256":"` + strings.Repeat("a", 64) + `","matrix_sha256":"` + strings.Repeat("a", 64) + `","profile_sha256":"` + strings.Repeat("a", 64) + `","source_git_commit":"` + strings.Repeat("b", 40) + `","source_git_tag":"v0.0.0","generated_at_utc":"2026-01-01T00:00:00Z","orchestrator":"jcs-offline-replay","profile_name":"max","architecture":"x86_64","required_suites":["canonical-byte-stability"],"hard_release_gate":true,"node_replays":[{"node_id":"c1","mode":"container","distro":"debian","kernel_family":"host","replay_index":1,"session_id":"s","started_at_utc":"2026-01-01T00:00:00Z","completed_at_utc":"2026-01-01T00:00:01Z","case_count":1,"passed":true,"canonical_sha256":"` + strings.Repeat("a", 64) + `","verify_sha256":"` + strings.Repeat("a", 64) + `","failure_class_sha256":"` + strings.Repeat("a", 64) + `","exit_code_sha256":"` + strings.Repeat("a", 64) + `"}],"aggregate_canonical_sha256":"` + strings.Repeat("a", 64) + `","aggregate_verify_sha256":"` + strings.Repeat("a", 64) + `","aggregate_failure_class_sha256":"` + strings.Repeat("a", 64) + `","aggregate_exit_code_sha256":"` + strings.Repeat("a", 64) + `","unknown":true}`
	if err := os.WriteFile(unknownPath, []byte(unknownDoc), 0o600); err != nil {
		t.Fatalf("write unknown fixture: %v", err)
	}
	if _, err := replay.LoadEvidence(unknownPath); err == nil {
		t.Fatal("expected unknown field error")
	}

	trailingPath := filepath.Join(dir, "trailing.json")
	if err := os.WriteFile(trailingPath, []byte(unknownDoc[:len(unknownDoc)-17]+`}`+`{"trailing":true}`), 0o600); err != nil {
		t.Fatalf("write trailing fixture: %v", err)
	}
	if _, err := replay.LoadEvidence(trailingPath); err == nil {
		t.Fatal("expected trailing json error")
	}
}
