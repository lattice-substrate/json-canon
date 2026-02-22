package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// Matrix defines the offline replay execution lanes.
type Matrix struct {
	Version      string     `yaml:"version" json:"version"`
	Architecture string     `yaml:"architecture" json:"architecture"`
	Nodes        []NodeSpec `yaml:"nodes" json:"nodes"`
}

// NodeSpec is one distro/kernel lane.
type NodeSpec struct {
	ID           string       `yaml:"id" json:"id"`
	Mode         NodeMode     `yaml:"mode" json:"mode"`
	Distro       string       `yaml:"distro" json:"distro"`
	KernelFamily string       `yaml:"kernel_family" json:"kernel_family"`
	Replays      int          `yaml:"replays" json:"replays"`
	Runner       RunnerConfig `yaml:"runner" json:"runner"`
}

// NodeMode represents the node execution mode.
type NodeMode string

const (
	// NodeModeContainer executes a node via container runtime commands.
	NodeModeContainer NodeMode = "container"
	// NodeModeVM executes a node via VM/libvirt runtime commands.
	NodeModeVM NodeMode = "vm"
)

const architectureARM64 = "arm64"

// RunnerConfig is an execution command contract for a node lane.
type RunnerConfig struct {
	Kind    string            `yaml:"kind" json:"kind"`
	Prepare []string          `yaml:"prepare" json:"prepare"`
	Replay  []string          `yaml:"replay" json:"replay"`
	Cleanup []string          `yaml:"cleanup" json:"cleanup"`
	Env     map[string]string `yaml:"env" json:"env"`
}

// Profile defines required suites and gate policy.
type Profile struct {
	Version          string   `yaml:"version" json:"version"`
	Name             string   `yaml:"name" json:"name"`
	RequiredNodes    []string `yaml:"required_nodes" json:"required_nodes"`
	RequiredSuites   []string `yaml:"required_suites" json:"required_suites"`
	MinColdReplays   int      `yaml:"min_cold_replays" json:"min_cold_replays"`
	HardReleaseGate  bool     `yaml:"hard_release_gate" json:"hard_release_gate"`
	EvidenceRequired bool     `yaml:"evidence_required" json:"evidence_required"`
}

// LoadMatrix reads, decodes, and validates a replay matrix document.
//
//nolint:gosec // REQ:OFFLINE-MATRIX-001 matrix path is explicit operator input for release-gate validation.
func LoadMatrix(path string) (*Matrix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read matrix: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Matrix
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode matrix json: %w", err)
	}
	if err := ensureSingleJSONDocument(dec); err != nil {
		return nil, fmt.Errorf("decode matrix json: %w", err)
	}
	if err := ValidateMatrix(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadProfile reads, decodes, and validates a replay profile document.
//
//nolint:gosec // REQ:OFFLINE-COLD-001 profile path is explicit operator input for release-gate validation.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Profile
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode profile json: %w", err)
	}
	if err := ensureSingleJSONDocument(dec); err != nil {
		return nil, fmt.Errorf("decode profile json: %w", err)
	}
	if err := ValidateProfile(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func ensureSingleJSONDocument(dec *json.Decoder) error {
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing json content")
		}
		return fmt.Errorf("decode trailing json token: %w", err)
	}
	return nil
}

// ValidateMatrix validates replay matrix semantics and release-policy requirements.
//
//nolint:gocyclo,cyclop,gocognit // REQ:OFFLINE-MATRIX-001 matrix policy validation is explicit to keep release-gate failures actionable.
func ValidateMatrix(m *Matrix) error {
	if m == nil {
		return fmt.Errorf("matrix is nil")
	}
	if m.Version == "" {
		return fmt.Errorf("matrix version is required")
	}
	if m.Architecture == "" {
		return fmt.Errorf("matrix architecture is required")
	}
	if len(m.Nodes) == 0 {
		return fmt.Errorf("matrix must include at least one node")
	}

	seen := make(map[string]struct{}, len(m.Nodes))
	hasContainer := false
	hasVM := false
	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.ID == "" {
			return fmt.Errorf("node[%d] id is required", i)
		}
		if _, ok := seen[n.ID]; ok {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		seen[n.ID] = struct{}{}

		switch n.Mode {
		case NodeModeContainer:
			hasContainer = true
		case NodeModeVM:
			hasVM = true
		default:
			return fmt.Errorf("node %s: invalid mode %q", n.ID, n.Mode)
		}
		if n.Distro == "" {
			return fmt.Errorf("node %s: distro is required", n.ID)
		}
		if n.KernelFamily == "" {
			return fmt.Errorf("node %s: kernel_family is required", n.ID)
		}
		if n.Replays < 0 {
			return fmt.Errorf("node %s: replays cannot be negative", n.ID)
		}
		if len(n.Runner.Replay) == 0 {
			return fmt.Errorf("node %s: runner.replay command is required", n.ID)
		}
		if n.Runner.Kind == "" {
			return fmt.Errorf("node %s: runner.kind is required", n.ID)
		}
	}
	if !hasContainer {
		return fmt.Errorf("matrix must include at least one container node")
	}
	if !hasVM {
		return fmt.Errorf("matrix must include at least one vm node")
	}
	return nil
}

// ValidateProfile validates profile semantics required by replay gates.
func ValidateProfile(p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if p.Version == "" {
		return fmt.Errorf("profile version is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if len(p.RequiredSuites) == 0 {
		return fmt.Errorf("profile required_suites cannot be empty")
	}
	if p.MinColdReplays < 1 {
		return fmt.Errorf("profile min_cold_replays must be >= 1")
	}
	if !p.EvidenceRequired {
		return fmt.Errorf("profile evidence_required must be true")
	}
	return nil
}

// ValidateReleaseArchitecture enforces the release architecture scope policy.
func ValidateReleaseArchitecture(m *Matrix) error {
	if m == nil {
		return fmt.Errorf("matrix is nil")
	}
	switch m.Architecture {
	case "x86_64", architectureARM64:
		return nil
	default:
		return fmt.Errorf("release architecture must be one of x86_64, %s, got %q", architectureARM64, m.Architecture)
	}
}

// ValidatePhaseOneArchitecture is retained as a compatibility wrapper.
func ValidatePhaseOneArchitecture(m *Matrix) error {
	return ValidateReleaseArchitecture(m)
}

func requiredNodeIDs(m *Matrix, p *Profile) ([]string, error) {
	if len(p.RequiredNodes) == 0 {
		ids := make([]string, 0, len(m.Nodes))
		for _, n := range m.Nodes {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		return ids, nil
	}
	nodeIndex := make(map[string]struct{}, len(m.Nodes))
	for _, n := range m.Nodes {
		nodeIndex[n.ID] = struct{}{}
	}
	ids := make([]string, 0, len(p.RequiredNodes))
	seen := make(map[string]struct{}, len(p.RequiredNodes))
	for _, id := range p.RequiredNodes {
		if _, ok := nodeIndex[id]; !ok {
			return nil, fmt.Errorf("required node %q not present in matrix", id)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func requiredReplayCount(node NodeSpec, p *Profile) int {
	count := p.MinColdReplays
	if node.Replays > count {
		count = node.Replays
	}
	return count
}
