package mount

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the real installSubmoduleViaGit /
// swapSubmoduleViaGit / migrateGitignoredMountViaGit code paths
// against a local bare-repo fixture (no network). They run in every
// test invocation — git is a hard dependency of the package, so
// "git missing" would already break the package's primary callers.
// If git is somehow unavailable, individual tests skip rather than
// fail.
//
// The fixture pattern: build a small bare repo with two tags
// (v0.0.1 and v0.0.2) at $TMPDIR/fake-framework.git, point
// FrameworkRepoURL at a file:// URL for that bare, then drive the
// install/swap/migrate primitives against fresh "consumer" repos.

// fixtureFramework builds a bare git repo with two tagged commits and
// returns its file:// URL. Cleanup is registered via t.Cleanup. The
// fixture is rebuilt per test to keep tests independent.
func fixtureFramework(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Working dir to seed the framework content into.
	work := t.TempDir()
	mustGit(t, work, "init", "--quiet", "-b", "main")
	mustGit(t, work, "config", "user.email", "test@example.com")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")

	// v0.0.1 content.
	if err := os.WriteFile(filepath.Join(work, "RULEBOOK.md"), []byte("# v0.0.1 rules"), 0o644); err != nil {
		t.Fatalf("write RULEBOOK.md: %v", err)
	}
	mustGit(t, work, "add", "RULEBOOK.md")
	mustGit(t, work, "commit", "--quiet", "-m", "v0.0.1")
	mustGit(t, work, "tag", "v0.0.1")

	// v0.0.2 content.
	if err := os.WriteFile(filepath.Join(work, "RULEBOOK.md"), []byte("# v0.0.2 rules"), 0o644); err != nil {
		t.Fatalf("write RULEBOOK.md: %v", err)
	}
	mustGit(t, work, "add", "RULEBOOK.md")
	mustGit(t, work, "commit", "--quiet", "-m", "v0.0.2")
	mustGit(t, work, "tag", "v0.0.2")

	// Clone the working repo into a bare so we can use it as a remote.
	bare := filepath.Join(t.TempDir(), "fake-framework.git")
	if out, err := exec.Command("git", "clone", "--bare", "--quiet", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone --bare: %v\n%s", err, out)
	}

	return "file://" + bare
}

// consumerRepo creates a fresh git repo and returns its working-tree
// root. Used as the "domain repo" target for install / swap /
// migrate operations.
func consumerRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustGit(t, root, "init", "--quiet", "-b", "main")
	mustGit(t, root, "config", "user.email", "test@example.com")
	mustGit(t, root, "config", "user.name", "Test")
	mustGit(t, root, "config", "commit.gpgsign", "false")
	// Allow file:// URLs for submodule add — newer git refuses them
	// by default for security (CVE-2022-39253).
	mustGit(t, root, "config", "protocol.file.allow", "always")
	// Seed an initial commit so the worktree has a HEAD to anchor
	// the submodule add against.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# consumer"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustGit(t, root, "add", "README.md")
	mustGit(t, root, "commit", "--quiet", "-m", "initial")
	return root
}

// withFrameworkRepoURL temporarily swaps FrameworkRepoURL for the test
// and restores on cleanup. It also injects the `protocol.file.allow=always`
// git config via environment variables so `git submodule add` against a
// `file://` URL works — submodule operations spawn child gits that don't
// inherit per-repo config, so per-call env-var injection is the only
// reliable knob (CVE-2022-39253 default-deny mitigation).
func withFrameworkRepoURL(t *testing.T, url string) {
	t.Helper()
	original := FrameworkRepoURL
	FrameworkRepoURL = url
	t.Cleanup(func() { FrameworkRepoURL = original })

	// `GIT_CONFIG_COUNT/KEY/VALUE` lets us layer a one-off config entry
	// onto every git invocation in this process and its children, with
	// no global-config pollution.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
}

// mustGit runs git in dir and fatals on error. Used for fixture
// scaffolding only; production code paths use the package's runGit.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, strings.TrimSpace(string(out)))
	}
}

