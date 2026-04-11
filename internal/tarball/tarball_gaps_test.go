package tarball

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── ReadTemplateConfig — primary .ai/config.yml path ─────────────────────────

func TestReadTemplateConfig_AIConfigYML_Valid_ReturnsValues(t *testing.T) {
	root := t.TempDir()
	aiDir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "template: owner/template-repo\nversion: v1.2.3\n"
	if err := os.WriteFile(filepath.Join(aiDir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, version, err := ReadTemplateConfig(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo != "owner/template-repo" {
		t.Errorf("repo = %q, want %q", repo, "owner/template-repo")
	}
	if version != "v1.2.3" {
		t.Errorf("version = %q, want %q", version, "v1.2.3")
	}
}

func TestReadTemplateConfig_AIConfigYML_EmptyFields_FallsBackToLegacy(t *testing.T) {
	root := t.TempDir()
	aiDir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// config.yml present but template is empty → falls through to legacy files.
	if err := os.WriteFile(filepath.Join(aiDir, "config.yml"), []byte("template: \nversion: v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Provide legacy fallback files.
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/legacy-repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v0.9.0"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, version, err := ReadTemplateConfig(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo != "owner/legacy-repo" {
		t.Errorf("repo = %q, want %q", repo, "owner/legacy-repo")
	}
	if version != "v0.9.0" {
		t.Errorf("version = %q, want %q", version, "v0.9.0")
	}
}

func TestReadTemplateConfig_AIConfigYML_InvalidYAML_FallsBackToLegacy(t *testing.T) {
	root := t.TempDir()
	aiDir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Invalid YAML → yaml.Unmarshal fails → falls through.
	if err := os.WriteFile(filepath.Join(aiDir, "config.yml"), []byte(":\t: bad yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/fallback"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v2.0.0"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, version, err := ReadTemplateConfig(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo != "owner/fallback" {
		t.Errorf("repo = %q, want %q", repo, "owner/fallback")
	}
	if version != "v2.0.0" {
		t.Errorf("version = %q, want %q", version, "v2.0.0")
	}
}

// ── defaultFetch — error paths via newCommand injection ──────────────────────

type fakeCommand struct {
	stdoutPipeErr error
	startErr      error
}

func (f *fakeCommand) StdoutPipe() (io.ReadCloser, error) {
	if f.stdoutPipeErr != nil {
		return nil, f.stdoutPipeErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeCommand) Start() error { return f.startErr }
func (f *fakeCommand) Wait() error  { return nil }

func TestDefaultFetch_StdoutPipeFails_ReturnsError(t *testing.T) {
	orig := newCommand
	t.Cleanup(func() { newCommand = orig })
	newCommand = func(name string, args ...string) command {
		return &fakeCommand{stdoutPipeErr: fmt.Errorf("pipe error")}
	}

	_, err := defaultFetch("owner/repo", "v1.0.0")
	if err == nil {
		t.Fatal("expected error when StdoutPipe fails, got nil")
	}
	if !strings.Contains(err.Error(), "creating pipe") {
		t.Errorf("expected 'creating pipe' in error, got: %v", err)
	}
}

func TestDefaultFetch_StartFails_ReturnsError(t *testing.T) {
	orig := newCommand
	t.Cleanup(func() { newCommand = orig })
	newCommand = func(name string, args ...string) command {
		return &fakeCommand{startErr: fmt.Errorf("exec: start failed")}
	}

	_, err := defaultFetch("owner/repo", "v1.0.0")
	if err == nil {
		t.Fatal("expected error when Start fails, got nil")
	}
	if !strings.Contains(err.Error(), "starting download") {
		t.Errorf("expected 'starting download' in error, got: %v", err)
	}
}

// ── writeFile — OpenFile fails when path is a directory ──────────────────────

func TestWriteFile_OpenFileFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Place a directory at the target path so OpenFile fails with "is a directory".
	target := filepath.Join(dir, "output.txt")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	err := writeFile(target, strings.NewReader("content"), 0o644)
	if err == nil {
		t.Fatal("expected error when path is a directory, got nil")
	}
}

// ── copyTree — walk error when src file becomes unreadable ───────────────────

func TestCopyTree_ReadFileFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read all files — chmod test not meaningful")
	}
	src := t.TempDir()
	dst := t.TempDir()

	// Create a file then remove read permission.
	secret := filepath.Join(src, "secret.txt")
	if err := os.WriteFile(secret, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o644) })

	err := copyTree(src, dst)
	if err == nil {
		t.Fatal("expected error when file is unreadable, got nil")
	}
}
