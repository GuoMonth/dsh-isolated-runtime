package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
)

func TestProbeAgainstOfficialDSH(t *testing.T) {
	cli := os.Getenv("DSH_REAL_CLI")
	if cli == "" {
		t.Skip("DSH_REAL_CLI is set by the exact-upstream compatibility gate")
	}
	patch, err := filepath.Abs("../../../images/environment/environment.patch.yml")
	if err != nil {
		t.Fatal(err)
	}
	const authority = "cell.test"
	cfg := launcher.Config{
		DSHCommand:      []string{"node", cli},
		PatchFiles:      []string{patch},
		WorkingDir:      t.TempDir(),
		Environment:     append(os.Environ(), "HOME="+t.TempDir(), "DSH_HOME="+t.TempDir(), "DSH_TELEMETRY_DISABLED=1"),
		PublicAuthority: authority,
	}
	stateFile := filepath.Join(t.TempDir(), "probe.json")
	for _, resume := range []bool{false, true} {
		instance, err := launcher.StartContext(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := stopProbeLauncher(instance); err != nil {
				t.Errorf("close launcher: %v", err)
			}
		})
		if err := run(t.Context(), strings.TrimPrefix(instance.URL, "http://"), authority, stateFile, resume); err != nil {
			t.Fatalf("probe with resume=%t: %v", resume, err)
		}
		if err := stopProbeLauncher(instance); err != nil {
			t.Fatalf("close launcher: %v", err)
		}
	}
}

func stopProbeLauncher(instance *launcher.Instance) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return instance.Close(ctx)
}
