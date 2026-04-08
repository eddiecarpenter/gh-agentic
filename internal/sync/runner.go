package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// SpinnerFunc renders a single step with a spinner while fn runs, then prints
// ✔ or ✖ based on the result. Injected so tests can substitute a plain
// text renderer without requiring a real TTY.
type SpinnerFunc func(w io.Writer, label string, fn func() error) error

// ConfirmFunc presents a yes/no prompt and returns the user's choice.
// Injected so tests can simulate user input without a real TTY.
type ConfirmFunc func(prompt string) (bool, error)

// SelectFunc presents a version picker to the user when multiple releases are
// available. Returns the selected release. Injected so tests can substitute a
// fake without requiring a real TTY.
type SelectFunc func(releases []Release) (Release, error)

// ClearFunc clears the terminal screen. Injected so tests can substitute a
// no-op without requiring a real TTY.
type ClearFunc func(w io.Writer)

// DefaultSpinner is the production SpinnerFunc. Prints "⠸ label..." then
// "✔ label" or "✖ label: error".
func DefaultSpinner(w io.Writer, label string, fn func() error) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("⠸ "+label+"..."))
	if err := fn(); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError(label+": "+err.Error()))
		return err
	}
	fmt.Fprintln(w, "  "+ui.RenderOK(label))
	return nil
}

