package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectMountState_None(t *testing.T) {
	root := t.TempDir()
	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateNone {
		t.Errorf("expected MountStateNone, got %d", state)
	}
}

func TestDetectMountState_Symlink(t *testing.T) {
	root := t.TempDir()
	// Create .ai as a symlink to .
	if err := os.Symlink(".", filepath.Join(root, ".ai")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateSymlink {
		t.Errorf("expected MountStateSymlink, got %d", state)
	}
}

func TestDetectMountState_Submodule(t *testing.T) {
	root := t.TempDir()

	// Create .gitmodules with a .ai entry.
	gitmodules := `[submodule ".ai"]
	path = .ai
	url = https://github.com/eddiecarpenter/gh-agentic.git
`
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}
	// .ai may or may not exist — submodules can be uninitialised. We test
	// the detector is keying off the .gitmodules entry, not the directory.

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateSubmodule {
		t.Errorf("expected MountStateSubmodule, got %d", state)
	}
}

func TestDetectMountState_GitignoredMount(t *testing.T) {
	root := t.TempDir()

	// Create .ai/ as a regular directory (not a symlink).
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("creating .ai: %v", err)
	}
	// Add .ai/ to .gitignore.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.ai/\nvendor/\n"), 0o644); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateGitignoredMount {
		t.Errorf("expected MountStateGitignoredMount, got %d", state)
	}
}

func TestDetectMountState_EmptyAIClassifiedAsNone(t *testing.T) {
	// An empty .ai/ directory (left over from a failed install attempt)
	// is recoverable — DetectMountState classifies it as MountStateNone
	// so the install path can run with its defensive cleanup.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("creating .ai: %v", err)
	}

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateNone {
		t.Errorf("expected MountStateNone for empty .ai/, got %d", state)
	}
}

func TestDetectMountState_AbortedCloneClassifiedAsNone(t *testing.T) {
	// .ai/ with only a .git/ directory inside (the partial-clone state
	// after `git submodule add` failed at checkout) is also recoverable.
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".ai", ".git"), 0o755)

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateNone {
		t.Errorf("expected MountStateNone for aborted-clone .ai/, got %d", state)
	}
}

func TestDetectMountState_PopulatedAIClassifiedAsInconsistent(t *testing.T) {
	// .ai/ with real content but no submodule entry and no gitignore
	// entry is genuinely inconsistent — refuse rather than wipe.
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".ai", "user-file.md"), []byte("important"), 0o644)

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateInconsistent {
		t.Errorf("expected MountStateInconsistent, got %d", state)
	}
}

