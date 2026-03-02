package direct_test

import (
	"context"
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
