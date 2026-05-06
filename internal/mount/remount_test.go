package mount

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRemount_Success(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstall(t, map[string]string{
		"RULEBOOK.md":            "# Rules refreshed",
		"skills/session-init.md": "# Session Init",
	})

	err := RunRemount(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify framework files exist.
	if _, err := os.Stat(filepath.Join(root, ".agents", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".agents/RULEBOOK.md should exist")
	}

	// Verify output.
	output := buf.String()
	if !strings.Contains(output, "Mounting AI Framework (v2.0.0)") {
		t.Errorf("output should show version, got:\n%s", output)
	}
	if !strings.Contains(output, "Framework mounted at .agents/") {
		t.Errorf("output should show success, got:\n%s", output)
	}
	// No confirmation prompt.
	if strings.Contains(output, "[y/N]") || strings.Contains(output, "confirm") {
		t.Error("remount should not show confirmation prompt")
	}
}

func TestRunRemount_RefreshesExistingSubmodule(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	// Pre-existing submodule state: .gitmodules has the .agents entry plus
	// a populated .agents/ with stale content. RunRemount must dispatch to
	// SwapSubmodule (not Install), and the swap stub repopulates .agents/
	// with the fresh files.
	aiDir := filepath.Join(root, ".agents")
	_ = os.MkdirAll(aiDir, 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "stale.txt"), []byte("stale"), 0o644)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Old rules"), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule ".agents"]`+"\n\turl = "+FrameworkRepoURL+"\n"), 0o644)

	withStubSwap(t, map[string]string{
		"RULEBOOK.md": "# Fresh rules",
	})

	err := RunRemount(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stale file should be gone (swap stub wipes .agents/ before repopulating).
	if _, err := os.Stat(filepath.Join(aiDir, "stale.txt")); err == nil {
		t.Error("stale.txt should be removed")
	}

	// Fresh content should be present.
	data, _ := os.ReadFile(filepath.Join(aiDir, "RULEBOOK.md"))
	if string(data) != "# Fresh rules" {
		t.Errorf("expected fresh content, got: %s", data)
	}
}

func TestRunRemount_DownloadFailure(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstallError(t, "network")

	err := RunRemount(&buf, root, "v2.0.0", nil)
	if err == nil {
		t.Fatal("expected error on download failure")
	}
	if !strings.Contains(err.Error(), "remounting framework") {
		t.Errorf("error should mention remounting, got: %v", err)
	}
}

func TestRunRemount_SilentNoPrompt(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstall(t, map[string]string{
		"RULEBOOK.md": "# Rules",
	})

	err := RunRemount(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Switch") || strings.Contains(output, "[y/N]") {
		t.Error("remount should be silent — no switch prompt")
	}
}
