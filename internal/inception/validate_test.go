package inception

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- ValidateEnvironment tests ---

func TestValidateEnvironment_NoReposMD_ReturnsError(t *testing.T) {
	// Use a temp directory without REPOS.md.
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd) //nolint:errcheck

	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	_, err := ValidateEnvironment(run)
	if err == nil {
		t.Fatal("ValidateEnvironment() expected error when REPOS.md missing, got nil")
	}
	if !strings.Contains(err.Error(), "REPOS.md") {
		t.Errorf("error should mention REPOS.md, got: %v", err)
	}
}

func TestValidateEnvironment_WithReposMD_ReturnsEnvContext(t *testing.T) {
	dir := t.TempDir()

	// Create REPOS.md.
	if err := os.WriteFile(filepath.Join(dir, "REPOS.md"), []byte("# REPOS.md"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create AGENTS.local.md with owner info.
	agentsLocal := "# AGENTS.local.md\n\n## Repo\n\n- **GitHub:** https://github.com/acme-org/my-project\n- **Owner:** acme-org\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(agentsLocal), 0644); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd) //nolint:errcheck

	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	ctx, err := ValidateEnvironment(run)
	if err != nil {
		t.Fatalf("ValidateEnvironment() unexpected error: %v", err)
	}
	if ctx.Owner != "acme-org" {
		t.Errorf("ctx.Owner = %q, want %q", ctx.Owner, "acme-org")
	}
	if ctx.AgenticRepoRoot != dir {
		t.Errorf("ctx.AgenticRepoRoot = %q, want %q", ctx.AgenticRepoRoot, dir)
	}
}

func TestValidateEnvironment_OwnerFromGitRemote(t *testing.T) {
	dir := t.TempDir()

	// Create REPOS.md.
	if err := os.WriteFile(filepath.Join(dir, "REPOS.md"), []byte("# REPOS.md"), 0644); err != nil {
		t.Fatal(err)
	}

	// No AGENTS.local.md — owner must come from git remote.
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd) //nolint:errcheck

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "remote" {
			return "git@github.com:my-org/my-repo.git\n", nil
		}
		return "", nil
	}

	ctx, err := ValidateEnvironment(run)
	if err != nil {
		t.Fatalf("ValidateEnvironment() unexpected error: %v", err)
	}
	if ctx.Owner != "my-org" {
		t.Errorf("ctx.Owner = %q, want %q", ctx.Owner, "my-org")
	}
}

func TestValidateEnvironment_NoOwnerFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	// Create REPOS.md but no AGENTS.local.md and git remote fails.
	if err := os.WriteFile(filepath.Join(dir, "REPOS.md"), []byte("# REPOS.md"), 0644); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd) //nolint:errcheck

	run := func(name string, args ...string) (string, error) {
		return "", nil // empty output — no owner extractable
	}

	_, err := ValidateEnvironment(run)
	if err == nil {
		t.Fatal("ValidateEnvironment() expected error when owner cannot be determined, got nil")
	}
	if !strings.Contains(err.Error(), "owner") {
		t.Errorf("error should mention owner, got: %v", err)
	}
}

func TestValidateEnvironment_GoModule_Extracted(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "REPOS.md"), []byte("# REPOS.md"), 0644); err != nil {
		t.Fatal(err)
	}
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(agentsLocal), 0644); err != nil {
		t.Fatal(err)
	}
	goMod := "module github.com/alice/my-project\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd) //nolint:errcheck

	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	ctx, err := ValidateEnvironment(run)
	if err != nil {
		t.Fatalf("ValidateEnvironment() unexpected error: %v", err)
	}
	if ctx.Module != "github.com/alice/my-project" {
		t.Errorf("ctx.Module = %q, want %q", ctx.Module, "github.com/alice/my-project")
	}
}

// --- extractOwnerFromAgentsLocal tests ---

func TestExtractOwnerFromAgentsLocal_GitHubURL(t *testing.T) {
	dir := t.TempDir()
	content := "## Repo\n\n- **GitHub:** https://github.com/myorg/my-project\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := extractOwnerFromAgentsLocal(dir)
	if got != "myorg" {
		t.Errorf("extractOwnerFromAgentsLocal() = %q, want %q", got, "myorg")
	}
}

func TestExtractOwnerFromAgentsLocal_OwnerField(t *testing.T) {
	dir := t.TempDir()
	content := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := extractOwnerFromAgentsLocal(dir)
	if got != "alice" {
		t.Errorf("extractOwnerFromAgentsLocal() = %q, want %q", got, "alice")
	}
}

func TestExtractOwnerFromAgentsLocal_NoFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got := extractOwnerFromAgentsLocal(dir)
	if got != "" {
		t.Errorf("extractOwnerFromAgentsLocal() = %q, want empty string", got)
	}
}

// --- extractOwnerFromGitRemote tests ---

func TestExtractOwnerFromGitRemote_SSH(t *testing.T) {
	run := func(name string, args ...string) (string, error) {
		return "git@github.com:acme-org/my-repo.git\n", nil
	}
	got := extractOwnerFromGitRemote(run)
	if got != "acme-org" {
		t.Errorf("extractOwnerFromGitRemote() = %q, want %q", got, "acme-org")
	}
}

func TestExtractOwnerFromGitRemote_HTTPS(t *testing.T) {
	run := func(name string, args ...string) (string, error) {
		return "https://github.com/acme-org/my-repo.git\n", nil
	}
	got := extractOwnerFromGitRemote(run)
	if got != "acme-org" {
		t.Errorf("extractOwnerFromGitRemote() = %q, want %q", got, "acme-org")
	}
}

func TestExtractOwnerFromGitRemote_NoRemote_ReturnsEmpty(t *testing.T) {
	run := func(name string, args ...string) (string, error) {
		return "", nil
	}
	got := extractOwnerFromGitRemote(run)
	if got != "" {
		t.Errorf("extractOwnerFromGitRemote() = %q, want empty string", got)
	}
}

// --- extractGoModule tests ---

func TestExtractGoModule_Present(t *testing.T) {
	dir := t.TempDir()
	content := "module github.com/alice/my-project\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got := extractGoModule(dir)
	if got != "github.com/alice/my-project" {
		t.Errorf("extractGoModule() = %q, want %q", got, "github.com/alice/my-project")
	}
}

func TestExtractGoModule_NoGoMod_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got := extractGoModule(dir)
	if got != "" {
		t.Errorf("extractGoModule() = %q, want empty string", got)
	}
}
