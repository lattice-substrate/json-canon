package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// NodeAdapter executes replay operations for one node lane.
type NodeAdapter interface {
	Prepare(ctx context.Context, node NodeSpec, bundlePath string, replayIndex int) error
	RunReplay(ctx context.Context, node NodeSpec, bundlePath string, evidencePath string, replayIndex int) error
	Cleanup(ctx context.Context, node NodeSpec, replayIndex int) error
}

// AdapterFactory selects the correct adapter for each node mode.
type AdapterFactory func(node NodeSpec) (NodeAdapter, error)

// RunOptions configures matrix orchestration.
type RunOptions struct {
	BundlePath          string
	BundleSHA256        string
	ControlBinarySHA256 string
	MatrixSHA256        string
	ProfileSHA256       string
	Orchestrator        string
	GlobalEnv           map[string]string
	Now                 func() time.Time
}

func RunMatrix(ctx context.Context, matrix *Matrix, profile *Profile, factory AdapterFactory, opts RunOptions) (*EvidenceBundle, error) {
	if matrix == nil || profile == nil {
		return nil, fmt.Errorf("matrix and profile are required")
	}
	if err := ValidateMatrix(matrix); err != nil {
		return nil, err
	}
	if err := ValidateProfile(profile); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("adapter factory is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if opts.Orchestrator == "" {
		opts.Orchestrator = "jcs-offline-replay"
	}

	requiredNodes, err := requiredNodeIDs(matrix, profile)
	if err != nil {
		return nil, err
	}
	nodeIndex := make(map[string]NodeSpec, len(matrix.Nodes))
	for _, n := range matrix.Nodes {
		nodeIndex[n.ID] = n
	}

	bundle := &EvidenceBundle{
		SchemaVersion:    EvidenceSchemaVersion,
		BundleSHA256:     opts.BundleSHA256,
		ControlBinarySHA: opts.ControlBinarySHA256,
		MatrixSHA256:     opts.MatrixSHA256,
		ProfileSHA256:    opts.ProfileSHA256,
		GeneratedAtUTC:   now().UTC().Format(time.RFC3339Nano),
		Orchestrator:     opts.Orchestrator,
		ProfileName:      profile.Name,
		Architecture:     matrix.Architecture,
		RequiredSuites:   append([]string(nil), profile.RequiredSuites...),
		HardReleaseGate:  profile.HardReleaseGate,
	}

	tmpRoot, err := os.MkdirTemp("", "jcs-offline-replay-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	for _, nodeID := range requiredNodes {
		node := nodeIndex[nodeID]
		if len(opts.GlobalEnv) != 0 {
			merged := make(map[string]string, len(node.Runner.Env)+len(opts.GlobalEnv))
			for k, v := range node.Runner.Env {
				merged[k] = v
			}
			for k, v := range opts.GlobalEnv {
				merged[k] = v
			}
			node.Runner.Env = merged
		}
		adapter, err := factory(node)
		if err != nil {
			return nil, fmt.Errorf("node %s adapter: %w", node.ID, err)
		}
		for replayIdx := 1; replayIdx <= requiredReplayCount(node, profile); replayIdx++ {
			if err := adapter.Prepare(ctx, node, opts.BundlePath, replayIdx); err != nil {
				return nil, fmt.Errorf("node %s replay %d prepare: %w", node.ID, replayIdx, err)
			}

			evidencePath := filepath.Join(tmpRoot, fmt.Sprintf("%s-replay-%03d.json", node.ID, replayIdx))
			runErr := adapter.RunReplay(ctx, node, opts.BundlePath, evidencePath, replayIdx)
			cleanupErr := adapter.Cleanup(ctx, node, replayIdx)
			if runErr != nil {
				return nil, fmt.Errorf("node %s replay %d run: %w", node.ID, replayIdx, runErr)
			}
			if cleanupErr != nil {
				return nil, fmt.Errorf("node %s replay %d cleanup: %w", node.ID, replayIdx, cleanupErr)
			}

			runEvidence, err := LoadNodeRunEvidence(evidencePath)
			if err != nil {
				return nil, fmt.Errorf("node %s replay %d load evidence: %w", node.ID, replayIdx, err)
			}
			bundle.NodeReplays = append(bundle.NodeReplays, *runEvidence)
		}
	}
	if len(bundle.NodeReplays) == 0 {
		return nil, fmt.Errorf("matrix execution produced no replay evidence")
	}

	sort.Slice(bundle.NodeReplays, func(i, j int) bool {
		if bundle.NodeReplays[i].NodeID == bundle.NodeReplays[j].NodeID {
			return bundle.NodeReplays[i].ReplayIndex < bundle.NodeReplays[j].ReplayIndex
		}
		return bundle.NodeReplays[i].NodeID < bundle.NodeReplays[j].NodeID
	})

	base := bundle.NodeReplays[0]
	bundle.AggregateCanonical = base.CanonicalSHA256
	bundle.AggregateVerify = base.VerifySHA256
	bundle.AggregateClass = base.FailureClassSHA256
	bundle.AggregateExitCode = base.ExitCodeSHA256

	if err := ValidateEvidenceBundle(bundle, matrix, profile); err != nil {
		return nil, err
	}
	return bundle, nil
}

func LoadNodeRunEvidence(path string) (*NodeRunEvidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read node evidence: %w", err)
	}
	var run NodeRunEvidence
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("decode node evidence: %w", err)
	}
	return &run, nil
}
