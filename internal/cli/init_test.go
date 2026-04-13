package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/initv2"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func initFakeClone() mount.CloneFunc {
	return func(repoURL, tag, destDir string) error {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)
	}
}

func TestInitCmd_BlockedWithoutForce(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := initv2.Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		Clone: initFakeClone(),
		CollectConfig: func(w io.Writer, repo string) (*initv2.InitConfig, error) {
			return &initv2.InitConfig{Version: "v2.0.0"}, nil
		},
	}

	cmd := newInitCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "init") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error when .ai/ exists without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected --force in error, got: %v", err)
	}
}

func TestInitCmd_ProceedsWithForce(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := initv2.Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		Clone: initFakeClone(),
		CollectConfig: func(w io.Writer, repo string) (*initv2.InitConfig, error) {
			return &initv2.InitConfig{
				Version:      "v2.0.0",
				RepoFullName: "owner/repo",
				Owner:        "owner",
				RepoName:     "repo",
			}, nil
		},
	}

	cmd := newInitCmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "init") {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"init", "--force"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("expected no error with --force, got: %v\nOutput:\n%s", err, buf.String())
	}

	// Verify framework was mounted.
	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".ai/RULEBOOK.md should exist")
	}
}

func TestInitCmd_ForceFlagRegistered(t *testing.T) {
	deps := initv2.Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		Clone: initFakeClone(),
		CollectConfig: func(w io.Writer, repo string) (*initv2.InitConfig, error) {
			return &initv2.InitConfig{Version: "v2.0.0"}, nil
		},
	}

	cmd := newInitCmdWithDeps(deps)
	f := cmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("--force flag not registered")
	}
	if f.DefValue != "false" {
		t.Errorf("--force default should be false, got %q", f.DefValue)
	}
}
