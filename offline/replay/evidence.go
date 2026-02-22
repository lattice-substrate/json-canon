package replay

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const EvidenceSchemaVersion = "evidence.v1"

// EvidenceBundle is the machine-consumed replay output artifact.
type EvidenceBundle struct {
	SchemaVersion      string            `json:"schema_version"`
	BundleSHA256       string            `json:"bundle_sha256"`
	ControlBinarySHA   string            `json:"control_binary_sha256"`
	MatrixSHA256       string            `json:"matrix_sha256"`
	ProfileSHA256      string            `json:"profile_sha256"`
	GeneratedAtUTC     string            `json:"generated_at_utc"`
	Orchestrator       string            `json:"orchestrator"`
	ProfileName        string            `json:"profile_name"`
	Architecture       string            `json:"architecture"`
	RequiredSuites     []string          `json:"required_suites"`
	HardReleaseGate    bool              `json:"hard_release_gate"`
	NodeReplays        []NodeRunEvidence `json:"node_replays"`
	AggregateCanonical string            `json:"aggregate_canonical_sha256"`
	AggregateVerify    string            `json:"aggregate_verify_sha256"`
	AggregateClass     string            `json:"aggregate_failure_class_sha256"`
	AggregateExitCode  string            `json:"aggregate_exit_code_sha256"`
}

// NodeRunEvidence is one replay execution on one node.
type NodeRunEvidence struct {
	NodeID             string `json:"node_id"`
	Mode               string `json:"mode"`
	Distro             string `json:"distro"`
	KernelFamily       string `json:"kernel_family"`
	ReplayIndex        int    `json:"replay_index"`
	SessionID          string `json:"session_id"`
	StartedAtUTC       string `json:"started_at_utc"`
	CompletedAtUTC     string `json:"completed_at_utc"`
	CaseCount          int    `json:"case_count"`
	Passed             bool   `json:"passed"`
	CanonicalSHA256    string `json:"canonical_sha256"`
	VerifySHA256       string `json:"verify_sha256"`
	FailureClassSHA256 string `json:"failure_class_sha256"`
	ExitCodeSHA256     string `json:"exit_code_sha256"`
}

// EvidenceValidationOptions binds evidence metadata to expected immutable inputs.
type EvidenceValidationOptions struct {
	ExpectedBundleSHA256        string
	ExpectedControlBinarySHA256 string
	ExpectedMatrixSHA256        string
	ExpectedProfileSHA256       string
	ExpectedArchitecture        string
}

func WriteEvidence(path string, e *EvidenceBundle) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write evidence file: %w", err)
	}
	return nil
}

func LoadEvidence(path string) (*EvidenceBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}
	var e EvidenceBundle
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("decode evidence: %w", err)
	}
	return &e, nil
}

