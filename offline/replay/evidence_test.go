package replay

import (
	"testing"
)

func TestValidateEvidenceBundleParity(t *testing.T) {
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
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e := &EvidenceBundle{
		SchemaVersion:      EvidenceSchemaVersion,
		ProfileName:        "max",
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
	if err := ValidateEvidenceBundle(e, m, p); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
}

func TestValidateEvidenceBundleDetectsDrift(t *testing.T) {
	m := &Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []NodeSpec{
			{ID: "c1", Mode: NodeModeContainer, Distro: "debian", KernelFamily: "host", Replays: 1, Runner: RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
			{ID: "v1", Mode: NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 1, Runner: RunnerConfig{Kind: "libvirt_command", Replay: []string{"true"}}},
		},
	}
	p := &Profile{
		Version:          "v1",
		Name:             "max",
		RequiredSuites:   []string{"canonical-byte-stability"},
		MinColdReplays:   1,
		HardReleaseGate:  true,
		EvidenceRequired: true,
	}
	d1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	d2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	e := &EvidenceBundle{
		SchemaVersion:      EvidenceSchemaVersion,
		ProfileName:        "max",
		HardReleaseGate:    true,
		RequiredSuites:     []string{"canonical-byte-stability"},
		AggregateCanonical: d1,
		AggregateVerify:    d1,
		AggregateClass:     d1,
		AggregateExitCode:  d1,
		NodeReplays: []NodeRunEvidence{
			mkRun("c1", "container", "debian", "host", 1, d1),
			mkRun("v1", "vm", "ubuntu", "ga", 1, d2),
		},
	}
	if err := ValidateEvidenceBundle(e, m, p); err == nil {
		t.Fatal("expected drift validation error")
	}
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
