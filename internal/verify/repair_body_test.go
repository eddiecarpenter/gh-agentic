package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── RepairAIDirWithWriter — body paths (aiDir present/absent, git checkout) ──

func TestRepairAIDir_AIDir_Exists_GitCheckoutFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Create .ai/ so the os.IsNotExist branch is false.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("git: not a git repository")
	}

	result := RepairAIDir(root, fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when git checkout fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git checkout failed") {
		t.Errorf("expected 'git checkout failed' in message, got: %s", result.Message)
	}
}

func TestRepairAIDir_AIDir_Exists_GitCheckoutSucceeds_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil // git checkout succeeds
	}

	result := RepairAIDir(root, fakeRun, nil)
	if result.Status != Pass {
		t.Errorf("expected Pass when git checkout succeeds, got %v: %s", result.Status, result.Message)
	}
}
