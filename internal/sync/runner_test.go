package sync

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// noopClear is a ClearFunc that does nothing. Used in tests to avoid
// writing ANSI sequences to test buffers.
func noopClear(_ io.Writer) {}

// fakeReleases returns a FetchReleasesFunc that returns the given releases.
// The returned function ignores the repo argument.
func fakeReleases(releases []Release, err error) FetchReleasesFunc {
	return func(_ string) ([]Release, error) {
		return releases, err
	}
}

// singleRelease returns a FetchReleasesFunc that returns a single release
// with the given tag. Useful for tests that only need one version.
func singleRelease(tag string) FetchReleasesFunc {
	return fakeReleases([]Release{
		{TagName: tag, Name: "Release " + tag, Body: "Release notes for " + tag, TarballURL: "https://api.github.com/repos/owner/repo/tarball/" + tag},
	}, nil)
}

// tarballFetchSetup overrides the package-level fetchTarballFn with a fake that
// returns a gzipped tar containing the given files. The file map keys use the
// template repo layout (e.g. "base/AGENTS.md", "base/.github/workflows/ci.yml",
// ".goose/recipes/dev.yaml"). Returns a cleanup function that restores the
// original fetchTarballFn.
func tarballFetchSetup(t *testing.T, files map[string]string) func() {
	t.Helper()
	orig := fetchTarballFn
	fetchTarballFn = func(repo, version string) (io.ReadCloser, error) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		prefix := "repo-" + version + "/"
		_ = tw.WriteHeader(&tar.Header{
			Name: prefix, Typeflag: tar.TypeDir, Mode: 0o755,
		})

		// Track created directories to avoid duplicates.
		createdDirs := make(map[string]bool)

		for path, content := range files {
			fullPath := prefix + path

			// Create parent directories.
			parts := strings.Split(filepath.Dir(path), string(filepath.Separator))
			accumulated := prefix
			for _, p := range parts {
				if p == "." {
					continue
				}
				accumulated += p + "/"
				if !createdDirs[accumulated] {
					_ = tw.WriteHeader(&tar.Header{
						Name: accumulated, Typeflag: tar.TypeDir, Mode: 0o755,
					})
					createdDirs[accumulated] = true
				}
			}

			if err := tw.WriteHeader(&tar.Header{
				Name: fullPath, Size: int64(len(content)),
				Mode: 0o644, Typeflag: tar.TypeReg,
			}); err != nil {
				t.Fatalf("writing header for %s: %v", path, err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatalf("writing content for %s: %v", path, err)
			}
		}

		_ = tw.Close()
		_ = gw.Close()
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}
	return func() { fetchTarballFn = orig }
}

// tarballRunner returns a run function that uses a MockRunner for all commands.
// Unlike the old cloneRunner, there is no git clone interception — the tarball
// fetch is handled by the package-level fetchTarballFn.
func tarballRunner(mock *testutil.MockRunner) func(string, ...string) (string, error) {
	return mock.RunCommand
}

// defaultTarballSetup sets up fetchTarballFn with a tarball containing
// base/AGENTS.md with the given content. Returns a cleanup function.
func defaultTarballSetup(t *testing.T, baseContent string) func() {
	t.Helper()
	return tarballFetchSetup(t, map[string]string{
		"base/AGENTS.md": baseContent,
	})
}

// workflowTarballSetup sets up fetchTarballFn with a tarball containing
// base/AGENTS.md and workflow files under base/.github/workflows/.
func workflowTarballSetup(t *testing.T, baseContent string, workflows []string) func() {
	t.Helper()
	files := map[string]string{
		"base/AGENTS.md": baseContent,
	}
	for _, wf := range workflows {
		files["base/.github/workflows/"+wf] = "workflow: " + wf
	}
	return tarballFetchSetup(t, files)
}

// recipeTarballSetup sets up fetchTarballFn with a tarball containing
// base/AGENTS.md, optional workflows, and recipe files under .goose/recipes/.
func recipeTarballSetup(t *testing.T, baseContent string, workflows []string, recipes []string) func() {
	t.Helper()
	files := map[string]string{
		"base/AGENTS.md": baseContent,
	}
	for _, wf := range workflows {
		files["base/.github/workflows/"+wf] = "workflow: " + wf
	}
	for _, r := range recipes {
		files[".goose/recipes/"+r] = "recipe: " + r
	}
	return tarballFetchSetup(t, files)
}

// fakeDetectOwnerType returns a DetectOwnerTypeFunc that always returns the given type.
func fakeDetectOwnerType(ownerType string) bootstrap.DetectOwnerTypeFunc {
	return func(owner string) (string, error) {
		return ownerType, nil
	}
}

