package replay_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

const archArm64 = "arm64"

func TestLoadMatrix_OFFLINE_MATRIX_001(t *testing.T) {
	m, err := replay.LoadMatrix(filepath.Join("..", "matrix.yaml"))
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	if m.Architecture != "x86_64" {
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
		Architecture: "x86_64",
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
	m := &replay.Matrix{Version: "v1", Architecture: "x86_64"}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected architecture validation failure: %v", err)
	}
	m.Architecture = archArm64
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("unexpected arm64 architecture validation failure: %v", err)
	}
	for _, winArch := range []string{"windows_amd64", "windows_arm64"} {
		m.Architecture = winArch
		if err := replay.ValidateReleaseArchitecture(m); err != nil {
			t.Fatalf("unexpected %s architecture validation failure: %v", winArch, err)
		}
	}
	m.Architecture = "ppc64"
	if err := replay.ValidateReleaseArchitecture(m); err == nil {
		t.Fatal("expected architecture validation failure")
	}
}

func TestLoadWindowsAMD64Matrix(t *testing.T) {
	m, err := replay.LoadMatrix(filepath.Join("..", "matrix.windows-amd64.yaml"))
	if err != nil {
		t.Fatalf("load windows amd64 matrix: %v", err)
	}
	if m.Architecture != "windows_amd64" {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("windows_amd64 architecture validation failed: %v", err)
	}
	hasDirect := false
	for _, node := range m.Nodes {
		if node.Mode == replay.NodeModeDirect {
			hasDirect = true
		}
	}
	if !hasDirect {
		t.Fatal("windows matrix must have at least one direct node")
	}
}

func TestLoadWindowsARM64Matrix(t *testing.T) {
	m, err := replay.LoadMatrix(filepath.Join("..", "matrix.windows-arm64.yaml"))
	if err != nil {
		t.Fatalf("load windows arm64 matrix: %v", err)
	}
	if m.Architecture != "windows_arm64" {
		t.Fatalf("unexpected architecture %q", m.Architecture)
	}
	if err := replay.ValidateReleaseArchitecture(m); err != nil {
		t.Fatalf("windows_arm64 architecture validation failed: %v", err)
	}
}

func TestLoadWindowsProfiles(t *testing.T) {
	for _, tc := range []struct {
		path string
		name string
	}{
		{filepath.Join("..", "profiles", "maximal.windows-amd64.yaml"), "maximal-offline-windows-amd64"},
		{filepath.Join("..", "profiles", "maximal.windows-arm64.yaml"), "maximal-offline-windows-arm64"},
	} {
		p, err := replay.LoadProfile(tc.path)
		if err != nil {
			t.Fatalf("load profile %s: %v", tc.path, err)
		}
		if p.Name != tc.name {
			t.Fatalf("expected profile name %q, got %q", tc.name, p.Name)
		}
		if !p.HardReleaseGate {
			t.Fatalf("expected hard_release_gate=true for %s", tc.path)
		}
		if p.MinColdReplays < 5 {
			t.Fatalf("expected min_cold_replays>=5 for %s, got %d", tc.path, p.MinColdReplays)
		}
	}
}

func TestValidateWindowsMatrixRequiresDirect(t *testing.T) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "windows_amd64",
		Nodes: []replay.NodeSpec{
			{ID: "a", Mode: replay.NodeModeContainer, Distro: "debian", KernelFamily: "host", Runner: replay.RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
		},
	}
	err := replay.ValidateMatrix(m)
	if err == nil {
		t.Fatal("expected windows matrix with only container nodes to fail validation")
	}
}

func TestValidateWindowsMatrixRejectsContainerWithDirect(t *testing.T) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "windows_amd64",
		Nodes: []replay.NodeSpec{
			{ID: "win1", Mode: replay.NodeModeDirect, Distro: "windows-ltsc2022", KernelFamily: "ntkernel", Runner: replay.RunnerConfig{Kind: "direct_command", Replay: []string{"true"}}},
			{ID: "bad", Mode: replay.NodeModeContainer, Distro: "debian", KernelFamily: "host", Runner: replay.RunnerConfig{Kind: "container_command", Replay: []string{"true"}}},
		},
	}
	err := replay.ValidateMatrix(m)
	if err == nil || !strings.Contains(err.Error(), "must not include container or vm") {
		t.Fatalf("expected windows matrix to reject container nodes, got %v", err)
	}
}

func TestValidateWindowsMatrixRejectsVM(t *testing.T) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "windows_arm64",
		Nodes: []replay.NodeSpec{
			{ID: "win1", Mode: replay.NodeModeDirect, Distro: "windows-ltsc2022", KernelFamily: "ntkernel", Runner: replay.RunnerConfig{Kind: "direct_command", Replay: []string{"true"}}},
			{ID: "bad", Mode: replay.NodeModeVM, Distro: "windows", KernelFamily: "ntkernel", Runner: replay.RunnerConfig{Kind: "vm_command", Replay: []string{"true"}}},
		},
	}
	err := replay.ValidateMatrix(m)
	if err == nil || !strings.Contains(err.Error(), "must not include container or vm") {
		t.Fatalf("expected windows matrix to reject vm nodes, got %v", err)
	}
}

func TestValidateWindowsMatrixAcceptsDirect(t *testing.T) {
	m := &replay.Matrix{
		Version:      "v1",
		Architecture: "windows_amd64",
		Nodes: []replay.NodeSpec{
			{ID: "win1", Mode: replay.NodeModeDirect, Distro: "windows-ltsc2022", KernelFamily: "ntkernel", Runner: replay.RunnerConfig{Kind: "direct_command", Replay: []string{"true"}}},
		},
	}
	if err := replay.ValidateMatrix(m); err != nil {
		t.Fatalf("expected valid windows direct matrix, got %v", err)
	}
}

func TestIsWindowsArchitecture(t *testing.T) {
	for _, arch := range []string{"windows_amd64", "windows_arm64"} {
		if !replay.IsWindowsArchitecture(arch) {
			t.Fatalf("expected %q to be a windows architecture", arch)
		}
	}
	for _, arch := range []string{"x86_64", archArm64, "ppc64", ""} {
		if replay.IsWindowsArchitecture(arch) {
			t.Fatalf("expected %q to NOT be a windows architecture", arch)
		}
	}
}
