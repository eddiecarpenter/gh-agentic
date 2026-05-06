package project

import (
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount/mounttest"
)

// fakeFrameworkFiles is the canonical set of files the project tests
// expect to see inside .agents/ after a successful install. Used by the
// mounttest stubs below.
var fakeFrameworkFiles = map[string]string{
	"RULEBOOK.md":            "# Rules",
	"skills/session-init.md": "# Session Init",
}

// withFakeInstall swaps mount.InstallSubmodule with a stub for the
// duration of the test. Use in any project test whose code path
// reaches mount.RunFirstTime / DownloadFramework against a fresh
// (no-.ai/, no-.gitmodules) state.
func withFakeInstall(t *testing.T) {
	t.Helper()
	mounttest.StubInstall(t, fakeFrameworkFiles)
}

// withFakeSwap swaps mount.SwapSubmodule with a stub. Use in tests
// that exercise mount.RunSwitch / DownloadFramework against a
// pre-existing submodule state.
func withFakeSwap(t *testing.T) {
	t.Helper()
	mounttest.StubSwap(t, fakeFrameworkFiles)
}
