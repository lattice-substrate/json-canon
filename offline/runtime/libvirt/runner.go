package libvirt

import (
	"context"
	"fmt"
	"strconv"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/executil"
)

// Adapter runs replay lanes through Libvirt/KVM-oriented commands.
type Adapter struct {
	runner executil.CommandRunner
}

func NewAdapter(r executil.CommandRunner) *Adapter {
	if r == nil {
		r = executil.OSRunner{}
	}
	return &Adapter{runner: r}
}

func (a *Adapter) Prepare(ctx context.Context, node replay.NodeSpec, bundlePath string, replayIndex int) error {
	if len(node.Runner.Prepare) == 0 {
		return nil
	}
	_, err := a.runner.Run(ctx, node.Runner.Prepare, commandEnv(node, bundlePath, "", replayIndex))
	if err != nil {
		return fmt.Errorf("libvirt prepare command: %w", err)
	}
	return nil
}

func (a *Adapter) RunReplay(ctx context.Context, node replay.NodeSpec, bundlePath string, evidencePath string, replayIndex int) error {
	if len(node.Runner.Replay) == 0 {
		return fmt.Errorf("libvirt replay command is required")
	}
	_, err := a.runner.Run(ctx, node.Runner.Replay, commandEnv(node, bundlePath, evidencePath, replayIndex))
	if err != nil {
		return fmt.Errorf("libvirt replay command: %w", err)
	}
	return nil
}

func (a *Adapter) Cleanup(ctx context.Context, node replay.NodeSpec, replayIndex int) error {
	if len(node.Runner.Cleanup) == 0 {
		return nil
	}
	_, err := a.runner.Run(ctx, node.Runner.Cleanup, commandEnv(node, "", "", replayIndex))
	if err != nil {
		return fmt.Errorf("libvirt cleanup command: %w", err)
	}
	return nil
}

func commandEnv(node replay.NodeSpec, bundlePath, evidencePath string, replayIndex int) map[string]string {
	env := make(map[string]string, len(node.Runner.Env)+8)
	for k, v := range node.Runner.Env {
		env[k] = v
	}
	env["JCS_NODE_ID"] = node.ID
	env["JCS_NODE_MODE"] = string(node.Mode)
	env["JCS_NODE_DISTRO"] = node.Distro
	env["JCS_NODE_KERNEL_FAMILY"] = node.KernelFamily
	env["JCS_REPLAY_INDEX"] = strconv.Itoa(replayIndex)
	if bundlePath != "" {
		env["JCS_BUNDLE_PATH"] = bundlePath
	}
	if evidencePath != "" {
		env["JCS_EVIDENCE_PATH"] = evidencePath
	}
	return env
}