func TestDetectMountState_SubmoduleTakesPrecedenceOverGitignore(t *testing.T) {
	// If both a submodule entry and a gitignore entry exist (transitional
	// state during migration), the submodule classification wins — the
	// gitlink is the durable record.
	root := t.TempDir()

	gitmodules := `[submodule ".ai"]
	path = .ai
	url = https://github.com/eddiecarpenter/gh-agentic.git
`
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"), []byte(gitmodules), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ai/\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)

	state, err := DetectMountState(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != MountStateSubmodule {
		t.Errorf("expected MountStateSubmodule (precedence), got %d", state)
	}
}

func TestDownloadFramework_RefusesSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(".", filepath.Join(root, ".ai")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	err := DownloadFramework(root, "v2.0.0", nil)
	if err == nil {
		t.Fatal("expected refusal on symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

// Inconsistent-state refusal is now exercised by
// TestDownloadFramework_RefusesInconsistentExistingAI in mount_test.go,
// which seeds .ai/ with user content. An empty .ai/ is recoverable and
// no longer triggers a refusal — see TestDetectMountState_EmptyAIClassifiedAsNone.

func TestDownloadFramework_DispatchesToInstallOnFreshState(t *testing.T) {
	root := t.TempDir()

	called := false
	var capturedRoot, capturedTag string
	original := InstallSubmodule
	InstallSubmodule = func(r, t string) error {
		called = true
		capturedRoot = r
		capturedTag = t
		return nil
	}
	defer func() { InstallSubmodule = original }()

	if err := DownloadFramework(root, "v2.0.0", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected InstallSubmodule to be invoked for fresh state")
	}
	if capturedRoot != root || capturedTag != "v2.0.0" {
		t.Errorf("InstallSubmodule called with (%q, %q), want (%q, %q)", capturedRoot, capturedTag, root, "v2.0.0")
	}
}

func TestDownloadFramework_DispatchesToSwapOnSubmoduleState(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule ".ai"]`+"\n\turl = https://github.com/eddiecarpenter/gh-agentic.git\n"),
		0o644)

	called := false
	original := SwapSubmodule
	SwapSubmodule = func(r, t string) error { called = true; return nil }
	defer func() { SwapSubmodule = original }()

	if err := DownloadFramework(root, "v2.1.0", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected SwapSubmodule to be invoked for submodule state")
	}
}

func TestDownloadFramework_DispatchesToMigrateOnGitignoredState(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ai/\n"), 0o644)

	called := false
	original := MigrateGitignoredMount
	MigrateGitignoredMount = func(r, t string) error { called = true; return nil }
	defer func() { MigrateGitignoredMount = original }()

	if err := DownloadFramework(root, "v2.1.0", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected MigrateGitignoredMount to be invoked for gitignored state")
	}
}

func TestRemoveFromGitignore_RemovesMatchingLine(t *testing.T) {
	root := t.TempDir()
	gitignore := "node_modules/\n.ai/\nvendor/\n"
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644)

	if err := removeFromGitignore(root, ".ai/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	want := "node_modules/\nvendor/\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveFromGitignore_PreservesUnrelatedLines(t *testing.T) {
	root := t.TempDir()
	gitignore := "# comment\nnode_modules/\n  .ai/  \nvendor/\n.env\n"
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644)

	if err := removeFromGitignore(root, ".ai/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	want := "# comment\nnode_modules/\nvendor/\n.env\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveFromGitignore_NoOpWhenEntryAbsent(t *testing.T) {
	root := t.TempDir()
	gitignore := "node_modules/\nvendor/\n"
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644)

	if err := removeFromGitignore(root, ".ai/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(got) != gitignore {
		t.Errorf(".gitignore changed when entry was absent: got %q, want %q", got, gitignore)
	}
}

func TestRemoveFromGitignore_NoOpWhenFileMissing(t *testing.T) {
	root := t.TempDir()
	if err := removeFromGitignore(root, ".ai/"); err != nil {
		t.Errorf("unexpected error when .gitignore is missing: %v", err)
	}
}

func TestMigrateGitignoredMount_RemovesDirGitignoreEntryAndDelegatesToInstall(t *testing.T) {
	// Exercises the migration sequence end-to-end with stubs: the legacy
	// .ai/ directory is removed, the .ai/ line is stripped from
	// .gitignore (preserving every other line), and InstallSubmodule is
	// called with the requested tag. The git-side effects (`git add
	// .gitignore`, the submodule add itself) are stubbed because they
	// need a real repo; this test owns the higher-level orchestration.
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".ai", "stale.txt"), []byte("stale"), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.ai/\nvendor/\n"), 0o644)

	installCalled := false
	var installedTag string
	originalInstall := InstallSubmodule
	InstallSubmodule = func(_, tag string) error {
		installCalled = true
		installedTag = tag
		return nil
	}
	defer func() { InstallSubmodule = originalInstall }()

	// Stub out the `git add .gitignore` runGit invocation by initialising
	// a real (empty) git repo so the command succeeds. The test still
	// asserts orchestration, not git's behaviour.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("seed .git: %v", err)
	}
	// Use a real `git init` to make the directory a valid repo so the
	// `git add` step inside MigrateGitignoredMount succeeds.
	_ = os.RemoveAll(filepath.Join(root, ".git"))
	if err := runGit(root, "init", "--quiet"); err != nil {
		t.Skipf("git not available for this test: %v", err)
	}

	if err := MigrateGitignoredMount(root, "v2.5.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".ai")); err == nil {
		t.Error("expected legacy .ai/ to be removed")
	}

	gi, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Contains(string(gi), ".ai/") {
		t.Errorf("expected .ai/ line removed from .gitignore, got %q", gi)
	}
	if !strings.Contains(string(gi), "node_modules/") || !strings.Contains(string(gi), "vendor/") {
		t.Errorf("expected unrelated lines preserved, got %q", gi)
	}

	if !installCalled {
		t.Fatal("expected InstallSubmodule to be called after migration prep")
	}
	if installedTag != "v2.5.0" {
		t.Errorf("InstallSubmodule called with tag %q, want %q", installedTag, "v2.5.0")
	}
}

func TestGitmodulesHasAI_True(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule ".ai"]`+"\n\tpath = .ai\n"), 0o644)

	got, err := gitmodulesHasAI(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true when .gitmodules has .ai entry")
	}
}

func TestGitmodulesHasAI_FalseWhenMissing(t *testing.T) {
	root := t.TempDir()
	got, err := gitmodulesHasAI(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false when .gitmodules does not exist")
	}
}

func TestGitmodulesHasAI_FalseWhenOtherSubmodule(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule "vendor/foo"]`+"\n\tpath = vendor/foo\n"), 0o644)

	got, err := gitmodulesHasAI(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false when .gitmodules has only unrelated submodules")
	}
}