// fakeDetectOwnerTypeError returns a DetectOwnerTypeFunc that always returns an error.
func fakeDetectOwnerTypeError() bootstrap.DetectOwnerTypeFunc {
	return func(owner string) (string, error) {
		return "", fmt.Errorf("API error")
	}
}

func TestRunSync_UpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		singleRelease("v1.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "up to date") {
		t.Errorf("expected 'up to date' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ConfirmAndStageOnly(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("base/AGENTS.md = %q, want 'updated content'", data)
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got: %s", output)
	}
	if !strings.Contains(output, "git diff --cached") {
		t.Errorf("expected review instructions, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ConfirmAndCommit(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		true,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("base/AGENTS.md = %q, want 'updated content'", data)
	}

	output := buf.String()
	if !strings.Contains(output, "Sync committed") {
		t.Errorf("expected 'Sync committed' message, got: %s", output)
	}
	if !strings.Contains(output, "Remember to push") {
		t.Errorf("expected push reminder, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_DeclineThenAccept(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	confirmCalls := 0
	confirmFn := func(_ string) (bool, error) {
		confirmCalls++
		// First call declines, second accepts.
		return confirmCalls >= 2, nil
	}

	clearCalls := 0
	trackingClear := func(_ io.Writer) {
		clearCalls++
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		confirmFn,
		nil, // selectVersion
		trackingClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Confirm was called twice (decline + accept).
	if confirmCalls != 2 {
		t.Errorf("expected confirm called 2 times, got %d", confirmCalls)
	}

	// ClearScreen should be called 3 times:
	// 1. picker → notes on first iteration
	// 2. decline → back to picker
	// 3. picker → notes on second iteration
	// 4. install → results
	// Total: 4
	if clearCalls != 4 {
		t.Errorf("expected clearScreen called 4 times (notes+decline+notes+results), got %d", clearCalls)
	}

	// Install still completed — TEMPLATE_VERSION updated.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ClearScreenCount_NormalFlow(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	clearCalls := 0
	trackingClear := func(_ io.Writer) {
		clearCalls++
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		trackingClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Normal flow (accept on first try): clear at picker→notes + install→results = 2.
	if clearCalls != 2 {
		t.Errorf("expected clearScreen called 2 times (notes+results), got %d", clearCalls)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_DeclineThenAccept_MultipleReleases(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "selected content")()

	mock := &testutil.MockRunner{}

	confirmCalls := 0
	confirmFn := func(_ string) (bool, error) {
		confirmCalls++
		return confirmCalls >= 2, nil
	}

	selectCalls := 0
	fakeSelect := func(releases []Release) (Release, error) {
		selectCalls++
		// Always select the first release.
		return releases[0], nil
	}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
		{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		multiReleases,
		testutil.NoopSpinner,
		confirmFn,
		fakeSelect,
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SelectVersion should be called twice (once per loop iteration).
	if selectCalls != 2 {
		t.Errorf("expected selectVersion called 2 times, got %d", selectCalls)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_FetchError(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		fakeReleases(nil, fmt.Errorf("API error")),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("unexpected error: %v", err)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ForceResyncsWhenUpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "re-synced content")()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v1.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		true,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "force") && !strings.Contains(output, "re-sync") {
		t.Errorf("expected force/re-sync warning in output, got: %s", output)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "re-synced content" {
		t.Errorf("base/AGENTS.md = %q, want 're-synced content'", data)
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v1.0.0", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_BaseMissing_RestoresOnConfirm(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "restored content")()

	if err := os.RemoveAll(filepath.Join(repo.Root, "base")); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v1.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "missing") {
		t.Errorf("expected 'missing' warning in output, got: %s", output)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "restored content" {
		t.Errorf("base/AGENTS.md = %q, want 'restored content'", data)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_YesAutoConfirms(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "auto-confirmed content")()

	mock := &testutil.MockRunner{}

	confirmCalled := 0
	var confirmPrompt string
	confirmFn := func(prompt string) (bool, error) {
		confirmCalled++
		confirmPrompt = prompt
		return true, nil
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		confirmFn,
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if confirmCalled != 1 {
		t.Errorf("expected confirm called 1 time, got %d", confirmCalled)
	}

	// Verify the new prompt text matches the UX design.
	if confirmPrompt != "Install v2.0.0?" {
		t.Errorf("expected confirm prompt %q, got %q", "Install v2.0.0?", confirmPrompt)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_UserOwner_SkipsSyncStatusToLabel(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}
	defer workflowTarballSetup(t, "updated", workflows)()

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a pre-existing sync-status-to-label.yml to test retroactive removal.
	wfDir := filepath.Join(repo.Root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "sync-status-to-label.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		fakeDetectOwnerType(bootstrap.OwnerTypeUser),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml was NOT deployed and pre-existing one was removed.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); err == nil {
		t.Error("sync-status-to-label.yml should NOT exist for personal repos")
	}

	// Verify dev-session.yml WAS deployed.
	devPath := filepath.Join(repo.Root, ".github", "workflows", "dev-session.yml")
	if _, err := os.Stat(devPath); os.IsNotExist(err) {
		t.Error("dev-session.yml should be deployed")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_OrgOwner_IncludesSyncStatusToLabel(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}
	defer workflowTarballSetup(t, "updated", workflows)()

	// Write AGENTS.local.md with org owner info.
	agentsLocal := "## Repo\n\n- **Owner:** acme-org\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		fakeDetectOwnerType(bootstrap.OwnerTypeOrg),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed for org repos.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed for org repos")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_DetectOwnerTypeError_FallbackDeploysAll(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}
	defer workflowTarballSetup(t, "updated", workflows)()

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		fakeDetectOwnerTypeError(), // detection fails
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed (safe fallback).
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed when detection fails (safe fallback)")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_NilDetectOwnerType_DeploysAll(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}
	defer workflowTarballSetup(t, "updated", workflows)()

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,   // no detect function
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed when detectOwnerType is nil.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed when detectOwnerType is nil")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_MultipleReleases_CallsSelectFunc(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "selected content")()

	mock := &testutil.MockRunner{}

	selectCalled := false
	fakeSelect := func(releases []Release) (Release, error) {
		selectCalled = true
		if len(releases) < 2 {
			t.Fatalf("expected multiple releases, got %d", len(releases))
		}
		// Select the second release (not the newest).
		return releases[1], nil
	}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
		{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
		{TagName: "v1.1.0", Name: "Older", Body: "Older notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.1.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		multiReleases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		fakeSelect,
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !selectCalled {
		t.Error("expected select function to be called for multiple releases")
	}

	// Verify the selected version (v1.5.0) was used.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.5.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v1.5.0", string(data))
	}

	output := buf.String()
	if !strings.Contains(output, "releases available") {
		t.Errorf("expected 'releases available' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_SingleRelease_SkipsSelectFunc(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "single content")()

	mock := &testutil.MockRunner{}

	selectCalled := false
	fakeSelect := func(releases []Release) (Release, error) {
		selectCalled = true
		return releases[0], nil
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		fakeSelect,
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selectCalled {
		t.Error("select function should NOT be called for single release")
	}

	output := buf.String()
	if !strings.Contains(output, "Update available") {
		t.Errorf("expected 'Update available' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ListMode_DisplaysReleases(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
		{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		multiReleases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { t.Fatal("confirm should not be called in list mode"); return false, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		true, // list
		"",
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "v2.0.0") {
		t.Errorf("expected v2.0.0 in output, got: %s", output)
	}
	if !strings.Contains(output, "v1.5.0") {
		t.Errorf("expected v1.5.0 in output, got: %s", output)
	}
	if !strings.Contains(output, "Latest notes") {
		t.Errorf("expected release notes in output, got: %s", output)
	}

	// Verify no sync was performed — TEMPLATE_VERSION unchanged.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION changed during --list: %q", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ListMode_UpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		singleRelease("v1.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		nil,
		noopClear,
		false,
		false,
		true, // list
		"",
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "up to date") {
		t.Errorf("expected 'up to date' message in list mode, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ReleaseTag_SyncsToSpecificVersion(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "targeted content")()

	mock := &testutil.MockRunner{}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
		{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
		{TagName: "v1.1.0", Name: "Older", Body: "Older notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.1.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		multiReleases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false,    // list
		"v1.5.0", // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the specified version was used.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.5.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v1.5.0", string(data))
	}

	output := buf.String()
	if !strings.Contains(output, "Middle notes") {
		t.Errorf("expected release notes for v1.5.0 in output, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ReleaseTag_NotFound(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		multiReleases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		nil,
		noopClear,
		false,
		false,
		false,    // list
		"vX.Y.Z", // non-existent tag
		nil,
	)

	if err == nil {
		t.Fatal("expected error for non-existent release tag")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "vX.Y.Z") {
		t.Errorf("expected tag in error message, got: %v", err)
	}

	// Verify no sync was performed.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION should be unchanged: %q", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_WithRecipes_DeploysRecipes(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer recipeTarballSetup(t, "updated", nil, []string{"dev-session.yaml", "review.yaml"})()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify recipes were deployed.
	data, err := os.ReadFile(filepath.Join(repo.Root, ".goose", "recipes", "dev-session.yaml"))
	if err != nil {
		t.Fatalf("recipe file missing: %v", err)
	}
	if string(data) != "recipe: dev-session.yaml" {
		t.Errorf("recipe content mismatch: %s", data)
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, ".goose", "recipes", "review.yaml"))
	if err != nil {
		t.Fatalf("review recipe file missing: %v", err)
	}
	if string(data) != "recipe: review.yaml" {
		t.Errorf("review recipe content mismatch: %s", data)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_WithoutRecipes_SucceedsGracefully(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .goose/recipes/ was NOT created (template has no recipes).
	if _, err := os.Stat(filepath.Join(repo.Root, ".goose", "recipes")); err == nil {
		t.Error(".goose/recipes/ should not exist when template has no recipes")
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_WithRecipes_DeclineRestoresRecipes(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer recipeTarballSetup(t, "updated", nil, []string{"new.yaml"})()

	// Create existing recipes in the repo.
	recipesDir := filepath.Join(repo.Root, ".goose", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "original.yaml"), []byte("original-recipe"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	// Decline permanently — confirm always returns false.
	// To avoid infinite loop, use a counter to accept on second call.
	confirmCalls := 0
	confirmFn := func(_ string) (bool, error) {
		confirmCalls++
		return confirmCalls >= 2, nil
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		confirmFn,
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sync completed (accepted on second call), so recipes should be from template.
	data, err := os.ReadFile(filepath.Join(repo.Root, ".goose", "recipes", "new.yaml"))
	if err != nil {
		t.Fatalf("new recipe file missing: %v", err)
	}
	if string(data) != "recipe: new.yaml" {
		t.Errorf("recipe content mismatch: %s", data)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_CommitMessage_IncludesRecipes(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer recipeTarballSetup(t, "updated", nil, []string{"dev.yaml"})()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// The output should contain the updated commit message with "recipes".
	if !strings.Contains(output, "recipes") {
		t.Errorf("expected 'recipes' in output, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ReleaseTag_SkipsPicker(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "targeted content")()

	mock := &testutil.MockRunner{}

	selectCalled := false
	fakeSelect := func(releases []Release) (Release, error) {
		selectCalled = true
		return releases[0], nil
	}

	multiReleases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
		{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		multiReleases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		fakeSelect,
		noopClear,
		false,
		false,
		false,    // list
		"v1.5.0", // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if selectCalled {
		t.Error("select function should NOT be called when --release is specified")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_EmptyTarballURL_FailsBeforeModification(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	// Create a release with empty TarballURL.
	noTarballRelease := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Bad release", Body: "Notes", TarballURL: ""},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		noTarballRelease,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err == nil {
		t.Fatal("expected error for empty tarball URL")
	}
	if !strings.Contains(err.Error(), "no tarball URL") {
		t.Errorf("error should mention missing tarball URL: %v", err)
	}

	// Verify no files were modified — TEMPLATE_VERSION should still be v1.0.0.
	data, readErr := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION should be unchanged: %q", string(data))
	}

	// Verify base/AGENTS.md is unchanged.
	data, readErr = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "# AGENTS.md\n" {
		t.Errorf("base/AGENTS.md should be unchanged: %q", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_TarballFetchFailure_RestoresBackup(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	// Set up a failing fetch function.
	orig := fetchTarballFn
	fetchTarballFn = func(_, _ string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("network timeout: connection refused")
	}
	defer func() { fetchTarballFn = orig }()

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		singleRelease("v2.0.0"),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("error should contain fetch error detail: %v", err)
	}

	// Verify base/ was restored from backup.
	data, readErr := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if readErr != nil {
		t.Fatalf("base/AGENTS.md should exist after restore: %v", readErr)
	}
	if string(data) != "# AGENTS.md\n" {
		t.Errorf("base/AGENTS.md should be restored to original: %q", string(data))
	}

	// Verify TEMPLATE_VERSION is unchanged.
	data, readErr = os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION should be unchanged: %q", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_WritesPostSyncMD(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer defaultTarballSetup(t, "updated content")()

	mock := &testutil.MockRunner{}

	releaseBody := "## What's Changed\n\n- Updated template\n- Fixed sync"
	releases := fakeReleases([]Release{
		{TagName: "v2.0.0", Name: "Latest", Body: releaseBody, TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
	}, nil)

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		tarballRunner(mock),
		releases,
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		nil, // selectVersion
		noopClear,
		false,
		false,
		false, // list
		"",    // releaseTag
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify POST_SYNC.md was written with the release body.
	data, readErr := os.ReadFile(filepath.Join(repo.Root, "POST_SYNC.md"))
	if readErr != nil {
		t.Fatalf("POST_SYNC.md should exist: %v", readErr)
	}
	if string(data) != releaseBody {
		t.Errorf("POST_SYNC.md content = %q, want %q", string(data), releaseBody)
	}

	mock.AssertExpectations(t)
}
