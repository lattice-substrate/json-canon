package direct_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/direct"
)

type fakeRunner struct {
	argv []string
	env  map[string]string
}

func (f *fakeRunner) Run(_ context.Context, argv []string, env map[string]string) (string, error) {
	f.argv = append([]string(nil), argv...)
	f.env = map[string]string{}
	for k, v := range env {
		f.env[k] = v
	}
	return "", nil
}

func TestAdapterRunReplaySetsEnv(t *testing.T) {
	fr := &fakeRunner{}
	a := direct.NewAdapter(fr)
	n := replay.NodeSpec{
		ID:           "win1",
		Mode:         replay.NodeModeDirect,
		Distro:       "windows-ltsc2022",
		KernelFamily: "ntkernel",
		Runner: replay.RunnerConfig{
			Kind:   "direct_command",
			Replay: []string{"runner", "arg"},
			Env: map[string]string{
				"X": "1",
			},
		},
	}
	if err := a.RunReplay(context.Background(), n, "/bundle.tgz", "/evidence.json", 3); err != nil {
		t.Fatalf("run replay: %v", err)
	}
	if fr.argv[0] != "runner" {
		t.Fatalf("unexpected argv: %#v", fr.argv)
	}
	if fr.env["JCS_REPLAY_INDEX"] != "3" {
		t.Fatalf("missing replay env: %#v", fr.env)
	}
	if fr.env["JCS_BUNDLE_PATH"] != "/bundle.tgz" || fr.env["JCS_EVIDENCE_PATH"] != "/evidence.json" {
		t.Fatalf("missing bundle/evidence env: %#v", fr.env)
	}
	if fr.env["JCS_NODE_MODE"] != "direct" {
		t.Fatalf("expected direct mode, got %q", fr.env["JCS_NODE_MODE"])
	}
}

func TestAdapterRunReplayGoNative(t *testing.T) {
	fr := &fakeRunner{}
	fakeExtractor := func(bundlePath, destDir string) (string, error) {
		return filepath.Join(destDir, "jcs-offline-worker"), nil
	}
	a := direct.NewAdapterWithExtractor(fr, fakeExtractor)
	n := replay.NodeSpec{
		ID:           "win1",
		Mode:         replay.NodeModeDirect,
		Distro:       "windows-ltsc2022",
		KernelFamily: "ntkernel",
		Runner: replay.RunnerConfig{
			Kind:   "direct_go",
			Replay: []string{"go:builtin"},
			Env: map[string]string{
				"CUSTOM": "val",
			},
		},
	}
	if err := a.RunReplay(context.Background(), n, "/bundle.tgz", "/evidence.json", 2); err != nil {
		t.Fatalf("run replay: %v", err)
	}
	// Verify argv[0] is the extracted worker path.
	if !containsSuffix(fr.argv[0], "jcs-offline-worker") {
		t.Fatalf("expected argv[0] to end with jcs-offline-worker, got %q", fr.argv[0])
	}
	// Verify worker flags are present in argv.
	wantFlags := map[string]string{
		"--bundle":        "/bundle.tgz",
		"--evidence":      "/evidence.json",
		"--node-id":       "win1",
		"--mode":          "direct",
		"--distro":        "windows-ltsc2022",
		"--kernel-family": "ntkernel",
		"--replay-index":  "2",
	}
	for flag, want := range wantFlags {
		found := false
		for i, a := range fr.argv {
			if a == flag && i+1 < len(fr.argv) && fr.argv[i+1] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing flag %s=%s in argv: %v", flag, want, fr.argv)
		}
	}
	// Verify deterministic env vars are set.
	for _, key := range []string{"LC_ALL", "LANG", "TZ"} {
		if fr.env[key] != "C" && fr.env[key] != "UTC" {
			t.Errorf("expected deterministic env %s, got %q", key, fr.env[key])
		}
	}
	if fr.env["LC_ALL"] != "C" {
		t.Fatalf("expected LC_ALL=C, got %q", fr.env["LC_ALL"])
	}
	if fr.env["TZ"] != "UTC" {
		t.Fatalf("expected TZ=UTC, got %q", fr.env["TZ"])
	}
	// Verify custom env is inherited.
	if fr.env["CUSTOM"] != "val" {
		t.Fatalf("expected CUSTOM=val, got %q", fr.env["CUSTOM"])
	}
	// Verify standard JCS env vars are still set.
	if fr.env["JCS_NODE_ID"] != "win1" {
		t.Fatalf("expected JCS_NODE_ID=win1, got %q", fr.env["JCS_NODE_ID"])
	}
}

func TestAdapterRunReplayGoNativeExtractorError(t *testing.T) {
	fr := &fakeRunner{}
	failExtractor := func(bundlePath, destDir string) (string, error) {
		return "", fmt.Errorf("extraction failed")
	}
	a := direct.NewAdapterWithExtractor(fr, failExtractor)
	n := replay.NodeSpec{
		ID:           "win1",
		Mode:         replay.NodeModeDirect,
		Distro:       "windows-ltsc2022",
		KernelFamily: "ntkernel",
		Runner: replay.RunnerConfig{
			Kind:   "direct_go",
			Replay: []string{"go:builtin"},
		},
	}
	err := a.RunReplay(context.Background(), n, "/bundle.tgz", "/evidence.json", 1)
	if err == nil {
		t.Fatal("expected error from failed extractor")
	}
	if !containsStr(err.Error(), "extraction failed") {
		t.Fatalf("expected extraction error, got: %v", err)
	}
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestAdapterPrepareNoOp(t *testing.T) {
	fr := &fakeRunner{}
	a := direct.NewAdapter(fr)
	n := replay.NodeSpec{
		ID:           "win1",
		Mode:         replay.NodeModeDirect,
		Distro:       "windows-ltsc2022",
		KernelFamily: "ntkernel",
		Runner: replay.RunnerConfig{
			Kind:   "direct_command",
			Replay: []string{"runner"},
		},
	}
	if err := a.Prepare(context.Background(), n, "/bundle.tgz", 1); err != nil {
		t.Fatalf("prepare should be no-op without prepare commands: %v", err)
	}
}

func TestAdapterCleanupNoOp(t *testing.T) {
	fr := &fakeRunner{}
	a := direct.NewAdapter(fr)
	n := replay.NodeSpec{
		ID:           "win1",
		Mode:         replay.NodeModeDirect,
		Distro:       "windows-ltsc2022",
		KernelFamily: "ntkernel",
		Runner: replay.RunnerConfig{
			Kind:   "direct_command",
			Replay: []string{"runner"},
		},
	}
	if err := a.Cleanup(context.Background(), n, 1); err != nil {
		t.Fatalf("cleanup should be no-op without cleanup commands: %v", err)
	}
}