func ValidateEvidenceBundle(e *EvidenceBundle, m *Matrix, p *Profile, opts EvidenceValidationOptions) error {
	if e == nil {
		return fmt.Errorf("evidence bundle is nil")
	}
	if m == nil || p == nil {
		return fmt.Errorf("matrix and profile are required")
	}
	if e.SchemaVersion != EvidenceSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", e.SchemaVersion)
	}
	if e.ProfileName != p.Name {
		return fmt.Errorf("profile mismatch: evidence=%q profile=%q", e.ProfileName, p.Name)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "bundle_sha256", value: e.BundleSHA256},
		{name: "control_binary_sha256", value: e.ControlBinarySHA},
		{name: "matrix_sha256", value: e.MatrixSHA256},
		{name: "profile_sha256", value: e.ProfileSHA256},
	} {
		if err := validateSHA256Token(field.name, field.value); err != nil {
			return err
		}
	}
	expectedArch := m.Architecture
	if strings.TrimSpace(opts.ExpectedArchitecture) != "" {
		expectedArch = strings.TrimSpace(opts.ExpectedArchitecture)
	}
	if e.Architecture != expectedArch {
		return fmt.Errorf("architecture mismatch: evidence=%q expected=%q", e.Architecture, expectedArch)
	}
	if opts.ExpectedBundleSHA256 != "" && e.BundleSHA256 != opts.ExpectedBundleSHA256 {
		return fmt.Errorf("bundle_sha256 mismatch: evidence=%q expected=%q", e.BundleSHA256, opts.ExpectedBundleSHA256)
	}
	if opts.ExpectedControlBinarySHA256 != "" && e.ControlBinarySHA != opts.ExpectedControlBinarySHA256 {
		return fmt.Errorf("control_binary_sha256 mismatch: evidence=%q expected=%q", e.ControlBinarySHA, opts.ExpectedControlBinarySHA256)
	}
	if opts.ExpectedMatrixSHA256 != "" && e.MatrixSHA256 != opts.ExpectedMatrixSHA256 {
		return fmt.Errorf("matrix_sha256 mismatch: evidence=%q expected=%q", e.MatrixSHA256, opts.ExpectedMatrixSHA256)
	}
	if opts.ExpectedProfileSHA256 != "" && e.ProfileSHA256 != opts.ExpectedProfileSHA256 {
		return fmt.Errorf("profile_sha256 mismatch: evidence=%q expected=%q", e.ProfileSHA256, opts.ExpectedProfileSHA256)
	}
	if !e.HardReleaseGate {
		return fmt.Errorf("evidence must record hard_release_gate=true")
	}
	if len(e.NodeReplays) == 0 {
		return fmt.Errorf("evidence must include node_replays")
	}

	requiredNodes, err := requiredNodeIDs(m, p)
	if err != nil {
		return err
	}
	matrixByID := make(map[string]NodeSpec, len(m.Nodes))
	for _, node := range m.Nodes {
		matrixByID[node.ID] = node
	}

	byNode := make(map[string][]NodeRunEvidence)
	for _, r := range e.NodeReplays {
		if r.NodeID == "" {
			return fmt.Errorf("node replay has empty node_id")
		}
		node, ok := matrixByID[r.NodeID]
		if !ok {
			return fmt.Errorf("node replay references unknown node_id %q", r.NodeID)
		}
		if r.Mode != string(node.Mode) {
			return fmt.Errorf("node %s mode mismatch: got=%q want=%q", r.NodeID, r.Mode, node.Mode)
		}
		if r.Distro != node.Distro {
			return fmt.Errorf("node %s distro mismatch: got=%q want=%q", r.NodeID, r.Distro, node.Distro)
		}
		if r.KernelFamily != node.KernelFamily {
			return fmt.Errorf("node %s kernel_family mismatch: got=%q want=%q", r.NodeID, r.KernelFamily, node.KernelFamily)
		}
		if r.ReplayIndex < 1 {
			return fmt.Errorf("node %s replay_index must be >=1", r.NodeID)
		}
		if r.CaseCount < 1 {
			return fmt.Errorf("node %s replay %d must have case_count >=1", r.NodeID, r.ReplayIndex)
		}
		if !r.Passed {
			return fmt.Errorf("node %s replay %d is marked failed", r.NodeID, r.ReplayIndex)
		}
		for _, token := range []struct {
			name  string
			value string
		}{
			{"session_id", r.SessionID},
			{"started_at_utc", r.StartedAtUTC},
			{"completed_at_utc", r.CompletedAtUTC},
			{"canonical_sha256", r.CanonicalSHA256},
			{"verify_sha256", r.VerifySHA256},
			{"failure_class_sha256", r.FailureClassSHA256},
			{"exit_code_sha256", r.ExitCodeSHA256},
		} {
			if strings.TrimSpace(token.value) == "" {
				return fmt.Errorf("node %s replay %d missing %s", r.NodeID, r.ReplayIndex, token.name)
			}
		}
		byNode[r.NodeID] = append(byNode[r.NodeID], r)
	}

	var baseline *NodeRunEvidence
	for _, id := range requiredNodes {
		runs := byNode[id]
		wantReplays := requiredReplayCount(matrixByID[id], p)
		if len(runs) < wantReplays {
			return fmt.Errorf("node %s has %d replays, want at least %d", id, len(runs), wantReplays)
		}
		seenReplay := make(map[int]struct{}, len(runs))
		for _, run := range runs {
			seenReplay[run.ReplayIndex] = struct{}{}
			if baseline == nil {
				r := run
				baseline = &r
				continue
			}
			if run.CanonicalSHA256 != baseline.CanonicalSHA256 {
				return fmt.Errorf("canonical digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.VerifySHA256 != baseline.VerifySHA256 {
				return fmt.Errorf("verify digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.FailureClassSHA256 != baseline.FailureClassSHA256 {
				return fmt.Errorf("failure-class digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
			if run.ExitCodeSHA256 != baseline.ExitCodeSHA256 {
				return fmt.Errorf("exit-code digest drift at node %s replay %d", run.NodeID, run.ReplayIndex)
			}
		}
		for i := 1; i <= wantReplays; i++ {
			if _, ok := seenReplay[i]; !ok {
				return fmt.Errorf("node %s missing replay index %d", id, i)
			}
		}
	}

	if baseline == nil {
		return fmt.Errorf("no baseline replay digest found")
	}
	if e.AggregateCanonical != baseline.CanonicalSHA256 {
		return fmt.Errorf("aggregate canonical digest mismatch")
	}
	if e.AggregateVerify != baseline.VerifySHA256 {
		return fmt.Errorf("aggregate verify digest mismatch")
	}
	if e.AggregateClass != baseline.FailureClassSHA256 {
		return fmt.Errorf("aggregate failure-class digest mismatch")
	}
	if e.AggregateExitCode != baseline.ExitCodeSHA256 {
		return fmt.Errorf("aggregate exit-code digest mismatch")
	}

	suites := append([]string(nil), e.RequiredSuites...)
	sort.Strings(suites)
	wantSuites := append([]string(nil), p.RequiredSuites...)
	sort.Strings(wantSuites)
	if strings.Join(suites, ",") != strings.Join(wantSuites, ",") {
		return fmt.Errorf("required_suites mismatch")
	}

	return nil
}

func validateSHA256Token(name, value string) error {
	token := strings.TrimSpace(value)
	if len(token) != 64 {
		return fmt.Errorf("%s must be 64 hex characters", name)
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("%s must be valid hex: %w", name, err)
	}
	return nil
}
