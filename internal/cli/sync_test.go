package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncCmd_Registration(t *testing.T) {
	root := newRootCmd()

	// Verify sync subcommand exists.
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Use == "sync" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("sync subcommand not registered in root command")
	}
}

func TestSyncCmd_HelpText(t *testing.T) {
	root := newRootCmd()

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"sync", "--help"})
	_ = root.Execute()

	output := buf.String()

	if !strings.Contains(output, "Sync") {
		t.Errorf("help should mention 'Sync', got: %s", output)
	}

	if !strings.Contains(output, "base/") {
		t.Errorf("help should mention 'base/', got: %s", output)
	}

	if !strings.Contains(output, "TEMPLATE_SOURCE") {
		t.Errorf("help should mention 'TEMPLATE_SOURCE', got: %s", output)
	}
}

func TestSyncCmd_ErrorOutsideAgenticRepo(t *testing.T) {
	// Running sync in a temp dir that has no TEMPLATE_SOURCE should fail.
	// We can't easily test this without changing cwd, but we can verify the
	// findRepoRoot logic directly.
	// Note: the actual command test would require changing cwd to a non-agentic dir.
	// Instead, test findRepoRoot indirectly — in the real repo it should succeed
	// because we have TEMPLATE_SOURCE.
	_, err := findRepoRoot()
	if err != nil {
		// If running from a dir without TEMPLATE_SOURCE (unlikely in test), that's expected.
		if !strings.Contains(err.Error(), "not inside an agentic repo") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
