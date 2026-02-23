package libvirt_test

import (
	"context"
	"testing"

	"github.com/SolutionsExcite/json-canon/offline/replay"
	"github.com/SolutionsExcite/json-canon/offline/runtime/libvirt"
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
	a := libvirt.NewAdapter(fr)
	n := replay.NodeSpec{
		ID:           "n1",
		Mode:         replay.NodeModeVM,
		Distro:       "ubuntu",
		KernelFamily: "ga",
		Runner: replay.RunnerConfig{
			Kind:   "libvirt_command",
			Replay: []string{"runner", "arg"},
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
	if fr.env["JCS_NODE_MODE"] != "vm" {
		t.Fatalf("missing mode env: %#v", fr.env)
	}
}
