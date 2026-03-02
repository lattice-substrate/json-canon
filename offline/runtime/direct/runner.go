// Package direct provides node adapters for direct host-execution offline replay lanes.
// Direct mode executes the replay worker binary directly on the host OS without
// container or VM isolation. This is used for cross-OS determinism verification
// where the binary under test is a cross-compiled artifact (e.g., a Windows .exe).
package direct

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/lattice-substrate/json-canon/offline/replay"
	"github.com/lattice-substrate/json-canon/offline/runtime/executil"
)

// KindGoNative is the runner kind for Go-native direct execution without shell scripts.
const KindGoNative = "direct_go"

// WorkerExtractor extracts the worker binary from a bundle archive.
type WorkerExtractor func(bundlePath, destDir string) (string, error)

// Adapter runs replay lanes by executing the worker binary directly on the host.
type Adapter struct {
	runner    executil.CommandRunner
	extractor WorkerExtractor
}

// NewAdapter constructs a direct adapter with the provided command runner.
func NewAdapter(r executil.CommandRunner) *Adapter {
	if r == nil {
		r = executil.OSRunner{}
	}
	return &Adapter{runner: r, extractor: replay.ExtractWorkerBinary}
}

// NewAdapterWithExtractor constructs a direct adapter with a custom worker extractor.
// Intended for testing.
func NewAdapterWithExtractor(r executil.CommandRunner, ext WorkerExtractor) *Adapter {
	a := NewAdapter(r)
	if ext != nil {
		a.extractor = ext
	}
	return a
}

// Prepare runs pre-replay setup commands for a direct node.
func (a *Adapter) Prepare(ctx context.Context, node replay.NodeSpec, bundlePath string, replayIndex int) error {
	if len(node.Runner.Prepare) == 0 {
		return nil
	}
	_, err := a.runner.Run(ctx, node.Runner.Prepare, commandEnv(node, bundlePath, "", replayIndex))
	if err != nil {
		return fmt.Errorf("direct prepare command: %w", err)
	}
	return nil
}

// RunReplay executes one replay command for a direct node.
func (a *Adapter) RunReplay(ctx context.Context, node replay.NodeSpec, bundlePath string, evidencePath string, replayIndex int) error {
	if node.Runner.Kind == KindGoNative {
		return a.runGoNative(ctx, node, bundlePath, evidencePath, replayIndex)
	}
	if len(node.Runner.Replay) == 0 {
		return fmt.Errorf("direct replay command is required")
	}
	_, err := a.runner.Run(ctx, node.Runner.Replay, commandEnv(node, bundlePath, evidencePath, replayIndex))
	if err != nil {
		return fmt.Errorf("direct replay command: %w", err)
	}
	return nil
}

// runGoNative extracts the worker from the bundle and invokes it directly via os/exec,
// eliminating any shell dependency. Used for Windows where bash is not available.
func (a *Adapter) runGoNative(ctx context.Context, node replay.NodeSpec, bundlePath string, evidencePath string, replayIndex int) error {
	tmpDir, err := os.MkdirTemp("", "jcs-direct-go-*")
	if err != nil {
		return fmt.Errorf("direct_go create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	workerPath, err := a.extractor(bundlePath, tmpDir)
	if err != nil {
		return fmt.Errorf("direct_go extract worker: %w", err)
	}

	argv := []string{
		workerPath,
		"--bundle", bundlePath,
		"--evidence", evidencePath,
		"--node-id", node.ID,
		"--mode", string(node.Mode),
		"--distro", node.Distro,
		"--kernel-family", node.KernelFamily,
		"--replay-index", strconv.Itoa(replayIndex),
	}

	env := deterministicEnv(node, bundlePath, evidencePath, replayIndex)
	_, runErr := a.runner.Run(ctx, argv, env)
	if runErr != nil {
		return fmt.Errorf("direct_go replay command: %w", runErr)
	}
	return nil
}

// deterministicEnv builds the environment for Go-native direct execution.
// It includes LC_ALL=C, LANG=C, TZ=UTC for locale/timezone determinism,
// matching the behavior of replay-direct.sh.
func deterministicEnv(node replay.NodeSpec, bundlePath, evidencePath string, replayIndex int) map[string]string {
	env := commandEnv(node, bundlePath, evidencePath, replayIndex)
	env["LC_ALL"] = "C"
	env["LANG"] = "C"
	env["TZ"] = "UTC"
	return env
}

// Cleanup runs post-replay cleanup commands for a direct node.
func (a *Adapter) Cleanup(ctx context.Context, node replay.NodeSpec, replayIndex int) error {
	if len(node.Runner.Cleanup) == 0 {
		return nil
	}
	_, err := a.runner.Run(ctx, node.Runner.Cleanup, commandEnv(node, "", "", replayIndex))
	if err != nil {
		return fmt.Errorf("direct cleanup command: %w", err)
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
