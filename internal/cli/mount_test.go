package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
)

// mountFakeFetch creates a FetchTarballFunc that returns a tarball with
// framework files for testing.
func mountFakeFetch() mount.FetchTarballFunc {
	return func(repo, version string) (io.ReadCloser, error) {
		files := map[string]string{
			"RULEBOOK.md":            "# Rules for " + version,
			"skills/session-init.md": "# Session Init",
			"standards/go.md":        "# Go Standards",
			"recipes/dev.yaml":       "recipe: dev",
			"concepts/philosophy.md": "# Philosophy",
		}

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		prefix := "gh-agentic-" + version + "/"
		_ = tw.WriteHeader(&tar.Header{
			Name: prefix, Typeflag: tar.TypeDir, Mode: 0o755,
		})

		for path, content := range files {
			dir := filepath.Dir(path)
			if dir != "." {
				_ = tw.WriteHeader(&tar.Header{
					Name: prefix + dir + "/", Typeflag: tar.TypeDir, Mode: 0o755,
				})
			}
			_ = tw.WriteHeader(&tar.Header{
				Name: prefix + path, Size: int64(len(content)),
				Mode: 0o644, Typeflag: tar.TypeReg,
			})
			_, _ = tw.Write([]byte(content))
		}

		_ = tw.Close()
		_ = gw.Close()
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}
}

// mountFakeReleases returns a FetchReleasesFunc with test releases.
func mountFakeReleases() mount.FetchReleasesFunc {
	return func(repo string) ([]sync.Release, error) {
		return []sync.Release{
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
		fetchTarball:  mountFakeFetch(),
	}

	cmd := newMountCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	// Replace the mount command with our test version.
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
	rootCmd.SetArgs([]string{"--v2", "mount", "v2.0.0"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()

	// Verify output messages.
	if !strings.Contains(output, "AI Framework successfully mounted") {
		t.Errorf("expected success message, got:\n%s", output)
	}

	// Verify .ai/RULEBOOK.md exists.
	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".ai/RULEBOOK.md should exist")
	}

	// Verify .ai-version.
	v, err := mount.ReadAIVersion(root)
	if err != nil {
		t.Fatalf("reading .ai-version: %v", err)
	}
	if v != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", v)
	}

	// Verify .gitignore.
	gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(gitignore), ".ai/") {
		t.Error(".gitignore should contain .ai/")
	}

	// Verify workflows.
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
		fetchTarball:  mountFakeFetch(),
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
	rootCmd.SetArgs([]string{"--v2", "mount", "v9.9.9"})
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

func TestMountCmd_WithoutV2Flag(t *testing.T) {
	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		fetchTarball:  mountFakeFetch(),
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
	rootCmd.SetArgs([]string{"mount", "v2.0.0"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error without -v2 flag")
	}
	if !strings.Contains(err.Error(), "requires the --v2 flag") {
		t.Errorf("error should mention --v2 flag, got: %v", err)
	}
}

func TestMountCmd_NoVersionNoAIVersion(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := mountDeps{
		fetchReleases: mountFakeReleases(),
		fetchTarball:  mountFakeFetch(),
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
	rootCmd.SetArgs([]string{"--v2", "mount"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error when no version and no .ai-version")
	}
	if !strings.Contains(err.Error(), "no version specified") {
		t.Errorf("error should mention missing version, got: %v", err)
	}
}
