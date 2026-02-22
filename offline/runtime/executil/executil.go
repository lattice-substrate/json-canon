// Package executil provides command execution helpers for offline runtime adapters.
package executil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// CommandRunner abstracts command execution for runtime adapters.
type CommandRunner interface {
	Run(ctx context.Context, argv []string, env map[string]string) (string, error)
}

// OSRunner executes commands on the host.
type OSRunner struct{}

// Run executes argv with merged environment variables and combined output capture.
func (OSRunner) Run(ctx context.Context, argv []string, env map[string]string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("empty argv")
	}
	// #nosec G204 -- argv is policy-controlled by offline matrix/profile inputs.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(env) != 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		merged := cmd.Environ()
		for _, k := range keys {
			merged = append(merged, fmt.Sprintf("%s=%s", k, env[k]))
		}
		cmd.Env = merged
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg != "" {
			return out.String(), fmt.Errorf("run %q failed: %w: %s", argv, err, msg)
		}
		return out.String(), fmt.Errorf("run %q failed: %w", argv, err)
	}
	return out.String(), nil
}