// TestInstallSubmoduleViaGit_FreshInstall exercises the full real-git
// install path: `submodule add` against a file:// remote, fetch tags,
// checkout the requested tag, stage the gitlink. Verifies the post
// state matches the production contract: .gitmodules has the .ai
// entry, the gitlink is staged, and `.ai` contents reflect the tag.
func TestInstallSubmoduleViaGit_FreshInstall(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	if err := installSubmoduleViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("installSubmoduleViaGit: %v", err)
	}

	// .gitmodules must contain the .ai entry pointing at the fixture.
	gm, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		t.Fatalf("read .gitmodules: %v", err)
	}
	if !strings.Contains(string(gm), `[submodule ".ai"]`) {
		t.Errorf(".gitmodules missing [submodule \".ai\"] entry: %s", gm)
	}
	if !strings.Contains(string(gm), fwURL) {
		t.Errorf(".gitmodules missing fixture URL: %s", gm)
	}

	// .ai/ must be populated with v0.0.1 content.
	rb, err := os.ReadFile(filepath.Join(root, ".ai", "RULEBOOK.md"))
	if err != nil {
		t.Fatalf("read .ai/RULEBOOK.md: %v", err)
	}
	if !strings.Contains(string(rb), "v0.0.1") {
		t.Errorf(".ai/RULEBOOK.md content does not match v0.0.1: %q", rb)
	}

	// The gitlink must be staged for commit.
	out, err := exec.Command("git", "-C", root, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("git diff --cached: %v", err)
	}
	staged := string(out)
	if !strings.Contains(staged, ".gitmodules") {
		t.Errorf("expected .gitmodules to be staged: %s", staged)
	}
	if !strings.Contains(staged, ".ai") {
		t.Errorf("expected .ai gitlink to be staged: %s", staged)
	}
}

// TestInstallSubmoduleViaGit_RecoversFromOrphanModuleDir verifies the
// defensive cleanup that fixed v2.5.5's "A git directory for '.ai'
// is found locally" error: when `.git/modules/.ai/` is left over from
// a previous failed run, install should still succeed.
func TestInstallSubmoduleViaGit_RecoversFromOrphanModuleDir(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	// Seed a stale .git/modules/.ai/ directory (simulating an aborted
	// previous install). The defensive cleanup in installSubmoduleViaGit
	// must remove it before `git submodule add` runs.
	gitDir, err := resolveGitDir(root)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "modules", ".ai", "leftover"), 0o755); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}

	if err := installSubmoduleViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("installSubmoduleViaGit with orphan: %v", err)
	}

	// The orphan leftover/ subdir must be gone.
	if _, err := os.Stat(filepath.Join(gitDir, "modules", ".ai", "leftover")); err == nil {
		t.Error("expected orphan leftover/ to be cleaned up")
	}
}

// TestInstallSubmoduleViaGit_RecoversFromOrphanAIDir verifies the
// defensive cleanup of a leftover `.ai/` directory in the working
// tree (clone succeeded but checkout failed in a previous run).
func TestInstallSubmoduleViaGit_RecoversFromOrphanAIDir(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	// Seed a stale .ai/ directory.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("seed orphan .ai/: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai", "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale.txt: %v", err)
	}

	if err := installSubmoduleViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("installSubmoduleViaGit with orphan .ai/: %v", err)
	}

	// stale.txt must be gone (defensive RemoveAll wipes the dir).
	if _, err := os.Stat(filepath.Join(root, ".ai", "stale.txt")); err == nil {
		t.Error("expected stale.txt to be cleaned up")
	}
	// The new content must be in place.
	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); err != nil {
		t.Errorf(".ai/RULEBOOK.md should exist after install: %v", err)
	}
}

// TestInstallSubmoduleViaGit_EmptyTagRefuses guards the early-exit
// for an empty tag. Pre-flight check, no git operations should run.
func TestInstallSubmoduleViaGit_EmptyTagRefuses(t *testing.T) {
	root := consumerRepo(t)
	err := installSubmoduleViaGit(root, "")
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
	if !strings.Contains(err.Error(), "tag is empty") {
		t.Errorf("error should mention empty tag: %v", err)
	}
}

// TestSwapSubmoduleViaGit_VersionSwap exercises the real deinit/rm/
// re-add dance: install at v0.0.1, swap to v0.0.2, verify the
// resulting gitlink and content.
func TestSwapSubmoduleViaGit_VersionSwap(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	if err := installSubmoduleViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	// Commit so the swap operates on a clean state — git rm refuses
	// when the gitlink has uncommitted changes.
	mustGit(t, root, "commit", "--quiet", "-m", "install v0.0.1")

	if err := swapSubmoduleViaGit(root, "v0.0.2"); err != nil {
		t.Fatalf("swap to v0.0.2: %v", err)
	}

	rb, err := os.ReadFile(filepath.Join(root, ".ai", "RULEBOOK.md"))
	if err != nil {
		t.Fatalf("read .ai/RULEBOOK.md after swap: %v", err)
	}
	if !strings.Contains(string(rb), "v0.0.2") {
		t.Errorf("post-swap content should be v0.0.2, got: %q", rb)
	}

	// .git/modules/.ai must exist (the new install) but should be
	// the freshly-rebuilt one, not the old one. Existence is the
	// observable: a successful swap means submodule add succeeded
	// despite the previous module dir.
	gitDir, _ := resolveGitDir(root)
	if _, err := os.Stat(filepath.Join(gitDir, "modules", ".ai")); err != nil {
		t.Errorf(".git/modules/.ai/ should exist after swap: %v", err)
	}
}

