package executil_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lattice-substrate/json-canon/offline/runtime/executil"
)

func TestOSRunnerRunRejectsEmptyArgv(t *testing.T) {
	_, err := (executil.OSRunner{}).Run(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected empty argv error")
	}
	if !strings.Contains(err.Error(), "empty argv") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOSRunnerRunCapturesCombinedOutputAndEnv(t *testing.T) {
	out, err := (executil.OSRunner{}).Run(
		context.Background(),
		[]string{"sh", "-c", `printf "%s-%s\n" "$BETA" "$ALPHA"; echo "stderr-line" >&2`},
		map[string]string{
			"BETA":  "two",
			"ALPHA": "one",
		},
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "two-one") {
		t.Fatalf("missing env output: %q", out)
	}
	if !strings.Contains(out, "stderr-line") {
		t.Fatalf("missing stderr output: %q", out)
	}
}

func TestOSRunnerRunIncludesProcessOutputInErrors(t *testing.T) {
	out, err := (executil.OSRunner{}).Run(context.Background(), []string{"sh", "-c", `echo "boom" >&2; exit 7`}, nil)
	if err == nil {
		t.Fatal("expected non-zero command error")
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("missing command output: %q", out)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("missing output in wrapped error: %v", err)
	}
}
