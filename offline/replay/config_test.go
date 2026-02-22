package replay

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMatrix_OFFLINE_MATRIX_001(t *testing.T) {
	m, err := LoadMatrix(filepath.Join("..", "matrix.yaml"))
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	if m.Architecture != "x86_64" {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("release architecture validation failed: %v", err)
	}
	if len(m.Nodes) < 10 {
		t.Fatalf("expected maximal node coverage, got %d", len(m.Nodes))
	}
}

func TestLoadArm64Matrix_OFFLINE_ARCH_001(t *testing.T) {
	m, err := LoadMatrix(filepath.Join("..", "matrix.arm64.yaml"))
	if err != nil {
		t.Fatalf("load arm64 matrix: %v", err)
	}
	if m.Architecture != "arm64" {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("arm64 architecture validation failed: %v", err)
	}
}

func TestLoadProfile_OFFLINE_COLD_001(t *testing.T) {
	p, err := LoadProfile(filepath.Join("..", "profiles", "maximal.yaml"))
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if !p.HardReleaseGate {
		t.Fatal("expected hard_release_gate=true")
	}
	if p.MinColdReplays < 5 {
		t.Fatalf("expected min_cold_replays>=5, got %d", p.MinColdReplays)
	}
}

func TestValidateMatrixRequiresContainerAndVM(t *testing.T) {
	m := &Matrix{
		Version:      "v1",
		Architecture: "x86_64",
		Nodes: []NodeSpec{
			{ID: "a", Mode: NodeModeContainer, Distro: "debian", KernelFamily: "host", Runner: RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
		},
	}
	err := ValidateMatrix(m)
	if err == nil || !strings.Contains(err.Error(), "vm") {
		t.Fatalf("expected vm validation error, got %v", err)
	}
}

func TestValidateReleaseArchitecture_OFFLINE_ARCH_001(t *testing.T) {
	m := &Matrix{Version: "v1", Architecture: "x86_64"}
	if err := ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected architecture validation failure: %v", err)
	}
	m.Architecture = "arm64"
	if err := ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected arm64 architecture validation failure: %v", err)
	}
	m.Architecture = "ppc64"
	if err := ValidateReleaseArchitecture(m); err == nil {
		t.Fatal("expected architecture validation failure")
	}
}