// TestSwapSubmoduleViaGit_EmptyTagRefuses guards the early-exit.
func TestSwapSubmoduleViaGit_EmptyTagRefuses(t *testing.T) {
	root := consumerRepo(t)
	err := swapSubmoduleViaGit(root, "")
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
	if !strings.Contains(err.Error(), "tag is empty") {
		t.Errorf("error should mention empty tag: %v", err)
	}
}

// TestMigrateGitignoredMountViaGit_LegacyState exercises the full
// migration path: a pre-existing gitignored .ai/ + .ai/ entry in
// .gitignore is converted to a tracked submodule.
func TestMigrateGitignoredMountViaGit_LegacyState(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	// Seed the legacy state: .ai/ directory with content + .gitignore
	// entry.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("seed legacy .ai/: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai", "old.txt"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("seed legacy content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("node_modules/\n.ai/\nvendor/\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	if err := migrateGitignoredMountViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("migrateGitignoredMountViaGit: %v", err)
	}

	// .ai/ should now contain v0.0.1 framework content (legacy old.txt gone).
	if _, err := os.Stat(filepath.Join(root, ".ai", "old.txt")); err == nil {
		t.Error("legacy .ai/old.txt should have been removed")
	}
	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); err != nil {
		t.Errorf(".ai/RULEBOOK.md should exist after migration: %v", err)
	}

	// .gitignore should no longer contain .ai/, but should preserve
	// other entries.
	gi, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Contains(string(gi), ".ai/") {
		t.Errorf(".ai/ should be removed from .gitignore: %s", gi)
	}
	if !strings.Contains(string(gi), "node_modules/") || !strings.Contains(string(gi), "vendor/") {
		t.Errorf("unrelated .gitignore lines should be preserved: %s", gi)
	}

	// .gitmodules should exist with the .ai entry.
	gm, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		t.Fatalf("read .gitmodules: %v", err)
	}
	if !strings.Contains(string(gm), `[submodule ".ai"]`) {
		t.Errorf(".gitmodules missing .ai entry: %s", gm)
	}
}

// TestResolveGitDir_RegularRepo verifies the worktree-aware git-dir
// resolver handles the regular case (.git is a directory).
func TestResolveGitDir_RegularRepo(t *testing.T) {
	root := consumerRepo(t)
	gitDir, err := resolveGitDir(root)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	expected := filepath.Join(root, ".git")
	if gitDir != expected {
		t.Errorf("got %q, want %q", gitDir, expected)
	}
}

// TestResolveGitDir_NonRepo returns an error when called outside a
// git working tree.
func TestResolveGitDir_NonRepo(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveGitDir(root); err == nil {
		t.Error("expected error in non-git directory")
	}
}

// TestReadAIVersionFromGit_TagPresent verifies the version reader
// against a real installed submodule. Uses the same fixture pattern
// as the install tests: create a fake framework with v0.0.1 and
// v0.0.2 tags, install the submodule at v0.0.1, then read it back.
func TestReadAIVersionFromGit_TagPresent(t *testing.T) {
	fwURL := fixtureFramework(t)
	withFrameworkRepoURL(t, fwURL)
	root := consumerRepo(t)

	if err := installSubmoduleViaGit(root, "v0.0.1"); err != nil {
		t.Fatalf("install: %v", err)
	}

	v, err := ReadAIVersionFromGit(root)
	if err != nil {
		t.Fatalf("ReadAIVersionFromGit: %v", err)
	}
	if v != "v0.0.1" {
		t.Errorf("got version %q, want v0.0.1", v)
	}
}

// TestReadAIVersionFromGit_NoMount returns an error when there is no
// .ai/ git checkout to read from.
func TestReadAIVersionFromGit_NoMount(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadAIVersionFromGit(root); err == nil {
		t.Error("expected error when .ai/ is missing")
	}
}

// TestRemoveAIFromGitignore_PublicWrapper verifies the exported
// helper that the doctor's repair pass invokes. Logic is delegated to
// removeFromGitignore (whose own tests live in submodule_test.go);
// this is a thin assertion that the public API plumbs through correctly.
func TestRemoveAIFromGitignore_PublicWrapper(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("node_modules/\n.ai/\nvendor/\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}
	if err := RemoveAIFromGitignore(root); err != nil {
		t.Fatalf("RemoveAIFromGitignore: %v", err)
	}
	gi, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Contains(string(gi), ".ai/") {
		t.Errorf("expected .ai/ removed from .gitignore: %s", gi)
	}
}

// TestDefaultClone_NoOp documents that DefaultClone is now a no-op
// retained only for source compatibility with the Clone field on
// Deps structs. Production no longer consults the value.
func TestDefaultClone_NoOp(t *testing.T) {
	if err := DefaultClone("https://example.com/repo.git", "v1.0.0", "/tmp/anywhere"); err != nil {
		t.Errorf("DefaultClone should be a no-op, got error: %v", err)
	}
}