// DefaultConfirm is the production ConfirmFunc. Uses huh to present a
// yes/no confirmation to the user.
func DefaultConfirm(prompt string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(prompt).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// DefaultClear is the production ClearFunc. Delegates to ui.ClearScreen
// to clear the terminal and reset the cursor position.
func DefaultClear(w io.Writer) {
	ui.ClearScreen(w)
}

// DefaultSelect is the production SelectFunc. Uses huh.Select to present an
// interactive version picker. Each option shows the tag and release name;
// the description shows the release body (notes).
func DefaultSelect(releases []Release) (Release, error) {
	opts := make([]huh.Option[int], len(releases))
	for i, r := range releases {
		label := r.TagName
		if r.Name != "" {
			label += " — " + r.Name
		}
		opts[i] = huh.NewOption(label, i)
	}

	var selected int
	sel := huh.NewSelect[int]().
		Title("Select a version to sync to").
		Options(opts...).
		Value(&selected)

	form := huh.NewForm(huh.NewGroup(sel))
	if err := form.Run(); err != nil {
		return Release{}, err
	}

	return releases[selected], nil
}

// RunSync orchestrates the full sync flow: read config → fetch releases →
// display release notes → confirm → clone → copy → stage (and optionally commit) or restore.
//
// When commit is false (the default), changes are staged but not committed,
// leaving the human to review, commit, and raise a PR. When commit is true,
// the changes are committed automatically with the standard message.
//
// All dependencies are injectable for testing.
func RunSync(
	w io.Writer,
	repoRoot string,
	run bootstrap.RunCommandFunc,
	fetchReleases FetchReleasesFunc,
	spinner SpinnerFunc,
	confirm ConfirmFunc,
	selectVersion SelectFunc,
	clearScreen ClearFunc,
	force bool,
	commit bool,
	list bool,
	releaseTag string,
	detectOwnerType bootstrap.DetectOwnerTypeFunc,
) error {
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SectionHeading.Render("  Sync — update base/ from upstream template"))
	fmt.Fprintln(w)

	// Step 1: Read config files.
	var cfg SyncConfig
	cfg.RepoRoot = repoRoot

	var err error
	cfg.TemplateRepo, err = ReadTemplateSource(repoRoot)
	if err != nil {
		return err
	}
	cfg.CurrentVersion, err = ReadTemplateVersion(repoRoot)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "  "+ui.RenderOK("Template: "+cfg.TemplateRepo+" (current: "+cfg.CurrentVersion+")"))

	// Step 2: Fetch all releases and filter to those newer than current version.
	var available []Release
	if err := spinner(w, "Checking for updates", func() error {
		all, fetchErr := fetchReleases(cfg.TemplateRepo)
		if fetchErr != nil {
			return fetchErr
		}
		available = FilterReleasesSince(all, cfg.CurrentVersion)
		return nil
	}); err != nil {
		return err
	}

	// --list mode: display available releases and exit.
	if list {
		if len(available) == 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  "+ui.RenderOK("Already up to date ("+cfg.CurrentVersion+")"))
			return nil
		}
		fmt.Fprintln(w)
		DisplayReleaseList(w, available)
		return nil
	}

	// --release mode: find specific tag and use it.
	if releaseTag != "" {
		all, fetchErr := fetchReleases(cfg.TemplateRepo)
		if fetchErr != nil {
			return fetchErr
		}
		// Use all releases (not just filtered) so we can find any tag.
		found, ok := FindReleaseByTag(all, releaseTag)
		if !ok {
			return fmt.Errorf("release tag %s not found in %s", releaseTag, cfg.TemplateRepo)
		}
		// Override available with just this release.
		available = []Release{found}
	}

	// Step 3: Check if up to date — bypass when base/ is missing or --force is set.
	baseDir := filepath.Join(repoRoot, "base")
	_, statErr := os.Stat(baseDir)
	baseMissing := os.IsNotExist(statErr)
	if len(available) == 0 && !baseMissing && !force {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("Already up to date ("+cfg.CurrentVersion+")"))
		return nil
	}

	// Determine the target version.
	var targetRelease Release
	switch {
	case baseMissing:
		fmt.Fprintln(w, "  "+ui.RenderWarning("base/ is missing — restoring from "+cfg.CurrentVersion))
		// When base is missing, target the latest available or current version.
		if len(available) > 0 {
			targetRelease = available[0]
		} else {
			targetRelease = Release{TagName: cfg.CurrentVersion}
		}
	case force && len(available) == 0:
		fmt.Fprintln(w, "  "+ui.RenderWarning("--force set — re-syncing from "+cfg.CurrentVersion))
		targetRelease = Release{TagName: cfg.CurrentVersion}
	default:
		// Use the newest available release as the default target.
		targetRelease = available[0]
	}

	cfg.LatestVersion = targetRelease.TagName

	if len(available) == 1 {
		// Single version available — display release notes directly, skip picker.
		fmt.Fprintln(w, "  "+ui.RenderOK(fmt.Sprintf("Update available: %s → %s", cfg.CurrentVersion, targetRelease.TagName)))
		fmt.Fprintln(w)
		DisplayReleaseNotes(w, targetRelease)
	} else if len(available) > 1 {
		// Multiple versions available — show picker.
		fmt.Fprintln(w, "  "+ui.RenderOK(fmt.Sprintf("%d releases available since %s", len(available), cfg.CurrentVersion)))
		fmt.Fprintln(w)

		if selectVersion != nil {
			selected, selectErr := selectVersion(available)
			if selectErr != nil {
				return fmt.Errorf("version selection: %w", selectErr)
			}
			targetRelease = selected
			cfg.LatestVersion = targetRelease.TagName
		}

		DisplayReleaseNotes(w, targetRelease)
	} else {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Update available: "+cfg.CurrentVersion+" → "+cfg.LatestVersion))
	}

	fmt.Fprintln(w)

	// Step 4: Confirm with user before any destructive work.
	confirmed, err := confirm(fmt.Sprintf("Install %s?", cfg.LatestVersion))
	if err != nil {
		return fmt.Errorf("confirmation prompt: %w", err)
	}

	if !confirmed {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.Muted.Render("Sync cancelled — no changes made"))
		return nil
	}

	// Step 5: Clone template to temp dir.
	tmpDir, err := os.MkdirTemp("", "agentic-sync-clone-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer func() { _ = CleanupTemp(tmpDir) }()

	if err := spinner(w, "Cloning template", func() error {
		return CloneTemplate(cfg.TemplateRepo, tmpDir, run)
	}); err != nil {
		return err
	}

	// Step 6: Backup existing base/.
	var backupDir string
	if err := spinner(w, "Backing up base/", func() error {
		var backupErr error
		backupDir, backupErr = BackupBase(repoRoot)
		return backupErr
	}); err != nil {
		return err
	}
	defer func() { _ = CleanupTemp(backupDir) }()

	// Step 7: Copy base/ from template.
	if err := spinner(w, "Copying base/", func() error {
		return CopyBase(tmpDir, repoRoot)
	}); err != nil {
		// On copy error, try to restore.
		_ = RestoreBase(repoRoot, backupDir)
		return err
	}

	// Step 7b: Resolve owner type to determine workflow excludes.
	var workflowExcludes []string
	if detectOwnerType != nil {
		owner := extractOwnerFromAgentsLocal(repoRoot)
		if owner != "" {
			ownerType, detectErr := detectOwnerType(owner)
			if detectErr == nil && ownerType == bootstrap.OwnerTypeUser {
				workflowExcludes = []string{"sync-status-to-label.yml"}
			}
			// If detection fails, default to deploying all workflows (safe fallback).
		}
	}

	// Step 7c: Deploy workflows from template.
	if err := spinner(w, "Deploying workflows", func() error {
		return DeployWorkflows(tmpDir, repoRoot, workflowExcludes)
	}); err != nil {
		_ = RestoreBase(repoRoot, backupDir)
		return err
	}

	// Step 8: Update version and stage (optionally commit).
	if err := spinner(w, "Updating TEMPLATE_VERSION", func() error {
		return UpdateVersion(repoRoot, cfg.LatestVersion)
	}); err != nil {
		return err
	}

	commitMsg := fmt.Sprintf("chore: sync base/ and workflows from %s %s", cfg.TemplateRepo, cfg.LatestVersion)

	if commit {
		if err := spinner(w, "Committing changes", func() error {
			return CommitSync(repoRoot, cfg.TemplateRepo, cfg.LatestVersion, run)
		}); err != nil {
			return err
		}

		// Print success with commit.
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("Sync committed — "+commitMsg))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Remember to push and raise a PR for review"))
	} else {
		if err := spinner(w, "Staging changes", func() error {
			return StageSync(repoRoot, run)
		}); err != nil {
			return err
		}

		// Print success with stage-only.
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("Sync applied — changes staged"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Review: git diff --cached"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Then:   git commit -m \""+commitMsg+"\""))
		fmt.Fprintln(w, "  "+ui.Muted.Render("        git push origin <branch> && gh pr create"))
	}

	return nil
}
