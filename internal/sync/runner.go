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

// RunSync orchestrates the full sync flow: read config → check version →
// clone → copy → show diff → confirm → stage (and optionally commit) or restore.
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
	fetchRelease FetchReleaseFunc,
	spinner SpinnerFunc,
	confirm ConfirmFunc,
	force bool,
	commit bool,
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

	// Step 2: Fetch latest release.
	if err := spinner(w, "Checking latest release", func() error {
		tag, fetchErr := FetchLatestRelease(cfg.TemplateRepo, fetchRelease)
		if fetchErr != nil {
			return fetchErr
		}
		cfg.LatestVersion = tag
		return nil
	}); err != nil {
		return err
	}

	// Step 3: Check if up to date — bypass when base/ is missing or --force is set.
	baseDir := filepath.Join(repoRoot, "base")
	_, statErr := os.Stat(baseDir)
	baseMissing := os.IsNotExist(statErr)
	if IsUpToDate(cfg.CurrentVersion, cfg.LatestVersion) && !baseMissing && !force {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("Already up to date ("+cfg.CurrentVersion+")"))
		return nil
	}
	switch {
	case baseMissing:
		fmt.Fprintln(w, "  "+ui.RenderWarning("base/ is missing — restoring from "+cfg.CurrentVersion))
	case force:
		fmt.Fprintln(w, "  "+ui.RenderWarning("--force set — re-syncing from "+cfg.CurrentVersion))
	}

	fmt.Fprintln(w, "  "+ui.RenderWarning("Update available: "+cfg.CurrentVersion+" → "+cfg.LatestVersion))
	fmt.Fprintln(w)

	// Step 4: Clone template to temp dir.
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

	// Step 5: Backup existing base/.
	var backupDir string
	if err := spinner(w, "Backing up base/", func() error {
		var backupErr error
		backupDir, backupErr = BackupBase(repoRoot)
		return backupErr
	}); err != nil {
		return err
	}
	defer func() { _ = CleanupTemp(backupDir) }()

	// Step 6: Copy base/ from template.
	if err := spinner(w, "Copying base/", func() error {
		return CopyBase(tmpDir, repoRoot)
	}); err != nil {
		// On copy error, try to restore.
		_ = RestoreBase(repoRoot, backupDir)
		return err
	}

	// Step 6b: Resolve owner type to determine workflow excludes.
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

	// Step 6c: Deploy workflows from template.
	if err := spinner(w, "Deploying workflows", func() error {
		return DeployWorkflows(tmpDir, repoRoot, workflowExcludes)
	}); err != nil {
		_ = RestoreBase(repoRoot, backupDir)
		return err
	}

	// Step 7: Show diff.
	fmt.Fprintln(w)
	diff, err := ShowDiff(repoRoot, run)
	if err != nil {
		_ = RestoreBase(repoRoot, backupDir)
		return err
	}

	if diff != "" {
		fmt.Fprintln(w, ui.Muted.Render("  ─── Changes ───"))
		fmt.Fprintln(w, diff)
		fmt.Fprintln(w, ui.Muted.Render("  ─── End ───"))
	} else {
		fmt.Fprintln(w, "  "+ui.Muted.Render("No visible changes in base/"))
	}

	fmt.Fprintln(w)

	// Step 8: Confirm with user.
	confirmed, err := confirm("Apply these changes and update TEMPLATE_VERSION to " + cfg.LatestVersion + "?")
	if err != nil {
		_ = RestoreBase(repoRoot, backupDir)
		return fmt.Errorf("confirmation prompt: %w", err)
	}

	if !confirmed {
		// Step 10: Decline — restore.
		if err := spinner(w, "Restoring base/", func() error {
			return RestoreBase(repoRoot, backupDir)
		}); err != nil {
			return err
		}

		// Also reset any git changes to base/ and .github/workflows/.
		_, _ = runInDir(run, repoRoot, "git", "checkout", "--", "base/")
		_, _ = runInDir(run, repoRoot, "git", "checkout", "--", ".github/workflows/")

		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.Muted.Render("Sync cancelled — no changes committed"))
		return nil
	}

	// Step 9: Confirmed — update version and stage (optionally commit).
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
