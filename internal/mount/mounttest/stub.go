// Package mounttest provides test-only helpers that swap the
// internal/mount package's submodule operations with stubs that fake
// the on-disk state. Production code does not depend on this package.
//
// Use from any test that exercises a code path leading to
// mount.DownloadFramework / RunFirstTime / RunSwitch — call
// StubInstall (and optionally StubSwap) at the top of the test, and
// the cleanup is registered automatically via t.Cleanup.
package mounttest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// StubInstall replaces mount.InstallSubmodule for the duration of the
// test with a stub that creates the supplied files inside <root>/.ai/
// and writes a [submodule ".ai"] entry to <root>/.gitmodules. The
// stub mirrors the on-disk state a real `git submodule add` would
// produce, so subsequent calls to mount.DetectMountState classify the
// directory as MountStateSubmodule.
func StubInstall(t *testing.T, files map[string]string) {
	t.Helper()
	original := mount.InstallSubmodule
	mount.InstallSubmodule = makeStub(files, true)
	t.Cleanup(func() { mount.InstallSubmodule = original })
}

// StubSwap replaces mount.SwapSubmodule for the duration of the test
// with a stub that wipes <root>/.ai/ then repopulates it with the
// supplied files (no .gitmodules write — the parent test scenario is
// expected to have already seeded the submodule entry).
func StubSwap(t *testing.T, files map[string]string) {
	t.Helper()
	original := mount.SwapSubmodule
	mount.SwapSubmodule = makeStub(files, false)
	t.Cleanup(func() { mount.SwapSubmodule = original })
}

// StubInstallError replaces mount.InstallSubmodule with a stub that
// always returns an error containing the supplied message. Use to
// simulate a `git submodule add` failure without engaging real git.
func StubInstallError(t *testing.T, msg string) {
	t.Helper()
	original := mount.InstallSubmodule
	mount.InstallSubmodule = func(root, tag string) error { return stubError(msg) }
	t.Cleanup(func() { mount.InstallSubmodule = original })
}

// StubMigrate replaces mount.MigrateGitignoredMount with a no-op stub
// that just records the invocation. Use sparingly — most tests don't
// need to exercise the migration path explicitly.
func StubMigrate(t *testing.T, files map[string]string) {
	t.Helper()
	original := mount.MigrateGitignoredMount
	mount.MigrateGitignoredMount = func(root, tag string) error {
		_ = os.RemoveAll(filepath.Join(root, ".ai"))
		stub := makeStub(files, true)
		return stub(root, tag)
	}
	t.Cleanup(func() { mount.MigrateGitignoredMount = original })
}

// makeStub returns a function that fakes a submodule operation by
// creating the .ai/ directory, writing the supplied files, and
// optionally appending a [submodule ".ai"] entry to .gitmodules.
func makeStub(files map[string]string, writeGitmodules bool) func(root, tag string) error {
	return func(root, tag string) error {
		aiDir := filepath.Join(root, ".ai")
		if err := os.RemoveAll(aiDir); err != nil {
			return err
		}
		if err := os.MkdirAll(aiDir, 0o755); err != nil {
			return err
		}
		for path, content := range files {
			full := filepath.Join(aiDir, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
		// Mark .ai as a git checkout so DetectMountState sees a
		// populated submodule on subsequent calls.
		if err := os.MkdirAll(filepath.Join(aiDir, ".git"), 0o755); err != nil {
			return err
		}
		if !writeGitmodules {
			return nil
		}
		gm := filepath.Join(root, ".gitmodules")
		entry := `[submodule ".ai"]
	path = .ai
	url = ` + mount.FrameworkRepoURL + "\n"
		f, err := os.OpenFile(gm, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(entry)
		return err
	}
}

// stubError is a small comparable error type used by StubInstallError
// so tests can assert on substring matches via err.Error().
type stubError string

func (e stubError) Error() string { return string(e) }
