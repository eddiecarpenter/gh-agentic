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

	"github.com/eddiecarpenter/gh-agentic/internal/initv2"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func initFakeFetch() mount.FetchTarballFunc {
	return func(repo, version string) (io.ReadCloser, error) {
		files := map[string]string{
			"RULEBOOK.md": "# Rules",
		}

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		prefix := "gh-agentic-" + version + "/"
		_ = tw.WriteHeader(&tar.Header{
			Name: prefix, Typeflag: tar.TypeDir, Mode: 0o755,
		})

		for path, content := range files {
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

func TestInitCmd_WithoutV2Flag(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"init"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error without -v2 flag")
	}
	if !strings.Contains(err.Error(), "requires the -v2 flag") {
		t.Errorf("expected v2 flag error, got: %v", err)
	}
}

func TestInitCmd_BlockedWithoutForce(t *testing.T) {
	root := t.TempDir()
	_ = mount.WriteAIVersion(root, "v1.0.0")

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := initv2.Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		FetchTarball: initFakeFetch(),
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
	rootCmd.SetArgs([]string{"--v2", "init"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error when .ai-version exists without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected --force in error, got: %v", err)
	}
}

func TestInitCmd_ProceedsWithForce(t *testing.T) {
	root := t.TempDir()
	_ = mount.WriteAIVersion(root, "v1.0.0")

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := initv2.Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		FetchTarball: initFakeFetch(),
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
	rootCmd.SetArgs([]string{"--v2", "init", "--force"})
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
		FetchTarball: initFakeFetch(),
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
