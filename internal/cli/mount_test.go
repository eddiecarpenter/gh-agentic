package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// mountFakeClone creates a CloneFunc that writes framework files into destDir.
func mountFakeClone() mount.CloneFunc {
	return func(repoURL, tag, destDir string) error {
		files := map[string]string{
			"RULEBOOK.md":            "# Rules for " + tag,
			"skills/session-init.md": "# Session Init",
			"standards/go.md":        "# Go Standards",
			"recipes/dev.yaml":       "recipe: dev",
			"concepts/philosophy.md": "# Philosophy",
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		for path, content := range files {
			full := filepath.Join(destDir, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

// mountFakeReleases returns a FetchReleasesFunc with test releases.
func mountFakeReleases() mount.FetchReleasesFunc {
	return func(repo string) ([]mount.Release, error) {
		return []mount.Release{
			{TagName: "v2.0.0", Name: "v2.0.0", TarballURL: "https://example.com/v2.0.0.tar.gz"},
			{TagName: "v1.5.0", Name: "v1.5.0", TarballURL: "https://example.com/v1.5.0.tar.gz"},
		}, nil
	}
}

func TestMountCmd_FirstTimeMount(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		clone:         mountFakeClone(),
		resolveVersion: func(root string) (string, error) {
			return "v2.0.0", nil
		},
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for i, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "mount") {
			rootCmd.RemoveCommand(c)
			_ = i
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mount"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()

	if !strings.Contains(output, "AI Framework successfully mounted") {
		t.Errorf("expected success message, got:\n%s", output)
	}

	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".ai/RULEBOOK.md should exist")
	}

	// The flat .ai-version file was removed in #585; the mounted version
	// is tracked via .ai/.git metadata. We can't read that here because
	// the fake clone does not set up a git repo, so the success-message
	// assertion above is our "the right version flowed through" proof.

	gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(gitignore), ".ai/") {
		t.Error(".gitignore should contain .ai/")
	}

	pipeline, _ := os.ReadFile(filepath.Join(root, ".github", "workflows", "agentic-pipeline.yml"))
	if !strings.Contains(string(pipeline), "@v2.0.0") {
		t.Errorf("pipeline workflow should reference @v2.0.0, got: %s", pipeline)
	}
}

func TestMountCmd_InvalidTag(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		clone:         mountFakeClone(),
		resolveVersion: func(root string) (string, error) {
			return "v9.9.9", nil
		},
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "mount") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mount"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid tag")
	}
	if !strings.Contains(err.Error(), "v9.9.9 not found") {
		t.Errorf("error should mention invalid tag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "v2.0.0") {
		t.Errorf("error should mention latest version, got: %v", err)
	}
}

func TestMountCmd_FederatedMountsControlPlane(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var cpCalls []string
	syncCP := func(cpNameWithOwner, destDir string) error {
		cpCalls = append(cpCalls, cpNameWithOwner)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destDir, "marker"), []byte(cpNameWithOwner), 0o644)
	}

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		clone:         mountFakeClone(),
		resolveVersion: func(root string) (string, error) {
			return "v2.0.0", nil
		},
		resolveCP: func(root string) string { return "org/control-plane" },
		syncCP:    syncCP,
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "mount") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mount"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	if len(cpCalls) != 1 || cpCalls[0] != "org/control-plane" {
		t.Errorf("expected one CP sync call for org/control-plane, got %v", cpCalls)
	}

	if _, err := os.Stat(filepath.Join(root, ".cp", "marker")); os.IsNotExist(err) {
		t.Error(".cp/marker should exist after federated mount")
	}

	gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(gitignore), ".cp/") {
		t.Errorf(".gitignore should contain .cp/ entry, got: %s", gitignore)
	}
}

func TestMountCmd_NonFederatedSkipsControlPlane(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var cpCalls int
	syncCP := func(cpNameWithOwner, destDir string) error {
		cpCalls++
		return nil
	}

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		clone:         mountFakeClone(),
		resolveVersion: func(root string) (string, error) {
			return "v2.0.0", nil
		},
		resolveCP: func(root string) string { return "" },
		syncCP:    syncCP,
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "mount") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mount"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	if cpCalls != 0 {
		t.Errorf("expected no CP sync calls for non-federated repo, got %d", cpCalls)
	}
	if _, err := os.Stat(filepath.Join(root, ".cp")); !os.IsNotExist(err) {
		t.Error(".cp/ should not be created for non-federated repos")
	}
}

func TestMountCmd_NoVersionNoAIVersion(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		clone:         mountFakeClone(),
		resolveVersion: func(root string) (string, error) {
			return "", fmt.Errorf("no version found — run 'gh agentic project init' to set up this repo")
		},
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "mount") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"mount"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error when no version and no .ai-version")
	}
	if !strings.Contains(err.Error(), "no version found") {
		t.Errorf("error should mention missing version, got: %v", err)
	}
}
