package replay

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

type fakeAdapter struct{}

func (fakeAdapter) Prepare(_ context.Context, _ NodeSpec, _ string, _ int) error { return nil }
func (fakeAdapter) Cleanup(_ context.Context, _ NodeSpec, _ int) error           { return nil }
func (fakeAdapter) RunReplay(_ context.Context, node NodeSpec, _ string, evidencePath string, replayIndex int) error {
	d := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	run := NodeRunEvidence{
		NodeID:             node.ID,
		Mode:               string(node.Mode),
		Distro:             node.Distro,
		KernelFamily:       node.KernelFamily,
		ReplayIndex:        replayIndex,
		SessionID:          node.ID + "-session",
		StartedAtUTC:       "2026-01-01T00:00:00Z",
		CompletedAtUTC:     "2026-01-01T00:00:01Z",
		CaseCount:          74,
		Passed:             true,
		CanonicalSHA256:    d,
		VerifySHA256:       d,
		FailureClassSHA256: d,
		ExitCodeSHA256:     d,
	}
	b, err := json.Marshal(run)
	if err != nil {
		return err
	}
	return os.WriteFile(evidencePath, b, 0o600)
}

func TestRunMatrix(t *testing.T) {
	m := &Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []NodeSpec{
			{ID: "c1", Mode: NodeModeContainer, Distro: "debian", KernelFamily: "host", Replays: 2, Runner: RunnerConfig{Kind: "container_command", Replay: []string{"echo", "run"}}},
			{ID: "v1", Mode: NodeModeVM, Distro: "ubuntu", KernelFamily: "ga", Replays: 2, Runner: RunnerConfig{Kind: "libvirt_command", Replay: []string{"echo", "run"}}},
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

	bundle, err := RunMatrix(context.Background(), m, p, func(node NodeSpec) (NodeAdapter, error) {
		_ = node
		return fakeAdapter{}, nil
	}, RunOptions{
		BundlePath:          "bundle.tgz",
		BundleSHA256:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ControlBinarySHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		MatrixSHA256:        "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		ProfileSHA256:       "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Now: func() time.Time {
			return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("run matrix: %v", err)
	}
	if bundle.SchemaVersion != EvidenceSchemaVersion {
		t.Fatalf("unexpected schema: %s", bundle.SchemaVersion)
	}
	if len(bundle.NodeReplays) != 4 {
		t.Fatalf("unexpected replay count: %d", len(bundle.NodeReplays))
	}
}
