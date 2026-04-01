package replay_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const (
	archArm64  = "arm64"
	archX86_64 = "x86_64"
)

func TestLoadMatrix_OFFLINE_MATRIX_001(t *testing.T) {
	m, err := replay.LoadMatrix(filepath.Join("..", "matrix.yaml"))
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	if m.Architecture != archX86_64 {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("release architecture validation failed: %v", err)
	}
	if len(m.Nodes) < 10 {
		t.Fatalf("expected maximal node coverage, got %d", len(m.Nodes))
	}
}

func TestLoadArm64Matrix_OFFLINE_ARCH_001(t *testing.T) {
	m, err := replay.LoadMatrix(filepath.Join("..", "matrix.arm64.yaml"))
	if err != nil {
		t.Fatalf("load arm64 matrix: %v", err)
	}
	if m.Architecture != archArm64 {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("arm64 architecture validation failed: %v", err)
	}
}

func TestLoadServerMatrices(t *testing.T) {
	tests := []struct {
		path string
		arch string
	}{
		{path: filepath.Join("..", "matrix.server-x86_64.yaml"), arch: archX86_64},
		{path: filepath.Join("..", "matrix.server-arm64.yaml"), arch: archArm64},
	}
	for _, tc := range tests {
		m, err := replay.LoadMatrix(tc.path)
		if err != nil {
			t.Fatalf("load server matrix %s: %v", tc.path, err)
		}
		if m.Architecture != tc.arch {
			t.Fatalf("unexpected architecture %q for %s", m.Architecture, tc.path)
		}
		if err := replay.ValidateMatrix(m); err != nil {
			t.Fatalf("validate server matrix %s: %v", tc.path, err)
		}
	}
}

func TestLoadProfile_OFFLINE_COLD_001(t *testing.T) {
	p, err := replay.LoadProfile(filepath.Join("..", "profiles", "maximal.yaml"))
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
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: archX86_64,
		Nodes: []replay.NodeSpec{
			{ID: "a", Mode: replay.NodeModeContainer, Distro: "debian", KernelFamily: "host", Runner: replay.RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
		},
	}
	err := replay.ValidateMatrix(m)
	if err == nil || !strings.Contains(err.Error(), "vm") {
		t.Fatalf("expected vm validation error, got %v", err)
	}
}

func TestValidateReleaseArchitecture_OFFLINE_ARCH_001(t *testing.T) {
	m := &replay.Matrix{Version: "v1", Architecture: archX86_64}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected architecture validation failure: %v", err)
	}
	m.Architecture = archArm64
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected arm64 architecture validation failure: %v", err)
	}
	m.Architecture = "ppc64"
	if err := replay.ValidateReleaseArchitecture(m); err == nil {
		t.Fatal("expected architecture validation failure")
	}
}
