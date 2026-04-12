package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func setupDoctorV2HealthyRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(filepath.Join(aiDir, "skills"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiDir, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)
	_ = mount.WriteAIVersion(root, "v2.0.0")
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ai/\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Claude\n@AGENTS.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agents\n@.ai/RULEBOOK.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "LOCALRULES.md"), []byte("# Local"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# Readme"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "skills"), 0o755)

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@v2.0.0"), 0o644)

	return root
}

func fakeResolveRepo() (repoInfo, error) {
	return repoInfo{
		FullName:  "owner/repo",
		Owner:     "owner",
		RepoName:  "repo",
		OwnerType: "User",
	}, nil
}

func TestDoctorV2Cmd_HealthyRepo_GroupedOutput(t *testing.T) {
	root := setupDoctorV2HealthyRepo(t)

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := doctorV2Deps{
		run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "variable" {
				return "some-value", nil
			}
			if name == "gh" && len(args) > 0 && args[0] == "secret" {
				return "GOOSE_AGENT_PAT\tUpdated", nil
			}
			return "", nil
		},
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
		resolveRepo: fakeResolveRepo,
	}

	cmd := newDoctorV2CmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	// Replace doctor-v2 with test version.
	for _, c := range rootCmd.Commands() {
		if c.Use == "doctor-v2" {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"doctor-v2"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()

	// Verify grouped output.
	for _, group := range []string{"Repository", "Framework", "Agent files", "Workflows"} {
		if !strings.Contains(output, group) {
			t.Errorf("expected group heading %q in output, got:\n%s", group, output)
		}
	}

	// Healthy repo should show check marks.
	if !strings.Contains(output, "✓") {
		t.Error("expected check marks in output")
	}
}

func TestDoctorV2Cmd_MissingFramework_ExitCode1(t *testing.T) {
	root := t.TempDir()

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := doctorV2Deps{
		run: func(name string, args ...string) (string, error) {
			return "", nil
		},
		resolveRepo: fakeResolveRepo,
	}

	cmd := newDoctorV2CmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if c.Use == "doctor-v2" {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"doctor-v2"})
	err := rootCmd.Execute()

	// Should return ErrSilent (which is non-nil).
	if err == nil {
		t.Fatal("expected error for unhealthy repo")
	}

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Error("expected fail marks in output")
	}
}

func TestDoctorV2Cmd_VersionMismatch_ShowsRemediation(t *testing.T) {
	root := t.TempDir()
	_ = mount.WriteAIVersion(root, "v2.0.0")

	// Create workflow with wrong version.
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v1.0.0"), 0o644)

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := doctorV2Deps{
		run:         func(name string, args ...string) (string, error) { return "", nil },
		resolveRepo: fakeResolveRepo,
	}

	cmd := newDoctorV2CmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if c.Use == "doctor-v2" {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"doctor-v2"})
	_ = rootCmd.Execute()

	output := buf.String()
	if !strings.Contains(output, "mismatch") {
		t.Errorf("expected version mismatch in output, got:\n%s", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("expected remediation arrow in output, got:\n%s", output)
	}
}

func TestDoctorV2Cmd_WarningExitCode0(t *testing.T) {
	root := t.TempDir()

	// Minimal setup — only mandatory files, missing optional ones.
	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(filepath.Join(aiDir, "skills"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiDir, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)
	_ = mount.WriteAIVersion(root, "v2.0.0")
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ai/\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Claude"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agents"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# Readme"), 0o644)

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@v2.0.0"), 0o644)

	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	deps := doctorV2Deps{
		run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "variable" {
				return "some-value", nil
			}
			if name == "gh" && len(args) > 0 && args[0] == "secret" {
				return "GOOSE_AGENT_PAT\tUpdated", nil
			}
			return "", nil
		},
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
		resolveRepo: fakeResolveRepo,
	}

	cmd := newDoctorV2CmdWithDeps(deps)
	rootCmd := newRootCmd("dev", "")
	for _, c := range rootCmd.Commands() {
		if c.Use == "doctor-v2" {
			rootCmd.RemoveCommand(c)
			break
		}
	}
	rootCmd.AddCommand(cmd)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"doctor-v2"})
	err := rootCmd.Execute()

	// Should exit 0 (nil error) even with warnings (missing LOCALRULES.md, skills/).
	if err != nil {
		t.Fatalf("expected nil error for warnings-only, got: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "⚠") {
		t.Error("expected warning marks in output")
	}
}
