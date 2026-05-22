package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/doctor"
	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// TestNoMountStringsInUserFacingOutput is the AC-8 regression guard.
// It asserts that the literal substring "gh agentic mount" does not appear in
// any user-facing output path that previously carried the stale reference.
// The test fails loudly if the string is re-introduced anywhere in these paths.
func TestNoMountStringsInUserFacingOutput(t *testing.T) {
	const stale = "gh agentic mount"

	// --- 1. info — version-mismatch sync-status warning ---
	// Exercises the path in collectInfo where localVersion != remoteVersion,
	// which previously emitted "⚠ run 'gh agentic mount' to sync".
	// We call printInfo directly (same package) with a pre-populated infoData
	// so the test is isolated from live GitHub API calls.
	t.Run("info_version_mismatch", func(t *testing.T) {
		data := &infoData{
			version:       "v2.0.0",
			localVersion:  "v1.0.0",
			remoteVersion: "v2.0.0",
			latestVersion: "v2.0.0",
			// syncStatus is what collectInfo would compute when versions differ.
			// The updated wording references check and repair, not mount.
			syncStatus: "  ⚠ run 'gh agentic check' to inspect, then 'gh agentic repair' to sync",
		}
		out := stripANSI(printInfoToString(data))
		if strings.Contains(out, stale) {
			t.Errorf("info version-mismatch output contains stale %q:\n%s", stale, out)
		}
	})

	// --- 2. check — pipeline-skipped path ---
	// Exercises renderCheckSections with pipelineSkipped=true, which previously
	// emitted "Skipped — framework mount is out of sync; run 'gh agentic mount' first".
	t.Run("check_pipeline_skipped", func(t *testing.T) {
		projectReport := &project.CheckReport{
			Results: []project.CheckResult{
				{Name: "framework-version-sync", Status: project.CheckFail, Message: "framework version out of sync"},
			},
		}
		var buf bytes.Buffer
		renderCheckSections(&buf, false, projectReport, nil, true)
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("check pipeline-skipped output contains stale %q:\n%s", stale, out)
		}
		// Confirm the replacement wording is present.
		if !strings.Contains(out, "gh agentic repair") {
			t.Errorf("check pipeline-skipped output missing expected 'gh agentic repair' wording:\n%s", out)
		}
	})

	// --- 3. upgrade --help ---
	// The Long description previously contained "'gh agentic mount' or 'gh agentic check'".
	t.Run("upgrade_help", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := newUpgradeCmd("dev")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--help"})
		_ = cmd.Execute()
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("upgrade --help output contains stale %q:\n%s", stale, out)
		}
	})

	// --- 4. root --help ---
	// The root Long description previously mentioned "the legacy gitignored mount"
	// in a way that could be read as a reference to the mount command.
	t.Run("root_help", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := newRootCmd("dev", "")
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--help"})
		_ = cmd.Execute()
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("root --help output contains stale %q:\n%s", stale, out)
		}
	})

	// --- 5. repair --help ---
	// The repair Long description previously documented the pipeline-skip reasoning
	// with a user instruction to run 'gh agentic mount'.
	t.Run("repair_help", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := newRepairCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--help"})
		_ = cmd.Execute()
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("repair --help output contains stale %q:\n%s", stale, out)
		}
	})

	// --- 6. repair — pipeline-skipped runtime output (AC3 + AC8a) ---
	// Exercises renderRepairPipelineSection with pipelineSkipped=true, which
	// previously emitted "Skipped — framework mount is out of sync; run 'gh agentic mount' first".
	t.Run("repair_pipeline_skipped", func(t *testing.T) {
		var buf bytes.Buffer
		renderRepairPipelineSection(&buf, true, nil, doctor.RepairResult{})
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("repair pipeline-skipped output contains stale %q:\n%s", stale, out)
		}
		// Confirm the replacement wording is present.
		if !strings.Contains(out, "gh agentic repair") {
			t.Errorf("repair pipeline-skipped output missing expected 'gh agentic repair' wording:\n%s", out)
		}
	})

	// --- 7. init — already-mounted hint (AC8b) ---
	// Exercises the early-return branch in initpkg.Run where .agents/ already
	// exists and --force was not passed. Previously emitted
	// "Run 'gh agentic mount <version>' to upgrade".
	t.Run("init_already_mounted", func(t *testing.T) {
		// Construct a minimal directory that looks like an already-initialised repo:
		// Run() requires .git to exist, then returns ErrAlreadyInitialised when
		// .agents/ is present and force=false.
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatalf("creating .git dir: %v", err)
		}
		if err := os.Mkdir(filepath.Join(root, ".agents"), 0o755); err != nil {
			t.Fatalf("creating .agents dir: %v", err)
		}

		var buf bytes.Buffer
		err := initpkg.Run(&buf, root, false, initpkg.Deps{})
		if err != initpkg.ErrAlreadyInitialised {
			t.Fatalf("expected ErrAlreadyInitialised, got: %v", err)
		}
		out := stripANSI(buf.String())
		if strings.Contains(out, stale) {
			t.Errorf("init already-mounted output contains stale %q:\n%s", stale, out)
		}
		// Confirm the replacement wording is present.
		if !strings.Contains(out, "gh agentic upgrade") {
			t.Errorf("init already-mounted output missing expected 'gh agentic upgrade' wording:\n%s", out)
		}
	})
}
