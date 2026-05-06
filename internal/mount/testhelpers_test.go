package mount

import (
	"os"
	"path/filepath"
	"testing"
)

// installSubmoduleStub returns a stub for the InstallSubmodule var that
// fakes the on-disk side-effects of a real `git submodule add`:
//
//   - creates root/.agents/ with the supplied files
//   - creates a `.git` marker inside .agents/ so callers (e.g. the doctor's
//     ReadAIVersionFromGit) see a populated submodule
//   - appends a [submodule ".agents"] entry to root/.gitmodules
//
// This lets tests exercise the real DownloadFramework dispatch
// (DetectMountState → InstallSubmodule) without requiring a network or
// a real git repo with a remote.
func installSubmoduleStub(files map[string]string) func(root, tag string) error {
	return func(root, tag string) error {
		aiDir := filepath.Join(root, ".agents")
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
		// Mark .ai as a git checkout so DetectMountState sees the
		// submodule on subsequent calls.
		if err := os.MkdirAll(filepath.Join(aiDir, ".git"), 0o755); err != nil {
			return err
		}
		// Append a .gitmodules entry so DetectMountState classifies
		// future calls as MountStateSubmodule.
		gm := filepath.Join(root, ".gitmodules")
		entry := `[submodule ".agents"]
	path = .agents
	url = ` + FrameworkRepoURL + "\n"
		f, err := os.OpenFile(gm, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(entry)
		return err
	}
}

// swapSubmoduleStub returns a stub for SwapSubmodule that simulates
// the deinit + re-add: it wipes .agents/, then runs an install stub. The
// .gitmodules entry stays (no need to rewrite it for tests) and the
// gitlink "moves" by virtue of .agents/ being repopulated with the new
// files.
func swapSubmoduleStub(files map[string]string) func(root, tag string) error {
	return func(root, tag string) error {
		_ = os.RemoveAll(filepath.Join(root, ".agents"))
		// Same shape as install but skip re-adding to .gitmodules.
		aiDir := filepath.Join(root, ".agents")
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
		return os.MkdirAll(filepath.Join(aiDir, ".git"), 0o755)
	}
}

// withStubInstall swaps the package-level InstallSubmodule for the
// duration of the test, restoring the original on cleanup. Use when a
// test exercises DownloadFramework or RunFirstTime in the fresh-install
// state.
func withStubInstall(t *testing.T, files map[string]string) {
	t.Helper()
	original := InstallSubmodule
	InstallSubmodule = installSubmoduleStub(files)
	t.Cleanup(func() { InstallSubmodule = original })
}

// withStubSwap swaps the package-level SwapSubmodule for the duration
// of the test. Use when a test exercises DownloadFramework or RunSwitch
// against a pre-existing submodule state.
func withStubSwap(t *testing.T, files map[string]string) {
	t.Helper()
	original := SwapSubmodule
	SwapSubmodule = swapSubmoduleStub(files)
	t.Cleanup(func() { SwapSubmodule = original })
}

// withStubInstallError swaps InstallSubmodule with a stub that returns
// the given error. Use to simulate `git submodule add` failure
// (network error, bad tag, etc.) in tests.
func withStubInstallError(t *testing.T, errMsg string) {
	t.Helper()
	original := InstallSubmodule
	InstallSubmodule = func(root, tag string) error { return testStubError(errMsg) }
	t.Cleanup(func() { InstallSubmodule = original })
}

// testStubError is a tiny helper to produce a comparable error from a
// stub. Keeping it as a value (not a fmt.Errorf wrap) keeps assertions
// straightforward (`strings.Contains(err.Error(), errMsg)`).
type testStubError string

func (e testStubError) Error() string { return string(e) }
