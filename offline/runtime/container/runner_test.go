package container_test

import (
	"context"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/container"
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
	a := container.NewAdapter(fr)
	n := replay.NodeSpec{
		ID:           "n1",
		Mode:         replay.NodeModeContainer,
		Distro:       "debian",
		KernelFamily: "host",
		Runner: replay.RunnerConfig{
			Kind:   "container_command",
			Replay: []string{"runner", "arg"},
			Env: map[string]string{
				"X": "1",
			},
		},
	}
	if err := a.RunReplay(context.Background(), n, "/bundle.tgz", "/evidence.json", 2); err != nil {
		t.Fatalf("run replay: %v", err)
	}
	if fr.argv[0] != "runner" {
		t.Fatalf("unexpected argv: %#v", fr.argv)
	}
	if fr.env["JCS_REPLAY_INDEX"] != "2" {
		t.Fatalf("missing replay env: %#v", fr.env)
	}
	if fr.env["JCS_BUNDLE_PATH"] != "/bundle.tgz" || fr.env["JCS_EVIDENCE_PATH"] != "/evidence.json" {
		t.Fatalf("missing bundle/evidence env: %#v", fr.env)
	}
}
